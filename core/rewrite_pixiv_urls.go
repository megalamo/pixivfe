// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
URL rewriting module:
  - replace *.pixiv.net links with ours
  - replace some i.pximg.net images with WebP

Rewriting should happen whenever we get a response from pixiv.net, so it works for all proxy servers.
*/
package core

import (
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

const (
	// pathSegmentsCountDirect is for paths like "/users/ID".
	pathSegmentsCountDirect = 2

	// pathSegmentsCountWithLang is for paths with a language code like "/en/users/ID".
	pathSegmentsCountWithLang = 3
)

var (
	// sizeQualityRe is a regular expression that matches the "/c/{parameters}/" segment.
	sizeQualityRe = regexp.MustCompile(`/c/[^/]+/`)

	// baseFileRe finds the base ID (which can be numeric or alphanumeric), an optional
	// page number (e.g., _p0), an optional known suffix, and the file extension.
	// It's used to normalize the filename for master WebP conversion.
	baseFileRe = regexp.MustCompile(`([a-zA-Z0-9]+(?:_p\d+)?)(?:_(?:square|custom|master)1200)?\.(?:jpg|png|jpeg)$`)

	// pixivRedirectRegexp matches Pixiv redirect URLs (`/jump.php?...`).
	//
	// It captures the target URL component (the part after `?` up to a delimiter).
	pixivRedirectRegexp = regexp.MustCompile(`\/jump\.php\?([^"&\s]+)`)

	// absolutePixivLinkRegexp matches absolute pixiv.net URLs.
	//
	// Used to identify standalone pixiv links for potential conversion to relative paths,
	// distinct from those found within /jump.php redirects.
	absolutePixivLinkRegexp = regexp.MustCompile(`https?://(?:[a-zA-Z0-9\-]+\.)*pixiv\.net[^\s<>"']*`)

	// excludedPaths defines i.pximg.net URL patterns that should not be converted to WebP.
	excludedPaths = []string{
		"/background/",
		"/user-profile/",
		"/img-original/",
		"/illust-series-cover-original/",
		"/imgaz/upload/",
	}

	pathMarkers = []string{
		"/c/",
		"/img-master/",
		"/custom-thumb/",
		"/img-original/",
		"/novel-cover-original/",
		"/novel-cover-master/",
		"/img/",
	}
)

// RewriteEscapedImageURLs replaces image URLs with their proxied equivalents.
//
// It handles pre-escaped URL patterns (with escaped forward slashes).
//
// You want to call this when handling JSON payloads from the pixiv API.
//
// Parameters:
//   - r: The HTTP request containing proxy configuration in cookies.
//   - data: The string containing image URLs to be proxied.
//
// Returns the processed data as a slice of bytes.
func RewriteEscapedImageURLs(r *http.Request, data []byte) []byte {
	return []byte(rewriteImageURLsInternal(r, string(data), true))
}

// RewriteImageURLs replaces image URLs with their proxied equivalents.
//
// Unlike RewriteEscapedImageURLs, it handles non-escaped URL patterns.
//
// You want to call this when handling HTML payloads that embed images directly.
//
// Parameters:
//   - r: The HTTP request containing proxy configuration in cookies.
//   - data: The string containing image URLs to be proxied.
//
// Returns the processed string.
func RewriteImageURLs(r *http.Request, data string) string {
	return rewriteImageURLsInternal(r, data, false)
}

// rewriteImageURLsInternal is a helper function that handles the common logic
// for rewriting image URLs, either escaped or non-escaped.
func rewriteImageURLsInternal(r *http.Request, data string, useEscapedPatterns bool) string {
	// rules defines rewrite rules for each pixiv domain.
	// Each handler receives an unescaped URL and returns its transformed, unescaped equivalent.
	rules := []struct {
		domain  string
		handler func(url string) string
	}{
		{
			"source.pixiv.net",
			func(u string) string {
				return strings.Replace(u, "https://source.pixiv.net", "/proxy/source.pixiv.net", 1)
			},
		},
		{
			"booth.pximg.net",
			func(u string) string {
				return strings.Replace(u, "https://booth.pximg.net", "/proxy/booth.pximg.net", 1)
			},
		},
		{
			"i.pximg.net",
			func(u string) string {
				proxyBase := utils.GetProxyBase(untrusted.GetImageProxy(r))

				// Handle image URLs that should not be converted to WebP.
				for _, excludedPath := range excludedPaths {
					if strings.Contains(u, excludedPath) {
						return strings.Replace(u, "https://i.pximg.net", proxyBase, 1)
					}
				}

				return generateMasterWebpURL(u, proxyBase)
			},
		},
		{
			"s.pximg.net",
			func(u string) string {
				return strings.Replace(u, "https://s.pximg.net", utils.GetProxyBase(untrusted.GetStaticProxy(r)), 1)
			},
		},
	}

	protocolPart := `https://`
	endCharClass := `[^\s"'>\]]*`

	if useEscapedPatterns {
		protocolPart = `https:\\?/\\?/`
		endCharClass = `[^\s"'}\]]*`
	}

	result := data

	// Apply each rule to the data.
	for _, rule := range rules {
		// Compiling the regex here is acceptable since there are few rules
		// and this function is called once per response body.
		pattern := regexp.MustCompile(protocolPart + strings.ReplaceAll(rule.domain, ".", `\.`) + endCharClass)

		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// 1. Unescape the found URL if necessary.
			processedURL := match
			if useEscapedPatterns {
				processedURL = strings.ReplaceAll(processedURL, `\/`, `/`)
			}

			// 2. Apply the domain-specific transformation logic.
			replacementURL := rule.handler(processedURL)

			// 3. Re-escape the result if the original was escaped.
			if useEscapedPatterns {
				return strings.ReplaceAll(replacementURL, `/`, `\/`)
			}

			return replacementURL
		})
	}

	return result
}

