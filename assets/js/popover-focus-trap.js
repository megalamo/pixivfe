import onPageLoad from "./init-helper.js";
/**
 * @fileoverview Manages focus trapping and accessibility for popover elements.
 *
 * - Traps Tab key navigation within an open popover.
 * - Closes the popover when clicking outside of it.
 * - Re-initializes on HTMX content swaps to support dynamic popovers.
 */

// Global registry for popover handlers to manage cleanup.
const popoverHandlers = new Map();

/**
 * Removes all previously attached event listeners for popovers.
 */
function cleanupPopoverHandlers() {
  popoverHandlers.forEach((handlers) => {
    document.removeEventListener("mousedown", handlers.clickOutsideHandler);
    document.removeEventListener("focusin", handlers.focusInHandler);
    if (handlers.element) {
      handlers.element.removeEventListener("keydown", handlers.keydownHandler);
    }
  });
  popoverHandlers.clear();
}

function createClickOutsideHandler(popover) {
  return function (event) {
    if (!popover.matches(":popover-open")) return;
    const invoker = document.querySelector(`[popovertarget="${popover.id}"]`);
    if (!popover.contains(event.target) && invoker !== event.target) {
      popover.hidePopover();
    }
  };
}

function createFocusInHandler(popover) {
  return function (event) {
    if (!popover.matches(":popover-open")) return;
    if (!popover.contains(event.target)) {
      const focusable = popover.querySelector(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable) focusable.focus();
    }
  };
}

function handlePopoverKeydown(event) {
  if (event.key !== "Tab") return;

  const focusable = Array.from(
    this.querySelectorAll(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    )
  );
  if (focusable.length === 0) return;

  const first = focusable[0];
  const last = focusable[focusable.length - 1];

  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

/**
 * Finds all popover elements and attaches focus trap and keyboard navigation handlers.
 * It's idempotent and safe to call multiple times.
 */
function setupPopoverFocusTrap() {
  // First, clean up any existing handlers to prevent duplicates.
  cleanupPopoverHandlers();

  const popovers = document.querySelectorAll("[popover]");
  popovers.forEach((popover) => {
    const popoverId = popover.id || `popover-${Date.now()}-${Math.random()}`;
    if (!popover.id) popover.id = popoverId;

    const handlers = {
      clickOutsideHandler: createClickOutsideHandler(popover),
      focusInHandler: createFocusInHandler(popover),
      keydownHandler: (e) => handlePopoverKeydown.call(popover, e),
      element: popover,
    };

    popoverHandlers.set(popoverId, handlers);

    popover.addEventListener("keydown", handlers.keydownHandler);
    document.addEventListener("mousedown", handlers.clickOutsideHandler);
    document.addEventListener("focusin", handlers.focusInHandler);
  });
}

onPageLoad(setupPopoverFocusTrap);
