// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

// The code in this file redirects pixiv.net URLs to ours. It works for libredirect.
//
// Add more redirects in (*Router).DefineRoutes

package router

import (
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// redirectWithQueryParam is a helper function to redirect requests to
// a target path while preserving the specified query parameter.
//
// Example:   /member.php?id=<id>   ->   /users/id
func redirectWithQueryParam(targetPath, preservedParam string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetPath+utils.GetQueryParam(r, preservedParam), http.StatusPermanentRedirect)
	}
}
