// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package limiter

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// Header policy parameters.
var (
	// requiredEncodings lists content codings that a user agent must accept.
	requiredEncodings = []string{
		"identity",
		"gzip",
		"deflate",
	}

	// requiredSecFetchHeaders lists Fetch Metadata Request Headers that must be present in secure contexts (HTTPS).
	requiredSecFetchHeaders = []string{
		"Sec-Fetch-Dest",
		"Sec-Fetch-Mode",
		"Sec-Fetch-Site",
	}

	// botSubstrings lists case-insensitive substrings that identify known bots or non-browser clients.
	botSubstrings = []string{
		"abonti",
		"ahrefsbot",
		"archive.org_bot",
		"baiduspider",
		"bingbot",
		"blexbot",
		"bitlybot",
		"curl",
		"exabot",
		"farside/0.1.0",
		"feedfetcher",
		"go-http-client",
		"googlebot",
		"googleimageproxy",
		"headlesschrome",
		"httpclient",
		"jakarta",
		"james bot",
		"java",
		"javafx",
		"jersey",
		"libwww-perl",
		"linkdexbot",
		"mj12bot",
		"msnbot",
		"netvibes",
		"okhttp",
		"petalbot",
		"pixray",
		"python",
		"python-requests",
		"ruby",
		"scrapy",
		"semrushbot",
		"seznambot",
		"sogou",
		"spinn3r",
		"splash",
		"synhttpclient",
		"universalfeedparser",
		"unknown",
		"wget",
		"yahoo! slurp",
		"yacybot",
		"yandexbot",
		"yandexmobilebot",
		"zmeu",
	}
)

// mimeRule describes the Accept header requirements for a resource type.
// The zero value matches no MIME types and produces an empty error message.
type mimeRule struct {
	// required is a list of MIME type substrings that will be searched in the Accept header.
	required []string

	// errorMsg is an error message template if none of the required types are present.
	errorMsg string

	// shouldFormat reports whether errorMsg needs the list of MIME types inserted.
	shouldFormat bool
}

// Accept header policy defining required MIME types per file extension.
var (
	// defaultAcceptRule enforces text/html for HTML files or paths without a recognized extension.
	// Applies when no extension-specific rule matches.
	defaultAcceptRule = mimeRule{
		required:     []string{"text/html"},
		errorMsg:     "HTML file requires text/html Accept type",
		shouldFormat: false,
	}

	// acceptHeaderRules maps file extensions to their Accept header requirements.
	acceptHeaderRules = map[string]mimeRule{
		".js": {
			required:     []string{"application/javascript", "text/javascript"},
			errorMsg:     "JavaScript file requires JavaScript Accept type",
			shouldFormat: false,
		},
		".css": {
			required:     []string{"text/css"},
			errorMsg:     "CSS file requires text/css Accept type",
			shouldFormat: false,
		},
		// For image files, we only check for "image/" anywhere in the Accept header.
		".png": {
			required:     []string{"image/"},
			errorMsg:     "Image file requires image/* Accept type",
			shouldFormat: false,
		},
		".jpeg": {
			required:     []string{"image/"},
			errorMsg:     "Image file requires image/* Accept type",
			shouldFormat: false,
		},
		".jpg": {
			required:     []string{"image/"},
			errorMsg:     "Image file requires image/* Accept type",
			shouldFormat: false,
		},
		".gif": {
			required:     []string{"image/"},
			errorMsg:     "Image file requires image/* Accept type",
			shouldFormat: false,
		},
		".svg": {
			required:     []string{"image/"},
			errorMsg:     "Image file requires image/* Accept type",
			shouldFormat: false,
		},
		".json": {
			required:     []string{"application/json"},
			errorMsg:     "JSON file requires application/json Accept type",
			shouldFormat: false,
		},
		".txt": {
			required:     []string{"text/plain"},
			errorMsg:     "Text file requires text/plain Accept type",
			shouldFormat: false,
		},
		".map": {
			required:     []string{"text/plain"},
			errorMsg:     "Text file requires text/plain Accept type",
			shouldFormat: false,
		},
		".scss": {
			required:     []string{"text/plain"},
			errorMsg:     "Text file requires text/plain Accept type",
			shouldFormat: false,
		},
		".woff2": {
			required:     []string{"application/font-woff2", "application/font-woff", "font/woff"},
			errorMsg:     "WOFF2 font file requires Accept type: %s",
			shouldFormat: true,
		},
	}
)

