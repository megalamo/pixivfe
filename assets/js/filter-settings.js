(() => {
  const MODE_RANK = { show: 0, censor: 1, hide: 2 };
  const rankOf = (mode) => MODE_RANK[mode] ?? -1;
  const modeFromRank = (rank) => {
    if (rank >= MODE_RANK.hide) return "hide";
    if (rank >= MODE_RANK.censor) return "censor";
    return "show";
  };

  const FILTER_SETTINGS = {
    // Order is from least to most restrictive
    hierarchy: [
      { name: "mode_r15" },
      { name: "mode_r18" },
      { name: "mode_r18g" },
    ],

    // Track which forms have had listeners attached so init is idempotent
    _boundForms: new WeakSet(),

    /**
     * Get the currently selected mode for the given radio group name,
     * scoping the lookup to root (defaults to document).
     * @param {string} name
     * @param {ParentNode} root
     * @returns {"show"|"censor"|"hide"|null}
     */
    getSelectedMode(name, root = document) {
      const checked = root.querySelector(
        `input[type="radio"][name="${name}"]:checked`
      );
      return checked ? checked.value : null;
    },

    /**
     * Set a radio group to a specific mode, scoped to root.
     * @param {string} name
     * @param {"show"|"censor"|"hide"} mode
     * @param {ParentNode} root
     */
    setMode(name, mode, root = document) {
      const input = root.querySelector(
        `input[type="radio"][name="${name}"][value="${mode}"]`
      );
      if (input && !input.checked) {
        input.checked = true; // Programmatic changes do not fire 'change'
      }
    },

    /**
     * Enforce invariants on user change without overriding his selection:
     * - Never change the radio the user just set.
     * - Enforce non‑decreasing restrictiveness left‑to‑right:
     *   left side, cap each earlier level to be no stricter than the next one.
     *   right side, raise each later level to be at least as strict as the previous one.
     */
    handleChange(event) {
      const target = event.target;
      if (!target || target.type !== "radio") return;

      const levelName = target.name;
      const currentLevelIndex = FILTER_SETTINGS.hierarchy.findIndex(
        (level) => level.name === levelName
      );
      if (currentLevelIndex === -1) return;

      const root = event.currentTarget || target.form || document;
      const userMode = target.value;

      // Defensive guard for unexpected values
      if (!(userMode in MODE_RANK)) return;

      const userRank = rankOf(userMode);

      // Walk leftwards, ensuring each earlier level is no stricter than the next one.
      // This may demote earlier levels but never touches the clicked one.
      let nextRank = userRank;
      for (let i = currentLevelIndex - 1; i >= 0; i--) {
        const name = FILTER_SETTINGS.hierarchy[i].name;
        const currentRank = rankOf(FILTER_SETTINGS.getSelectedMode(name, root));
        const adjustedRank = Math.min(currentRank, nextRank);
        if (adjustedRank !== currentRank) {
          FILTER_SETTINGS.setMode(name, modeFromRank(adjustedRank), root);
        }
        // Maintain monotonicity across the left segment
        nextRank = adjustedRank;
      }

      // Walk rightwards, ensuring each later level is at least as strict as the previous one.
      // This may promote later levels but never touches the clicked one.
      let prevRank = userRank;
      for (
        let i = currentLevelIndex + 1;
        i < FILTER_SETTINGS.hierarchy.length;
        i++
      ) {
        const name = FILTER_SETTINGS.hierarchy[i].name;
        const currentRank = rankOf(FILTER_SETTINGS.getSelectedMode(name, root));
        const adjustedRank = Math.max(currentRank, prevRank);
        if (adjustedRank !== currentRank) {
          FILTER_SETTINGS.setMode(name, modeFromRank(adjustedRank), root);
        }
        prevRank = adjustedRank;
      }
    },

    /**
     * Normalise the current state so the UI starts consistent with the rules.
     * 1) If any level is 'show', all less restrictive levels become 'show'.
     * 2) Enforce non‑decreasing restrictiveness from least to most restrictive.
     */
    normalise(root) {
      if (!root) return;

      // Step 1, forward 'show' cascade from the highest 'show' found
      let maxShowIndex = -1;
      for (let i = 0; i < this.hierarchy.length; i++) {
        const name = this.hierarchy[i].name;
        if (this.getSelectedMode(name, root) === "show") maxShowIndex = i;
      }
      if (maxShowIndex >= 0) {
        for (let j = 0; j < maxShowIndex; j++) {
          this.setMode(this.hierarchy[j].name, "show", root);
        }
      }

      // Step 2, backward restrictiveness cascade
      let maxRank = -1;
      for (let i = 0; i < this.hierarchy.length; i++) {
        const name = this.hierarchy[i].name;
        const currentRank = rankOf(this.getSelectedMode(name, root));
        if (currentRank < maxRank) {
          this.setMode(name, modeFromRank(maxRank), root);
        } else {
          maxRank = currentRank;
        }
      }
    },

    /**
     * Find the filter form(s) and attach the change handler, once per form.
     * Also normalise the initial state.
     */
    init() {
      const forms = document.querySelectorAll("#content_filters_form");
      forms.forEach((form) => {
        if (this._boundForms.has(form)) return; // idempotent
        form.addEventListener("change", this.handleChange);
        this._boundForms.add(form);
        this.normalise(form);
      });
    },
  };

  // Initialise on initial load and after htmx settles new content.
  addEventListener("DOMContentLoaded", () => {
    FILTER_SETTINGS.init();
  });

  addEventListener("htmx:afterSettle", () => {
    FILTER_SETTINGS.init(); // Rebind to swapped nodes, no duplicate listeners
  });
})();
