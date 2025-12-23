// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core_test

import (
	"net/url"
	"reflect"
	"testing"

	"codeberg.org/pixivfe/pixivfe/v3/core"
)

// TestGetArtworkSearchURL provides tests for GetArtworkSearchURL.
func TestGetArtworkSearchURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		settings core.WorkSearchSettings
		want     string
		wantErr  bool
	}{
		{
			name: "error on empty name parameter",
			settings: core.WorkSearchSettings{
				Name:     "",
				Category: "illustrations",
			},
			// A search term must be provided. An empty string is invalid input
			// and should result in an error.
			wantErr: true,
		},
		{
			name: "error on empty category parameter",
			settings: core.WorkSearchSettings{
				Name:     "cat",
				Category: "",
			},
			// A search category must be provided. An empty string would produce a
			// malformed URL path and must be rejected with an error.
			wantErr: true,
		},
		{
			name: "category with special path characters",
			settings: core.WorkSearchSettings{
				Name:     "test",
				Category: "illustrations/../artworks",
			},
			// The category input should be path-escaped, just like the search name.
			want:    "https://www.pixiv.net/ajax/search/illustrations%2F..%2Fartworks/test?word=test",
			wantErr: false,
		},
		{
			name: "name is already query-encoded",
			settings: core.WorkSearchSettings{
				Name:     "spaced%20out", // A literal string containing a percent sign.
				Category: "artworks",
			},
			// The function should not try to unescape input. It should treat the input
			// literally and encode it to prevent double-encoding.
			// The '%' character must be escaped to '%25'.
			want:    "https://www.pixiv.net/ajax/search/artworks/spaced%2520out?word=spaced%2520out",
			wantErr: false,
		},
		{
			name: "name contains a plus sign",
			settings: core.WorkSearchSettings{
				Name:     "C++",
				Category: "artworks",
			},
			// The '+' character has a special meaning in query strings (space) and should
			// be escaped to '%2B' to preserve the literal input.
			// Unescaping first would corrupt "C++" into "C  ".
			want:    "https://www.pixiv.net/ajax/search/artworks/C%2B%2B?word=C%2B%2B",
			wantErr: false,
		},
		{
			name: "optional parameter is an empty string",
			settings: core.WorkSearchSettings{
				Name:     "test",
				Category: "artworks",
				Mode:     "safe",
				Order:    "", // Explicitly test an empty optional param
			},
			// The 'order' parameter should be completely omitted from the final query string.
			want:    "https://www.pixiv.net/ajax/search/artworks/test?mode=safe&word=test",
			wantErr: false,
		},
		{
			name: "basic search with minimal settings",
			settings: core.WorkSearchSettings{
				Name:     "cat",
				Category: "illustrations",
				Page:     "1",
				Order:    "date_d",
				Mode:     "safe",
			},
			// NOTE: url.Values.Encode sorts query keys alphabetically.
			want:    "https://www.pixiv.net/ajax/search/illustrations/cat?mode=safe&order=date_d&p=1&word=cat",
			wantErr: false,
		},
		{
			name: "search with all parameters filled",
			settings: core.WorkSearchSettings{
				Name:     "full test",
				Category: "manga",
				Order:    "popular_d",
				Mode:     "r18",
				Ratio:    "-0.5",
				Page:     "42",
				Smode:    "s_tag",
				Wlt:      "1000",
				Wgt:      "2000",
				Hlt:      "800",
				Hgt:      "1600",
				Tool:     "CLIP STUDIO PAINT",
				Scd:      "2023-01-01",
				Ecd:      "2023-12-31",
			},
			// Query keys are sorted alphabetically for a predictable 'want' string.
			want:    "https://www.pixiv.net/ajax/search/manga/full%20test?ecd=2023-12-31&hgt=1600&hlt=800&mode=r18&order=popular_d&p=42&ratio=-0.5&s_mode=s_tag&scd=2023-01-01&tool=CLIP+STUDIO+PAINT&wgt=2000&wlt=1000&word=full+test",
			wantErr: false,
		},
		{
			name: "search with special characters in name",
			settings: core.WorkSearchSettings{
				Name:     "東方Project/靈夢",
				Category: "artworks",
				Page:     "2",
			},
			// The '/' in the name must be path-escaped to form a single segment.
			// The query parameter 'word' must also correctly escape it.
			want:    "https://www.pixiv.net/ajax/search/artworks/%E6%9D%B1%E6%96%B9Project%2F%E9%9D%88%E5%A4%A2?p=2&word=%E6%9D%B1%E6%96%B9Project%2F%E9%9D%88%E5%A4%A2",
			wantErr: false,
		},
		{
			name: "search with spaces and other url-unfriendly chars in name",
			settings: core.WorkSearchSettings{
				Name:     "search? for a tag&value",
				Category: "artworks",
				Page:     "1",
			},
			// Correctly demonstrates different encoding for path (%20) vs. query (+).
			want:    "https://www.pixiv.net/ajax/search/artworks/search%3F%20for%20a%20tag%26value?p=1&word=search%3F+for+a+tag%26value",
			wantErr: false,
		},
		{
			name: "search with leading and trailing whitespace",
			settings: core.WorkSearchSettings{
				Name:     "  spaced out  ",
				Category: "illustrations",
			},
			// Whitespace should be preserved and encoded, not trimmed.
			// Path encodes space as %20, query encodes as +.
			want:    "https://www.pixiv.net/ajax/search/illustrations/%20%20spaced%20out%20%20?word=++spaced+out++",
			wantErr: false,
		},
		{
			name: "search with parameter value containing only whitespace",
			settings: core.WorkSearchSettings{
				Name:     "tag",
				Category: "artworks",
				Mode:     "  ", // Whitespace-only value
			},
			// A non-empty string should be included, even if it's only whitespace.
			// The 'mode' value of "  " becomes "++" in the query string.
			want:    "https://www.pixiv.net/ajax/search/artworks/tag?mode=++&word=tag",
			wantErr: false,
		},
		{
			name: "search with unicode in category",
			settings: core.WorkSearchSettings{
				Name:     "Reimu",
				Category: "東方", // Tōhō
			},
			// The category, like the name, must be properly path-escaped.
			want:    "https://www.pixiv.net/ajax/search/%E6%9D%B1%E6%96%B9/Reimu?word=Reimu",
			wantErr: false,
		},
	}

	for _, tt := range testCases {
		// Run each test case as a sub-test.
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := core.GetArtworkSearchURL(tt.settings)

			// Check if the error status matches the expectation.
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Fatalf("GetArtworkSearchURL() expected an error, but got nil")
				} else {
					t.Fatalf("GetArtworkSearchURL() returned an unexpected error: %v", err)
				}
			}

			// If an error was expected, we're done with this test case.
			if tt.wantErr {
				return
			}

			// To robustly compare URLs, parse both and compare their components.
			// This avoids brittle tests that fail due to query parameter reordering.
			gotURL, err := url.Parse(got)
			if err != nil {
				t.Fatalf("Failed to parse the generated URL: %v", err)
			}

			wantURL, err := url.Parse(tt.want)
			if err != nil {
				t.Fatalf("Failed to parse the expected URL: %v", err)
			}

			// Compare the URL parts that should be identical.
			if gotURL.Scheme != wantURL.Scheme {
				t.Errorf("Scheme mismatch: got %q, want %q", gotURL.Scheme, wantURL.Scheme)
			}

			if gotURL.Host != wantURL.Host {
				t.Errorf("Host mismatch: got %q, want %q", gotURL.Host, wantURL.Host)
			}

			// Using EscapedPath ensures that characters like '/' are correctly
			// checked for their escaped state, which .Path would otherwise decode.
			if gotURL.EscapedPath() != wantURL.EscapedPath() {
				t.Errorf("Path mismatch:\ngot:  %q\nwant: %q", gotURL.EscapedPath(), wantURL.EscapedPath())
			}

			// Compare the parsed query parameters using reflect.DeepEqual.
			gotQuery := gotURL.Query()

			wantQuery := wantURL.Query()
			if !reflect.DeepEqual(gotQuery, wantQuery) {
				t.Errorf("Query parameters mismatch:\ngot:  %v\nwant: %v", gotQuery, wantQuery)
			}
		})
	}
}
