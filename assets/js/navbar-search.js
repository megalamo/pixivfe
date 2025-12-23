/** navbar-search.js
 * @fileoverview
 * Adds smart tag-suggestion acceptance to the navbar search with:
 * - IME-like backspace-undo for the most recent accepted suggestion.
 * - Dedicated undo/redo (Ctrl/Cmd+Z and Ctrl/Cmd+Y or Shift+Ctrl/Cmd+Z) for suggestion acceptances.
 * - Shortcut Alt+Shift+1..9 to accept the Nth suggestion from the current list, without consuming bare number keys.
 * - "Smart literal" behaviour so the next space inserts a literal character instead of re-accepting.
 * - Visibility of the suggestions popover is controlled via a `data-visible` attribute set by JS, allowing presentation to be driven by CSS.
 *
 * WAI-ARIA combobox alignment:
 * - The search input has role="combobox", aria-autocomplete="list", and aria-controls points to a popup with role="listbox".
 * - DOM focus remains on the input; assistive technology focus moves in the popup via aria-activedescendant on the input.
 * - ArrowDown and ArrowUp move selection among [role="option"] items in the listbox. Home and End jump to first or last option.
 * - Enter accepts the highlighted option and closes the popup for keyboard interaction, as per the pattern. The popup may reopen immediately when new suggestions are fetched while focus remains, which is intentional to support rapid chaining with the keyboard.
 * - Escape dismisses the popup. If the popup is already closed, it clears the input.
 * - Printable input cancels the current selection and continues editing.
 *
 * Product decision, documented here because it deviates from the ARIA listbox "close on accept" convention:
 * - Pointer click acceptance keeps the popup open to support rapid chaining. Keyboard Enter acceptance closes the popup, but we intentionally allow it to reopen right away if new suggestions arrive while the input remains focused, enabling chaining via keyboard as well. The same applies to Alt+Shift+1..9 numeric acceptance.
 *
 * Notes:
 * - The system allows accepting multiple suggestions in sequence, e.g., for different keywords.
 * - The backspace-to-undo feature only reverts the most recent suggestion acceptance.
 * - Pointer hover sets the active option, ensuring only one appears active at a time and that arrow keys start from the hovered option.
 *
 * Pointerover and pointermove, and why we use both:
 * - We use both `pointerover` and `pointermove` to drive `aria-activedescendant` and `aria-selected`.
 *   - `pointerover` is essential because it fires when the pointer enters an option and also when the list scrolls under a stationary pointer.
 *   - `pointermove` is also needed to ensure the active item updates correctly if the user first uses the keyboard to change the selection, and then moves the mouse over a different item without leaving and re-entering it. `pointerover` alone would not fire in this scenario.
 * This combination ensures the "hovered" item stays in sync with visuals and hit-testing across all interaction patterns.
 */

/**
 * A single suggestion-acceptance change in the input.
 * Used for IME-like backspace undo and the explicit undo/redo stacks.
 * @typedef {Object} SuggestionChange
 * @property {string} before - The input value prior to accepting a suggestion.
 * @property {string} after - The input value after accepting a suggestion.
 */

const MAX_HISTORY = 100; // modest cap to keep memory bounded in long sessions

/**
 * Computes a replacement value that substitutes only the token around the caret.
 * Tokens are delimited by whitespace or string boundaries. If the token around the
 * caret does not match oldKeywordText exactly, returns null so the caller can decide
 * whether to fall back to a broader replacement.
 *
 * This keeps acceptance precise and avoids replacing every occurrence in the input.
 *
 * Additionally returns the ideal caret position so the caret remains just after the
 * inserted tag text, which feels natural when continuing to type.
 *
 * @param {HTMLInputElement} inputEl
 * @param {string} oldKeywordText
 * @param {string} newTagText
 * @returns {{ value: string, caretAfter: number } | null} The new value and caret position if a token was replaced; otherwise null.
 */
function replaceTokenAtCaret(inputEl, oldKeywordText, newTagText) {
  const value = inputEl.value;
  // Use selectionStart as caret position. If unknown, treat as end-of-string.
  const pos =
    typeof inputEl.selectionStart === "number"
      ? inputEl.selectionStart
      : value.length;

  // Identify token bounds by scanning left and right for whitespace.
  let start = pos;
  let end = pos;
  const isSpace = (ch) => /\s/.test(ch);

  while (start > 0 && !isSpace(value[start - 1])) start--;
  while (end < value.length && !isSpace(value[end])) end++;

  const token = value.slice(start, end);
  if (token !== oldKeywordText) {
    // Not the expected token, signal caller to consider fallback.
    return null;
  }

  // Replace the token, preserving the rest of the input.
  const nextValue = value.slice(0, start) + newTagText + value.slice(end);
  const caretAfter = start + newTagText.length;
  return { value: nextValue, caretAfter };
}

