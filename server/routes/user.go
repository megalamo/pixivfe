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

func UserPage(w http.ResponseWriter, r *http.Request) error {
	data, err := fetchUserData(r)
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

	return views.User(data).Render(r.Context(), w)
}

// fetchUserData parses user profile parameters from the request and fetches the data from the core.
func fetchUserData(r *http.Request) (core.UserData, error) {
	id := utils.GetPathVar(r, "id")
	category := utils.GetQueryParam(r, "category", "")
	mode := utils.GetQueryParam(r, "mode", "show")
	currentPageStr := utils.GetQueryParam(r, "page", "1")

	currentPage, err := strconv.Atoi(currentPageStr)
	if err != nil {
		currentPage = 1
	}

	return core.GetUserProfile(r, id, category, mode, currentPage)
}
