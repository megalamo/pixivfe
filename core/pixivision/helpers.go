// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
Helpers for core pixivision code
*/
package pixivision

import (
	"fmt"
	"regexp"
)

const defaultLanguage = "en" // defaultLanguage defines the default language used for pixivision requests.

// generatePixivisionURL creates a URL for pixivision based on the route and language.
func generatePixivisionURL(route string, lang []string) string {
	template := "https://www.pixivision.net/%s/%s"
	language := defaultLanguage
	availableLangs := []string{"en", "zh", "ja", "zh-tw", "ko"}

	// Validate and set the language if provided
	if len(lang) > 0 {
		t := lang[0]

		for _, i := range availableLangs {
			if t == i {
				language = t
			}
		}
	}

	return fmt.Sprintf(template, language, route)
}

// langRegexp is a regular expression to extract the language code from a URL.
var langRegexp = regexp.MustCompile(`.*\/\/.*?\/(.*?)\/`)

// determineUserLang determines the language to use for the user_lang cookie.
func determineUserLang(url string, lang ...string) string {
	// Check if language is provided in parameters
	if len(lang) > 0 && lang[0] != "" {
		return lang[0]
	}

	// Try to extract language from URL
	matches := langRegexp.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}

	// Fallback to default language
	return defaultLanguage
}

// idRegexp is a regular expression to extract the ID from a pixiv URL.
var idRegexp = regexp.MustCompile(`.*\/(\d+)`)

// parseIDFromPixivLink extracts the numeric ID from a pixiv URL.
func parseIDFromPixivLink(link string) string {
	const expectedMatchCount = 2

	matches := idRegexp.FindStringSubmatch(link)

	// Check if the regex found a match AND captured the group
	// (should always be 2 elements if matched)
	if len(matches) < expectedMatchCount {
		return ""
	}

	// If we have at least 2 elements, index 1 contains the captured digits
	return matches[1]
}

// imgRegexp is a regular expression to extract the image URL from a CSS background-image property.
var imgRegexp = regexp.MustCompile(`background-image:\s*url\(([^)]+)\)`)

// parseBackgroundImage extracts the image URL from a CSS background-image property.
func parseBackgroundImage(link string) string {
	const expectedMatchCount = 2

	matches := imgRegexp.FindStringSubmatch(link)

	if len(matches) < expectedMatchCount {
		return ""
	}

	return matches[1]
}

// Better than constructing href values in templates manually.
func normalizeHeadingLink(href string) string {
	if href == "" {
		return ""
	}

	// Strip language prefix if present (e.g. "/en/..." -> "/...")
	if len(href) >= 4 && href[0] == '/' && href[3] == '/' {
		href = "/" + href[4:]
	}

	// Add pixivision prefix
	return "/pixivision" + href
}