/**
 * NavbarSearchController
 *
 * Encapsulates:
 * - Input and suggestions container discovery.
 * - Visibility control and pointer-guarded hiding.
 * - ARIA combobox semantics, including listbox navigation using aria-activedescendant and aria-selected.
 * - Smart acceptance, IME-like undo, and explicit undo/redo stacks.
 * - Alt+Shift+number shortcuts for accepting suggestions.
 * - Caret placement on focus and defocus rules on navigation.
 *
 * Behavioural equivalence and ARIA conformance:
 * - IME-like backspace undo triggers only when the input still matches the last acceptance's "after" string.
 * - Explicit undo and redo trigger only when the input matches the expected "after" or "before" respectively, otherwise the browser's native undo/redo takes over.
 * - Numeric acceptance uses Alt+Shift+1..9 for the Nth suggestion, and bare number keys are never consumed by this feature.
 * - Space immediately after an acceptance or explicit undo inserts a literal space instead of triggering acceptance.
 * - Suggestions are shown only when the input is focused and suggestion data has been fetched.
 * - ArrowDown/ArrowUp navigate options, Home/End jump to first/last option, Enter accepts and closes the popup for keyboard users, Escape closes. After keyboard acceptance, the popup may reopen as soon as new suggestions arrive while focus remains, to support chaining.
 * - Hovering a suggestion makes it the active option and updates aria-activedescendant, so only one option appears active and keyboard navigation starts from the hovered option.
 */
class NavbarSearchController {
  constructor() {
    /** @type {AbortController | null} */
    this.ac = null;

    /**
     * Internal state grouped for clarity.
     * - history keeps suggestion acceptance history, and the lastCompositionIndex for IME-like backspace.
     * - smartLiteral arms literal insertion for space.
     * - pointer tracks whether the pointer is down inside the suggestions and manages delayed hide on blur.
     * - activeIndex tracks the highlighted option in the listbox, used with aria-activedescendant.
     */
    this.state = {
      history: {
        /** @type {SuggestionChange[]} */
        undo: [],
        /** @type {SuggestionChange[]} */
        redo: [],
        /**
         * Index of the last acceptance in the undo stack that is eligible for IME-like backspace undo.
         * -1 means no IME-like undo is currently armed.
         * Tracking the index, not a copy, ensures we always refer to the top-of-undo acceptance.
         * @type {number}
         */
        lastCompositionIndex: -1,
      },
      smartLiteral: {
        /** @type {boolean} */
        spaceArmed: false,
      },
      pointer: {
        /** @type {boolean} */
        downInSuggestions: false,
        /** @type {number | null} */
        hideTimeoutId: null,
      },
      /** @type {number} */
      activeIndex: -1,
      /**
       * Tracks whether the pointer or keyboard last set the active option.
       * Used to clear selection on pointer leave only if set by pointer.
       * @type {'pointer' | 'keyboard' | null}
       */
      activeSetBy: null,
    };
  }

  /** @returns {HTMLInputElement | null} */
  get input() {
    return /** @type {HTMLInputElement | null} */ (
      document.getElementById("navbar_search_input")
    );
  }

  /** @returns {HTMLElement | null} */
  get container() {
    return /** @type {HTMLElement | null} */ (
      document.getElementById("navbar_search_suggestions")
    );
  }

  /** @returns {HTMLElement | null} */
  get listbox() {
    return /** @type {HTMLElement | null} */ (
      document.getElementById("navbar_search_listbox")
    );
  }

  /** @returns {HTMLElement[]} */
  get options() {
    const lb = this.listbox;
    return lb
      ? /** @type {HTMLElement[]} */ (
          Array.from(lb.querySelectorAll('[role="option"]'))
        )
      : [];
  }

  /**
   * Checks presence via a server-provided flag to avoid DOM queries.
   * Server sets data-has-items="true" if there is suggestion data for the current state,
   * "false" otherwise. Note this indicates "data present", not necessarily "non-empty options".
   */
  suggestionsExist() {
    const c = this.container;
    return !!c && c.dataset.hasItems === "true";
  }

  /**
   * True if the suggestions popover is currently shown to the user.
   * Constrains numeric acceptance to visible state, which matches user expectation.
   */
  isShown() {
    const c = this.container;
    return !!c && c.dataset.visible === "true" && this.suggestionsExist();
  }

