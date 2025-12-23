"use strict";

/**
 * @fileoverview Attaches a delegated click listener to handle "select all"
 * functionality for checkboxes.
 */

// This script requires a <button type="button" data-check-all> trigger.
// Clicking the button will toggle all target checkboxes within the inferred scope.
//
// Optional attributes supported on the button:
// - data-scope: "fieldset" | "form" | "<css selector>"  (defaults to "fieldset")
// - data-target: CSS selector to directly pick target checkboxes within scope
// - data-check-name: restricts to input[type="checkbox"][name="<value>"] within scope
//
// Heuristics when neither data-target nor data-check-name are provided:
// - Prefer the nearest [data-selectable-grid] relative to the button
// - Fall back to all checkboxes in the resolved scope

const MASTER_SELECTOR = 'button[type="button"][data-check-all]';
const CHECKBOX_SELECTOR = 'input[type="checkbox"]';

/**
 * Converts a NodeList or similar array-like object to a true Array.
 * @param {NodeList|ArrayLike} list The list to convert.
 * @returns {Array<Node>}
 */
function toArray(list) {
  return Array.prototype.slice.call(list || []);
}

/**
 * Determines the DOM scope in which to search for target checkboxes.
 * @param {Element} master The "select all" button element.
 * @returns {Element|Document} The root element for the search.
 */
function getScope(master) {
  const scope = master.getAttribute("data-scope");
  const form = master.closest && master.closest("form");
  const base = form || document;

  if (!scope || scope === "fieldset") {
    return (master.closest && master.closest("fieldset")) || form || document;
  }
  if (scope === "form") {
    return form || document;
  }
  const node = base.querySelector(scope);
  return node || base;
}

/**
 * Finds the most relevant `[data-selectable-grid]` container relative to the button.
 * This is a heuristic used when no explicit target is provided.
 * @param {Element|Document} root The scope to search within.
 * @param {Element} master The "select all" button element.
 * @returns {Element|null} The nearest grid element, or null if none is found.
 */
function nearestGrid(root, master) {
  // If the master lives inside a grid, use that immediately.
  const inGrid = master.closest && master.closest("[data-selectable-grid]");
  if (inGrid && root.contains(inGrid)) return inGrid;

  const grids = toArray(root.querySelectorAll("[data-selectable-grid]"));
  if (grids.length === 0) return null;
  if (grids.length === 1) return grids[0];

  let nearestBefore = null;
  let nearestAfter = null;

  for (let i = 0; i < grids.length; i++) {
    const g = grids[i];
    const pos = g.compareDocumentPosition(master);

    // g is before master in the document
    if (pos & Node.DOCUMENT_POSITION_FOLLOWING) {
      nearestBefore = g; // last one wins, so it is the closest preceding
      continue;
    }
    // g is after master in the document
    if (!nearestAfter && pos & Node.DOCUMENT_POSITION_PRECEDING) {
      nearestAfter = g; // first after is the closest following
    }
  }

  return nearestBefore || nearestAfter || grids[0];
}

/**
 * Gathers all target checkbox elements for a given "select all" button.
 * @param {Element} master The "select all" button element.
 * @returns {Array<HTMLInputElement>} An array of checkbox elements.
 */
function getTargets(master) {
  const root = getScope(master);
  const css = master.getAttribute("data-target");
  const byName = master.getAttribute("data-check-name");

  let nodes;

  if (css) {
    nodes = root.querySelectorAll(css);
  } else if (byName) {
    // Use `CSS.escape` if available for robustness.
    const esc =
      window.CSS && CSS.escape
        ? CSS.escape(byName)
        : byName.replace(/["\\]/g, "\\$&");
    nodes = root.querySelectorAll('input[type="checkbox"][name="' + esc + '"]');
  } else {
    // Heuristic: find the nearest grid if no explicit target is given.
    const grid = nearestGrid(root, master);
    nodes = grid
      ? grid.querySelectorAll(CHECKBOX_SELECTOR)
      : root.querySelectorAll(CHECKBOX_SELECTOR);
  }

  return toArray(nodes).filter(function (n) {
    return (
      n instanceof HTMLInputElement && n.type === "checkbox" && !n.disabled
    );
  });
}

/**
 * Toggles the checked state of all target checkboxes.
 * If any are unchecked, all will be checked. Otherwise, all will be unchecked.
 * @param {Element} button The "select all" button element.
 */
function toggleAll(button) {
  const items = getTargets(button);
  if (items.length === 0) return;

  // If any item is unchecked, the desired state is to check all.
  const shouldCheck = items.some((i) => !i.checked);

  items.forEach((i) => {
    if (i.checked !== shouldCheck) {
      i.checked = shouldCheck;
      // Dispatch a change event so other scripts can react.
      i.dispatchEvent(new Event("change", { bubbles: true }));
    }
  });

  // Reflect the state for assistive technologies.
  const allChecked = items.every((i) => i.checked);
  button.setAttribute("aria-pressed", allChecked ? "true" : "false");
}

/**
 * The delegated event handler for clicks on the document.
 * Checks if a "select all" button was clicked and triggers the action.
 * @param {MouseEvent} event The click event.
 */
function handleSelectAllClick(event) {
  if (!(event.target instanceof Element)) {
    return;
  }

  const selectAllButton = event.target.closest(MASTER_SELECTOR);
  if (selectAllButton) {
    event.preventDefault();
    toggleAll(selectAllButton);
  }
}

/**
 * Attaches the single, delegated event listener to the document.
 */
function init() {
  document.addEventListener("click", handleSelectAllClick);
}

// Run initialization once the DOM is ready.
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  // DOM is already ready.
  init();
}
