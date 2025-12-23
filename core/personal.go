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
	NewestFromFollowingDefaultCategory = "illust"
	NewestFromFollowingDefaultPage     = 1
	NewestFromFollowingDefaultPageStr  = "1"
)

// FollowingData defines the data used to render the Following page.
type FollowingData struct {
	Title       string
	Data        newestFromFollowingResponse
	CurrentPage int
}

type newestFromFollowingResponse struct {
	Page struct {
		IDs        []int `json:"ids"`
		IsLastPage bool  `json:"isLastPage"`
		Tags       Tags  `json:"-"` // a "tags" field exists in the response, but isn't populated
	} `json:"page"`
	TagTranslation TagTranslationWrapper `json:"tagTranslation"`
	Thumbnails     struct {
		Illust      []ArtworkItem `json:"illust"`
		Novel       []NovelBrief  `json:"novel"`
		NovelSeries []NovelSeries `json:"novelSeries"`
		NovelDraft  []any         `json:"novelDraft"`
		Collection  []any         `json:"collection"`
	} `json:"thumbnails"`
	IllustSeries []IllustSeries `json:"illustSeries"`
	Requests     []any          `json:"requests"`
	Users        []User         `json:"users"`
}

func GetNewestFromFollowing(r *http.Request, contentType, mode, page string) (newestFromFollowingResponse, error) {
	var data newestFromFollowingResponse

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetNewestFromFollowingURL(contentType, mode, page),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return newestFromFollowingResponse{}, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, resp), &data)
	if err != nil {
		return newestFromFollowingResponse{}, err
	}

	// Convert tag translations to Tag objects
	data.Page.Tags = data.TagTranslation.ToTags(nil)

	// Populate thumbnails for each artwork and filter R-18 content if mode is "safe"
	//
	// We need to do this manually as the pixiv API doesn't
	// support this functionality natively
	filteredIllusts := make([]ArtworkItem, 0, len(data.Thumbnails.Illust))

	for _, artwork := range data.Thumbnails.Illust {
		if mode == "safe" && artwork.XRestrict > 0 {
			continue
		}

		if err := artwork.PopulateThumbnails(); err != nil {
			return newestFromFollowingResponse{},
				fmt.Errorf("failed to populate thumbnails for artwork ID %s: %w", artwork.ID, err)
		}

		filteredIllusts = append(filteredIllusts, artwork)
	}

	data.Thumbnails.Illust = filteredIllusts

	return data, nil
}
