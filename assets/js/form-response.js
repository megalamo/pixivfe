import onPageLoad from "./init-helper.js";

/**
 * @fileoverview Handles the close button on dismissible form response messages.
 */

/**
 * Handles the click event for closing the form response message.
 * @param {MouseEvent} event
 */
function handleFormResponseCloseClick(event) {
  const closeButton = event.target.closest(".js-close-form-response-btn");
  if (!closeButton) return;

  const formResponseEl = closeButton.closest("#form-htmx-response");
  if (formResponseEl) {
    // Fade out before removing for a smooth transition.
    formResponseEl.style.opacity = "0";
    setTimeout(() => formResponseEl.remove(), 200); // Matches CSS transition duration.
  }
}

/**
 * Sets up a single, delegated event listener on the document body.
 * This function is idempotent.
 */
function setupFormResponseCloseHandler() {
  // Remove any existing listener to be safe, then add it.
  document.body.removeEventListener("click", handleFormResponseCloseClick);
  document.body.addEventListener("click", handleFormResponseCloseClick);
}

onPageLoad(setupFormResponseCloseHandler);
