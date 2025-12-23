// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package untrusted

import (
	"net/http"
	"net/url"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
)

// GetUserToken retrieves an authentication token for
// the pixiv API from the request's 'pixivfe-Token' cookie.
func GetUserToken(r *http.Request) string {
	return GetCookie(r, cookie.TokenCookie)
}

// GetImageProxy returns the content proxy URL for i.pximg.net content.
//
// The proxy URL is retrieved from cookies if available, otherwise falls back
// to the default configuration.
func GetImageProxy(r *http.Request) url.URL {
	return getProxy(r, cookie.ImageProxyCookie, config.Global.ContentProxies.Image)
}

// GetStaticProxy returns the content proxy URL for s.pximg.net content.
//
// The proxy URL is retrieved from cookies if available, otherwise falls back
// to the default configuration.
func GetStaticProxy(r *http.Request) url.URL {
	return getProxy(r, cookie.StaticProxyCookie, config.Global.ContentProxies.Static)
}

// GetUgoiraProxy returns the content proxy URL for ugoira.com content.
//
// The proxy URL is retrieved from cookies if available, otherwise falls back
// to the default configuration.
func GetUgoiraProxy(r *http.Request) url.URL {
	return getProxy(r, cookie.UgoiraProxyCookie, config.Global.ContentProxies.Ugoira)
}

// getProxy retrieves a content proxy URL from a cookieName.
//
// If the cookie value is present but fails to parse, the provided
// defaultProxy is returned.
func getProxy(r *http.Request, cookieName cookie.CookieName, defaultProxy url.URL) url.URL {
	value := GetCookie(r, cookieName)

	if value == "" {
		return defaultProxy
	}

	proxyURL, err := url.Parse(value)
	if err != nil {
		return defaultProxy
	}

	return *proxyURL
}
