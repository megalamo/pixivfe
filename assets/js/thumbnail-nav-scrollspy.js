import onPageLoad from "./init-helper.js";

if (typeof window.updateActiveThumbnail !== "function") {
  window.updateActiveThumbnail = function (activeIndex) {
    const nav = document.getElementById("thumbnail_nav");
    if (!nav) return;

    const thumbnails = nav.querySelectorAll("a[id^='thumbnail-link-']");
    thumbnails.forEach((thumb, i) => {
      const img = thumb.querySelector("img");
      if (!img) return;

      const isActive = i + 1 === activeIndex;
      img.classList.toggle("brightness-100", isActive);
      img.classList.toggle("brightness-60", !isActive);

      if (isActive) {
        thumb.scrollIntoView({ behavior: "smooth", block: "nearest" });
      }
    });
  };
}

/**
 * Scrollspy for image thumbnails in the lightbox.
 *
 * Handles multiple landscape images visible simultaneously by tracking intersection
 * ratios and highlighting the thumbnail of the most visible image. This prevents
 * thumbnail jumping when several images are in viewport at once.
 */
function initializeScrollspy() {
  const container = document.getElementById("artwork_images_container");
  if (!container || container.dataset.scrollspyInitialized) return;

  const images = container.querySelectorAll('img[id^="artwork_image_"]');
  if (images.length === 0) return;

  // Track visible images with their intersection ratios
  const visibleImages = new Map();
  // Track the currently active thumbnail to prevent redundant updates
  let currentActiveIndex = -1;

  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          visibleImages.set(entry.target.id, entry.intersectionRatio);
        } else {
          visibleImages.delete(entry.target.id);
        }
      });

      // Find image with highest visibility ratio
      let mostVisible = { id: null, ratio: 0 };
      for (const [id, ratio] of visibleImages) {
        if (ratio > mostVisible.ratio) {
          mostVisible = { id, ratio };
        }
      }

      if (mostVisible.id) {
        const activeIndex = parseInt(mostVisible.id.split("_")[2], 10);
        if (activeIndex !== currentActiveIndex) {
          currentActiveIndex = activeIndex;
          window.updateActiveThumbnail(activeIndex);
        }
      }
    },
    {
      rootMargin: "-20% 0px -20% 0px",
      threshold: Array.from({ length: 21 }, (_, i) => i * 0.05),
    }
  );

  images.forEach((image) => observer.observe(image));
  container.dataset.scrollspyInitialized = "true";
}

onPageLoad(initializeScrollspy);
