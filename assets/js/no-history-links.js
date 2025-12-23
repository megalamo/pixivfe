/**
 * @fileoverview Prevents anchor links with [data-no-history] from pushing to browser history.
 *
 * This is useful for in-page tabs or accordions where updating the URL is not desired.
 * It manually scrolls the target element into view.
 */
function preventHistoryPush(e) {
  const link = e.target.closest("a[data-no-history]");

  if (link && link.hash) {
    // Prevent default navigation and history push.
    e.preventDefault();

    const target = document.querySelector(link.hash);
    if (target) {
      // Manually scroll to the target.
      target.scrollIntoView({ behavior: "auto" });
    }
  }
}

// This is a delegated listener on the document, so it only needs to be attached once.
document.addEventListener("click", preventHistoryPush);
