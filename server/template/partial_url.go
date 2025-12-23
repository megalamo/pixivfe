// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
This file provides utilities for manipulating URL paths and query parameters.
*/
package template

import (
	"fmt"
	"net/url"
	"strings"
)

// UnfinishedQuery builds a URL string by including all query parameters except a specified key,
// which it leaves empty and places at the end of the query string.
//
// Useful for preparing a URL where the value for the key is appended later (e.g. during query parameter replacement).
func UnfinishedQuery(urlStr, key string) string {
	return buildUnfinishedQuery(urlStr, key)
}

// UnfinishedQueryNoPage is a convenience wrapper around UnfinishedQuery that
// excludes both 'p' and 'page' parameters from the query string.
func UnfinishedQueryNoPage(urlStr, key string) string {
	return buildUnfinishedQuery(urlStr, key, "p", "page")
}

// buildUnfinishedQuery is a helper function that builds a URL string by including all query parameters
// except specified keys, which it leaves empty and places at the end of the query string.
func buildUnfinishedQuery(urlStr, key string, excludeKeys ...string) string {
	// Parse the URL string
	u, err := url.Parse(urlStr)
	if err != nil {
		// If parsing fails, treat the entire string as a path
		u = &url.URL{Path: urlStr}
	}

	// Create a set of keys to exclude for efficient lookup
	excludeSet := make(map[string]bool)

	excludeSet[key] = true

	for _, excludeKey := range excludeKeys {
		excludeSet[excludeKey] = true
	}

	// Start building the URL with the path
	result := u.Path
	// Flag to check if we are adding the first query parameter
	firstQueryPair := true

	// Iterate over the query parameters, excluding the specified keys
	for queryKey, queryValues := range u.Query() {
		// Lowercase the first character of the key to standardize it
		queryKey = strings.ToLower(queryKey[0:1]) + queryKey[1:]

		// Skip excluded keys
		if excludeSet[queryKey] {
			continue
		}

		// Use the first value if multiple values exist
		queryValue := ""
		if len(queryValues) > 0 {
			queryValue = queryValues[0]
		}

		// Skip parameters with empty values to avoid cluttering the URL
		if queryValue == "" {
			continue
		}

		// Add '?' before the first query parameter, '&' before subsequent ones
		if firstQueryPair {
			result += "?"

			firstQueryPair = false
		} else {
			result += "&"
		}
		// Append the key-value pair to the result
		result += fmt.Sprintf("%s=%s", queryKey, queryValue)
	}

	// Append the specified key at the end with an empty value
	// This ensures the key appears last in the query string and can be easily modified
	var queryParamSeparator string
	if firstQueryPair {
		// No query parameters were added before, so use '?'
		queryParamSeparator = "?"
	} else {
		// Query parameters were added before, so use '&'
		queryParamSeparator = "&"
	}

	result += fmt.Sprintf("%s%s=", queryParamSeparator, key)

	return result
}
