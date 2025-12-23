// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package limiter

import (
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/server/routes"
)

// Rate limiting header names.
//
// ref: https://www.ietf.org/archive/id/draft-polli-ratelimit-headers-02.html
const (
	HeaderRateLimitLimit     string = "RateLimit-Limit" // This is intended.
	HeaderRateLimitRemaining string = "RateLimit-Remaining"
	HeaderRateLimitReset     string = "RateLimit-Reset"
	HeaderRateLimitStatus    string = "RateLimit-Status" // Non-standard.
)

// excludedPaths won't have traffic filtered by the limiter middleware.
var excludedPaths = []string{
	"/limiter/", // CSS token endpoint or Turnstile verification endpoint.
	"/proxy/",   // Content proxy endpoints.
	"/about",
	"/css/",
	"/fonts/",
	"/icons/",
	"/img/",
	"/js/",
	"/manifest.json",
	"/robots.txt",
}

// headerCheckExcludedPaths won't have header checks applied by the limiter middleware.
var headerCheckExcludedPaths = []string{
	"/atom.xml", // Atom feed endpoints should not have header checks.
}

// isAtomXMLPath returns true if the request path is an atom.xml route.
func isAtomXMLPath(path string) bool {
	return strings.Contains(path, "/atom.xml")
}

// Evaluate is the entrypoint to the limiter middleware.
//
// The logic was originally based on the reference SearXNG code in searxng/searx/botdetection.
//
// Implementation notes:
//   - In the original SearXNG implementation, the HTTP header checks only occur for /search requests,
//     but here we do them for all requests as we have far more endpoints to protect (/artworks, /users, etc.);
//     better to ennumerate goodness via excluded paths than badness.
func Evaluate(w http.ResponseWriter, r *http.Request, next http.Handler) {
	defer DoCleanup()

	client, err := newClientInfo(r)
	if client == nil || err != nil {
		// newClient has already written an error response.
		return
	}

	// 1: Fast-path exclusions - check if the path is completely exempt from filtering.
	if client.isFullyExcludedPath(r) {
		next.ServeHTTP(w, r)

		return
	}

	// 2: IP-based filtering - explicit allow/deny lists take precedence.
	if allowed, blocked := client.checkIPLists(); allowed {
		log.Info().
			Str("ip", client.ip.String()).
			Str("network", client.network.String()).
			Msg("Request allowed, IP in pass-list")
		next.ServeHTTP(w, r)

		return
	} else if blocked {
		log.Warn().
			Str("ip", client.ip.String()).
			Str("network", client.network.String()).
			Msg("Request blocked, IP in block-list")

		routes.BlockPage(w, routes.BlockData{Reason: "IP in block-list"}, http.StatusForbidden)

		return
	}

	// 3: Local network filtering (optional based on configuration).
	if !config.Global.Limiter.FilterLocal && client.isLocalLink() {
		next.ServeHTTP(w, r)

		return
	}

	// 4: Check request headers (conditionally).
	if config.Global.Limiter.CheckHeaders && !client.isHeaderCheckExcludedPath(r) {
		if blockReason := client.blockedByHeaders(r); blockReason != "" {
			log.Warn().
				Str("ip", client.ip.String()).
				Str("network", client.network.String()).
				Str("reason", blockReason).
				Msg("Request blocked, headers")

			routes.BlockPage(w, routes.BlockData{Reason: blockReason}, http.StatusForbidden)

			return
		}
	}

	// 5: Special handling for atom.xml routes.
	if isAtomXMLPath(r.URL.Path) {
		// For atom.xml routes, use unconditional intermediate token bucket config.
		client.limiter = getOrCreateAtomXMLLimiter(client.network.String())
	} else {
		// 5: DetectionMethod handling for non-atom.xml routes.
		detectionMethod := config.Global.Limiter.DetectionMethod

		switch detectionMethod {
		case config.LinkToken, config.Turnstile:
			// For both link token and Turnstile, the client is expected to present a valid pingCookieName cookie.
			// Always redirect to the dedicated challenge page if the client failed to provide a valid cookie.
			if !client.validatePingCookie(r) {
				log.Warn().
					Str("ip", client.ip.String()).
					Str("network", client.network.String()).
					Str("method", string(detectionMethod)).
					Msg("Redirecting to challenge page, missing or invalid ping cookie")

				// Redirect to the challenge page.
				w.Header().Set("Cache-Control", "no-store")
				setVaryHeaders(w)
				http.Redirect(w, r, "/limiter/challenge?"+returnPathFormat+"="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)

				return
			}

			// Client passed cookie validation.
		case config.None:
			// No specific challenge method. Always treat clients as non-suspicious.
			client.clearSuspiciousStatus()

			client.limiter = getOrCreateLimiter(client.network.String(), client.isSuspicious)
			updateNetworkHistory(client.limiter, client.network.String(), client.isSuspicious)
		}
	}
	// At this point:
	// - client.isSuspicious is set according to the detection method outcome.
	// - client.limiter is initialized with rates corresponding to the suspicious status.

	// 6: Rate limiting - apply limits based on client's (suspicious) status.
	if blockReason := checkRateLimit(client.limiter, client.network.String()); blockReason != "" {
		log.Warn().
			Str("ip", client.ip.String()).
			Str("network", client.network.String()).
			Bool("suspicious", client.isSuspicious).
			Str("reason", blockReason).
			Msg("Request blocked, exceeded rate limit")
		addRateLimitHeaders(w, client)

		routes.BlockPage(w, routes.BlockData{Reason: blockReason}, http.StatusTooManyRequests)

		return
	}

	// All checks passed - serve the request.
	addRateLimitHeaders(w, client)
	setVaryHeaders(w)
	next.ServeHTTP(w, r)
}

// addRateLimitHeaders adds rate limiting information to the response headers.
func addRateLimitHeaders(w http.ResponseWriter, client *ClientInfo) {
	if client == nil || client.limiter == nil {
		return
	}

	client.limiter.mu.Lock()
	defer client.limiter.mu.Unlock()

	limiter := client.limiter.limiter

	// Get current tokens and limit info.
	currentTokens := limiter.Tokens()
	burst := limiter.Burst()
	limit := limiter.Limit()

	// Calculate tokens remaining (can't exceed burst).
	remaining := int(math.Min(float64(burst), currentTokens))

	// Calculate seconds until full bucket replenishment (if not already full).
	var resetTime int64

	if currentTokens < float64(burst) {
		tokenDeficit := float64(burst) - currentTokens
		if limit > 0 {
			resetTime = int64(math.Ceil(tokenDeficit / float64(limit)))
		}
	}

	// Add Rate-Limit headers.
	resetStr := strconv.FormatInt(resetTime, 10)

	w.Header().Set(HeaderRateLimitLimit, strconv.Itoa(burst))
	w.Header().Set(HeaderRateLimitRemaining, strconv.Itoa(remaining))
	w.Header().Set(HeaderRateLimitReset, resetStr)

	// Add Retry-After header if rate limited (remaining = 0).
	// Retry-After should be seconds.
	if remaining <= 0 && resetTime >= 0 {
		w.Header().Set("Retry-After", resetStr)
	}

	// Add status headers.
	var statusValue string
	if client.isSuspicious {
		statusValue = "Suspicious"
	} else {
		statusValue = "Normal"
	}

	w.Header().Set(HeaderRateLimitStatus, statusValue)
}