  /** Shows the suggestions container and updates ARIA state. */
  show() {
    const c = this.container;
    if (c) {
      c.dataset.visible = "true";
      c.setAttribute("aria-hidden", "false");
    }
    const i = this.input;
    if (i) i.setAttribute("aria-expanded", "true");
  }

  /** Hides the suggestions container and updates ARIA state. */
  hide() {
    const c = this.container;
    if (c) {
      c.dataset.visible = "false";
      c.setAttribute("aria-hidden", "true");
    }
    const i = this.input;
    if (i) i.setAttribute("aria-expanded", "false");
    this.clearActive();
    this.announceCount(); // clear any status text
  }

  /**
   * Updates visibility based on focus and whether suggestion data exists.
   * Only shows when the search input is focused and there is suggestion data.
   * Also updates the live region with a count, if any.
   */
  updateVisibility() {
    const input = this.input;
    const shouldShow =
      !!input && document.activeElement === input && this.suggestionsExist();

    if (shouldShow) this.show();
    else this.hide();

    this.announceCount();
  }

  /** Clears the active option selection and related ARIA attributes. */
  clearActive() {
    const input = this.input;
    if (input) input.removeAttribute("aria-activedescendant");
    const opts = this.options;
    for (const o of opts) o.setAttribute("aria-selected", "false");
    this.state.activeIndex = -1;
    this.state.activeSetBy = null;
  }

  /**
   * Sets the active option by index, updating aria-activedescendant and aria-selected.
   * Clamps the index to available options. Does nothing if there are no options.
   * Only toggles the previously active and the newly active option for simplicity and efficiency.
   * @param {number} index
   * @param {'pointer' | 'keyboard'} [source='keyboard'] - The input method that triggered the change.
   */
  setActive(index, source = "keyboard") {
    const input = this.input;
    const opts = this.options;
    if (!input || opts.length === 0) return;

    const clamped = Math.max(0, Math.min(index, opts.length - 1));
    if (clamped === this.state.activeIndex) return;

    const prevIdx = this.state.activeIndex;
    const nextEl = opts[clamped];

    input.setAttribute("aria-activedescendant", nextEl.id);

    if (prevIdx >= 0 && prevIdx < opts.length) {
      const prevEl = opts[prevIdx];
      prevEl.setAttribute("aria-selected", "false");
    }
    nextEl.setAttribute("aria-selected", "true");

    this.state.activeIndex = clamped;
    this.state.activeSetBy = source;

    // Keep the active option visible inside the scroll container.
    try {
      nextEl.scrollIntoView({ block: "nearest" });
    } catch {
      // No-op if not scrollable.
    }
  }

  /** Announces the current suggestions count in a polite live region for screen reader users. */
  announceCount() {
    const status = /** @type {HTMLElement | null} */ (
      document.getElementById("navbar_search_status")
    );
    if (!status) return;

    if (!this.isShown()) {
      status.textContent = "";
      return;
    }
    const count = this.options.length;
    // Announce the count and a short hint. Empty count announcement is optional; keep it short.
    if (count > 0) {
      status.textContent = `${count} suggestion${count === 1 ? "" : "s"} available. Use Down Arrow to navigate.`;
    } else {
      status.textContent = `No suggestions.`;
    }
  }

