/**
 * @file Manages a loading bar that shows htmx request activity.
 *
 * This script provides a loading indicator that shows the progress of htmx
 * network requests. The indicator uses timestamps to determine which requests
 * should be displayed, which helps handle navigation requests and page
 * transitions.
 *
 * # How it works
 *
 * The indicator tracks requests using a "last page settle time" to determine
 * which requests are relevant. This timestamp-based approach avoids complex
 * context management by using time comparisons.
 *
 * # Request states
 *
 * The indicator moves through several states. When the first relevant request
 * begins, the indicator starts a progress animation that moves toward a target
 * width less than 100 percent. When the last relevant request completes, a
 * grace period begins to prevent flickering from rapid successive requests.
 * After the grace period, the bar animates to completion and slides off screen.
 *
 * # Request relevance
 *
 * A request affects the loading indicator only if it started at or after the
 * last page settle time. Older in-flight requests are ignored to avoid
 * race conditions during rapid navigations where older requests could keep the
 * indicator alive indefinitely.
 *
 * # Page transitions
 *
 * For htmx-driven navigation, the last page settle time is updated when
 * `htmx:afterSettle` fires on the document body. The timestamp rules then
 * determine which requests should continue their animations. For
 * browser-driven navigation, all state is reset when `htmx:historyRestore`
 * fires to ensure a clean state.
 *
 * # Request queueing during finishing
 *
 * When new requests arrive while the indicator is in a finishing state, they
 * are queued rather than interrupting the current completion sequence. Queued
 * requests are preserved in the tracking map even after they complete, which
 * prevents a race condition where fast-completing requests would be removed
 * before the finishing animation ends. After the finishing animation completes
 * entirely, any queued requests trigger a new loading cycle seamlessly.
 *
 * # Adaptive grace period
 *
 * The grace period before starting the completion animation scales dynamically
 * based on recent request activity. A sliding window tracks request frequency,
 * and the grace period increases during periods of high activity to provide
 * better debouncing for rapid successive requests.
 *
 * # Cleanup
 *
 * A timeout-based cleanup mechanism removes old requests to prevent memory
 * leaks or a stuck indicator in edge cases.
 *
 * # Excluding Requests
 *
 * To prevent a request from triggering the loading indicator, add the boolean
 * attribute specified by the `NO_INDICATOR_ATTRIBUTE` constant to the element
 * that initiates the htmx request. By default, this attribute is
 * `data-no-loading-indicator`.
 *
 * # Debugging
 *
 * When the `DEBUG` flag is enabled, the indicator logs its state changes to
 * the console. Additionally, a 10ms polling interval is activated while the
 * indicator is visible, logging its position, dimensions, and computed styles
 * to aid in debugging animations and behavior.
 *
 * # Dependencies
 *
 * This script requires htmx.js to be present.
 *
 * # Usage
 *
 * The script requires an HTML element with the `id` specified by the
 * `INDICATOR_ID` constant, which is "loading-indicator" by default. This
 * element must also have the `hx-preserve="true"` attribute to prevent htmx
 * from swapping it during content replacement. The script initializes on
 * `DOMContentLoaded` and attaches to the element.
 *
 * terser --compress --mangle --output loading-indicator@0.1.0.min.js loading-indicator@0.1.0.js
 */

/* eslint-disable no-console */

/**
 * The `id` attribute of the DOM element used as the loading indicator.
 * @type {string}
 */
const INDICATOR_ID = "loading-indicator";

/**
 * The boolean attribute to add to an element to prevent its htmx requests
 * from triggering the loading indicator. For example, add
 * `data-no-loading-indicator` to a polling element.
 * @type {string}
 */
const NO_INDICATOR_ATTRIBUTE = "data-no-loading-indicator";

/**
 * Manages timeout operations with automatic cleanup and optional naming.
 *
 * This class consolidates timeout management. It tracks active timeouts to
 * allow for centralized clearing and provides optional naming for timeouts to
 * assist with debugging.
 */
class TimeoutManager {
  constructor() {
    /**
     * Maps timeout IDs to their descriptive names for debugging purposes.
     * @type {Map<number, string>}
     */
    this.activeTimeouts = new Map();
  }

