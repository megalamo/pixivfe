// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

const (
	MangaSeriesDefaultPage = "1"

	// MangaSeriesPageSize defines the number of artwork entries per page.
	MangaSeriesPageSize = 12

	// mangaSeriesMaxTags defines the maximum number of tags to return for a manga series.
	mangaSeriesMaxTags = 10

	mangaSeriesDefaultTitle = "Manga series"
)

// MangaSeriesData defines the data used to render a manga series page.
type MangaSeriesData struct {
	mangaSeriesResponse

	Title string
}

// mangaSeriesResponse defines the API response structure for /ajax/series/{ seriesID }.
type mangaSeriesResponse struct {
	Tags       Tags
	RawTags    TagTranslationWrapper `json:"tagTranslation"`
	Thumbnails struct {
		Illust []ArtworkItem `json:"illust"`
	} `json:"thumbnails"`
	IllustSeries []IllustSeries `json:"illustSeries"` // Other manga series by the same user
	Users        []*User        `json:"users"`
	Page         struct {
		Series               []seriesEntry `json:"series"`
		IsSetCover           bool          `json:"isSetCover"`
		SeriesID             int           `json:"seriesId"`
		OtherSeriesID        string        `json:"otherSeriesId"`
		RecentUpdatedWorkIDs []int         `json:"recentUpdatedWorkIds"`
		Total                int           `json:"total"`
		IsWatched            bool          `json:"isWatched"`
		IsNotifying          bool          `json:"isNotifying"`
	} `json:"page"`
}

// IllustSeries represents a specific manga series with its associated artworks.
//
// Essentially a simplified view of a series, analogous to ArtworkBrief.
type IllustSeries struct {
	ID             string        `json:"id"`
	UserID         string        `json:"userId"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	Caption        string        `json:"caption"`
	Total          int           `json:"total"`
	ContentOrder   any           `json:"content_order"`
	Thumbnail      string        `json:"url"`
	CoverImageSl   int           `json:"coverImageSl"`
	FirstIllustID  string        `json:"firstIllustId"`
	LatestIllustID string        `json:"latestIllustId"`
	CreateDate     time.Time     `json:"createDate"`
	UpdateDate     time.Time     `json:"updateDate"`
	WatchCount     any           `json:"watchCount"`
	IsWatched      bool          `json:"isWatched"`
	IsNotifying    bool          `json:"isNotifying"`
	List           []ArtworkItem // Artworks specific to this series
}

// seriesEntry represents an entry in the series page with ordering.
//
// Only used to ensure that the artworks in the main series are ordered correctly.
type seriesEntry struct {
	WorkID string `json:"workId"`
	Order  int    `json:"order"`
}

// GetMangaSeriesByID retrieves the content of a manga series by its ID and page number.
func GetMangaSeriesByID(r *http.Request, userID, id string, page int) (*MangaSeriesData, error) {
	var data mangaSeriesResponse

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetMangaSeriesContentURL(id, page),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, fmt.Errorf("fetching manga series content: %w", err)
	}

	if err := json.Unmarshal(RewriteEscapedImageURLs(r, resp), &data); err != nil {
		return nil, fmt.Errorf("unmarshaling manga series content: %w", err)
	}

	// Determine the effective UserID for fetching user information.
	// Prefer UserID from series data if available and non-empty, otherwise use the provided userID.
	effectiveUserID := userID
	if len(data.IllustSeries) > 0 && data.IllustSeries[0].UserID != "" {
		effectiveUserID = data.IllustSeries[0].UserID
	}

	// Concurrently fetch user information and process artwork data
	var (
		user *User
		g    errgroup.Group
	)

	// Fetch user information
	g.Go(func() error {
		var err error

		user, err = GetUserBasicInformation(r, effectiveUserID)
		if err != nil {
			return fmt.Errorf("fetching user basic information for %s: %w", effectiveUserID, err)
		}

		return nil
	})

	// Process artworks in body (populate thumbnails, group into series, order main series)
	g.Go(func() error {
		illustSeriesMap := make(map[string]*IllustSeries, len(data.IllustSeries))

		for i := range data.IllustSeries {
			series := &data.IllustSeries[i]

			series.List = make([]ArtworkItem, 0)
			illustSeriesMap[series.ID] = series
		}

		// Populate thumbnails for each artwork and assign artworks to their respective series.
		for i := range data.Thumbnails.Illust {
			artwork := &data.Thumbnails.Illust[i]
			if err := artwork.PopulateThumbnails(); err != nil {
				return fmt.Errorf("populating thumbnails for artwork ID %s: %w", artwork.ID, err)
			}

			// If the artwork belongs to a known series, add a copy of it to that series' list.
			if series, exists := illustSeriesMap[artwork.SeriesID]; exists {
				series.List = append(series.List, *artwork)
			}
		}

		// Order the artworks in the main series as specified by body.Page.Series.
		mainSeriesIDStr := strconv.Itoa(data.Page.SeriesID)
		if mainSeries, exists := illustSeriesMap[mainSeriesIDStr]; exists {
			currentArtworksInMainSeries := make(map[string]ArtworkItem, len(mainSeries.List))
			for _, art := range mainSeries.List {
				currentArtworksInMainSeries[art.ID] = art
			}

			// Build the ordered list based on body.Page.Series.
			orderedList := make([]ArtworkItem, 0, len(data.Page.Series))

			for _, entry := range data.Page.Series {
				if art, ok := currentArtworksInMainSeries[entry.WorkID]; ok {
					orderedList = append(orderedList, art)
				}
			}

			mainSeries.List = orderedList // Replace the main series' artwork list with the ordered one.
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Update body.Users with the fetched user information.
	if len(data.Users) > 0 {
		data.Users[0] = user
	} else {
		data.Users = []*User{user}
	}

	// Determine the title for the page.
	pageTitle := mangaSeriesDefaultTitle
	if len(data.IllustSeries) > 0 && data.IllustSeries[0].Title != "" {
		pageTitle = data.IllustSeries[0].Title
	}

	// Convert raw tags to Tags and limit to first 10
	allTags := data.RawTags.ToTags(nil)
	if len(allTags) > mangaSeriesMaxTags {
		data.Tags = allTags[:mangaSeriesMaxTags]
	} else {
		data.Tags = allTags
	}

	// Process URL fields before returning
	for i := range data.IllustSeries {
		data.IllustSeries[i].Caption = parseDescriptionURLs(data.IllustSeries[i].Caption)
	}

	for _, user := range data.Users {
		if user != nil {
			user.Comment = parseDescriptionURLs(user.Comment)
		}
	}

	return &MangaSeriesData{
		Title:               pageTitle,
		mangaSeriesResponse: data,
	}, nil
}
