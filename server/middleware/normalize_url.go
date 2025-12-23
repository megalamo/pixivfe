// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// enPrefixPaths defines paths for which we should handle an "/en/" prefix.
var enPrefixPaths = []string{
	"users/",
	"artworks/",
	"novel/",
}

// NormalizeURL is a middleware that handles URL normalization by:
// 1. Removing trailing slashes from URLs (except root).
// 2. Removing /en/ prefix from supported paths.
func NormalizeURL(w http.ResponseWriter, r *http.Request, next http.Handler) {
	// Check for /en/ prefix first and redirect if found
	if hasEnPrefix(r) {
		removeEnPrefix(w, r)

		return
	}

	// Check for trailing slash and redirect if found
	if hasTrailingSlash(r) {
		removeTrailingSlash(w, r)

		return
	}

	// No normalization needed, continue to next handler
	next.ServeHTTP(w, r)
}

// hasTrailingSlash checks if a request path has a trailing slash (except root).
func hasTrailingSlash(r *http.Request) bool {
	return r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/")
}

// removeTrailingSlash removes trailing slash and redirects.
func removeTrailingSlash(w http.ResponseWriter, r *http.Request) {
	url := r.URL

	if len(url.Path) > 1 {
		url.Path = strings.TrimSuffix(url.Path, "/")
	}

	// @iacore: i think this won't have open redirect vuln
	http.Redirect(w, r, url.String(), http.StatusPermanentRedirect)
}

// hasEnPrefix checks if a request path starts with /en/ for supported paths.
func hasEnPrefix(r *http.Request) bool {
	path := r.URL.Path

	// Check if path starts with /en/
	if len(path) <= 4 || path[:4] != "/en/" {
		return false
	}

	// Check if what follows /en/ is one of enPrefixPaths
	for _, validPath := range enPrefixPaths {
		if strings.HasPrefix(path[4:], validPath) {
			return true
		}
	}

	return false
}

// removeEnPrefix removes the /en/ prefix from the request URL and redirects.
func removeEnPrefix(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	canonicalPath := path[3:] // Remove the '/en' part (keep the trailing slash)

	// Create new URL with the en prefix removed
	target, _ := url.Parse(r.URL.String())

	target.Path = canonicalPath

	http.Redirect(w, r, target.String(), http.StatusMovedPermanently)
}
