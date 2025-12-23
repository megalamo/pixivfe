import onPageLoad from "./init-helper.js";

/**
 * @fileoverview Range-selection behaviour for a checkbox-backed image grid.
 *
 * Intent:
 * Provide Nautilus-style Shift-based range selection on top of native checkboxes,
 * while deliberately not overriding normal one-by-one selection. This script also
 * adds form validation to ensure at least one checkbox is selected before submission.
 *
 * Deviation from the Nautilus UX specification:
 * Unlike Nautilus, a plain click or Space does not clear other selections. We let the
 * browser toggle only the targeted checkbox and merely update the anchor. Replacement
 * semantics apply only to Shift-based range operations. This is intentional, we are
 * copying the range-selection feel without taking over single-item selection.
 *
 * Mouse:
 * - Plain click, native toggle only, we set the anchor to the clicked item.
 * - Ctrl or Cmd + Click, native toggle only, anchor unchanged unless it is not set yet.
 * - Shift + Click, selects the contiguous range between the anchor and the clicked item;
 *   by default replaces prior selection, Ctrl or Cmd + Shift makes it a union.
 *
 * Keyboard:
 * - Space mirrors mouse semantics, we do not prevent the native toggle unless performing a range.
 * - Ctrl or Cmd + A selects all.
 *
 * Anchor policy:
 * - Plain click or Space sets the anchor to the interacted item.
 * - Ctrl or Cmd interaction does not change the anchor unless it is not set yet.
 * - Range operations keep the original pivot as the anchor so subsequent Shift actions
 *   continue from it.
 *
 * Validation:
 * - The script will find the parent <form> of the grid.
 * - It will find the <button type="submit"> within that form.
 * - The submit button will be disabled/enabled in real-time based on whether
 *   at least one checkbox is selected. A 'title' attribute is added when disabled.
 * - A final check on form submission prevents submission if no boxes are checked.
 *
 * Markup:
 * - Grid root has [data-selectable-grid].
 * - Grid should be a descendant of a <form> element.
 * - Form should contain a <button type="submit"> to be managed by the script.
 * - Each tile is a <label> linked to a single <input type="checkbox">, either immediately
 *   before it or via "for".
 *
 * Notes:
 * - Disabled checkboxes are skipped, programmatic changes dispatch a bubbling "change" event.
 * - No configuration attributes are read; range behaviour is fixed.
 */
