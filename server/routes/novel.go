// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"net/http"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

func NovelPage(w http.ResponseWriter, r *http.Request) error {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		w.Header().Add("Server-Timing", fmt.Sprintf("total;dur=%.0f;desc=\"Total Time\"", float64(duration.Milliseconds())))
	}()

	id := utils.GetPathVar(r, "id")

	// Fetch all novel page data
	pageData, err := core.GetNovelPageData(r, id)
	if err != nil {
		return err
	}

	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	return views.Novel(*pageData).Render(r.Context(), w)
}