  /**
   * Creates a timeout with automatic cleanup and optional naming.
   * @param {Function} callback The function to execute when the timeout fires.
   * @param {number} delay The delay in milliseconds.
   * @param {string} [name='unnamed'] Optional name for debugging purposes.
   * @returns {number} The timeout ID that can be used with clearTimeout.
   */
  setTimeout(callback, delay, name = "unnamed") {
    const id = setTimeout(() => {
      this.activeTimeouts.delete(id);
      callback();
    }, delay);
    this.activeTimeouts.set(id, name);
    return id;
  }

  /**
   * Clears a specific timeout if it exists and is still active.
   * @param {number|null} id The timeout ID to clear.
   */
  clearTimeout(id) {
    if (id && this.activeTimeouts.has(id)) {
      clearTimeout(id);
      this.activeTimeouts.delete(id);
    }
  }

  /**
   * Clears all active timeouts managed by this instance.
   */
  clearAll() {
    this.activeTimeouts.forEach((name, id) => clearTimeout(id));
    this.activeTimeouts.clear();
  }

  /**
   * Returns the number of active timeouts for debugging purposes.
   * @returns {number} The count of active timeouts.
   */
  getActiveCount() {
    return this.activeTimeouts.size;
  }
}

/**
 * Handles visual animations for the loading indicator.
 *
 * This class manages the visual state of the indicator element. The crawling
 * animation uses `requestAnimationFrame` with a custom easing function. This
 * approach is used for the main progress animation instead of a pure CSS
 * animation. The finish animation uses a single CSS transition to slide off screen.
 *
 * If a new request arrives while the completion animation is running, the
 * request is queued and processed after the current animation completes.
 */
class LoadingIndicatorAnimationController {
  /**
   * Creates a new animation controller for the specified element.
   * @param {HTMLElement} element The DOM element to animate.
   * @param {Object} config Configuration object containing timing constants.
   * @param {TimeoutManager} timeoutManager The timeout manager for managing delayed operations.
   */
  constructor(element, config, timeoutManager) {
    /** @type {HTMLElement} */
    this.element = element;
    /** @type {Object} */
    this.config = config;
    /** @type {TimeoutManager} */
    this.timeoutManager = timeoutManager;

    /** @type {number|null} */
    this.crawlStartTime = null;
    /** @type {number|null} */
    this.crawlAnimationId = null;
  }

  /**
   * Starts the progress crawling animation using requestAnimationFrame.
   *
   * This method uses a JavaScript-based animation with a custom arctangent
   * easing function. This implementation was chosen over a pure CSS approach.
   */
  startCrawling() {
    this.element.style.setProperty("transition", "none", "important");
    this.element.style.opacity = "1";
    this.element.style.transform = "translateX(-100%)";

    this.crawlStartTime = performance.now();
    this._updateCrawlAnimation();
  }

  /**
   * Stops the crawling animation by canceling the requestAnimationFrame loop.
   */
  stopCrawling() {
    if (this.crawlAnimationId !== null) {
      cancelAnimationFrame(this.crawlAnimationId);
      this.crawlAnimationId = null;
    }
  }

  /**
   * Animates the indicator to slide off screen and scale down.
   *
   * This method uses a single CSS transition to move the indicator off screen
   * while scaling it down, providing a clean completion effect.
   *
   * @param {Function} onComplete A function to execute when the animation ends.
   */
  finishAndHide(onComplete) {
    this.stopCrawling();

    this.element.style.transformOrigin = "right";
    this.element.style.setProperty(
      "transition",
      `transform ${this.config.FINISH_ANIMATION_MS}ms ease-out`,
      "important",
    );
    this.element.style.transform = "translateX(0%) scaleX(0)";

    this.timeoutManager.setTimeout(
      onComplete,
      this.config.FINISH_ANIMATION_MS,
      "finish-complete",
    );
  }

  /**
   * Resets all visual styles to the initial state.
   *
   * This method stops all animations and resets the element to its default
   * appearance.
   */
  reset() {
    this.stopCrawling();

    this.element.style.setProperty("transition", "none", "important");
    this.element.style.transform = "scaleX(0)";
    this.element.style.opacity = "1";
  }

