import onPageLoad from "./init-helper.js";

/**
 * Calculates and updates the zoom level for artwork images.
 * The zoom level is the ratio of the image's displayed width to its original width.
 */
function updateZoomLevels() {
  const imageWrappers = document.querySelectorAll(".artwork-image-wrapper");

  imageWrappers.forEach((wrapper) => {
    const image = wrapper.querySelector(".artwork-image");
    const zoomDisplay = wrapper.querySelector(".zoom-level-display");

    if (!image || !zoomDisplay) {
      return;
    }

    const originalWidth = parseInt(image.getAttribute("width"), 10);

    const displayedWidth = image.clientWidth;

    if (originalWidth > 0) {
      // The image is constrained by `max-w-full`, so its zoom can't be > 100%.
      let zoomPercentage = (displayedWidth / originalWidth) * 100;

      // We are only ever scaling down, so cap the value at 100.
      zoomPercentage = Math.min(zoomPercentage, 100);

      zoomDisplay.textContent = `${Math.round(zoomPercentage)}%`;
    }
  });
}

// Event listeners
onPageLoad(updateZoomLevels);
window.addEventListener("resize", updateZoomLevels);
