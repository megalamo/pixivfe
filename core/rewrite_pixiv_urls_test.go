// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
)

// NOTE: invalid and empty URL cases are intentionally omitted due to being out of scope.
func TestGenerateMasterWebpURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		// Basic img-master category cases
		{
			name:     "img-master with master1200 suffix",
			input:    "https://i.pximg.net/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},
		{
			name:     "img-master with square1200 suffix",
			input:    "https://i.pximg.net/img-master/img/2025/06/05/18/10/08/131206066_p0_square1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},
		{
			name:     "img-master with existing size transformation",
			input:    "https://i.pximg.net/c/600x600/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},
		{
			name:     "img-master with different existing size transformation",
			input:    "https://i.pximg.net/c/250x250_80_a2/img-master/img/2025/06/05/18/10/08/131206066_p0_square1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},

		// img-original URLs are now converted to WebP
		{
			name:     "img-original JPG should be converted",
			input:    "https://i.pximg.net/img-original/img/2025/06/05/18/10/08/131206066_p0.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},
		{
			name:     "img-original PNG should be converted",
			input:    "https://i.pximg.net/img-original/img/2023/08/20/06/04/20/110992799_p0.png",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2023/08/20/06/04/20/110992799_p0_master1200.jpg",
		},
		{
			name:     "img-original with existing size transformation should be converted",
			input:    "https://i.pximg.net/c/600x600/img-original/img/2025/06/05/18/10/08/131206066_p0.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},

		// custom-thumb category cases (should be converted to img-master)
		{
			name:     "custom-thumb without size transformation",
			input:    "https://i.pximg.net/custom-thumb/img/2025/06/05/18/10/08/131206066_p0_custom1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},
		{
			name:     "custom-thumb with existing size transformation",
			input:    "https://i.pximg.net/c/600x600/custom-thumb/img/2025/06/05/18/10/08/131206066_p0_custom1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},

		// Different page numbers
		{
			name:     "img-master with page 1",
			input:    "https://i.pximg.net/img-master/img/2025/06/05/18/10/08/131206066_p1_master1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p1_master1200.jpg",
		},

		// Edge case: URL without standard category prefix (fallback handling)
		{
			name:     "URL without standard category prefix",
			input:    "https://i.pximg.net/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},

		// Different domains (should preserve domain)
		{
			name:     "different domain",
			input:    "https://other.example.com/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
			expected: "https://other.example.com/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},

		// WebP variants that should be replaced
		{
			name:     "existing WebP variant should be replaced",
			input:    "https://i.pximg.net/c/540x540_10_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},

		// Complex size parameters should be replaced
		{
			name:     "complex pixivision crop parameters should be replaced",
			input:    "https://i.pximg.net/c/1200x630_q80_a2_g1_u1_icr0.093:0.014:0.938:0.758/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},

		// URLs with query parameters and fragments (should be preserved)
		{
			name:     "URL with query parameters",
			input:    "https://i.pximg.net/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg?param=value",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg?param=value",
		},
		{
			name:     "URL with fragment",
			input:    "https://i.pximg.net/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg#fragment",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg#fragment",
		},

		// URLs without page numbers (occurs for ugoira thumbnails)
		{
			name:     "img-master with square1200 suffix but no page number",
			input:    "https://pximg.perennialte.ch/c/1200x1200_80_webp/img-master/img/2025/05/21/18/01/06/130652065_square1200.jpg",
			expected: "https://pximg.perennialte.ch/c/1200x1200_80_webp/img-master/img/2025/05/21/18/01/06/130652065_master1200.jpg",
		},
		{
			name:     "img-master with custom1200 suffix but no page number",
			input:    "https://i.pximg.net/img-master/img/2025/05/21/18/01/06/130652065_custom1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/05/21/18/01/06/130652065_master1200.jpg",
		},
		{
			name:     "img-original/img/ without page number should be converted",
			input:    "https://i.pximg.net/img-original/img/2025/05/21/18/01/06/130652065.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/05/21/18/01/06/130652065_master1200.jpg",
		},

		// Novel cover URLs
		{
			name:     "/novel-cover-master/ segment should be preserved, but the URL converted",
			input:    "https://i.pximg.net/c/1200x1200/novel-cover-master/img/1970/01/01/00/00/00/deadbeef_master1200.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/novel-cover-master/img/1970/01/01/00/00/00/deadbeef_master1200.jpg",
		},
		{
			name:     "/novel-cover-original/ segment should be preserved, but the URL converted",
			input:    "https://i.pximg.net/novel-cover-original/img/1970/01/01/00/00/00/deadbeef.jpg",
			expected: "https://i.pximg.net/c/1200x1200_80_webp/novel-cover-master/img/1970/01/01/00/00/00/deadbeef_master1200.jpg",
		},

		// Built-in proxy
		{
			name:     "Built-in proxy",
			input:    "/proxy/i.pximg.net/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
			expected: "/proxy/i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/06/05/18/10/08/131206066_p0_master1200.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := generateMasterWebpURL(tt.input, "")

			if result != tt.expected {
				t.Errorf("GenerateMasterWebpURL(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateMasterWebpURLWithArbitraryProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		proxyPrefix string
		expected    string
	}{
		// Path-only proxy configurations
		{
			name:        "Custom path-based proxy",
			input:       "https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
			proxyPrefix: "/custom/image/proxy",
			expected:    "/custom/image/proxy/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},
		{
			name:        "Deep nested path proxy",
			input:       "https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_square1200.jpg",
			proxyPrefix: "/api/v2/images/pixiv/proxy",
			expected:    "/api/v2/images/pixiv/proxy/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},
		{
			name:        "Single level custom path",
			input:       "https://i.pximg.net/custom-thumb/img/2025/01/01/12/00/00/123456789_p0_custom1200.jpg",
			proxyPrefix: "/imgproxy",
			expected:    "/imgproxy/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},

		// Full URL proxy configurations (external domains)
		{
			name:        "External domain proxy",
			input:       "https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
			proxyPrefix: "https://proxy.example.com",
			expected:    "https://proxy.example.com/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},
		{
			name:        "External domain with path",
			input:       "https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_square1200.jpg",
			proxyPrefix: "https://cdn.example.com/pixiv/images",
			expected:    "https://cdn.example.com/pixiv/images/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},
		{
			name:        "Subdomain proxy with versioned API",
			input:       "https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_custom1200.jpg",
			proxyPrefix: "https://pixiv-proxy.example.com/api/v1/images",
			expected:    "https://pixiv-proxy.example.com/api/v1/images/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},
		{
			name:        "External proxy with trailing slash",
			input:       "https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
			proxyPrefix: "https://proxy.example.com/",
			expected:    "https://proxy.example.com/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},

		// Edge cases and special configurations
		{
			name:        "Complex external proxy with multiple path segments",
			input:       "https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_square1200.jpg",
			proxyPrefix: "https://media-cache.example.org/services/pixiv/image-proxy/v2",
			expected:    "https://media-cache.example.org/services/pixiv/image-proxy/v2/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},
		{
			name:        "Port-specific external proxy",
			input:       "https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
			proxyPrefix: "https://localhost:8080/proxy",
			expected:    "https://localhost:8080/proxy/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},

		// Traditional proxy (backward compatibility)
		{
			name:        "Traditional proxy base for comparison",
			input:       "https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
			proxyPrefix: "/proxy/i.pximg.net",
			expected:    "/proxy/i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},

		// URLs that should now be converted (img-original)
		{
			name:        "img-original with custom proxy should be converted",
			input:       "https://i.pximg.net/img-original/img/2025/01/01/12/00/00/123456789_p0.jpg",
			proxyPrefix: "/custom/proxy",
			expected:    "/custom/proxy/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},
		{
			name:        "img-original with external proxy should be converted",
			input:       "https://i.pximg.net/img-original/img/2025/01/01/12/00/00/123456789_p0.png",
			proxyPrefix: "https://proxy.example.com",
			expected:    "https://proxy.example.com/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},

		// URLs that already have proxy basees
		{
			name:        "URL with existing /proxy/ prefix and custom proxy",
			input:       "/proxy/i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
			proxyPrefix: "/custom/proxy",
			expected:    "/custom/proxy/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},
		{
			name:        "URL with existing /proxy/ prefix and external proxy",
			input:       "/proxy/i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_square1200.jpg",
			proxyPrefix: "https://cdn.example.com",
			expected:    "https://cdn.example.com/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := generateMasterWebpURL(tt.input, tt.proxyPrefix)

			if result != tt.expected {
				t.Errorf("generateMasterWebpURL(%q, %q) = %q, expected %q", tt.input, tt.proxyPrefix, result, tt.expected)
			}
		})
	}
}

func TestRewriteEscapedContentURLs(t *testing.T) {
	t.Parallel()

	// Test cases for built-in proxy base (/proxy/i.pximg.net)
	proxyTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escaped i.pximg.net URL with WebP conversion using proxy base",
			input:    `{"url":"https:\/\/i.pximg.net\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
			expected: `{"url":"\/proxy\/i.pximg.net\/c\/1200x1200_80_webp\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
		},
		{
			name:     "escaped user profile image without WebP conversion using proxy base",
			input:    `{"avatar":"https:\/\/i.pximg.net\/user-profile\/img\/2025\/01\/01\/12\/00\/00\/123456789_abc123.jpg"}`,
			expected: `{"avatar":"\/proxy\/i.pximg.net\/user-profile\/img\/2025\/01\/01\/12\/00\/00\/123456789_abc123.jpg"}`,
		},
		{
			name:     "escaped imgaz upload image without WebP conversion using proxy base",
			input:    `{"upload":"https:\/\/i.pximg.net\/imgaz\/upload\/2025\/01\/01\/12\/00\/00\/123456789_p0.jpg"}`,
			expected: `{"upload":"\/proxy\/i.pximg.net\/imgaz\/upload\/2025\/01\/01\/12\/00\/00\/123456789_p0.jpg"}`,
		},
		{
			name:     "escaped s.pximg.net URL using proxy base",
			input:    `{"static":"https:\/\/s.pximg.net\/common\/images\/logo.png"}`,
			expected: `{"static":"\/proxy\/s.pximg.net\/common\/images\/logo.png"}`,
		},
		{
			name:     "escaped img-original/img/ URL using proxy base should not be converted",
			input:    `{"original":"https:\/\/i.pximg.net\/img-original\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0.png"}`,
			expected: `{"original":"\/proxy\/i.pximg.net\/img-original\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0.png"}`,
		},
		{
			name:     "escaped background image without WebP conversion using proxy base",
			input:    `{"background":"https:\/\/i.pximg.net\/background\/img\/2025\/01\/01\/12\/00\/00\/123456789.jpg"}`,
			expected: `{"background":"\/proxy\/i.pximg.net\/background\/img\/2025\/01\/01\/12\/00\/00\/123456789.jpg"}`,
		},
		{
			name:     "escaped source.pixiv.net URL",
			input:    `{"source":"https:\/\/source.pixiv.net\/source\/123456789_p0.jpg"}`,
			expected: `{"source":"\/proxy\/source.pixiv.net\/source\/123456789_p0.jpg"}`,
		},
		{
			name:     "escaped booth.pximg.net URL",
			input:    `{"booth":"https:\/\/booth.pximg.net\/c\/300x300\/images\/item\/123456.jpg"}`,
			expected: `{"booth":"\/proxy\/booth.pximg.net\/c\/300x300\/images\/item\/123456.jpg"}`,
		},
		{
			name:     "no pixiv URLs",
			input:    `{"url":"https://example.com/image.jpg"}`,
			expected: `{"url":"https://example.com/image.jpg"}`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	// Test cases for external domain (https://pximg.exozy.me)
	externalTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escaped i.pximg.net URL with WebP conversion using external domain",
			input:    `{"url":"https:\/\/i.pximg.net\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
			expected: `{"url":"https:\/\/pximg.exozy.me\/c\/1200x1200_80_webp\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
		},
		{
			name:     "escaped user profile image without WebP conversion using external domain",
			input:    `{"avatar":"https:\/\/i.pximg.net\/user-profile\/img\/2025\/01\/01\/12\/00\/00\/123456789_abc123.jpg"}`,
			expected: `{"avatar":"https:\/\/pximg.exozy.me\/user-profile\/img\/2025\/01\/01\/12\/00\/00\/123456789_abc123.jpg"}`,
		},
		{
			name:     "escaped imgaz upload image without WebP conversion using external domain",
			input:    `{"upload":"https:\/\/i.pximg.net\/imgaz\/upload\/2025\/01\/01\/12\/00\/00\/123456789_p0.jpg"}`,
			expected: `{"upload":"https:\/\/pximg.exozy.me\/imgaz\/upload\/2025\/01\/01\/12\/00\/00\/123456789_p0.jpg"}`,
		},
		{
			name:     "escaped s.pximg.net URL using external domain",
			input:    `{"static":"https:\/\/s.pximg.net\/common\/images\/logo.png"}`,
			expected: `{"static":"https:\/\/static.exozy.me\/common\/images\/logo.png"}`,
		},
		{
			name:     "escaped img-original/img/ URL using external domain should not be converted",
			input:    `{"original":"https:\/\/i.pximg.net\/img-original\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0.png"}`,
			expected: `{"original":"https:\/\/pximg.exozy.me\/img-original\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0.png"}`,
		},
		{
			name:     "escaped background image without WebP conversion using external domain",
			input:    `{"background":"https:\/\/i.pximg.net\/background\/img\/2025\/01\/01\/12\/00\/00\/123456789.jpg"}`,
			expected: `{"background":"https:\/\/pximg.exozy.me\/background\/img\/2025\/01\/01\/12\/00\/00\/123456789.jpg"}`,
		},
	}

	// Test with built-in proxy base
	for _, tt := range proxyTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: string(cookie.ImageProxyCookie), Value: "/proxy/i.pximg.net"})
			req.AddCookie(&http.Cookie{Name: string(cookie.StaticProxyCookie), Value: "/proxy/s.pximg.net"})

			result := RewriteEscapedImageURLs(req, []byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("RewriteEscapedContentURLs(%q) = %q, expected %q", tt.input, string(result), tt.expected)
			}
		})
	}

	// Test with external domain
	for _, tt := range externalTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: string(cookie.ImageProxyCookie), Value: "https://pximg.exozy.me"})
			req.AddCookie(&http.Cookie{Name: string(cookie.StaticProxyCookie), Value: "https://static.exozy.me"})

			result := RewriteEscapedImageURLs(req, []byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("RewriteEscapedContentURLs(%q) = %q, expected %q", tt.input, string(result), tt.expected)
			}
		})
	}

	// Test with arbitrary proxy configurations
	arbitraryProxyTests := []struct {
		name        string
		input       string
		expected    string
		imageProxy  string
		staticProxy string
	}{
		{
			name:        "escaped i.pximg.net URL with custom path proxy",
			input:       `{"url":"https:\/\/i.pximg.net\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
			expected:    `{"url":"\/custom\/image\/proxy\/c\/1200x1200_80_webp\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
			imageProxy:  "/custom/image/proxy",
			staticProxy: "/custom/static/proxy",
		},
		{
			name:        "escaped i.pximg.net URL with deep nested path proxy",
			input:       `{"url":"https:\/\/i.pximg.net\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_square1200.jpg"}`,
			expected:    `{"url":"\/api\/v2\/images\/pixiv\/proxy\/c\/1200x1200_80_webp\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
			imageProxy:  "/api/v2/images/pixiv/proxy",
			staticProxy: "/api/v2/static/pixiv/proxy",
		},
		{
			name:        "escaped i.pximg.net URL with external domain proxy",
			input:       `{"url":"https:\/\/i.pximg.net\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_custom1200.jpg"}`,
			expected:    `{"url":"https:\/\/media-proxy.example.com\/c\/1200x1200_80_webp\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
			imageProxy:  "https://media-proxy.example.com",
			staticProxy: "https://static-proxy.example.com",
		},
		{
			name:        "escaped i.pximg.net URL with complex external proxy path",
			input:       `{"url":"https:\/\/i.pximg.net\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
			expected:    `{"url":"https:\/\/cdn.example.org\/services\/pixiv\/images\/c\/1200x1200_80_webp\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0_master1200.jpg"}`,
			imageProxy:  "https://cdn.example.org/services/pixiv/images",
			staticProxy: "https://cdn.example.org/services/pixiv/static",
		},
		{
			name:        "escaped user profile image without WebP conversion using custom proxy",
			input:       `{"avatar":"https:\/\/i.pximg.net\/user-profile\/img\/2025\/01\/01\/12\/00\/00\/123456789_abc123.jpg"}`,
			expected:    `{"avatar":"\/imgproxy\/user-profile\/img\/2025\/01\/01\/12\/00\/00\/123456789_abc123.jpg"}`,
			imageProxy:  "/imgproxy",
			staticProxy: "/staticproxy",
		},
		{
			name:        "escaped img-original should not be converted with custom proxy",
			input:       `{"original":"https:\/\/i.pximg.net\/img-original\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0.png"}`,
			expected:    `{"original":"\/custom\/proxy\/img-original\/img\/2025\/01\/01\/12\/00\/00\/123456789_p0.png"}`,
			imageProxy:  "/custom/proxy",
			staticProxy: "/custom/static",
		},
		{
			name:        "escaped background image without WebP conversion using custom proxy",
			input:       `{"background":"https:\/\/i.pximg.net\/background\/img\/2025\/01\/01\/12\/00\/00\/123456789.jpg"}`,
			expected:    `{"background":"\/imgproxy\/background\/img\/2025\/01\/01\/12\/00\/00\/123456789.jpg"}`,
			imageProxy:  "/imgproxy",
			staticProxy: "/staticproxy",
		},
		{
			name:        "escaped s.pximg.net URL with custom static proxy",
			input:       `{"static":"https:\/\/s.pximg.net\/common\/images\/logo.png"}`,
			expected:    `{"static":"\/custom\/static\/proxy\/common\/images\/logo.png"}`,
			imageProxy:  "/custom/image/proxy",
			staticProxy: "/custom/static/proxy",
		},
		{
			name:        "mixed URLs with different custom proxies",
			input:       `{"image":"https:\/\/i.pximg.net\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123_p0_master1200.jpg","static":"https:\/\/s.pximg.net\/common\/logo.png"}`,
			expected:    `{"image":"https:\/\/img.example.com\/api\/v1\/c\/1200x1200_80_webp\/img-master\/img\/2025\/01\/01\/12\/00\/00\/123_p0_master1200.jpg","static":"https:\/\/static.example.com\/api\/v1\/common\/logo.png"}`,
			imageProxy:  "https://img.example.com/api/v1",
			staticProxy: "https://static.example.com/api/v1",
		},
	}

	for _, tt := range arbitraryProxyTests {
		t.Run(tt.name+" (arbitrary proxy)", func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: string(cookie.ImageProxyCookie), Value: tt.imageProxy})
			req.AddCookie(&http.Cookie{Name: string(cookie.StaticProxyCookie), Value: tt.staticProxy})

			result := RewriteEscapedImageURLs(req, []byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("RewriteEscapedContentURLs(%q) = %q, expected %q", tt.input, string(result), tt.expected)
			}
		})
	}
}

func TestRewriteContentURLs(t *testing.T) {
	t.Parallel()

	// Test cases for built-in proxy base (/proxy/i.pximg.net)
	proxyTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "i.pximg.net URL with WebP conversion using proxy base",
			input:    `<img src="https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			expected: `<img src="/proxy/i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
		},
		{
			name:     "user profile image without WebP conversion using proxy base",
			input:    `<img src="https://i.pximg.net/user-profile/img/2025/01/01/12/00/00/123456789_abc123.jpg" alt="avatar">`,
			expected: `<img src="/proxy/i.pximg.net/user-profile/img/2025/01/01/12/00/00/123456789_abc123.jpg" alt="avatar">`,
		},
		{
			name:     "imgaz upload image without WebP conversion using proxy base",
			input:    `<img src="https://i.pximg.net/imgaz/upload/2025/01/01/12/00/00/123456789_p0.jpg">`,
			expected: `<img src="/proxy/i.pximg.net/imgaz/upload/2025/01/01/12/00/00/123456789_p0.jpg">`,
		},
		{
			name:     "s.pximg.net URL using proxy base",
			input:    `<link rel="stylesheet" href="https://s.pximg.net/common/styles/main.css">`,
			expected: `<link rel="stylesheet" href="/proxy/s.pximg.net/common/styles/main.css">`,
		},
		{
			name:     "img-original/img/ URL using proxy base should not be converted",
			input:    `<img src="https://i.pximg.net/img-original/img/2025/01/01/12/00/00/123456789_p0.png">`,
			expected: `<img src="/proxy/i.pximg.net/img-original/img/2025/01/01/12/00/00/123456789_p0.png">`,
		},
		{
			name:     "background image without WebP conversion using proxy base",
			input:    `<img src="https://i.pximg.net/background/img/2023/09/14/14/40/20/49675420_1429a9d81f8c429f032e690244530604.png">`,
			expected: `<img src="/proxy/i.pximg.net/background/img/2023/09/14/14/40/20/49675420_1429a9d81f8c429f032e690244530604.png">`,
		},
		{
			name:     "URL with existing size parameters using proxy base",
			input:    `<img src="https://i.pximg.net/c/600x600/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			expected: `<img src="/proxy/i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
		},
		{
			name:     "plain text with URL using proxy base",
			input:    `Check out this image: https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg`,
			expected: `Check out this image: /proxy/i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg`,
		},
		{
			name:     "multiple URLs in HTML using proxy base",
			input:    `<img src="https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123_p0_master1200.jpg"><link href="https://s.pximg.net/common/logo.png">`,
			expected: `<img src="/proxy/i.pximg.net/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123_p0_master1200.jpg"><link href="/proxy/s.pximg.net/common/logo.png">`,
		},
		{
			name:     "source.pixiv.net URL",
			input:    `<a href="https://source.pixiv.net/source/123456789_p0.jpg">Source</a>`,
			expected: `<a href="/proxy/source.pixiv.net/source/123456789_p0.jpg">Source</a>`,
		},
		{
			name:     "booth.pximg.net URL",
			input:    `<img src="https://booth.pximg.net/c/300x300/images/item/123456.jpg">`,
			expected: `<img src="/proxy/booth.pximg.net/c/300x300/images/item/123456.jpg">`,
		},
		{
			name:     "no pixiv URLs",
			input:    `<img src="https://example.com/image.jpg">`,
			expected: `<img src="https://example.com/image.jpg">`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	// Test cases for external domain (https://pximg.exozy.me)
	externalTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "i.pximg.net URL with WebP conversion using external domain",
			input:    `<img src="https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			expected: `<img src="https://pximg.exozy.me/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
		},
		{
			name:     "user profile image without WebP conversion using external domain",
			input:    `<img src="https://i.pximg.net/user-profile/img/2025/01/01/12/00/00/123456789_abc123.jpg" alt="avatar">`,
			expected: `<img src="https://pximg.exozy.me/user-profile/img/2025/01/01/12/00/00/123456789_abc123.jpg" alt="avatar">`,
		},
		{
			name:     "imgaz upload image without WebP conversion using external domain",
			input:    `<img src="https://i.pximg.net/imgaz/upload/2025/01/01/12/00/00/123456789_p0.jpg">`,
			expected: `<img src="https://pximg.exozy.me/imgaz/upload/2025/01/01/12/00/00/123456789_p0.jpg">`,
		},
		{
			name:     "s.pximg.net URL using external domain",
			input:    `<link rel="stylesheet" href="https://s.pximg.net/common/styles/main.css">`,
			expected: `<link rel="stylesheet" href="https://static.exozy.me/common/styles/main.css">`,
		},
		{
			name:     "img-original/img/ URL using external domain should not be converted",
			input:    `<img src="https://i.pximg.net/img-original/img/2025/01/01/12/00/00/123456789_p0.png">`,
			expected: `<img src="https://pximg.exozy.me/img-original/img/2025/01/01/12/00/00/123456789_p0.png">`,
		},
		{
			name:     "background image without WebP conversion using external domain",
			input:    `<img src="https://i.pximg.net/background/img/2023/09/14/14/40/20/49675420_1429a9d81f8c429f032e690244530604.png">`,
			expected: `<img src="https://pximg.exozy.me/background/img/2023/09/14/14/40/20/49675420_1429a9d81f8c429f032e690244530604.png">`,
		},
		{
			name:     "URL with existing size parameters using external domain",
			input:    `<img src="https://i.pximg.net/c/600x600/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			expected: `<img src="https://pximg.exozy.me/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
		},
		{
			name:     "plain text with URL using external domain",
			input:    `Check out this image: https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg`,
			expected: `Check out this image: https://pximg.exozy.me/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg`,
		},
		{
			name:     "multiple URLs in HTML using external domain",
			input:    `<img src="https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123_p0_master1200.jpg"><link href="https://s.pximg.net/common/logo.png">`,
			expected: `<img src="https://pximg.exozy.me/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123_p0_master1200.jpg"><link href="https://static.exozy.me/common/logo.png">`,
		},
	}

	// Test with built-in proxy base
	for _, tt := range proxyTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: string(cookie.ImageProxyCookie), Value: "/proxy/i.pximg.net"})
			req.AddCookie(&http.Cookie{Name: string(cookie.StaticProxyCookie), Value: "/proxy/s.pximg.net"})

			result := RewriteImageURLs(req, tt.input)
			if result != tt.expected {
				t.Errorf("RewriteContentURLs(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}

	// Test with external domain
	for _, tt := range externalTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: string(cookie.ImageProxyCookie), Value: "https://pximg.exozy.me"})
			req.AddCookie(&http.Cookie{Name: string(cookie.StaticProxyCookie), Value: "https://static.exozy.me"})

			result := RewriteImageURLs(req, tt.input)
			if result != tt.expected {
				t.Errorf("RewriteContentURLs(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}

	// Test with arbitrary proxy configurations
	arbitraryProxyTests := []struct {
		name        string
		input       string
		expected    string
		imageProxy  string
		staticProxy string
	}{
		{
			name:        "i.pximg.net URL with custom path proxy",
			input:       `<img src="https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			expected:    `<img src="/custom/image/proxy/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			imageProxy:  "/custom/image/proxy",
			staticProxy: "/custom/static/proxy",
		},
		{
			name:        "i.pximg.net URL with deep nested path proxy",
			input:       `<img src="https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_square1200.jpg">`,
			expected:    `<img src="/api/v2/images/pixiv/proxy/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			imageProxy:  "/api/v2/images/pixiv/proxy",
			staticProxy: "/api/v2/static/pixiv/proxy",
		},
		{
			name:        "i.pximg.net URL with external domain proxy",
			input:       `<img src="https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_custom1200.jpg">`,
			expected:    `<img src="https://media-proxy.example.com/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			imageProxy:  "https://media-proxy.example.com",
			staticProxy: "https://static-proxy.example.com",
		},
		{
			name:        "i.pximg.net URL with complex external proxy path",
			input:       `<img src="https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			expected:    `<img src="https://cdn.example.org/services/pixiv/images/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			imageProxy:  "https://cdn.example.org/services/pixiv/images",
			staticProxy: "https://cdn.example.org/services/pixiv/static",
		},
		{
			name:        "user profile image without WebP conversion using custom proxy",
			input:       `<img src="https://i.pximg.net/user-profile/img/2025/01/01/12/00/00/123456789_abc123.jpg" alt="avatar">`,
			expected:    `<img src="/imgproxy/user-profile/img/2025/01/01/12/00/00/123456789_abc123.jpg" alt="avatar">`,
			imageProxy:  "/imgproxy",
			staticProxy: "/staticproxy",
		},
		{
			name:        "img-original should not be converted with custom proxy",
			input:       `<img src="https://i.pximg.net/img-original/img/2025/01/01/12/00/00/123456789_p0.png">`,
			expected:    `<img src="/custom/proxy/img-original/img/2025/01/01/12/00/00/123456789_p0.png">`,
			imageProxy:  "/custom/proxy",
			staticProxy: "/custom/static",
		},
		{
			name:        "background image without WebP conversion using custom proxy",
			input:       `<img src="https://i.pximg.net/background/img/2023/09/14/14/40/20/49675420_1429a9d81f8c429f032e690244530604.png">`,
			expected:    `<img src="/imgproxy/background/img/2023/09/14/14/40/20/49675420_1429a9d81f8c429f032e690244530604.png">`,
			imageProxy:  "/imgproxy",
			staticProxy: "/staticproxy",
		},
		{
			name:        "s.pximg.net URL with custom static proxy",
			input:       `<link rel="stylesheet" href="https://s.pximg.net/common/styles/main.css">`,
			expected:    `<link rel="stylesheet" href="/custom/static/proxy/common/styles/main.css">`,
			imageProxy:  "/custom/image/proxy",
			staticProxy: "/custom/static/proxy",
		},
		{
			name:        "multiple URLs in HTML with different custom proxies",
			input:       `<img src="https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123_p0_master1200.jpg"><link href="https://s.pximg.net/common/logo.png">`,
			expected:    `<img src="https://img.example.com/api/v1/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123_p0_master1200.jpg"><link href="https://static.example.com/api/v1/common/logo.png">`,
			imageProxy:  "https://img.example.com/api/v1",
			staticProxy: "https://static.example.com/api/v1",
		},
		{
			name:        "plain text with URL using custom proxy",
			input:       `Check out this image: https://i.pximg.net/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg`,
			expected:    `Check out this image: /imgproxy/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg`,
			imageProxy:  "/imgproxy",
			staticProxy: "/staticproxy",
		},
		{
			name:        "URL with existing size parameters using external proxy",
			input:       `<img src="https://i.pximg.net/c/600x600/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			expected:    `<img src="https://proxy.example.com/c/1200x1200_80_webp/img-master/img/2025/01/01/12/00/00/123456789_p0_master1200.jpg">`,
			imageProxy:  "https://proxy.example.com",
			staticProxy: "https://static.example.com",
		},
	}

	for _, tt := range arbitraryProxyTests {
		t.Run(tt.name+" (arbitrary proxy)", func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: string(cookie.ImageProxyCookie), Value: tt.imageProxy})
			req.AddCookie(&http.Cookie{Name: string(cookie.StaticProxyCookie), Value: tt.staticProxy})

			result := RewriteImageURLs(req, tt.input)
			if result != tt.expected {
				t.Errorf("RewriteContentURLs(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDescriptionURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pixiv novel redirect",
			input:    "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fnovel%2Fshow.php%3Fid%3D24560221",
			expected: "/novel/24560221",
		},
		{
			name:     "pixiv novel redirect with language prefix",
			input:    "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fen%2Fnovel%2Fshow.php%3Fid%3D24560221",
			expected: "/novel/24560221",
		},
		{
			name:     "pixiv novel redirect with additional parameters",
			input:    "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fnovel%2Fshow.php%3Fsomeparam%3Dvalue%26id%3D24560221",
			expected: "/novel/24560221",
		},
		{
			name:     "pixiv user redirect",
			input:    "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fusers%2F12345",
			expected: "/users/12345",
		},
		{
			name:     "pixiv artwork redirect",
			input:    "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fartworks%2F67890",
			expected: "/artworks/67890",
		},
		{
			name:     "pixiv user redirect with language prefix",
			input:    "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fen%2Fusers%2F12345",
			expected: "/users/12345",
		},
		{
			name:     "pixiv artwork redirect with language prefix",
			input:    "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fen%2Fartworks%2F67890",
			expected: "/artworks/67890",
		},
		{
			name:     "pixiv non-user non-artwork redirect (e.g. /home)",
			input:    "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fhome",
			expected: "https://www.pixiv.net/home",
		},
		{
			name:     "Non-pixiv redirect",
			input:    "/jump.php?https%3A%2F%2Fexample.com",
			expected: "https://example.com",
		},
		{
			name:     "URL with special characters (percent-encoded ampersand in query)",
			input:    "/jump.php?https%3A%2F%2Fexample.com%2Fpath%3Fquery%3Dvalue%26another%3Dvalue",
			expected: "https://example.com/path?query=value&another=value",
		},
		{
			name:     "No redirect pattern, just plain URL",
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "Invalid percent encoding in jump.php",
			input:    "/jump.php?https%3A%2F%2Fexample.com%ZZ",
			expected: "/jump.php?https%3A%2F%2Fexample.com%ZZ",
		},
		{
			name:     "Non-http protocol (ftp) in jump.php",
			input:    "/jump.php?ftp%3A%2F%2Fexample.com",
			expected: "ftp://example.com",
		},
		{
			name:     "Mailto protocol in jump.php",
			input:    "/jump.php?mailto%3Atest%40example.com",
			expected: "mailto:test@example.com",
		},
		{
			name:     "Javascript URI in jump.php",
			input:    "/jump.php?javascript%3Aalert%281%29",
			expected: "",
		},
		{
			name:     "Non-URL text (no scheme) in jump.php",
			input:    "/jump.php?justsometext",
			expected: "justsometext",
		},
		{
			name:     "Empty string input",
			input:    "",
			expected: "",
		},
		{
			name:     "String with multiple jump.php links",
			input:    "First: /jump.php?https%3A%2F%2Fwww.pixiv.net%2Fusers%2F12345 Second: /jump.php?https%3A%2F%2Fexample.com",
			expected: "First: /users/12345 Second: https://example.com",
		},
		{
			name:     "Multiple eligible pixiv links in jump.php",
			input:    "User: /jump.php?https%3A%2F%2Fwww.pixiv.net%2Fusers%2F123 Art: /jump.php?https%3A%2F%2Fwww.pixiv.net%2Fartworks%2F456",
			expected: "User: /users/123 Art: /artworks/456",
		},
		{
			name:     "Mixed pixiv links in jump.php, some eligible for relative, some not",
			input:    "User: /jump.php?https%3A%2F%2Fwww.pixiv.net%2Fen%2Fusers%2F789 Home: /jump.php?https%3A%2F%2Fwww.pixiv.net%2Fdashboard",
			expected: "User: /users/789 Home: https://www.pixiv.net/dashboard",
		},
		{
			name:     "pixiv user redirect with http (not https)",
			input:    "/jump.php?http%3A%2F%2Fwww.pixiv.net%2Fusers%2F12345",
			expected: "/users/12345",
		},
		{
			name:     "jump.php with empty parameter",
			input:    "/jump.php?",
			expected: "/jump.php?",
		},
		{
			name:     "jump.php with parameter ending before ampersand (regex behavior)",
			input:    "/jump.php?https%3A%2F%2Fexample.com¶m=2",
			expected: "https://example.com¶m=2",
		},
		// Tests for standalone absolute pixiv URLs
		{
			name:     "Standalone pixiv user URL",
			input:    "Check this user: https://www.pixiv.net/users/12345",
			expected: "Check this user: /users/12345",
		},
		{
			name:     "Standalone pixiv artwork URL with lang",
			input:    "Artwork: https://www.pixiv.net/en/artworks/67890",
			expected: "Artwork: /artworks/67890",
		},
		{
			name:     "Standalone pixiv novel URL",
			input:    "Novel: https://www.pixiv.net/novel/show.php?id=24560221",
			expected: "Novel: /novel/24560221",
		},
		{
			name:     "Standalone pixiv home URL (should not change)",
			input:    "Home: https://www.pixiv.net/home",
			expected: "Home: https://www.pixiv.net/home",
		},
		{
			name:     "Standalone pixiv URL with www and http",
			input:    "Link: http://www.pixiv.net/users/987",
			expected: "Link: /users/987",
		},
		{
			name:     "Standalone pixiv URL without www",
			input:    "Link: https://pixiv.net/artworks/654",
			expected: "Link: /artworks/654",
		},
		{
			name:     "Standalone javascript URI (should not change, not targeted by absolutePixivLinkRegex)",
			input:    "Script: javascript:alert('danger')",
			expected: "Script: javascript:alert('danger')",
		},
		{
			name:     "Mixed jump.php and standalone absolute pixiv URLs",
			input:    "Jump: /jump.php?https%3A%2F%2Fwww.pixiv.net%2Fusers%2F123 Standalone: https://www.pixiv.net/artworks/456",
			expected: "Jump: /users/123 Standalone: /artworks/456",
		},
		{
			name:     "Mixed jump.php (non-pixiv) and standalone absolute pixiv URL (non-special)",
			input:    "Jump: /jump.php?https%3A%2F%2Fexample.com Standalone: https://www.pixiv.net/about",
			expected: "Jump: https://example.com Standalone: https://www.pixiv.net/about",
		},
		{
			name:     "Already relative pixiv URL (should not change, not matched by absolutePixivLinkRegex)",
			input:    "Link: /users/12345",
			expected: "Link: /users/12345",
		},
		{
			name:     "URL with trailing punctuation",
			input:    "Link: https://www.pixiv.net/users/12345.",
			expected: "Link: /users/12345.", // url.Parse includes trailing dot in path, current logic keeps it.
		},
		{
			name:  "URL within quotes",
			input: `Link: "https://www.pixiv.net/users/12345"`,
			// absolutePixivLinkRegex's [^\s<>"']* means it stops at the quote.
			expected: `Link: "/users/12345"`,
		},
		{
			name:     "URL within parentheses",
			input:    "Link: (https://www.pixiv.net/users/12345)",
			expected: "Link: (/users/12345)", // Similar to quotes, stops at ) due to [^\s<>"']*
		},
		{
			name:     "Path ending with slash /users/123/",
			input:    "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fusers%2F123%2F",
			expected: "/users/123", // TrimPrefix and Split handle this well for pathParts
		},
		{
			name:     "Standalone path ending with slash /users/123/",
			input:    "https://www.pixiv.net/users/123/",
			expected: "/users/123",
		},
		{
			name:  "Path like /users/ (no ID)",
			input: "/jump.php?https%3A%2F%2Fwww.pixiv.net%2Fusers%2F",
			// pathParts will be ["users", ""]. id == "" condition will make it not convert.
			expected: "https://www.pixiv.net/users/",
		},
		{
			name:     "Standalone path like /users/ (no ID)",
			input:    "https://www.pixiv.net/users/",
			expected: "https://www.pixiv.net/users/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := parseDescriptionURLs(tc.input)
			if result != tc.expected {
				t.Errorf("parseDescriptionURLs(%q):\n got: %q\nwant: %q", tc.input, result, tc.expected)
			}
		})
	}
}
