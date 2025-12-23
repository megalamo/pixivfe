// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"net/http"

	devviews "codeberg.org/pixivfe/pixivfe/v3/assets/views/dev"
)

// ComponentsPage is the handler for the /dev/components page.
func ComponentsPage(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "no-store")

	return devviews.Components().Render(r.Context(), w)
}
