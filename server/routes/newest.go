// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// NewestPage is the route handler for the Newest page.
func NewestPage(w http.ResponseWriter, r *http.Request) error {
	var pageData *core.NewestData

	category := utils.GetQueryParam(r, "category", core.NewestDefaultCategory)
	r18 := utils.GetQueryParam(r, "r18", core.NewestDefaultR18)

	if category == "novel" {
		temp, err := core.GetNewestNovel(r, r18)
		if err != nil {
			return err
		}

		pageData = temp
	} else {
		temp, err := core.GetNewestIllustManga(r, category, r18)
		if err != nil {
			return err
		}

		pageData = temp
	}

	w.Header().Set("Cache-Control", "no-store")

	return views.Newest(*pageData).Render(r.Context(), w)
}
