// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

// StreetPage is the route handler for the street view.
func StreetPage(w http.ResponseWriter, r *http.Request) error {
	if untrusted.GetUserToken(r) == "" {
		return NewUnauthorizedError("/", "/street")
	}

	w.Header().Set("Cache-Control", "no-cache")

	// The street view is always async loaded.
	return views.Street(core.StreetData{
		Title: "Street",
	}).Render(r.Context(), w)
}
