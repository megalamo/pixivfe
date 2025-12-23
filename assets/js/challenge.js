/**
 * @file challenge.js
 * @description Manages client-side logic for security challenges (Cloudflare Turnstile and link token).
 * Provides initialization, rendering, and lifecycle management for Turnstile widgets,
 * as well as an auto-redirect flow for link token based challenges.
 */

(function () {
  // Turnstile challenge logic

  /**
   * A unique identifier for the currently rendered Turnstile widget (if any).
   * This comes from `turnstile.render()` and is used for subsequent calls to
   * `turnstile.reset()` and `turnstile.remove()`.
   * @type {string|null}
   * @private
   */
  let currentTurnstileWidgetId = null;

  /**
   * Indicates whether the Turnstile API script has finished loading and invoked
   * `window.onloadTurnstileCallback`.
   * @type {boolean}
   * @private
   */
  let isTurnstileApiLoaded = false;

  /**
   * Callback for successful Turnstile challenge.
   * The token is automatically added to a hidden input `cf-turnstile-response` by Turnstile.
   * This function triggers submission of the verification form. If htmx is present,
   * it ensures the form is processed prior to submission so hx-* attributes take effect.
   *
   * @param {string} token The Turnstile token received upon successful completion.
   * @returns {void}
   */
  function handleTurnstileSuccess(token) {
    if (!token) {
      console.error("Challenge JS (Turnstile): No token received on success.");
      return;
    }

    /** @type {HTMLFormElement|null} */
    const verifyForm = document.getElementById("turnstile-verify-form");
    if (!verifyForm) {
      console.error(
        "Challenge JS (Turnstile): #turnstile-verify-form not found."
      );
      return;
    }

    // The token is automatically added to a hidden input `cf-turnstile-response` by Turnstile.
    // We just need to submit the form. htmx will handle the POST due to hx-post attribute.
    // Ensure htmx is available before calling process.
    if (window.htmx && typeof htmx.process === "function") {
      /** @type {HtmxAPI} */ (htmx).process(verifyForm); // Ensure htmx has processed the form
    }

    if (typeof verifyForm.requestSubmit === "function") {
      verifyForm.requestSubmit();
    } else {
      verifyForm.submit(); // Fallback for older browsers
    }
  }

  /**
   * Callback for Turnstile challenge errors.
   * Attempts to reset the widget when the error is a timeout.
   *
   * @param {string} errorCode The error code from Turnstile (for example, "timeout").
   * @returns {void}
   */
  function handleTurnstileError(errorCode) {
    console.error(
      `Challenge JS (Turnstile): Challenge error. Error Code: ${errorCode}`
    );
    // Attempt a reset on timeout.
    if (
      errorCode === "timeout" &&
      currentTurnstileWidgetId &&
      typeof turnstile !== "undefined"
    ) {
      /** @type {TurnstileAPI} */ (turnstile).reset(currentTurnstileWidgetId);
    }
  }

  /**
   * Callback for when the Turnstile token expires.
   * The widget will refresh automatically with `refresh-expired` defaulting to 'auto'.
   *
   * @returns {void}
   */
  function handleTurnstileExpired() {
    console.log("Challenge JS (Turnstile): Token expired.");
    // Widget will refresh automatically with `refresh-expired` defaulting to 'auto'.
  }

  /**
   * Callback for when an interactive challenge times out.
   * The widget will refresh automatically with `refresh-timeout` defaulting to 'auto'.
   *
   * @returns {void}
   */
  function handleTurnstileTimeout() {
    console.log("Challenge JS (Turnstile): Interactive challenge timed out.");
    // Widget will refresh automatically with `refresh-timeout` defaulting to 'auto'.
  }

  /**
   * Renders the Turnstile widget in the container with ID `turnstile-widget-container`.
   * If a previous widget exists, it is removed before rendering a new one.
   * Reads configuration values from the container's data attributes:
   * - data-sitekey (required)
   * - data-size
   * - data-appearance
   * - data-theme
   *
   * @returns {void}
   */
  function renderTurnstileWidget() {
    if (
      !isTurnstileApiLoaded ||
      typeof turnstile === "undefined" ||
      !turnstile.render
    ) {
      console.warn(
        "Challenge JS (Turnstile): API not ready or `turnstile.render` is undefined. Widget not rendered."
      );
      return;
    }

    /** @type {HTMLElement|null} */
    const container = document.getElementById("turnstile-widget-container");
    if (!container) {
      console.log(
        "Challenge JS (Turnstile): #turnstile-widget-container not found. Widget not rendered."
      );
      return;
    }

    // Clean up previous widget if any
    if (currentTurnstileWidgetId) {
      console.log(
        `Challenge JS (Turnstile): Removing previous widget (ID: ${currentTurnstileWidgetId}) before rendering new one.`
      );
      try {
        /** @type {TurnstileAPI} */ (turnstile).remove(
          currentTurnstileWidgetId
        );
      } catch (e) {
        // This might happen if the widget was already removed or ID is stale.
        console.warn(
          `Challenge JS (Turnstile): Error removing previous widget ${currentTurnstileWidgetId}: ${
            /** @type {Error} */ (e).message
          }`
        );
      }
      currentTurnstileWidgetId = null;
    }

    /** @type {string|undefined} */
    const siteKey = container.dataset.sitekey;
    if (!siteKey) {
      console.error(
        "Challenge JS (Turnstile): data-sitekey not found on container. Cannot render widget."
      );
      return;
    }

    /** @type {TurnstileRenderParams} */
    const renderParams = {
      sitekey: siteKey,
      callback: handleTurnstileSuccess,
      "error-callback": handleTurnstileError,
      "expired-callback": handleTurnstileExpired,
      "timeout-callback": handleTurnstileTimeout,

      // Read configuration from data attributes
      size: /** @type {TurnstileSize} */ (container.dataset.size),
      appearance: /** @type {TurnstileAppearance} */ (
        container.dataset.appearance
      ),
      theme: /** @type {TurnstileTheme} */ (container.dataset.theme),
    };

    try {
      console.log(
        "Challenge JS (Turnstile): Rendering widget with params:",
        renderParams
      );
      currentTurnstileWidgetId = /** @type {TurnstileAPI} */ (turnstile).render(
        container,
        renderParams
      );

      if (currentTurnstileWidgetId) {
        console.log(
          `Challenge JS (Turnstile): Widget rendered successfully. ID: ${currentTurnstileWidgetId}`
        );
      } else {
        console.error(
          "Challenge JS (Turnstile): turnstile.render() did not return a widgetId. Rendering may have failed."
        );
      }
    } catch (e) {
      console.error(
        "Challenge JS (Turnstile): Exception during turnstile.render():",
        e
      );
      currentTurnstileWidgetId = null;
    }
  }

  /**
   * Sets up all necessary callbacks and listeners for the Turnstile challenge.
   * - Assigns `window.onloadTurnstileCallback` for when the Turnstile API loads.
   * - Attaches a one-time `htmx:afterSwap` listener to re-render the widget when the container re-enters the DOM.
   * - Handles cases where scripts load in unexpected order by attempting a manual initialization.
   *
   * @returns {void}
   */
  function initTurnstile() {
    // This function is assigned to `window.onloadTurnstileCallback`.
    // It's called when the Turnstile API script finishes loading.
    /**
     * Cloudflare Turnstile invokes this when its API script finishes loading.
     * Responsible for marking the API as ready and performing an initial render.
     *
     * @type {() => void}
     * @name window.onloadTurnstileCallback
     */
    window.onloadTurnstileCallback = function () {
      if (isTurnstileApiLoaded) {
        console.log(
          "Challenge JS (Turnstile): API onload callback invoked again, but already processed. Ignoring."
        );
        return; // Prevent re-initialization
      }

      console.log(
        "Challenge JS (Turnstile): Cloudflare Turnstile API Loaded (onloadTurnstileCallback executed)."
      );
      isTurnstileApiLoaded = true;

      if (typeof turnstile === "undefined" || !turnstile.render) {
        console.error(
          "Challenge JS (Turnstile): onloadTurnstileCallback fired, but `turnstile` object or `turnstile.render` is undefined!"
        );
        return;
      }

      renderTurnstileWidget(); // Attempt to render if the container is already in the DOM
    };

    // Ensure htmx event listener is added only once to `document.body`
    if (!window.isChallengeHtmxListenerAttached) {
      /**
       * Re-render the widget after htmx swaps in new content that includes the container.
       * @listens HTMXAfterSwap
       */
      document.body.addEventListener("htmx:afterSwap", function () {
        if (!document.getElementById("turnstile-widget-container")) return;

        console.log(
          "Challenge JS (Turnstile): htmx:afterSwap - Container present. Re-rendering widget."
        );
        renderTurnstileWidget();
      });
      /**
       * Flag on the window object to ensure the htmx listener is attached only once.
       * @type {boolean}
       */
      window.isChallengeHtmxListenerAttached = true;
    }

    // Fallback for initial page load if scripts load in an unexpected order.
    const isRenderReady =
      typeof turnstile !== "undefined" &&
      typeof turnstile.render === "function";

    if (!isRenderReady) return;

    if (isTurnstileApiLoaded) {
      console.log(
        "Challenge JS (Turnstile): API was already loaded. Rendering widget."
      );
      renderTurnstileWidget();
      return;
    }

    console.warn(
      "Challenge JS (Turnstile): `turnstile` object exists but API not marked 'loaded'. Manually triggering setup."
    );
    window.onloadTurnstileCallback();
  }

  // Link token logic

  /**
   * Sets up auto-redirect for the Link Token challenge.
   * The page includes a stylesheet <link> tag with ID `link-token-stylesheet` whose load event
   * indicates the token/cookie has been set. After it loads (or after a fallback timeout),
   * the browser navigates to the desired return path.
   *
   * Required DOM:
   * - <link id="link-token-stylesheet" data-return-path="/some/path" ...>
   *
   * @returns {void}
   */
  function initLinkToken() {
    /** @type {HTMLLinkElement|null} */
    const el = document.getElementById("link-token-stylesheet");
    if (!el) return;

    /** @type {string} */
    const returnPath = el.dataset.returnPath || "/";

    /**
     * Navigates the browser to the provided return path.
     * Defined as a local helper to avoid duplicating try/catch.
     * @returns {void}
     * @inner
     */
    function go() {
      try {
        window.location.assign(returnPath);
      } catch (_) {
        // In case of error, a manual link is available on the page.
      }
    }

    // Best-effort: after the stylesheet loads (setting the cookie), wait a tick then redirect.
    /** @type {number} */
    const fallbackTimer = setTimeout(go, 1500); // Fallback timeout in case the 'load' event doesn't fire.
    el.addEventListener(
      "load",
      function () {
        // Clear fallback to avoid double navigation if 'load' fires.
        clearTimeout(fallbackTimer);
        setTimeout(go, 50);
      },
      { once: true }
    );
  }

  /**
   * Main bootstrapper that detects which challenge flow is present on the page
   * and initializes the appropriate logic when the DOM is ready.
   *
   * - If an element with ID `turnstile-widget-container` exists, initializes Turnstile.
   * - Else, if an element with ID `link-token-stylesheet` exists, initializes Link Token logic.
   *
   * @listens document#DOMContentLoaded
   */
  document.addEventListener("DOMContentLoaded", () => {
    // Check which challenge is active and initialize its specific logic.
    if (document.getElementById("turnstile-widget-container")) {
      console.log("Challenge JS: Turnstile challenge detected. Initializing.");
      initTurnstile();
    } else if (document.getElementById("link-token-stylesheet")) {
      console.log("Challenge JS: Link Token challenge detected. Initializing.");
      initLinkToken();
    }
  });
})();