(function () {
  /**
   * Attaches range-selection and validation event listeners to a single grid element.
   * @param {HTMLElement} grid The grid element with [data-selectable-grid].
   */
  function initializeSelectableGrid(grid) {
    const form = grid.closest("form");
    const submitButton = form
      ? form.querySelector('button[type="submit"]')
      : null;

    /**
     * Updates the disabled state and title attribute of the form's submit button
     * based on the number of selected checkboxes within the grid.
     */
    function updateSubmitButtonState() {
      if (!submitButton) return;

      const checkedCount = grid.querySelectorAll(
        'input[type="checkbox"]:checked'
      ).length;

      if (checkedCount === 0) {
        submitButton.disabled = true;
        submitButton.setAttribute(
          "title",
          "Please select at least one image to download."
        );
      } else {
        submitButton.disabled = false;
        submitButton.removeAttribute("title");
      }
    }

    const getCheckboxes = () =>
      Array.from(grid.querySelectorAll('input[type="checkbox"]'));

    let anchorEl = null;

    function isCtrlLike(event) {
      return event.ctrlKey || event.metaKey;
    }

    function setChecked(box, checked) {
      if (!box || box.disabled || box.checked === checked) return;
      box.checked = checked;
      box.dispatchEvent(new Event("change", { bubbles: true }));
    }

    function resolveCheckboxForLabel(label) {
      if (!label) return null;
      const prev = label.previousElementSibling;
      if (prev && prev.tagName === "INPUT" && prev.type === "checkbox")
        return prev;
      if (label.htmlFor) {
        try {
          const id =
            window.CSS && CSS.escape
              ? CSS.escape(label.htmlFor)
              : label.htmlFor;
          return grid.querySelector(`#${id}`);
        } catch {
          return grid.querySelector(`input[id="${label.htmlFor}"]`);
        }
      }
      return null;
    }

    function computeRangeEndpoints(aEl, bEl) {
      const checkboxes = getCheckboxes();
      const a = aEl ? checkboxes.indexOf(aEl) : -1;
      const b = bEl ? checkboxes.indexOf(bEl) : -1;
      // Allow a "range" of a single item (a === b).
      if (a === -1 || b === -1) {
        return { hasRange: false, checkboxes, a, b, start: -1, end: -1 };
      }
      const start = Math.min(a, b);
      const end = Math.max(a, b);
      return { hasRange: true, checkboxes, a, b, start, end };
    }

    function rangeReplace(endBox) {
      const { hasRange, checkboxes, start, end } = computeRangeEndpoints(
        anchorEl,
        endBox
      );
      if (!hasRange) return false;
      for (let i = 0; i < checkboxes.length; i++) {
        const inRange = i >= start && i <= end;
        setChecked(checkboxes[i], inRange);
      }
      try {
        endBox.focus({ preventScroll: true });
      } catch {}
      return true;
      // Anchor remains the original pivot.
    }

    function rangeUnion(endBox) {
      const { hasRange, checkboxes, start, end } = computeRangeEndpoints(
        anchorEl,
        endBox
      );
      if (!hasRange) return false;
      for (let i = start; i <= end; i++) {
        setChecked(checkboxes[i], true);
      }
      try {
        endBox.focus({ preventScroll: true });
      } catch {}
      return true;
      // Anchor remains the original pivot.
    }

    grid.addEventListener("click", (event) => {
      if (event.button !== 0) return;
      const label = event.target.closest("label");
      if (!label || !grid.contains(label)) return;

      const clickedCheckbox = resolveCheckboxForLabel(label);
      if (!clickedCheckbox) return;

      const checkboxes = getCheckboxes();
      if (checkboxes.indexOf(clickedCheckbox) === -1) return;

      const hasValidAnchor = !!anchorEl && checkboxes.indexOf(anchorEl) !== -1;

      // A range operation is any Shift-click when an anchor is set.
      // This correctly handles shift-clicking the anchor itself to deselect others.
      const isRange = event.shiftKey && hasValidAnchor;

      if (isRange) {
        // Intercept native toggle, perform programmatic range operation.
        event.preventDefault();
        if (isCtrlLike(event)) {
          rangeUnion(clickedCheckbox);
        } else {
          rangeReplace(clickedCheckbox);
        }
        return;
      }

      // Non-range clicks, do not interfere with native toggling.

      // Shift with no anchor, behave like a normal click, just record the anchor.
      if (event.shiftKey && !hasValidAnchor) {
        anchorEl = clickedCheckbox;
        return;
      }

      if (isCtrlLike(event)) {
        if (!anchorEl) anchorEl = clickedCheckbox;
        return;
      }

      // Plain click, native toggle only, set anchor.
      anchorEl = clickedCheckbox;
    });

    grid.addEventListener("keydown", (event) => {
      const target = event.target;
      if (!(target instanceof HTMLInputElement) || target.type !== "checkbox")
        return;

      // Ctrl or Cmd + A selects all
      if (isCtrlLike(event) && event.key.toLowerCase() === "a") {
        event.preventDefault();
        getCheckboxes().forEach((cb) => setChecked(cb, true));
        return;
      }

      // Space handling mirrors click semantics.
      if (event.key === " " || event.key === "Spacebar") {
        const checkboxes = getCheckboxes();
        const hasValidAnchor =
          !!anchorEl && checkboxes.indexOf(anchorEl) !== -1;

        // A range operation is any Shift-Space when an anchor is set.
        const isRange = event.shiftKey && hasValidAnchor;

        if (isRange) {
          event.preventDefault();
          if (isCtrlLike(event)) {
            rangeUnion(target);
          } else {
            rangeReplace(target);
          }
          return;
        }

        // Non-range Space, leave native toggle intact and update anchor policy.
        if (event.shiftKey && !hasValidAnchor) {
          anchorEl = target;
          return;
        }

        if (isCtrlLike(event)) {
          if (!anchorEl) anchorEl = target;
          return;
        }

        // Plain Space, set anchor, do not prevent default.
        anchorEl = target;
      }
    });

    // Listen for changes on any checkbox within the grid to update the button state.
    // This is efficient as it uses event delegation.
    grid.addEventListener("change", updateSubmitButtonState);

    // Add submit validation to the parent form.
    if (form && !form.dataset.validationInitialized) {
      // Mark the form as initialized to prevent attaching this listener multiple times.
      form.dataset.validationInitialized = "true";

      form.addEventListener("submit", (event) => {
        const checkedCount = grid.querySelectorAll(
          'input[type="checkbox"]:checked'
        ).length;

        if (checkedCount === 0) {
          // Prevent the form from submitting
          event.preventDefault();
          // Alert the user (can be replaced with a more elegant UI element)
          alert("Please select at least one image to download.");
        }
      });
    }

    // Call once on initialization to set the correct initial state of the button.
    updateSubmitButtonState();
  }

  /**
   * Finds and initializes all selectable grids on the page that haven't
   * been initialized yet.
   */
  function initializeAllSelectableGrids() {
    const grids = document.querySelectorAll(
      "[data-selectable-grid]:not([data-selectable-initialized])"
    );
    grids.forEach((grid) => {
      initializeSelectableGrid(grid);
      // Mark as initialized to prevent re-attaching listeners on subsequent calls.
      grid.dataset.selectableInitialized = "true";
    });
  }

  // Use the generic helper to initialize grids on page load and after HTMX swaps.
  onPageLoad(initializeAllSelectableGrids);

  // Expose for manual initialization if needed, e.g., from other JS modules.
  window.initializeSelectableGrid = initializeSelectableGrid;
})();
