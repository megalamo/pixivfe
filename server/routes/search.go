// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
Search logic
*/
package routes

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

var (
	errInvalidCategory  = errors.New(`invalid "category" query parameter: `)
	errInvalidPageParam = errors.New(`invalid "page" query parameter, must be a positive integer: `)
)

// SearchPage is the route handler for the search page.
//
// It routes to the correct helper function based on the chosen category.
//
// Requires the "name" query parameter to be set, which is the search query.
func SearchPage(w http.ResponseWriter, r *http.Request) error {
	searchQuery := utils.GetQueryParam(r, "name")

	// Check if the search query is a pixiv URL and redirect if so
	if subpath, found := strings.CutPrefix(searchQuery, "https://pixiv.net/"); found {
		http.Redirect(w, r, "/"+subpath, http.StatusSeeOther)

		return nil
	}

	if subpath, found := strings.CutPrefix(searchQuery, "https://www.pixiv.net/"); found {
		http.Redirect(w, r, "/"+subpath, http.StatusSeeOther)

		return nil
	}

	category := utils.GetQueryParam(r, "category", core.SearchDefaultCategory)
	if !slices.Contains(core.SearchAvailableCategories, category) {
		return fmt.Errorf("%w %q", errInvalidCategory, category)
	}

	queries := core.WorkSearchSettings{
		Name:     strings.TrimSpace(searchQuery),
		Category: category,
		Order:    utils.GetQueryParam(r, "order", string(core.SearchDefaultOrder)),
		Mode:     utils.GetQueryParam(r, "mode", core.SearchDefaultMode(r)),
		Ratio:    utils.GetQueryParam(r, "ratio"),
		Wlt:      utils.GetQueryParam(r, "wlt"),
		Wgt:      utils.GetQueryParam(r, "wgt"),
		Hlt:      utils.GetQueryParam(r, "hlt"),
		Hgt:      utils.GetQueryParam(r, "hgt"),
		Tool:     utils.GetQueryParam(r, "tool"),
		Scd:      utils.GetQueryParam(r, "scd"),
		Ecd:      utils.GetQueryParam(r, "ecd"),
		Page:     utils.GetQueryParam(r, "page", core.SearchDefaultPage),
	}

	pageInt, err := strconv.Atoi(queries.Page)
	if err != nil {
		return err
	}

	if pageInt < 1 {
		return fmt.Errorf("%w %q", errInvalidPageParam, queries.Page)
	}

	var result *core.SearchData
	if category == core.SearchUsersCategory {
		result, err = core.GetSearchUsers(r, queries)
	} else {
		result, err = core.GetSearch(r, queries)
	}

	if err != nil {
		return err
	}

	// Set the page number
	result.CurrentPage = pageInt

	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	// Preload image for tag cover artwork (only for non-user searches)
	if category != core.SearchUsersCategory && result.Tag.CoverArtwork.Thumbnails.Webp_1200 != "" {
		PreloadImage(w, result.Tag.CoverArtwork.Thumbnails.Webp_1200)
	}

	if config.Global.Response.EarlyHintsResponses {
		w.WriteHeader(http.StatusEarlyHints)
	}

	return views.SearchPage(*result).Render(r.Context(), w)
}
