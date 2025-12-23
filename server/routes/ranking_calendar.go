// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// RankingCalendarPage generates and renders the ranking calendar page.
func RankingCalendarPage(w http.ResponseWriter, r *http.Request) error {
	var (
		year  string
		month string
	)

	// Parse the date from the query parameter if provided
	date := utils.GetQueryParam(r, "date")

	const expectedDateLength = 10 // YYYY-MM-DD format
	if len(date) == expectedDateLength {
		year = date[:4]
		month = date[5:7]
	} else {
		// Use current date if no date is provided
		now := time.Now()

		year = strconv.Itoa(now.Year())
		month = fmt.Sprintf("%02d", int(now.Month()))
	}

	// Retrieve the ranking calendar data
	data, err := core.GetRankingCalendar(
		r,
		utils.GetQueryParam(r, "mode", core.RankingCalendarDefaultMode),
		year,
		month)
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

	return views.RankingCalendar(data).Render(r.Context(), w)
}
