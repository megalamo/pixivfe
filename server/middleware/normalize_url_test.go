// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		requestURL       string
		expectedStatus   int
		expectedLocation string
		shouldRedirect   bool
	}{
		{
			name:           "Root path should not redirect",
			requestURL:     "/",
			expectedStatus: http.StatusOK,
			shouldRedirect: false,
		},
		{
			name:           "Path without trailing slash should not redirect",
			requestURL:     "/users/123",
			expectedStatus: http.StatusOK,
			shouldRedirect: false,
		},
		{
			name:             "Path with trailing slash should redirect",
			requestURL:       "/users/123/",
			expectedStatus:   http.StatusPermanentRedirect,
			expectedLocation: "/users/123",
			shouldRedirect:   true,
		},
		{
			name:             "En prefix for users should redirect",
			requestURL:       "/en/users/123",
			expectedStatus:   http.StatusMovedPermanently,
			expectedLocation: "/users/123",
			shouldRedirect:   true,
		},
		{
			name:             "En prefix for artworks should redirect",
			requestURL:       "/en/artworks/456",
			expectedStatus:   http.StatusMovedPermanently,
			expectedLocation: "/artworks/456",
			shouldRedirect:   true,
		},
		{
			name:             "En prefix for novel should redirect",
			requestURL:       "/en/novel/789",
			expectedStatus:   http.StatusMovedPermanently,
			expectedLocation: "/novel/789",
			shouldRedirect:   true,
		},
		{
			name:           "En prefix for unsupported path should not redirect",
			requestURL:     "/en/about",
			expectedStatus: http.StatusOK,
			shouldRedirect: false,
		},
		{
			name:             "En prefix with trailing slash should redirect to canonical path",
			requestURL:       "/en/users/123/",
			expectedStatus:   http.StatusMovedPermanently,
			expectedLocation: "/users/123/",
			shouldRedirect:   true,
		},
		{
			name:             "Query parameters should be preserved in trailing slash redirect",
			requestURL:       "/users/123/?page=2&sort=desc",
			expectedStatus:   http.StatusPermanentRedirect,
			expectedLocation: "/users/123?page=2&sort=desc",
			shouldRedirect:   true,
		},
		{
			name:             "Query parameters should be preserved in en prefix redirect",
			requestURL:       "/en/users/123?page=2&sort=desc",
			expectedStatus:   http.StatusMovedPermanently,
			expectedLocation: "/users/123?page=2&sort=desc",
			shouldRedirect:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create a test handler that returns 200 OK
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with our middleware
			handler := Wrap(NormalizeURL, nextHandler)

			// Create test request
			req := httptest.NewRequest(http.MethodGet, tt.requestURL, nil)
			w := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check redirect location if expected
			if tt.shouldRedirect {
				location := w.Header().Get("Location")
				if location != tt.expectedLocation {
					t.Errorf("Expected location %q, got %q", tt.expectedLocation, location)
				}
			} else {
				// Should not have Location header if not redirecting
				if location := w.Header().Get("Location"); location != "" {
					t.Errorf("Expected no Location header, got %q", location)
				}
			}
		})
	}
}

func TestHasTrailingSlash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected bool
	}{
		{"/", false},                   // Root should not be considered as having trailing slash
		{"/users", false},              // No trailing slash
		{"/users/", true},              // Has trailing slash
		{"/users/123/artworks/", true}, // Has trailing slash
		{"/users/123/artworks", false}, // No trailing slash
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			result := hasTrailingSlash(req)
			if result != tt.expected {
				t.Errorf("hasTrailingSlash(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestHasEnPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected bool
	}{
		{"/en/users/123", true},    // Valid en prefix for users
		{"/en/artworks/456", true}, // Valid en prefix for artworks
		{"/en/novel/789", true},    // Valid en prefix for novel
		{"/en/about", false},       // Invalid en prefix (not in supported paths)
		{"/en/", false},            // Just /en/ without valid path
		{"/en", false},             // Just /en without trailing slash
		{"/users/123", false},      // No en prefix
		{"/en/users", false},       // En prefix but no trailing slash after users
		{"/en/users/", true},       // En prefix with trailing slash after users
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			result := hasEnPrefix(req)
			if result != tt.expected {
				t.Errorf("hasEnPrefix(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}
