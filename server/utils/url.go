// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package utils

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var errURLMustHaveBothSchemeAndAuthority = errors.New("url must have both scheme and authority or neither")

// ParseURL parses a URL string and returns a *url.URL.
func ParseURL(urlStr, urlType string) (*url.URL, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s URL: %w", urlType, err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf(
			"%s URL is invalid: %s. Please specify a complete URL with scheme and host, e.g. https://example.com",
			urlType,
			urlStr)
	}

	parsedURL.Path = strings.TrimSuffix(parsedURL.Path, "/")

	return parsedURL, nil
}

// GetQueryParam retrieves the value of a query parameter by name.
//
// If the parameter is not present, it returns the provided default value or an empty string.
func GetQueryParam(r *http.Request, name string, defaultValue ...string) string {
	v := r.URL.Query().Get(name)
	if v != "" {
		return v
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

// GetFormValue retrieves the value of a form parameter by name.
//
// It calls r.ParseForm() and then reads r.FormValue(name).
// If the parameter is not present, it returns the provided default value or an empty string.
func GetFormValue(r *http.Request, name string, defaultValue ...string) string {
	if err := r.ParseForm(); err == nil {
		if v := r.FormValue(name); v != "" {
			return v
		}
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

// GetPathVar retrieves the value of a path variable by name.
//
// If the variable is not present, it returns the provided default value or an empty string.
func GetPathVar(r *http.Request, name string, defaultValue ...string) string {
	v := r.PathValue(name)
	if v != "" {
		return v
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

// GetMapParam retrieves the value of a parameter from a map by key.
//
// If the key is not present or the value is empty, it returns the provided default value or an empty string.
func GetMapParam(params map[string]string, name string, defaultValue ...string) string {
	if v, ok := params[name]; ok && v != "" {
		return v
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

// GetOriginFromRequest returns the origin (scheme + host) from an HTTP request.
//
// The scheme is determined by first checking the X-Forwarded-Proto header,
// then the TLS connection status, defaulting to "http" if neither is set.
// The result is returned in the format "scheme://host".
func GetOriginFromRequest(r *http.Request) string {
	scheme := "http"

	// Check X-Forwarded-Proto header first
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}

	return scheme + "://" + r.Host
}

// GetOriginFromURL extracts the scheme and host from a URL to form its origin.
//
// Examples:
// -	"https://example.com/path" returns "https://example.com"
//
// Returns an empty string if either scheme or host is missing.
func GetOriginFromURL(u url.URL) string {
	if u.Scheme == "" || u.Host == "" {
		return ""
	}

	return u.Scheme + "://" + u.Host
}

// GetProxyBase constructs a complete proxy base from a URL object.
//
// It combines the URL's authority (scheme + host) with its path component.
//
// Note that a URL must either have both scheme and host components, or have neither.
//
// Examples:
//   - URL "https://proxy.com/img" returns "https://proxy.com/img"
//   - URL with path only "/proxy" returns "/proxy"
//
// Panics if the URL format is invalid.
func GetProxyBase(proxy url.URL) string {
	if (proxy.Scheme != "") != (proxy.Host != "") {
		panic(fmt.Errorf("%w: %s\nplease correct this in your proxy list", errURLMustHaveBothSchemeAndAuthority, proxy.String()))
	}

	// Handle path-only proxies (e.g., /proxy/i.pximg.net)
	if proxy.Scheme == "" {
		return proxy.Path
	}

	// Combine authority (scheme + host) with path
	return proxy.Scheme + "://" + proxy.Host + proxy.Path
}

// SanitizeReturnPath ensures that string s is a same-origin relative path (no scheme/host).
// Returns "" if the value is unsafe; callers should fallback to "/".
func SanitizeReturnPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Disallow absolute URLs and scheme-relative URLs to prevent open redirects.
	if strings.Contains(s, "://") || strings.HasPrefix(s, "//") {
		return ""
	}

	// Must be absolute-path reference on this origin.
	if !strings.HasPrefix(s, "/") {
		return ""
	}

	return s
}
