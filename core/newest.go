// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"fmt"
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

const (
	// NewestDefaultCategory is the default category for the Newest page.
	NewestDefaultCategory string = "illust"

	// NewestDefaultR18 is the default value for the R18 filter on the Newest page.
	//
	// NOTE: "mode" is not used as the pixiv API does not support R-18G for the Newest endpoint.
	NewestDefaultR18 string = "false"

	// lastID is the hardcoded last ID used for fetching the newest artworks.
	lastID string = "0"

	// newestPageSize is the number of works to fetch for the Newest page.
	//
	// NOTE: this is the upper limit enforced by the pixiv API.
	newestPageSize string = "20"
)

// NewestData defines the data used to render the Newest page.
type NewestData struct {
	newestIllustMangaResponse
	newestNovelResponse

	Title string
}

// newestIllustMangaResponse defines the API response structure
// for /ajax/illust/new?type=illust and /ajax/illust/new?type=manga.
type newestIllustMangaResponse struct {
	IllustManga []ArtworkItem `json:"illusts"`
	// LastID      string         `json:"last_id"`
}

// newestNovelResponse defines the API response structure for /ajax/novel/new.
type newestNovelResponse struct {
	Novels []*NovelBrief `json:"novels"`
	// LastID   string       `json:"last_id"`
}

func GetNewestIllustManga(r *http.Request, category, r18 string) (*NewestData, error) {
	var data newestIllustMangaResponse

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetNewestIllustMangaURL(newestPageSize, category, r18, lastID),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, resp), &data)
	if err != nil {
		return nil, err
	}

	// Populate thumbnails for each artwork
	for _, artwork := range data.IllustManga {
		if err := artwork.PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for artwork ID %s: %w", artwork.ID, err)
		}
	}

	return &NewestData{
		Title:                     "Newest works",
		newestIllustMangaResponse: data,
	}, nil
}

func GetNewestNovel(r *http.Request, r18 string) (*NewestData, error) {
	var data newestNovelResponse

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetNewestNovelURL(newestPageSize, r18, lastID),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, resp), &data)
	if err != nil {
		return nil, err
	}

	// Convert RawTags to Tags for each novel
	for i := range data.Novels {
		data.Novels[i].Tags = data.Novels[i].RawTags.ToTags()
	}

	return &NewestData{
		Title:               "Newest novels",
		newestNovelResponse: data,
	}, nil
}