  /**
   * Initialise, re-entrant and safe to call after htmx settles or on DOM ready.
   * Cleans up any previous listeners via AbortController, re-queries the current nodes,
   * then wires all handlers.
   */
  init() {
    if (this.ac) this.ac.abort();
    this.ac = new AbortController();
    const { signal } = this.ac;

    const input = this.input;
    const container = this.container;
    if (!input || !container) return;

    // Suggestions interactions
    container.addEventListener("click", this.onSuggestionClick, { signal });
    // Prevent blur when clicking on container padding.
    container.addEventListener("mousedown", this.onContainerMouseDown, {
      signal,
    });
    // Use pointer events for better cross-device support.
    container.addEventListener("pointerdown", this.onSuggestionsPointerDown, {
      signal,
    });
    // Use both pointerover and pointermove to drive hover-based activation.
    // - `pointerover` handles entering an option and the list scrolling under a stationary pointer.
    // - `pointermove` ensures the active item updates if keyboard selection changes, then the user
    //   moves the pointer over a different item without leaving and re-entering it.
    container.addEventListener("pointerover", this.onSuggestionPointerHover, {
      signal,
    });
    container.addEventListener("pointermove", this.onSuggestionPointerHover, {
      signal,
    });
    container.addEventListener("pointerleave", this.onSuggestionsPointerLeave, {
      signal,
    });

    // Input interactions
    input.addEventListener("keydown", this.onKeyDown, { signal });
    input.addEventListener("focus", this.onFocus, { signal });
    input.addEventListener("input", this.onInput, { signal });
    input.addEventListener("blur", this.onBlur, { signal });

    // Document interactions
    document.addEventListener("pointerup", this.onDocumentPointerUp, {
      signal,
    });
    document.addEventListener("click", this.onDocumentClick, { signal });

    // htmx lifecycle hooks for ARIA niceties
    container.addEventListener(
      "htmx:beforeRequest",
      () => {
        // Mark the region being updated as busy
        const c = this.container;
        if (c) c.setAttribute("aria-busy", "true");
        const lb = this.listbox;
        if (lb) lb.setAttribute("aria-busy", "true");
      },
      { signal },
    );
    container.addEventListener(
      "htmx:afterOnLoad",
      () => {
        // Clear busy on the updated region
        const c = this.container;
        if (c) c.removeAttribute("aria-busy");
        const lb = this.listbox;
        if (lb) lb.removeAttribute("aria-busy");
        // Clear any stale active selection, then update visibility and announce.
        this.clearActive();
        this.updateVisibility();
      },
      { signal },
    );
    container.addEventListener(
      "htmx:responseError",
      () => {
        // Clear busy even on errors
        const c = this.container;
        if (c) c.removeAttribute("aria-busy");
        const lb = this.listbox;
        if (lb) lb.removeAttribute("aria-busy");
        this.clearActive();
        this.updateVisibility();
      },
      { signal },
    );

    // When the suggestions container is about to be swapped, clear active so aria-activedescendant never points to a removed node.
    document.addEventListener(
      "htmx:beforeSwap",
      (evt) => {
        const target = /** @type {HTMLElement} */ (evt.target);
        if (target && target.id === "navbar_search_suggestions") {
          this.clearActive();
        }
      },
      { signal },
    );

    // Apply initial visibility state.
    this.updateVisibility();
  }

  /** Defocuses the search input, used on load and after page navigations initiated by htmx. */
  defocus() {
    const input = this.input;
    if (input) input.blur();
  }

  // Event handlers

  /**
   * Handles clicks within the suggestions container, accepting the clicked suggestion.
   * Arms the smart-literal-space behaviour to make the next space literal.
   *
   * Product decision: keep the popup open after pointer-based acceptance to allow rapid chaining,
   * which deviates from the "close on accept" convention in ARIA listbox examples.
   */
  onSuggestionClick = (event) => {
    const suggestion = /** @type {HTMLElement} */ (event.target).closest(
      '#navbar_search_suggestions [role="option"]',
    );
    if (!suggestion) return;

    event.preventDefault();
    event.stopPropagation();

    // Make the clicked element active for consistency before accept
    const idx = this.options.indexOf(suggestion);
    if (idx >= 0) this.setActive(idx, "pointer");

    this.acceptByElement(/** @type {HTMLElement} */ (suggestion));

    // Keep suggestions visible post-acceptance for pointer interaction.
    this.updateVisibility();
  };

  /**
   * Prevents the input from blurring when clicking on the non-interactive
   * parts of the suggestions container (e.g., padding, separators).
   * This stops the suggestions from closing unexpectedly.
   */
  onContainerMouseDown = (event) => {
    const suggestion = /** @type {HTMLElement} */ (event.target).closest(
      '#navbar_search_suggestions [role="option"]',
    );

    // If the mousedown is on the container's background but not an actual option,
    // prevent the default action. This stops the search input from losing focus,
    // which in turn prevents the `onBlur` handler from hiding the suggestions.
    if (!suggestion) {
      event.preventDefault();
    }
  };

  /** Track pointer pressed inside the suggestions container to keep it open through click. */
  onSuggestionsPointerDown = () => {
    this.state.pointer.downInSuggestions = true;
  };