  /**
   * Runs the progress animation loop using requestAnimationFrame.
   *
   * This method calculates the indicator's progress based on elapsed time and
   * applies a `translateX` transform. The progress easing uses an arctangent
   * function, which produces a fast start followed by a gradual slowdown. This
   * method is part of the JavaScript-based animation implementation.
   */
  _updateCrawlAnimation() {
    const elapsed = performance.now() - this.crawlStartTime;

    // An arctangent-based progress function provides smooth easing.
    // It has a fast initial acceleration followed by a gradual slowdown.
    // The (2 / Math.PI) factor normalizes the output of atan() to
    // approach 1. The shapeConstant controls steepness.
    const shapeConstant = this.config.CRAWL_ANIMATION_MS / 2.5;
    const progress = (2 / Math.PI) * Math.atan(elapsed / shapeConstant);
    const translateXPercent =
      -100 * (1 - progress * this.config.CRAWL_TARGET_SCALE);

    this.element.style.transform = `translateX(${translateXPercent}%)`;

    this.crawlAnimationId = requestAnimationFrame(() =>
      this._updateCrawlAnimation(),
    );
  }
}

/**
 * Manages the state and animations of the htmx loading indicator.
 *
 * This class uses timestamp-based logic to determine which requests affect
 * the indicator's display. It delegates timeout and animation tasks to the
 * `TimeoutManager` and `LoadingIndicatorAnimationController` classes,
 * respectively. The main responsibility of this class is request tracking,
 * state management, and coordinating the other components.
 */
class HtmxLoadingIndicator {
  /**
   * Enables or disables logging to the console.
   * @type {boolean}
   */
  static DEBUG = false;
  /**
   * The target duration for the initial progress animation, in milliseconds.
   * The animation approaches its target position over this duration.
   *
   * Ideally, this value should approximate the expected response time for requests.
   * @type {number}
   */
  static CRAWL_ANIMATION_MS = 300;
  /**
   * The target visibility ratio that the progress animation approaches. This
   * is less than 1 to ensure the bar does not appear complete until all
   * requests are finished.
   * @type {number}
   */
  static CRAWL_TARGET_SCALE = 0.4;
  /**
   * The duration of the finish animation that slides the indicator off screen.
   * @type {number}
   */
  static FINISH_ANIMATION_MS = 400;
  /**
   * The time window, in milliseconds, used to track recent request activity
   * for adaptive grace period calculation.
   * @type {number}
   */
  static ACTIVITY_WINDOW_MS = 2000;
  /**
   * The minimum grace period, in milliseconds, used when request activity is low.
   * @type {number}
   */
  static FINISH_GRACE_PERIOD_MIN_MS = 100;
  /**
   * The maximum grace period, in milliseconds, used when request activity is high.
   * @type {number}
   */
  static FINISH_GRACE_PERIOD_MAX_MS = 500;
  /**
   * The number of requests within the activity window that triggers the maximum
   * grace period. Activity levels above this value are capped at the maximum.
   * @type {number}
   */
  static MAX_ACTIVITY_FOR_SCALING = 8;
  /**
   * Maximum time to wait for old requests to complete before forcing cleanup.
   * This prevents memory leaks in edge cases.
   * @type {number}
   */
  static OLD_REQUEST_CLEANUP_TIMEOUT_MS = 5000;

