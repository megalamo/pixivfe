// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

func NovelSeriesPage(w http.ResponseWriter, r *http.Request) error {
	seriesID := utils.GetPathVar(r, "id")
	if _, err := strconv.Atoi(seriesID); err != nil {
		return fmt.Errorf("invalid ID: %s", seriesID)
	}

	currentPage, err := strconv.Atoi(utils.GetQueryParam(r, "p", core.NovelSeriesDefaultPage))
	if err != nil || currentPage < 1 {
		return fmt.Errorf("invalid page number: %d", currentPage)
	}

	pageData, err := core.GetNovelSeries(r, seriesID, currentPage)
	if err != nil {
		return err
	}

	// Validate page number against calculated limit
	if currentPage > pageData.MaxPage && pageData.MaxPage > 0 {
		return fmt.Errorf("invalid page number: %d", currentPage)
	}

	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	return views.NovelSeriesPage(*pageData).Render(r.Context(), w)
}