  /**
   * On pointer hover (move or over), make the currently hovered option the active one.
   * This ensures the active item stays in sync with the pointer's position.
   * We use both 'pointerover' and 'pointermove':
   * - 'pointerover' covers entering an option and the list scrolling under a stationary pointer.
   * - 'pointermove' covers the case where keyboard selection has moved focus, and the user then moves
   *   the mouse within the bounds of a different option, which needs to reclaim active state.
   */
  onSuggestionPointerHover = (event) => {
    // Only respond to mouse hover events.
    // Don't activate items when a user is scrolling the list on a touch device.
    if (event.pointerType !== "mouse") {
      return;
    }

    if (!this.isShown()) return;

    const toEl = /** @type {HTMLElement} */ (event.target).closest(
      '#navbar_search_suggestions [role="option"]',
    );
    // If not over an option (e.g., in padding), do nothing. `pointerleave` will handle clearing.
    if (!toEl) return;

    const idx = this.options.indexOf(toEl);
    // Only update if the pointer is over a *different* option than the currently active one.
    // This check also prevents redundant calls from pointermove.
    if (idx >= 0 && idx !== this.state.activeIndex) {
      this.setActive(idx, "pointer");
    }
  };

  /**
   * When the pointer leaves the suggestions list, clear the active selection
   * if it was set by the pointer. This preserves keyboard selection.
   */
  onSuggestionsPointerLeave = () => {
    if (this.state.activeSetBy === "pointer") {
      this.clearActive();
    }
  };

  /** Clear pointer state on pointer up anywhere and re-evaluate visibility. */
  onDocumentPointerUp = () => {
    this.state.pointer.downInSuggestions = false;
    // Recompute in case a delayed blur was pending.
    this.updateVisibility();
  };

  /** Close on outside clicks. */
  onDocumentClick = (event) => {
    const target = /** @type {HTMLElement} */ (event.target);
    const clickedInsideInput = !!target.closest("#navbar_search_input");
    const clickedInsideSuggestions = !!target.closest(
      "#navbar_search_suggestions",
    );
    if (!clickedInsideInput && !clickedInsideSuggestions) {
      this.hide();
    }
  };

  /**
   * On focus, fetch suggestions for the current value and place caret at end.
   * The caret placement uses requestAnimationFrame to run after native focus handling and layout.
   */
  onFocus = () => {
    const input = this.input;
    if (!input) return;

    // Trigger an input event so the htmx driver can load suggestions for the current value.
    input.dispatchEvent(new Event("input", { bubbles: true }));

    // Place caret at end on the next frame.
    requestAnimationFrame(() => {
      const len = input.value.length;
      try {
        input.setSelectionRange(len, len);
      } catch {
        // Ignore if the input type does not support selection.
      }
    });

    // Update visibility, will be re-evaluated again after htmx settles.
    this.updateVisibility();
  };

  /** Keep suggestions visible while typing if results exist, and cancel any active selection. */
  onInput = () => {
    this.clearActive();
    this.updateVisibility();
  };

  /**
   * Hide suggestions on blur after a small delay, but recompute instead of blindly hiding.
   * This avoids a race that could hide suggestions after a click accept that refocused the input.
   * If the pointer is currently down inside suggestions, skip changing visibility to allow click selection.
   */
  onBlur = () => {
    const s = this.state.pointer;
    if (s.hideTimeoutId) clearTimeout(s.hideTimeoutId);
    s.hideTimeoutId = window.setTimeout(() => {
      s.hideTimeoutId = null;
      if (s.downInSuggestions) return; // keep state, pointer-up will recompute
      this.updateVisibility();
    }, 100);
  };

