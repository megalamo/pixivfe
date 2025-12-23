// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"maps"
	"net/http"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

const (
	// cloudflareChallengesURL is the URL from which external Turnstile resources are served.
	cloudflareChallengesURL = "https://challenges.cloudflare.com"

	// dynamicCSPDirectivesCount is the number of additional CSP directives added in buildCSP.
	dynamicCSPDirectivesCount = 6
)

var (
	// baseHeaders defines the default headers to be set in responses.
	//
	// Pixivfe-Version and Pixivfe-Revision are added dynamically in SetResponseHeaders.
	//
	// NOTE: we intentionally don't set CORP or HSTS headers.
	baseHeaders = http.Header{
		"Referrer-Policy":        {"no-referrer"},
		"X-Frame-Options":        {"DENY"},
		"X-Content-Type-Options": {"nosniff"},
		"Permissions-Policy":     {strings.Join(defaultPermissionsPolicy, ", ")},
		"X-Powered-By":           {"hatsune miku"},
	}

	// baseCSP defines static CSP directives that don't change.
	baseCSP = []string{
		"base-uri 'self'",
		"default-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"font-src 'self'",
		"connect-src 'self'",
		"frame-ancestors 'none'",
	}

	// defaultPermissionsPolicy defines the default Permissions-Policy header.
	defaultPermissionsPolicy = []string{
		"accelerometer=()",
		"ambient-light-sensor=()",
		"battery=()",
		"camera=()",
		"display-capture=()",
		"document-domain=()",
		"encrypted-media=()",
		"execution-while-not-rendered=()",
		"execution-while-out-of-viewport=()",
		"geolocation=()",
		"gyroscope=()",
		"magnetometer=()",
		"microphone=()",
		"midi=()",
		"navigation-override=()",
		"payment=()",
		"publickey-credentials-get=()",
		"screen-wake-lock=()",
		"sync-xhr=()",
		"usb=()",
		"web-share=()",
		"xr-spatial-tracking=()",
	}
)

// SetResponseHeaders adds default headers to HTTP responses.
func SetResponseHeaders(w http.ResponseWriter, r *http.Request, next http.Handler) {
	headers := w.Header()

	maps.Insert(headers, maps.All(baseHeaders))

	if config.Global.Development.InDevelopment {
		invalidateCacheInDevelopment(headers)
	}

	setCacheControl(headers, r.URL.Path)

	headers.Set("Pixivfe-Version", config.BuildVersion)
	headers.Set("Pixivfe-Revision", config.Global.Build.Revision())
	headers.Set("Content-Security-Policy", buildCSP(r))

	next.ServeHTTP(w, r)
}

// for `invalidateCache`
var firstDevResponse = true

// clear cache in development
func invalidateCacheInDevelopment(headers http.Header) {
	if firstDevResponse {
		firstDevResponse = false

		headers.Set("Clear-Site-Data", "cache")
	}
}

// setCacheControl sets appropriate cache control headers for static assets.
func setCacheControl(headers http.Header, path string) {
	// Default to only storing in the browser cache and forcing revalidation
	cacheDuration := "private, no-cache"

	// Longer caching for fonts and icons (1 month)
	if strings.HasPrefix(path, "/fonts/") || strings.HasPrefix(path, "/icons/") {
		cacheDuration = "max-age=2592000"
	}

	// JavaScript and CSS get a moderate cache time (1 week)
	if strings.HasPrefix(path, "/js/") || strings.HasPrefix(path, "/css/") {
		cacheDuration = "max-age=604800"
	}

	// Images can be cached for 2 weeks
	if strings.HasPrefix(path, "/img/") {
		cacheDuration = "max-age=1209600"
	}

	// Text files (robots.txt) and JSON files (manifest.json) get moderate caching (1 day)
	if strings.HasSuffix(path, ".txt") || strings.HasSuffix(path, ".json") {
		cacheDuration = "max-age=86400"
	}

	headers.Set("Cache-Control", cacheDuration)
}

func buildCSP(r *http.Request) string {
	directives := make([]string, len(baseCSP), len(baseCSP)+dynamicCSPDirectivesCount)
	copy(directives, baseCSP)

	// Extract origins from session
	imgOrigins := strings.TrimSpace(utils.GetOriginFromURL(untrusted.GetImageProxy(r))) + " " +
		utils.GetOriginFromURL(untrusted.GetStaticProxy(r)) + " " + utils.GetOriginFromURL(untrusted.GetUgoiraProxy(r))
	mediaOrigins := utils.GetOriginFromURL(untrusted.GetUgoiraProxy(r))

	// Build and append dynamic directives
	scriptSrc := "script-src 'self'"
	scriptSrcElem := "script-src-elem 'self'"
	frameSrc := "frame-src 'self'"

	if config.Global.Limiter.DetectionMethod == config.Turnstile {
		scriptSrc += " " + cloudflareChallengesURL
		scriptSrcElem += " " + cloudflareChallengesURL // Not noted in the official docs, but required
		frameSrc += " " + cloudflareChallengesURL
	}

	imgSrc := "img-src 'self' data:"
	if imgOrigins != "" {
		imgSrc += " " + imgOrigins
	}

	mediaSrc := "media-src 'self'"
	if mediaOrigins != "" {
		mediaSrc += " " + mediaOrigins
	}

	formAction := "form-action 'self'"
	if mediaOrigins != "" {
		formAction += " " + mediaOrigins
	}

	directives = append(directives, scriptSrc, scriptSrcElem, imgSrc, mediaSrc, frameSrc, formAction)

	return strings.Join(directives, "; ") + ";"
}
