// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

const (
	RankingDefaultMode    = "daily"
	RankingDefaultContent = "all"
	RankingDefaultDate    = "" // RankingDefaultDate is effectively an alias for the current date.
	RankingDefaultPage    = "1"

	// maxPages defines the maximum number of pages for a given ranking date.
	maxPages = 28
)

var (
	errParseRankingDate     = errors.New("failed to parse ranking date")
	errNoIllustrationsFound = errors.New("no illustrations were found for the given date and mode")
	errFetchDetails         = errors.New("failed to fetch details")
	errUnmarshalDetailsData = errors.New("failed to unmarshal details data")
	errFetchRanking         = errors.New("failed to fetch ranking")
	errUnmarshalRankingData = errors.New("failed to unmarshal ranking data")
)

var (
	// modeDisplay defines the display names for different ranking modes.
	modeDisplay = map[string]string{
		"daily":        "Daily",
		"weekly":       "Weekly",
		"monthly":      "Monthly",
		"rookie":       "Weekly rookie",
		"original":     "Weekly original",
		"daily_ai":     "Daily AI-generated",
		"male":         "Popular among males",
		"female":       "Popular among females",
		"daily_r18":    "Daily R-18",
		"weekly_r18":   "Weekly R-18",
		"r18g":         "Weekly R-18G",
		"daily_r18_ai": "Daily R-18 AI-generated",
		"male_r18":     "Popular among males - R-18",
		"female_r18":   "Popular among females - R-18",
	}

	// typeDisplay defines the display names for different content types.
	typeDisplay = map[string]string{
		"all":    "",
		"illust": "illustration",
		"manga":  "manga",
		"ugoira": "ugoira",
	}
)

type RankingData struct {
	Contents    []ArtworkItem
	Title       string
	Mode        string
	Content     string
	Page        int
	MaxPages    int
	TotalItems  int
	CurrentDate string
	PrevDate    string
	NextDate    string
}

type RankingResponse struct {
	RankingDate string `json:"rankingDate"`
	Ranking     []struct {
		IllustID string `json:"illustId"`
		Rank     int    `json:"rank"`
	} `json:"ranking"`
}

func GetRanking(r *http.Request, mode, contentType, date, page string) (RankingData, error) {
	var (
		data                                        RankingData
		currentDate, prevDate, nextDate, rankingURL string
	)

	pageInt, _ := strconv.Atoi(page)

	if date == "" {
		rankingURL = GetRankingURL(mode, contentType, "", page)
	} else {
		var err error

		currentDate, prevDate, nextDate, err = getDateRange(date)
		if err != nil {
			return data, fmt.Errorf("get date range: %w", err)
		}

		rankingURL = GetRankingURL(mode, contentType, currentDate, page)
	}

	cookies := map[string]string{
		"PHPSESSID": untrusted.GetUserToken(r),
	}

	rankingResp, err := fetchRanking(r, rankingURL, cookies)
	if err != nil {
		return data, err
	}

	// Handle currentDate when date parameter is empty
	if date == "" {
		currentDate = rankingResp.RankingDate

		parsedCurrentDate, err := time.Parse("2006-01-02", currentDate)
		if err != nil {
			return data, fmt.Errorf("%w %s: %w", errParseRankingDate, currentDate, err)
		}

		prevDate = parsedCurrentDate.AddDate(0, 0, -1).Format("2006-01-02")
		nextDate = parsedCurrentDate.AddDate(0, 0, 1).Format("2006-01-02")
	}

	// Prepare illust IDs and rank map
	illustIDs := make([]string, 0, len(rankingResp.Ranking))
	rankMap := make(map[string]int)

	for _, item := range rankingResp.Ranking {
		illustIDs = append(illustIDs, item.IllustID)
		rankMap[item.IllustID] = item.Rank
	}

	if len(illustIDs) == 0 {
		return data, errNoIllustrationsFound
	}

	// Fetch details
	var detailsData illustDetailsManyResponse

	detailsResp, err := requests.GetJSONBody(
		r.Context(),
		GetIllustDetailsManyURL(illustIDs),
		cookies,
		r.Header)
	if err != nil {
		return data, fmt.Errorf("%w: %w", errFetchDetails, err)
	}

	if err := json.Unmarshal(RewriteEscapedImageURLs(r, detailsResp), &detailsData); err != nil {
		return data, fmt.Errorf("%w: %w", errUnmarshalDetailsData, err)
	}

	// Merge ranks.
	detailsMap := make(map[string]TouchArtwork)
	for _, detail := range detailsData.IllustDetails {
		detailsMap[detail.ID] = detail
	}

	// Combine ranking and details.
	var orderedContents []ArtworkItem

	for _, rankingItem := range rankingResp.Ranking {
		if detail, exists := detailsMap[rankingItem.IllustID]; exists {
			// Set the rank from the ranking response.
			detail.Rank = rankingItem.Rank

			// Populate thumbnails for the artwork.
			thumbnails, err := PopulateThumbnailsFor(detail.URL)
			if err != nil {
				return RankingData{}, err
			}

			detail.Thumbnails = thumbnails

			// Convert TouchArtwork to ArtworkBrief
			artworkBrief := convertTouchArtworkToArtworkBrief(detail)

			orderedContents = append(orderedContents, artworkBrief)
		}
	}

	return RankingData{
		Contents:    orderedContents,
		Title:       generateRankingTitle(mode, contentType),
		Mode:        mode,
		Content:     contentType,
		Page:        pageInt,
		MaxPages:    maxPages,
		TotalItems:  len(rankingResp.Ranking),
		CurrentDate: currentDate,
		PrevDate:    prevDate,
		NextDate:    nextDate,
	}, nil
}

// fetchRanking fetches and unmarshals the raw ranking data.
func fetchRanking(r *http.Request, rankingURL string, cookies map[string]string) (RankingResponse, error) {
	var data RankingResponse

	resp, err := requests.GetJSONBody(r.Context(), rankingURL, cookies, r.Header)
	if err != nil {
		return data, fmt.Errorf("%w: %w", errFetchRanking, err)
	}

	if err := json.Unmarshal(RewriteEscapedImageURLs(r, resp), &data); err != nil {
		return data, fmt.Errorf("%w: %w", errUnmarshalRankingData, err)
	}

	return data, nil
}

func getDateRange(date string) (string, string, string, error) {
	const dateFormat = "2006-01-02"

	if date == "" {
		now := time.Now().Add(PixivTimeOffset)

		return now.AddDate(0, 0, -1).Format(dateFormat),
			now.AddDate(0, 0, -2).Format(dateFormat),
			now.Format(dateFormat),
			nil
	}

	parsedDate, err := time.Parse(dateFormat, date)
	if err != nil {
		return "", "", "", fmt.Errorf("parse date: %w", err)
	}

	parsedDate = parsedDate.Add(PixivTimeOffset)

	return parsedDate.Format(dateFormat),
		parsedDate.AddDate(0, 0, -1).Format(dateFormat),
		parsedDate.AddDate(0, 0, 1).Format(dateFormat),
		nil
}

// generateRankingTitle generates a human-readable title based on mode and content type.
func generateRankingTitle(mode, contentType string) string {
	modePart, ok := modeDisplay[mode]
	if !ok {
		modePart = mode
	}

	typePart := typeDisplay[contentType]
	if typePart == "" {
		return modePart + " rankings"
	}

	return fmt.Sprintf("%s %s rankings", modePart, typePart)
}