  /**
   * Handles keydown events for the search input:
   * - ArrowDown/ArrowUp move selection within the listbox.
   * - Home/End jump to first/last option.
   * - Enter accepts the selected option and closes the popup.
   * - Escape closes the suggestions. If suggestions are already closed, it clears the input.
   * - Left/Right while navigating cancels selection and returns to text editing.
   * - Ctrl/Cmd+Z and Ctrl/Cmd+Y or Shift+Ctrl/Cmd+Z for undo/redo of suggestion acceptances.
   * - Backspace to IME-like undo the most recent suggestion acceptance.
   * - Alt+Shift+1..9 to accept the corresponding suggestion from the current list, only when shown.
   * - Smart-literal behaviour for Space after an acceptance or explicit undo.
   * Resets internal state on any "normal" edit.
   */
  onKeyDown = (event) => {
    // Respect IME composition, do not interfere while composing.
    if (event.isComposing) return;

    const input = this.input;
    if (!input) return;

    const key = event.key;
    const ctrlOrMeta = event.ctrlKey || event.metaKey;
    const isUndo = ctrlOrMeta && key.toLowerCase() === "z" && !event.shiftKey;
    const isRedo =
      (ctrlOrMeta && key.toLowerCase() === "y") ||
      (ctrlOrMeta && key.toLowerCase() === "z" && event.shiftKey);

    // Escape closes the suggestions, or clears the input if already closed.
    if (key === "Escape") {
      if (this.isShown()) {
        event.preventDefault();
        this.hide();
        // Intentionally do not prevent auto reopening. If new suggestions arrive while focus remains,
        // the popup may reopen to support rapid chaining via keyboard.
      } else if (input.value) {
        event.preventDefault();
        this.applyValue("", "deleteContentBackward");
      }
      return;
    }

    // Required combobox keys: ArrowDown / ArrowUp
    if (key === "ArrowDown") {
      if (this.suggestionsExist()) {
        event.preventDefault();
        if (!this.isShown()) this.show();
        if (this.state.activeIndex === -1) this.setActive(0);
        else this.setActive(this.state.activeIndex + 1);
      }
      return;
    }

    if (key === "ArrowUp") {
      if (this.suggestionsExist()) {
        event.preventDefault();
        if (!this.isShown()) this.show();
        if (this.state.activeIndex === -1)
          this.setActive(this.options.length - 1);
        else this.setActive(this.state.activeIndex - 1);
      }
      return;
    }

    // Optional additions: Home and End jump to first/last option when navigating
    if (key === "Home") {
      if (this.isShown() && this.state.activeIndex !== -1) {
        event.preventDefault();
        this.setActive(0);
      }
      return; // otherwise let default text editing occur
    }

    if (key === "End") {
      if (this.isShown() && this.state.activeIndex !== -1) {
        event.preventDefault();
        this.setActive(this.options.length - 1);
      }
      return;
    }

    // Enter accepts the selected option and closes the popup for keyboard interactions.
    if (key === "Enter") {
      const active =
        this.state.activeIndex >= 0
          ? this.options[this.state.activeIndex]
          : null;
      if (this.isShown() && active) {
        event.preventDefault();
        this.acceptByElement(active);
        this.hide(); // close-after-accept for keyboard, per ARIA expectations
        // Intentionally allow immediate reopening when new suggestions arrive while focus remains.
        return;
      }
      // Otherwise, allow normal form submit
    }

    // Optional open and close shortcuts
    if (event.altKey && !event.shiftKey && !ctrlOrMeta && key === "ArrowDown") {
      event.preventDefault();
      this.show();
      return;
    }
    if (event.altKey && !event.shiftKey && !ctrlOrMeta && key === "ArrowUp") {
      event.preventDefault();
      this.hide();
      // Intentionally allow auto reopen on subsequent data arrival.
      return;
    }

    // When navigating suggestions, Left/Right should return to editing and let the caret move
    if (
      (key === "ArrowLeft" || key === "ArrowRight") &&
      this.state.activeIndex !== -1
    ) {
      this.clearActive(); // returns to editing
      // Let the caret move normally
      return;
    }

    // Standard Undo/Redo shortcuts, prefer our custom acceptance history.
    if (isUndo) {
      if (this.tryExplicitUndo(event)) return;
      // If we did not handle, let the browser handle native undo, our IME-like link is no longer valid.
      this.state.history.lastCompositionIndex = -1;
      return;
    }
    if (isRedo) {
      if (this.tryExplicitRedo(event)) return;
      this.state.history.lastCompositionIndex = -1;
      return;
    }

    // Backspace IME-like behaviour.
    if (key === "Backspace") {
      // When a selection exists, Backspace should perform a normal deletion, not our custom undo.
      if (input.selectionStart !== input.selectionEnd) {
        return;
      }
      if (this.tryImeBackspaceUndo(event)) return;
      // Otherwise, normal backspace applies.
      return;
    }

    // Smart-literal Space: if we just accepted or explicitly undid, insert a literal space.
    if (key === " " || key === "Spacebar") {
      if (this.state.smartLiteral.spaceArmed) {
        this.state.smartLiteral.spaceArmed = false; // consume the arm
        this.state.history.lastCompositionIndex = -1; // a literal space is a new edit
        return; // allow default space insertion
      }
      // Fall through to normal edit processing below.
    }

    // Alt+Shift+number shortcuts, using event.code for layout-independent detection.
    // Do not consume bare number keys.
    const usesAltShiftNumericShortcut =
      event.altKey && event.shiftKey && !event.ctrlKey && !event.metaKey;

    if (usesAltShiftNumericShortcut) {
      const code = event.code || "";
      const m = /^(Digit|Numpad)([1-9])$/.exec(code);
      if (m) {
        const numberPressed = parseInt(m[2], 10); // 1..9
        // Only accept by shortcut when the popover is actually visible.
        if (this.isShown()) {
          const suggestion = this.nthSuggestion(numberPressed - 1);
          if (suggestion) {
            event.preventDefault();
            event.stopPropagation();
            this.acceptByElement(suggestion);
            this.hide(); // close on keyboard acceptance
            // Intentionally allow immediate reopening when new suggestions arrive while focus remains.
            return;
          }
        }
        // If no Nth suggestion exists or popover not shown, allow default behaviour.
        // Note, with Alt+Shift the default may insert a symbol on some layouts.
        return;
      }
    }

    // Any other key is a normal edit, clear smart-literal and IME-like link and any active selection.
    this.clearActive();
    this.state.smartLiteral.spaceArmed = false;
    this.state.history.lastCompositionIndex = -1;

    // Keep visibility updated on any normal edit.
    this.updateVisibility();
  };

