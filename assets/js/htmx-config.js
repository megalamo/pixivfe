/**
 * @fileoverview Global HTMX configuration listeners.
 * These listeners modify the default behavior of HTMX for all requests.
 */

// On a network error when clicking a link, navigate to the page directly.
// This provides a better user experience than a silent failure.
addEventListener("htmx:sendError", function (event) {
  if (event.target.tagName === "A") {
    // If the server is down, this will show the browser's default "Unable to connect" page.
    document.location = event.detail.pathInfo.requestPath;
  }
});

// Allow responses with 4xx and 5xx status codes to be swapped into the DOM.
// This is useful for displaying server-rendered error messages within the page.
addEventListener("htmx:beforeOnLoad", function (event) {
  if (event.detail.failed) {
    event.detail.shouldSwap = true;
    event.detail.isError = false;
  }
});
