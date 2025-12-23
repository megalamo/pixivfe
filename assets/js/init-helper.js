/**
 * A generic helper for initializing JavaScript on page load and after HTMX swaps.
 *
 * This function registers a callback to be executed both when the initial
 * document is fully loaded and after any subsequent content swaps performed by HTMX.
 *
 * @param {function} callback - The function to execute.
 */
export default function onPageLoad(callback) {
  // Run on initial page load
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", callback);
  } else {
    // DOMContentLoaded has already fired, run now.
    callback();
  }

  // Run after HTMX swaps new content into the DOM
  document.body.addEventListener("htmx:afterSwap", callback);
}