  // Core operations

  /**
   * Returns the Nth suggestion element within the suggestions container.
   * @param {number} index zero-based
   * @returns {HTMLElement | null}
   */
  nthSuggestion(index) {
    const nodes = this.options;
    return nodes[index] || null;
  }

  /**
   * Apply a new value to the input and dispatch an 'input' event, using InputEvent where available.
   * Uses an inputType that reflects the kind of change, which is friendlier for downstream listeners.
   * Falls back to a plain Event in older browsers.
   * @param {string} next
   * @param {string} inputType
   * @param {string | null} data
   */
  applyValue(next, inputType = "insertReplacementText", data = null) {
    const input = this.input;
    if (!input) return;
    input.value = next;
    try {
      input.dispatchEvent(
        new InputEvent("input", { bubbles: true, inputType, data }),
      );
    } catch {
      input.dispatchEvent(new Event("input", { bubbles: true }));
    }
  }

  /**
   * Accepts a suggestion by replacing the target keyword in the search input with the tag name.
   * Updates undo/redo stacks, dispatches an "input" event for listeners, e.g., htmx,
   * re-focuses the input field, and arms smart-literal flags.
   *
   * The suggestion element must provide:
   * - data-tag-name: The text to insert into the input.
   * - data-original-keyword: The keyword to replace in the current input value.
   *
   * Replacement behaviour:
   * - Prefer replacing only the token around the caret.
   * - If the caret is not inside the expected token, fall back to a whole-token global replace,
   *   matching only exact tokens separated by whitespace or string boundaries.
   * - When a precise caret-scoped replacement occurs, the caret is placed just after the inserted tag.
   *
   * Note: This function does not close the popup. Callers decide whether to keep it open
   * for pointer interaction or close it for keyboard acceptance.
   *
   * @param {HTMLElement} suggestionElement
   */
  acceptByElement(suggestionElement) {
    const input = this.input;
    if (!input) return;

    // 1. Get the tag name from the data attribute.
    const newTagText = suggestionElement.dataset.tagName;
    if (!newTagText) {
      console.warn("No tag name found in data-tag-name attribute");
      return;
    }

    // 2. Get the original keyword from the data attribute.
    const oldKeywordText = suggestionElement.dataset.originalKeyword;
    if (!oldKeywordText) {
      console.warn(
        "No original keyword found in data-original-keyword attribute",
      );
      return;
    }

    // 3. Get the original value before making any changes.
    const oldValue = input.value;

    // 4. Prefer replacing only the token around the caret.
    const caretScoped = replaceTokenAtCaret(input, oldKeywordText, newTagText);

    // 5. If caret is not inside the expected token, fall back to previous behaviour.
    let newValue,
      caretAfter = null;
    if (caretScoped) {
      newValue = caretScoped.value;
      caretAfter = caretScoped.caretAfter;
    } else {
      // Escape special RegExp characters in the keyword.
      const escapedKeyword = oldKeywordText.replace(
        /[.*+?^${}()|[\]\\]/g,
        "\\$&",
      );

      // Instead of word boundaries (\b), look for spaces or start/end of the string.
      // (^|\\s) captures leading space or start-of-string; (?=\\s|$) is a lookahead for trailing space or end-of-string.
      const regex = new RegExp(`(^|\\s)${escapedKeyword}(?=\\s|$)`, "g");

      // Replace the matched keyword with the tag name, preserving any leading space via $1.
      newValue = oldValue.replace(regex, `$1${newTagText}`);
    }

    // 6. Only proceed if the value has actually changed.
    if (oldValue !== newValue) {
      const change = { before: oldValue, after: newValue };
      this.commit(change, caretAfter);
    }

    // 7. Arm smart-literal flags.
    this.state.smartLiteral.spaceArmed = true;

    // 8. Re-focus the input and update visibility.
    input.focus();
    this.updateVisibility();
  }