  /**
   * Creates a new HtmxLoadingIndicator instance.
   * @param {HTMLElement} element The DOM element to use as the indicator.
   */
  constructor(element) {
    /** @type {HTMLElement} */
    this.element = element;

    /**
     * Manages all timeout operations with automatic cleanup.
     * @type {TimeoutManager}
     */
    this.timeoutManager = new TimeoutManager();

    /**
     * Handles all visual animations and DOM manipulations.
     * @type {LoadingIndicatorAnimationController}
     */
    this.animationController = new LoadingIndicatorAnimationController(
      element,
      {
        CRAWL_ANIMATION_MS: HtmxLoadingIndicator.CRAWL_ANIMATION_MS,
        CRAWL_TARGET_SCALE: HtmxLoadingIndicator.CRAWL_TARGET_SCALE,
        FINISH_ANIMATION_MS: HtmxLoadingIndicator.FINISH_ANIMATION_MS,
      },
      this.timeoutManager,
    );

    /**
     * Tracks all requests with their start times, queue status, and completion status.
     * Maps XMLHttpRequest to {startTime: number, isQueued: boolean, isCompleted: boolean}
     * @type {Map<XMLHttpRequest, {startTime: number, isQueued: boolean, isCompleted: boolean}>}
     */
    this.requests = new Map();

    /**
     * Tracks recent request activity for adaptive grace period calculation.
     * Contains timestamps of all recent requests within the activity window.
     * @type {number[]}
     */
    this.recentActivityTimestamps = [];

    /**
     * Timestamp of when the page last settled via `htmx:afterSettle` on body.
     * @type {number}
     */
    this.lastPageSettleTime = 0;

    /**
     * Explicitly tracks whether a finish timeout is scheduled. This avoids
     * tight coupling between timeout management and state logic by eliminating
     * the need to inspect timeout names.
     * @type {boolean}
     */
    this.isFinishPending = false;

    /**
     * ID of a scheduled finish timeout, if any. Used to cancel only the finish
     * debounce without affecting other timeouts like cleanup.
     * @type {number|null}
     */
    this.finishTimeoutId = null;

    /**
     * ID of a scheduled cleanup timeout, if any. Kept separate from finish to
     * avoid accidental cancellation.
     * @type {number|null}
     */
    this.cleanupTimeoutId = null;

    /** @type {'idle'|'crawling'|'finishing'} */
    this.state = "idle";

    /**
     * The ID of the interval used for debug polling.
     * @type {number|null}
     */
    this.debugPollIntervalId = null;

    this.log("Attaching to element");
    this.reset();
  }

  /**
   * Logs a message to the console if debugging is enabled.
   * Includes timestamp, current state, and relevant request count.
   * @param {string} message The message to log.
   */
  log(message) {
    if (!HtmxLoadingIndicator.DEBUG) return;
    const timestamp = new Date().toISOString().slice(11, 23);
    const relevantCount = this.getRelevantRequestCount();
    console.log(
      `[${timestamp}][hli][${this.state}][${relevantCount} req]`,
      message,
    );
  }

  /**
   * Tracks a request timestamp for activity-based grace period calculation.
   * Maintains a sliding window of recent activity by removing old timestamps.
   * @param {number} timestamp The request start timestamp.
   * @private
   */
  _trackActivity(timestamp) {
    this.recentActivityTimestamps.push(timestamp);

    // Remove timestamps outside the activity window
    const cutoffTime = timestamp - HtmxLoadingIndicator.ACTIVITY_WINDOW_MS;
    this.recentActivityTimestamps = this.recentActivityTimestamps.filter(
      (ts) => ts > cutoffTime,
    );
  }

  /**
   * Calculates the grace period based on recent request activity.
   * Returns a scaled value between the minimum and maximum grace periods.
   * @returns {number} The calculated grace period in milliseconds.
   * @private
   */
  _calculateGracePeriod() {
    const now = performance.now();
    const cutoffTime = now - HtmxLoadingIndicator.ACTIVITY_WINDOW_MS;

    // Clean up old timestamps
    this.recentActivityTimestamps = this.recentActivityTimestamps.filter(
      (ts) => ts > cutoffTime,
    );

    const activityCount = this.recentActivityTimestamps.length;
    const scaleFactor = Math.min(
      activityCount / HtmxLoadingIndicator.MAX_ACTIVITY_FOR_SCALING,
      1.0,
    );

    const gracePeriod = Math.round(
      HtmxLoadingIndicator.FINISH_GRACE_PERIOD_MIN_MS +
        scaleFactor *
          (HtmxLoadingIndicator.FINISH_GRACE_PERIOD_MAX_MS -
            HtmxLoadingIndicator.FINISH_GRACE_PERIOD_MIN_MS),
    );

    return gracePeriod;
  }

  /**
   * Cancel only the pending finish timeout, if present.
   * @private
   */
  _cancelFinishIfPending() {
    if (this.isFinishPending && this.finishTimeoutId) {
      this.timeoutManager.clearTimeout(this.finishTimeoutId);
      this.finishTimeoutId = null;
      this.isFinishPending = false;
      this.log("Canceled pending finish");
    }
  }

