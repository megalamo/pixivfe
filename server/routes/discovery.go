// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/server/request_context"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

func DiscoveryPage(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "no-store")

	// For all HTMX requests, return async loaded page
	if request_context.FromRequest(r).CommonData.IsHtmxRequest {
		return views.Discovery(core.DiscoveryData{
			Title: "Discovery",
		}).Render(r.Context(), w)
	}

	// For all other requests, return actual content
	works, err := core.GetDiscoveryArtworks(r, utils.GetQueryParam(r, "mode", core.SearchDefaultMode(r)))
	if err != nil {
		return err
	}

	return views.Discovery(core.DiscoveryData{
		Title:    "Discovery",
		Artworks: works,
	}).Render(r.Context(), w)
}

func NovelDiscoveryPage(w http.ResponseWriter, r *http.Request) error {
	works, err := core.GetDiscoveryNovels(r, utils.GetQueryParam(r, "mode", core.SearchDefaultMode(r)))
	if err != nil {
		return err
	}

	w.Header().Set("Cache-Control", "no-store")

	return views.NovelDiscovery(views.NovelDiscoveryData{
		Novels: works,
		Title:  "Discovery",
	}).Render(r.Context(), w)
}

func UserDiscoveryPage(w http.ResponseWriter, r *http.Request) error {
	users, err := core.GetDiscoveryUsers(r)
	if err != nil {
		return err
	}

	return views.UserDiscovery(views.UserDiscoveryData{
		Title: "Discovery",
		Users: users,
	}).Render(r.Context(), w)
}

// Discovery refresh route handlers - provides a redirect back to the
// discovery views.
//
// Separate handlers used to avoid browser form resubmission issues.

func DiscoveryPageRefresh(w http.ResponseWriter, r *http.Request) error {
	// Check for HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Call DiscoveryPage directly instead of redirecting
		return DiscoveryPage(w, r)
	}

	// Default redirect behavior for non-HTMX requests
	redirectURL := "/discovery"
	if r.FormValue("reset") != "on" && r.URL.RawQuery != "" {
		redirectURL += "?" + r.URL.RawQuery
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)

	return nil
}

func NovelDiscoveryPageRefresh(w http.ResponseWriter, r *http.Request) error {
	// Check for HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Call NovelDiscoveryPage directly instead of redirecting
		return NovelDiscoveryPage(w, r)
	}

	// Default redirect behavior for non-HTMX requests
	redirectURL := "/discovery/novel"
	if r.FormValue("reset") != "on" && r.URL.RawQuery != "" {
		redirectURL += "?" + r.URL.RawQuery
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)

	return nil
}

func UserDiscoveryPageRefresh(w http.ResponseWriter, r *http.Request) error {
	// Check for HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Call UserDiscoveryPage directly instead of redirecting
		return UserDiscoveryPage(w, r)
	}

	// Default redirect behavior for non-HTMX requests
	redirectURL := "/discovery/users"
	if r.FormValue("reset") != "on" && r.URL.RawQuery != "" {
		redirectURL += "?" + r.URL.RawQuery
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)

	return nil
}