// parseDescriptionURLs processes a description to convert Pixiv URLs to relative paths.
//
// It handles both /jump.php redirect URLs and standalone absolute pixiv.net URLs.
// This function should be called on description/comment fields before passing data to templates.
func parseDescriptionURLs(description string) string {
	// First, process /jump.php? redirect URLs.
	result := pixivRedirectRegexp.ReplaceAllStringFunc(description, processPixivRedirectMatch)

	// Second, process standalone absolute pixiv.net URLs that might not have been in redirects.
	// This ensures direct pixiv links (not via jump.php) are also considered for relativization.
	result = absolutePixivLinkRegexp.ReplaceAllStringFunc(result, func(match string) string {
		// tryMakePixivURLRelative attempts the conversion based on defined patterns.
		if relativeURL, converted := tryMakePixivURLRelative(match); converted {
			return relativeURL
		}

		// If not converted (e.g., it's https://www.pixiv.net/home), keep the original absolute URL.
		return match
	})

	return result
}

// tryMakePixivURLRelative attempts to convert a full pixiv.net URL string to a relative path
// if it matches specific patterns (users, artworks, novels).
//
// It returns the new URL string and a boolean indicating if a conversion occurred.
//
// If the input is not an HTTP/HTTPS pixiv.net URL eligible for conversion,
// it returns the original string and false.
func tryMakePixivURLRelative(fullURLString string) (string, bool) {
	parsedTargetURL, err := url.Parse(fullURLString)
	if err != nil {
		// Malformed URLs cannot be processed.
		return fullURLString, false
	}

	// Only http/https schemes are considered for relativization.
	if parsedTargetURL.Scheme != "http" && parsedTargetURL.Scheme != "https" {
		return fullURLString, false
	}

	if !strings.Contains(parsedTargetURL.Host, "pixiv.net") {
		// Not a pixiv.net URL.
		return fullURLString, false
	}

	// At this point, it's a valid http/https pixiv.net URL.
	// We also clean the path to handle any extraneous slashes.
	cleanedPath := path.Clean(parsedTargetURL.Path)
	query := parsedTargetURL.Query()

	// Handle novel URLs: /novel/show.php?id=... or /lang/novel/show.php?id=...
	// These are converted to /novels/ID.
	if strings.Contains(cleanedPath, "/novel/show.php") {
		if id := query.Get("id"); id != "" {
			return "/novel/" + id, true
		}
	}

	// Handle user/artwork paths: /users/ID, /artworks/ID, /lang/users/ID, /lang/artworks/ID.
	// These are converted to /users/ID or /artworks/ID.
	trimmedPath := strings.TrimPrefix(cleanedPath, "/")
	pathParts := strings.Split(trimmedPath, "/")

	// After cleaning and trimming, if original path was "/" or empty,
	// trimmedPath might be empty or ".", leading to pathParts like [""] or ["."].
	// These won't match lengths 2 or 3, correctly falling through.

	if len(pathParts) == pathSegmentsCountDirect { // e.g., "users", "123"
		key := pathParts[0]

		id := pathParts[1]
		if (key == "users" || key == "artworks") && id != "" { // Ensure ID is present.
			return "/" + key + "/" + id, true
		}
	}

	if len(pathParts) == pathSegmentsCountWithLang { // e.g., "en", "users", "123"
		// pathParts[0] is lang code (e.g., "en"), pathParts[1] is key, pathParts[2] is ID.
		key := pathParts[1]

		id := pathParts[2]
		if (key == "users" || key == "artworks") && id != "" { // Ensure ID is present.
			return "/" + key + "/" + id, true // Standardize to relative path, dropping lang code.
		}
	}

	// Matched a pixiv.net URL, but not a pattern we convert to relative
	// (e.g., pixiv.net/home, pixiv.net/ranking.php).
	return fullURLString, false
}