  /**
   * Schedule a one-shot cleanup if none is currently scheduled.
   * @private
   */
  _scheduleCleanup() {
    if (this.cleanupTimeoutId) return;
    this.cleanupTimeoutId = this.timeoutManager.setTimeout(
      () => {
        this.cleanupTimeoutId = null;
        this._cleanupOldRequests();
      },
      HtmxLoadingIndicator.OLD_REQUEST_CLEANUP_TIMEOUT_MS,
      "cleanup",
    );
  }

  /**
   * Determines if a request should affect the loading indicator.
   *
   * A request is considered relevant if it started at or after the last page
   * settle time. Older in-flight requests are ignored to avoid race conditions
   * during rapid navigations.
   *
   * @param {XMLHttpRequest} xhr The request to check.
   * @returns {boolean} True if the request is relevant.
   */
  isRequestRelevant(xhr) {
    const data = this.requests.get(xhr);
    if (!data || data.isQueued) return false;

    const startTime = data.startTime;
    return startTime >= this.lastPageSettleTime;
  }

  /**
   * Returns the number of requests that affect the loading indicator state.
   * @returns {number} The number of relevant active requests.
   */
  getRelevantRequestCount() {
    let count = 0;
    this.requests.forEach((data, xhr) => {
      if (!data.isQueued && !data.isCompleted && this.isRequestRelevant(xhr)) {
        count++;
      }
    });
    return count;
  }

  /**
   * Returns the number of queued requests, including completed ones.
   * This ensures that queued requests trigger a new loading cycle even if
   * they complete before the current finishing animation ends.
   * @returns {number} The number of queued requests.
   */
  getQueuedRequestCount() {
    let count = 0;
    this.requests.forEach((data) => {
      if (data.isQueued) count++;
    });
    return count;
  }

  /**
   * Removes old requests based on age. This prevents memory leaks
   * and stuck indicators in edge cases.
   */
  _cleanupOldRequests() {
    const now = performance.now();
    const cutoffTime =
      now - HtmxLoadingIndicator.OLD_REQUEST_CLEANUP_TIMEOUT_MS;

    const requestsToDelete = [];
    this.requests.forEach((data, xhr) => {
      if (data.startTime < cutoffTime) {
        requestsToDelete.push(xhr);
      }
    });

    if (requestsToDelete.length > 0) {
      requestsToDelete.forEach((xhr) => this.requests.delete(xhr));
      this.log(`Cleaned up ${requestsToDelete.length} old requests`);

      // Check if we should reset the indicator after cleanup
      if (this.getRelevantRequestCount() === 0 && this.state === "crawling") {
        this.log("No relevant requests after cleanup, resetting indicator");
        this.reset();
      }
    }
  }

  /**
   * Starts a polling interval that logs the indicator's DOM state.
   * This is only active when `DEBUG` is true.
   * @private
   */
  _startDebugPolling() {
    if (!HtmxLoadingIndicator.DEBUG || this.debugPollIntervalId) return;

    this.debugPollIntervalId = setInterval(() => {
      const rect = this.element.getBoundingClientRect();
      const styles = window.getComputedStyle(this.element);

      const debugInfo = {
        transform: styles.transform,
        opacity: styles.opacity,
        transition: styles.transition,
        width: `${rect.width.toFixed(2)}px`,
        left: `${rect.left.toFixed(2)}px`,
        state: this.state,
        reqs: this.getRelevantRequestCount(),
        queued: this.getQueuedRequestCount(),
        activity: this.recentActivityTimestamps.length,
      };
      // Use a distinct color for poll logs to differentiate them
      console.log("%c[POLL]", "color: cyan;", debugInfo);
    }, 1000);
  }

  /**
   * Stops the debug polling interval.
   * @private
   */
  _stopDebugPolling() {
    if (this.debugPollIntervalId) {
      clearInterval(this.debugPollIntervalId);
      this.debugPollIntervalId = null;
    }
  }

