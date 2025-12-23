// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

func LoginPage(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "no-store")

	pageData := views.LoginData{
		Title:            "Sign in",
		LoginReturnPath:  utils.GetQueryParam(r, "loginReturnPath"),
		NoAuthReturnPath: utils.GetQueryParam(r, "noAuthReturnPath"),
	}

	return views.Login(pageData).Render(r.Context(), w)
}