// processPixivRedirectMatch is a helper for pixivRedirectRegex.ReplaceAllStringFunc.
//
// It extracts the target URL from a /jump.php? redirect, URL-decodes it,
// attempts to convert known pixiv URLs to relative paths (via tryMakePixivURLRelative),
// and sanitizes `javascript:` URIs to an empty string. Other URLs are returned decoded.
func processPixivRedirectMatch(match string) string {
	// The match must be like "/jump.php?URL_PARAMS". Ensure query part exists.
	if len(match) <= 10 || !strings.HasPrefix(match, "/jump.php?") {
		// Not a valid jump link structure or empty query; return original match.
		return match
	}

	encodedURL := match[10:] // Extract the part after "/jump.php?"

	decodedURL, err := url.QueryUnescape(encodedURL)
	if err != nil {
		// Unescape failed; return original jump.php match as a fallback.
		return match
	}

	// Attempt to make it a relative pixiv URL if applicable.
	if relativeURL, converted := tryMakePixivURLRelative(decodedURL); converted {
		return relativeURL
	}

	// If not converted by tryMakePixivURLRelative, the decodedURL could be:
	// - An external HTTP/S link.
	// - A pixiv URL not matching relativization patterns (e.g., pixiv.net/home).
	// - A URL with a non-HTTP/S scheme (e.g., mailto:, javascript:).
	// - A malformed URL.
	// We need to parse it to specifically check for and sanitize `javascript:` URIs.
	parsedTargetURL, err := url.Parse(decodedURL)
	if err != nil {
		// If decodedURL itself is malformed and cannot be parsed (e.g., "http://["),
		// return the decoded string. It couldn't be made relative or sanitized by scheme.
		return decodedURL
	}

	if strings.ToLower(parsedTargetURL.Scheme) == "javascript" {
		return "" // Sanitize javascript: URIs by returning an empty string.
	}

	// For all other cases (non-convertible pixiv URLs, external links, mailto:, ftp:, etc.),
	// return the decoded URL.
	return decodedURL
}
