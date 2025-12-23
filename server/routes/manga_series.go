// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// MangaSeriesPage is the route handler for the Manga Series page.
func MangaSeriesPage(w http.ResponseWriter, r *http.Request) error {
	userID := utils.GetPathVar(r, "user_id")
	if _, err := strconv.Atoi(userID); err != nil {
		return fmt.Errorf("invalid user ID: %s", userID)
	}

	seriesID := utils.GetPathVar(r, "series_id")
	if _, err := strconv.Atoi(seriesID); err != nil {
		return fmt.Errorf("invalid series ID: %s", seriesID)
	}

	currentPage, err := strconv.Atoi(utils.GetQueryParam(r, "page", core.MangaSeriesDefaultPage))
	if err != nil || currentPage < 1 {
		return fmt.Errorf("invalid page")
	}

	pageData, err := core.GetMangaSeriesByID(r, userID, seriesID, currentPage)
	if err != nil {
		return err
	}

	// Handle redirect if user_id doesn't match the series owner
	if userID != pageData.IllustSeries[0].UserID {
		redirectURL := fmt.Sprintf("/users/%s/series/%s", pageData.IllustSeries[0].UserID, seriesID)
		if currentPage != 1 {
			redirectURL += fmt.Sprintf("?page=%d", currentPage)
		}

		http.Redirect(w, r, redirectURL, http.StatusPermanentRedirect)

		return nil
	}

	// Calculate pagination limits
	maxPage := int(math.Ceil(float64(pageData.IllustSeries[0].Total) / float64(core.MangaSeriesPageSize)))
	if currentPage > maxPage {
		return fmt.Errorf("invalid page")
	}

	// Set cache headers
	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	return views.MangaSeries(*pageData, currentPage, maxPage).Render(r.Context(), w)
}