  /**
   * Resets the indicator to its initial state.
   *
   * This method stops all animations, clears timeouts, and resets the element's
   * styles. All request tracking is cleared. It uses the `TimeoutManager` and
   * `LoadingIndicatorAnimationController` for cleanup operations.
   *
   * Except for full context switches, this method should not be called
   * while the indicator is in the "finishing" state, as completion animations will be interrupted.
   */
  reset() {
    if (this.state === "finishing") {
      this.log(
        "reset() called while finishing, this will interrupt completion animations",
      );
    }

    this.log("Resetting");
    this.animationController.reset();
    this.timeoutManager.clearAll();
    this._stopDebugPolling();

    // Reset timers and flags
    this.isFinishPending = false;
    this.finishTimeoutId = null;
    this.cleanupTimeoutId = null;

    // Reset activity and state
    this.recentActivityTimestamps = [];
    this.state = "idle";
    this.requests.clear();
  }

  /**
   * Handles the `htmx:beforeRequest` or `htmx:xhr:loadstart` events.
   *
   * This method tracks the start of a new request by adding it to the requests
   * map with its start time. Requests triggered by an element with the attribute
   * defined by `NO_INDICATOR_ATTRIBUTE` are ignored. If this is the first
   * relevant request, the progress animation begins. If a completion sequence
   * is already in progress, the request is queued for processing after the
   * current cycle completes.
   * @param {CustomEvent} evt The htmx event object.
   */
  onRequestStart(evt) {
    const triggerElement = evt.detail.elt;
    if (triggerElement && triggerElement.hasAttribute(NO_INDICATOR_ATTRIBUTE)) {
      this.log(
        `Ignoring request from element with '${NO_INDICATOR_ATTRIBUTE}'`,
      );
      return;
    }

    const xhr = evt.detail.xhr;
    if (!xhr) return;

    // Guard against duplicate start events for the same XHR (e.g., both beforeRequest and xhr:loadstart)
    if (this.requests.has(xhr)) {
      this.log(
        `Duplicate start ignored (${evt.type}): ${evt.detail.pathInfo?.path || "unknown"}`,
      );
      return;
    }

    const startTime = performance.now();
    const isQueued = this.state === "finishing";

    // Track activity for all requests (including queued ones)
    this._trackActivity(startTime);

    this.requests.set(xhr, { startTime, isQueued, isCompleted: false });

    this.log(
      `Request ${isQueued ? "queued" : "started"} (${evt.type}): ${evt.detail.pathInfo?.path || "unknown"}`,
    );

    if (isQueued) return;

    // Cancel only the pending finish sequence when new requests come in
    this._cancelFinishIfPending();

    const relevantCount = this.getRelevantRequestCount();
    const isFirstRelevantRequest = relevantCount === 1;

    if (isFirstRelevantRequest && this.state === "idle") {
      this.startCrawling();
    }

    // Ensure periodic cleanup is scheduled as a safety net
    this._scheduleCleanup();
  }

  /**
   * Handles all htmx events that signify the end of a request.
   *
   * This method marks the request as completed but preserves queued requests
   * in the tracking map to prevent race conditions. Only non-queued requests
   * are removed immediately. If this was the last relevant request, it schedules
   * the completion sequence after a grace period calculated based on recent activity.
   * @param {CustomEvent} evt The htmx event object.
   */
  onRequestEnd(evt) {
    const xhr = evt.detail.xhr;
    if (!xhr || !this.requests.has(xhr)) return;

    const data = this.requests.get(xhr);

    // If this was queued, just mark it complete and let finishComplete() handle it
    if (data.isQueued) {
      data.isCompleted = true;
      this.log(`Queued request completed (${evt.type})`);
      return;
    }

    // For logging only (do not gate finishing on this)
    const wasRelevantAtEnd = this.isRequestRelevant(xhr);

    this.requests.delete(xhr);
    this.log(`Request end (${evt.type})`);

    const relevantCount = this.getRelevantRequestCount();
    this.log(`${relevantCount} relevant requests remaining`);

    // If no relevant requests remain and we are still crawling, schedule a finish.
    if (
      relevantCount === 0 &&
      this.state === "crawling" &&
      !this.isFinishPending
    ) {
      const gracePeriod = this._calculateGracePeriod();
      const activityCount = this.recentActivityTimestamps.length;

      this.log(
        `All relevant requests finished. Waiting ${gracePeriod}ms to finish (activity: ${activityCount}; lastEndedWasRelevant=${wasRelevantAtEnd}).`,
      );

      this.isFinishPending = true;
      this.finishTimeoutId = this.timeoutManager.setTimeout(
        () => {
          this.finish();
        },
        gracePeriod,
        "finish",
      );
    }
  }

