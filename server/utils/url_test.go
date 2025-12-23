// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package utils_test

import (
	"testing"

	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

func TestParseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		urlStr   string
		urlType  string
		wantErr  bool
		expected string
	}{
		{"Valid URL", "https://example.com", "Test", false, "https://example.com"},
		{"Valid URL with path", "https://example.com/path", "Test", false, "https://example.com/path"},
		{"Missing scheme", "example.com", "Test", true, ""},
		{"Missing host", "https://", "Test", true, ""},
		{"Trailing slash", "https://example.com/", "Test", false, "https://example.com"},
		{"Path with trailing slash", "https://example.com/path/", "Test", false, "https://example.com/path"},
		{"Empty URL", "", "Test", true, ""},
		{"URL with query params", "https://example.com/path?q=test", "Test", false, "https://example.com/path?q=test"},
		{"URL with fragment", "https://example.com/path#fragment", "Test", false, "https://example.com/path#fragment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := utils.ParseURL(tt.urlStr, tt.urlType)
			if (err != nil) != tt.wantErr {
				t.Errorf("utils.ValidateURL() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				if got.String() != tt.expected {
					t.Errorf("utils.ValidateURL() got = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}