// blockedByHeaders evaluates a subset of HTTP request headers and reports a block reason.
// It returns an empty string when the request appears acceptable.
func blockedByHeaders(r *http.Request) string {
	// User-Agent must be present and non-empty, and not match a known bot.
	userAgent := r.Header.Get("User-Agent")
	if userAgent == "" {
		return "Blocked by User-Agent header, missing or empty"
	}

	for _, sub := range botSubstrings {
		if strings.Contains(strings.ToLower(userAgent), sub) {
			return "Blocked by User-Agent header, known bot"
		}
	}

	// Accept must be present and non-empty.
	if reason := checkAcceptHeader(r.URL.Path, r.Header.Get("Accept")); reason != "" {
		return "Blocked by Accept header, " + reason
	}

	// Accept-Encoding must contain at least one of [requiredEncodings].
	validEncoding := false

	for _, acceptedEnc := range requiredEncodings {
		if strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), acceptedEnc) {
			validEncoding = true

			break
		}
	}

	if !validEncoding {
		return "Blocked by Accept-Encoding header"
	}

	// Accept must be present and non-empty.
	if strings.TrimSpace(r.Header.Get("Accept-Language")) == "" {
		return "Blocked by Accept-Language header"
	}

	// Sec-Fetch-* must be present and non-empty.
	// NOTE: We do not make this check in an insecure context since a real browser may not set them.
	if utils.IsConnectionSecure(r) {
		if reason := checkSecFetch(r); reason != "" {
			return reason
		}
	}

	// Connection
	//
	// This check is actually quite useless as when the application is behind a reverse proxy,
	// all incoming connections will likely be over HTTP/1.1 with Connection: keep-alive anyway
	//
	// SearXNG also disables this check, but for a different reason related to uSWGI behavior,
	// see https://github.com/searxng/searxng/issues/2892
	//
	// conn := r.Header.Get("Connection")
	// if strings.EqualFold(strings.TrimSpace(conn), "close") {
	// 	return "Blocked by Connection header"
	// }

	return ""
}

// checkAcceptHeader validates that the provided Accept header is acceptable
// for the file at the given path.
// It returns an empty string when the header satisfies the policy,
// or a message describing the first mismatch found.
func checkAcceptHeader(path, accept string) string {
	// Every type is accepted; skip further checks.
	if strings.Contains(accept, "*/*") {
		return ""
	}

	rule, ok := acceptHeaderRules[strings.ToLower(filepath.Ext(path))]
	if !ok {
		rule = defaultAcceptRule
	}

	for _, r := range rule.required {
		if strings.Contains(accept, r) {
			return ""
		}
	}

	if rule.shouldFormat {
		return fmt.Sprintf(rule.errorMsg, strings.Join(rule.required, " or "))
	}

	return rule.errorMsg
}

// checkSecFetch reports a missing Fetch Metadata header when one or more
// required headers are absent for a secure request.
// It returns an empty string when all required headers are present.
func checkSecFetch(r *http.Request) string {
	var missingHeaders []string

	for _, name := range requiredSecFetchHeaders {
		if r.Header.Get(name) == "" {
			missingHeaders = append(missingHeaders, name)
		}
	}

	switch len(missingHeaders) {
	case 0:
		return ""
	case 1:
		return "Missing " + missingHeaders[0] + " header"
	default:
		sort.Strings(missingHeaders)

		return "Missing Sec-Fetch headers: " + strings.Join(missingHeaders, ", ")
	}
}