  /**
   * Commits a suggestion-acceptance change:
   * - Pushes onto the undo stack and clears redo, with a bounded history.
   * - Sets IME-like backspace link to the accepted change by index.
   * - Applies the value and dispatches an 'input' event.
   * - Optionally sets the caret position after the write.
   * @param {SuggestionChange} change
   * @param {number | null} caretAfter
   */
  commit(change, caretAfter = null) {
    const input = this.input;
    if (!input) return;

    const hist = this.state.history;

    hist.undo.push(change);
    if (hist.undo.length > MAX_HISTORY) {
      // Drop oldest item to bound memory and keep index alignment.
      hist.undo.shift();
      if (hist.lastCompositionIndex >= 0) {
        hist.lastCompositionIndex -= 1;
        if (hist.lastCompositionIndex < 0) hist.lastCompositionIndex = -1;
      }
    }
    hist.redo = [];
    hist.lastCompositionIndex = hist.undo.length - 1;

    this.applyValue(change.after, "insertReplacementText");

    if (caretAfter != null) {
      try {
        input.setSelectionRange(caretAfter, caretAfter);
      } catch {
        // Ignore if the input type does not support selection.
      }
    }
  }

  /**
   * Attempts to undo the most recent suggestion acceptance.
   * Only proceeds if the input's current value matches the "after" state of the last acceptance.
   * Moves the action to the redo stack, arms space literal, clears IME-like undo link,
   * refocuses, and updates visibility.
   * @param {KeyboardEvent} event
   * @returns {boolean}
   */
  tryExplicitUndo(event) {
    const hist = this.state.history;
    const last = hist.undo[hist.undo.length - 1];
    const input = this.input;
    if (!last || !input) return false;
    if (input.value !== last.after) return false;

    event.preventDefault();

    this.applyValue(last.before, "historyUndo");

    hist.undo.pop();
    hist.redo.push(last);

    this.state.smartLiteral.spaceArmed = true;
    hist.lastCompositionIndex = -1;

    input.focus();
    this.updateVisibility();
    return true;
  }

  /**
   * Attempts to redo the most recently undone suggestion acceptance.
   * Only proceeds if the input's current value matches the "before" state of the pending redo action.
   * Restores IME-like link, arms space literal, refocuses,
   * and updates visibility.
   * @param {KeyboardEvent} event
   * @returns {boolean}
   */
  tryExplicitRedo(event) {
    const hist = this.state.history;
    const last = hist.redo[hist.redo.length - 1];
    const input = this.input;
    if (!last || !input) return false;
    if (input.value !== last.before) return false;

    event.preventDefault();

    this.applyValue(last.after, "historyRedo");

    hist.redo.pop();
    hist.undo.push(last);
    hist.lastCompositionIndex = hist.undo.length - 1;

    this.state.smartLiteral.spaceArmed = true;

    input.focus();
    this.updateVisibility();
    return true;
  }

  /**
   * IME-like backspace undo, reverts the most recent acceptance if the input still matches
   * its "after" value. Also moves the action from undo to redo if it is on top, clears the
   * IME-like link, arms space insertion, and updates visibility.
   * @param {KeyboardEvent} event
   * @returns {boolean}
   */
  tryImeBackspaceUndo(event) {
    const input = this.input;
    const hist = this.state.history;
    if (!input) return false;

    const topIdx = hist.undo.length - 1;
    if (hist.lastCompositionIndex !== topIdx || topIdx < 0) return false;

    const last = hist.undo[topIdx];
    if (input.value !== last.after) return false;

    event.preventDefault();

    this.applyValue(last.before, "historyUndo");

    // Move the acceptance from undo to redo.
    hist.undo.pop();
    hist.redo.push(last);

    // Consume the IME-like link so it cannot be undone again via backspace.
    hist.lastCompositionIndex = -1;

    // Arm literal space after undo.
    this.state.smartLiteral.spaceArmed = true;

    this.updateVisibility();
    return true;
  }
}

// Singleton controller instance.
const NAVBAR_SEARCH = new NavbarSearchController();

// Initialise on initial load and after htmx settles new content.
addEventListener("DOMContentLoaded", () => {
  NAVBAR_SEARCH.init();
  NAVBAR_SEARCH.defocus(); // Defocus on load, as per previous behaviour.
});

addEventListener("htmx:afterSettle", (evt) => {
  NAVBAR_SEARCH.init(); // Rebind to swapped nodes.
  if (evt.detail && evt.detail.target === document.body) {
    NAVBAR_SEARCH.defocus(); // Defocus on page navigations initiated by htmx.
  }
});
