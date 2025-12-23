// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
)

// SPximgProxy handles requests for static assets from s.pximg.net.
func SPximgProxy(w http.ResponseWriter, r *http.Request) error {
	return requests.ProxyHandler(w, r, "https://s.pximg.net/")
}

// IPximgProxy handles requests for image assets from i.pximg.net.
func IPximgProxy(w http.ResponseWriter, r *http.Request) error {
	return requests.ProxyHandler(w, r, "https://i.pximg.net/")
}

// BoothPximgProxy handles requests for image assets from booth.pximg.net.
func BoothPximgProxy(w http.ResponseWriter, r *http.Request) error {
	return requests.ProxyHandler(w, r, "https://booth.pximg.net/")
}

// UgoiraProxy handles requests for video assets from ugoira.com.
func UgoiraProxy(w http.ResponseWriter, r *http.Request) error {
	return requests.ProxyHandler(w, r, "https://ugoira.com/api/mp4/")
}

// EmbedPixivProxy handles requests for image assets from embed.pixiv.net.
//
// Used by package pixivision.
func EmbedPixivProxy(w http.ResponseWriter, r *http.Request) error {
	return requests.ProxyHandler(w, r, "https://embed.pixiv.net/")
}

// SourcePixivProxy handles requests for image assets from source.pixiv.net.
//
// Used by package pixivision.
func SourcePixivProxy(w http.ResponseWriter, r *http.Request) error {
	return requests.ProxyHandler(w, r, "https://source.pixiv.net/")
}
