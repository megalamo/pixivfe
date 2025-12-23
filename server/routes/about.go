// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
)

// AboutPage is the handler for the /about page.
func AboutPage(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
		int(config.Global.HTTPCache.MaxAge.Seconds()),
		int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))

	pageData := views.AboutData{
		Title:          "About",
		Version:        config.BuildVersion,
		Time:           config.Global.Instance.StartingTime,
		ImageProxy:     config.Global.ContentProxies.Image.String(),
		AcceptLanguage: config.Global.Request.AcceptLanguage,
	}

	return views.About(pageData).Render(r.Context(), w)
}