  /**
   * Handles page settlements by updating the last settle time.
   *
   * This timestamp update automatically adjusts request relevance.
   */
  onPageSettle() {
    this.lastPageSettleTime = performance.now();
    this.log("Page settled, updated relevance baseline");

    const relevantCount = this.getRelevantRequestCount();

    if (relevantCount === 0) {
      // No relevant requests after settle. Prefer a smooth finish over an abrupt reset.
      if (this.state === "crawling") {
        if (!this.isFinishPending) {
          const gracePeriod = this._calculateGracePeriod();
          this.log(
            `No relevant requests after settle. Waiting ${gracePeriod}ms to finish.`,
          );
          this.isFinishPending = true;
          this.finishTimeoutId = this.timeoutManager.setTimeout(
            () => {
              this.finish();
            },
            gracePeriod,
            "finish",
          );
        } else {
          this.log("Finish already pending after settle");
        }
      } else {
        // If not crawling, nothing to animate; ensure a clean idle state.
        if (this.state !== "idle") {
          this.reset();
        }
      }
    } else {
      // Some requests are still relevant after settle
      this.log(
        `${relevantCount} requests still relevant after settle, continuing animation`,
      );

      // Don't interrupt finishing animations - let them complete naturally
      if (this.state !== "finishing") {
        // Cancel only any pending finish from before the settle
        this._cancelFinishIfPending();

        // Ensure we're in the crawling state for the continuing requests
        if (this.state === "idle") {
          this.startCrawling();
        }

        // Make sure cleanup remains scheduled
        this._scheduleCleanup();
      } else {
        this.log(
          "Indicator is finishing, allowing animation to complete naturally",
        );
      }
    }
  }

  /**
   * Initiates the progress animation.
   *
   * This method sets the state to "crawling" and delegates the visual
   * animation to the animation controller.
   */
  startCrawling() {
    this.log("Starting crawl");
    this.state = "crawling";
    this.animationController.startCrawling();
    this._startDebugPolling();
  }

  /**
   * Initiates the completion sequence.
   *
   * This method stops the progress animation and animates the bar off screen.
   */
  finish() {
    if (this.state === "finishing") {
      return;
    }

    this.log("Finishing");
    this.state = "finishing";
    this.isFinishPending = false;
    this.finishTimeoutId = null;

    this.animationController.finishAndHide(() => {
      this.finishComplete();
    });
  }

  /**
   * Handles the completion of a finish sequence.
   *
   * This method checks for queued requests and either starts a new loading
   * cycle or resets to idle state. Queued requests are now preserved in the
   * tracking map until this point, ensuring they trigger a new cycle even if
   * they completed during the finishing animation.
   */
  finishComplete() {
    this.log("Finish sequence completed");

    const queuedCount = this.getQueuedRequestCount();
    if (queuedCount > 0) {
      this.log(`Processing ${queuedCount} queued requests`);

      // Promote all queued requests to active status.
      this.requests.forEach((data) => {
        if (data.isQueued) {
          data.isQueued = false;
        }
      });

      // After promoting, check if any of these requests are still running.
      let incompleteCount = 0;
      const requestsToDelete = [];
      this.requests.forEach((data, xhr) => {
        if (data.isCompleted) {
          requestsToDelete.push(xhr);
        } else {
          incompleteCount++;
        }
      });

      // If there are still active (incomplete) requests, start a normal
      // loading cycle for them.
      if (incompleteCount > 0) {
        this.log(
          `Found ${incompleteCount} active requests from queue. Starting new crawl.`,
        );
        // Clean up only the completed requests.
        requestsToDelete.forEach((xhr) => this.requests.delete(xhr));
        this.state = "idle";
        this.startCrawling();

        // Ensure cleanup is scheduled for the new cycle.
        this._scheduleCleanup();
      } else {
        // All queued requests were already complete.
        this.log(
          "All queued requests were already complete. Cleaning up and checking for other active requests.",
        );
        // Clean up all the now-processed requests.
        requestsToDelete.forEach((xhr) => this.requests.delete(xhr));

        // Check if there are any other relevant requests still active
        const relevantCount = this.getRelevantRequestCount();
        if (relevantCount > 0) {
          this.log(
            `Found ${relevantCount} other active requests, starting new crawl.`,
          );
          this.state = "idle";
          this.startCrawling();

          // Ensure cleanup is scheduled for the new cycle.
          this._scheduleCleanup();
        } else {
          this.log("No active requests found, resetting to idle.");
          this.reset();
        }
      }
    } else {
      // No queued requests, so it's safe to reset to idle.
      this.reset();
    }
  }
}

