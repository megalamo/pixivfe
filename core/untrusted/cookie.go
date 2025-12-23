// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package untrusted

import (
	"net/http"
	"net/url"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// SameSite=Lax allows cookies on top-level navigations, preventing authentication issues
// when users arrive from external links (Strict would require a page refresh).
const CookieSameSite = http.SameSiteLaxMode

// Cookies will expire in 30 days from when they are set.
const cookieMaxAge = 30 * 24 * time.Hour

// Clear a cookie by setting its expiration date to this
var cookieExpireDelete = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

func createCookieUnencoded(name cookie.CookieName, value string, expires time.Time, isSecure bool) http.Cookie {
	return http.Cookie{
		Name:     string(name),
		Value:    value,
		Path:     "/",
		Expires:  expires,
		Secure:   isSecure,
		HttpOnly: cookie.IsHttpOnly(name),
		SameSite: CookieSameSite,
	}
}

func GetCookie(r *http.Request, name cookie.CookieName) string {
	cookie, err := r.Cookie(string(name))
	if err != nil {
		return ""
	}

	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return ""
	}

	return value
}

func SetCookie(w http.ResponseWriter, r *http.Request, name cookie.CookieName, value string) {
	if value == "" {
		ClearCookie(w, r, name)
	} else {
		cookie := createCookieUnencoded(
			name, url.QueryEscape(value),
			time.Now().Add(cookieMaxAge),
			utils.IsConnectionSecure(r))
		http.SetCookie(w, &cookie)
	}
}

func ClearCookie(w http.ResponseWriter, r *http.Request, name cookie.CookieName) {
	cookie := createCookieUnencoded(
		name, "",
		cookieExpireDelete,
		utils.IsConnectionSecure(r))
	http.SetCookie(w, &cookie)
}

func ClearAllCookies(w http.ResponseWriter, r *http.Request) {
	for _, name := range cookie.AllCookieNames {
		ClearCookie(w, r, name)
	}
}