/**
 * The singleton instance of the loading indicator controller.
 * @type {HtmxLoadingIndicator|null}
 */
let loadingIndicator = null;

/**
 * A flag to track whether global event listeners have been set up.
 * @type {boolean}
 */
let areEventListenersSetup = false;

/**
 * Sets up global event listeners for htmx request events.
 *
 * This function is called once, and the listeners work with the current
 * loadingIndicator instance.
 */
function setupGlobalEventListeners() {
  if (areEventListenersSetup) return;
  areEventListenersSetup = true;

  const on = (evt, fn) => document.addEventListener(evt, fn);

  // Track request starts at both the high-level (beforeRequest) and low-level XHR layer.
  on("htmx:beforeRequest", (e) => {
    if (loadingIndicator) {
      loadingIndicator.onRequestStart(e);
    }
  });
  on("htmx:xhr:loadstart", (e) => {
    if (loadingIndicator) {
      loadingIndicator.onRequestStart(e);
    }
  });

  // Track request ends across all known end conditions:
  // - Generic end of request lifecycle (afterRequest)
  // - Network/post-send errors (sendError)
  // - Timeout (timeout)
  // - Abort signals (sendAbort and xhr:abort)
  // - XHR load end (xhr:loadend) — authoritative end of the XHR lifecycle
  // - Errors during onload processing (onLoadError)
  [
    "htmx:afterRequest",
    "htmx:sendError",
    "htmx:timeout",
    "htmx:sendAbort",
    "htmx:responseError",
    "htmx:onLoadError",
    "htmx:xhr:loadend",
    "htmx:xhr:abort",
  ].forEach((evt) =>
    on(evt, (e) => {
      if (loadingIndicator) {
        loadingIndicator.onRequestEnd(e);
      }
    }),
  );

  // Handles browser back/forward navigation.
  on("htmx:historyRestore", () => {
    if (loadingIndicator) {
      loadingIndicator.log(
        "Browser navigation detected; resetting all loading state",
      );
      loadingIndicator.reset();
    }
  });
}

/**
 * Initializes or manages the loading indicator instance lifecycle.
 *
 * This function is called on initial page load and after htmx-driven
 * navigations. If the indicator element, identified by `INDICATOR_ID`, is not
 * found, any existing instance is cleaned up. If the element is found, a new
 * instance is created. If an instance already exists for that element, it
 * handles the page settlement.
 */
function syncIndicatorLifecycle() {
  const el = document.getElementById(INDICATOR_ID);

  if (!el) {
    if (loadingIndicator) {
      loadingIndicator.reset();
      loadingIndicator.log("Detached: indicator element not found on page.");
      loadingIndicator = null;
    }
    return;
  }

  if (!loadingIndicator || loadingIndicator.element !== el) {
    if (loadingIndicator) {
      loadingIndicator.reset();
    }
    loadingIndicator = new HtmxLoadingIndicator(el);
  } else {
    loadingIndicator.log("Page settlement detected");
    loadingIndicator.onPageSettle();
  }
}

// `DOMContentLoaded` handles the initial page load.
document.addEventListener("DOMContentLoaded", () => {
  setupGlobalEventListeners();
  syncIndicatorLifecycle();
});

// `htmx:afterSettle` handles subsequent htmx-driven page navigations.
document.addEventListener("htmx:afterSettle", (evt) => {
  if (evt.detail.elt === document.body) {
    syncIndicatorLifecycle();
  }
});

// htmx.logAll();
