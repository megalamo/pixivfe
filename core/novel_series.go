// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"math"
	"net/http"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

const (
	NovelSeriesDefaultPage = "1"

	// novelSeriesPageSize defines the number of novel entries per page.
	novelSeriesPageSize = 30
)

// NovelSeriesData defines the data used to render a novel series page.
type NovelSeriesData struct {
	Title               string
	NovelSeries         *NovelSeries
	NovelSeriesContents []*NovelBrief
	User                *User
	CurrentPage         int
	MaxPage             int
}

type NovelSeriesContentTitle struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Available bool   `json:"available"`
}

// NovelSeries defines the API structure of a novel series
// as returned by the /ajax/novel/series/{id} endpoint.
type NovelSeries struct {
	ID                            string    `json:"id"`
	UserID                        string    `json:"userId"`
	UserName                      string    `json:"userName"`
	UserAvatar                    string    `json:"profileImageUrl"`
	XRestrict                     XRestrict `json:"xRestrict"`
	IsOriginal                    bool      `json:"isOriginal"`
	IsConcluded                   bool      `json:"isConcluded"`
	GenreID                       string    `json:"genreId"`
	Title                         string    `json:"title"`
	Caption                       string    `json:"caption"`
	Language                      string    `json:"language"`
	Tags                          Tags
	RawTags                       StringTags `json:"tags"`
	PublishedContentCount         int        `json:"publishedContentCount"`
	PublishedTotalCharacterCount  int        `json:"publishedTotalCharacterCount"`
	PublishedTotalWordCount       int        `json:"publishedTotalWordCount"`
	PublishedReadingTime          int        `json:"publishedReadingTime"`
	UseWordCount                  bool       `json:"useWordCount"`
	LastPublishedContentTimestamp int        `json:"lastPublishedContentTimestamp"`
	CreatedTimestamp              int        `json:"createdTimestamp"`
	UpdatedTimestamp              int        `json:"updatedTimestamp"`
	CreateDate                    time.Time  `json:"createDate"`
	UpdateDate                    time.Time  `json:"updateDate"`
	FirstNovelID                  string     `json:"firstNovelId"`
	LatestNovelID                 string     `json:"latestNovelId"`
	DisplaySeriesContentCount     int        `json:"displaySeriesContentCount"`
	ShareText                     string     `json:"shareText"`
	Total                         int        `json:"total"`
	FirstEpisode                  struct {
		URL string `json:"url"`
	} `json:"firstEpisode"`
	WatchCount   any `json:"watchCount"`
	MaxXRestrict any `json:"maxXRestrict"`
	Cover        struct {
		Urls struct {
			Two40Mw     string `json:"240mw"`
			Four80Mw    string `json:"480mw"`
			One200X1200 string `json:"1200x1200"`
			One28X128   string `json:"128x128"`
			Original    string `json:"original"`
		} `json:"urls"`
	} `json:"cover"`
	CoverSettingData any    `json:"coverSettingData"`
	IsWatched        bool   `json:"isWatched"`
	IsNotifying      bool   `json:"isNotifying"`
	AIType           AIType `json:"aiType"`
	HasGlossary      bool   `json:"hasGlossary"`
}

// novelSeriesContentResponse defines the API response for /ajax/novel/series_content/{id}.
type novelSeriesContentResponse struct {
	TagTranslation TagTranslationWrapper `json:"tagTranslation"` // Estimated, not present in the response tested
	Thumbnails     struct {
		Illust      []*ArtworkItem `json:"illust"` // Estimated, not present in the response tested
		Novel       []*NovelBrief  `json:"novel"`
		NovelSeries []*NovelSeries `json:"novelSeries"` // Estimated, not present in the response tested
		NovelDraft  []*any         `json:"novelDraft"`  // We don't have a type for this, and not present
		Collection  []*any         `json:"collection"`  // We don't have a type for this, and not present
	} `json:"thumbnails"`
	IllustSeries []*IllustSeries `json:"illustSeries"` // Estimated, not present in the response tested
	Request      []*RequestSN    `json:"request"`      // Estimated, not present in the response tested
	Users        []*User         `json:"users"`        // Estimated, not present in the response tested
	Page         struct {
		SeriesContents []*novelSeriesMember `json:"seriesContents"`
	} `json:"page"`
}

// novelSeriesMember defines the API structure of a novel when present in a list under a novel series.
//
// Distinct from objects nested under `thumbnails` arrays (which we model as NovelBrief),
// using Unix timestamps for dates, and adding some fields while omitting others, but generally less detailed.
//
// Naming differences also exist between the two entities, for example `CommentHTML` versus `description`.
//
// NOTE: Prefer using NovelBrief or translate from this type for compatibility in templates.
type novelSeriesMember struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	Series struct {
		ID           int `json:"id"`
		ViewableType int `json:"viewableType"`
		ContentOrder int `json:"contentOrder"`
	} `json:"series"`
	Title             string `json:"title"`
	CommentHTML       string `json:"commentHtml"`
	Tags              Tags
	RawTags           StringTags `json:"tags"`
	Restrict          int        `json:"restrict"`
	XRestrict         XRestrict  `json:"xRestrict"`
	IsOriginal        bool       `json:"isOriginal"`
	TextLength        int        `json:"textLength"`
	CharacterCount    int        `json:"characterCount"`
	WordCount         int        `json:"wordCount"`
	UseWordCount      bool       `json:"useWordCount"`
	ReadingTime       int        `json:"readingTime"`
	Bookmarks         int        `json:"bookmarkCount"`
	CoverURL          string     `json:"url"`
	UploadTimestamp   int        `json:"uploadTimestamp"`
	ReuploadTimestamp int        `json:"reuploadTimestamp"`
	IsBookmarkable    bool       `json:"isBookmarkable"`
	BookmarkData      any        `json:"bookmarkData"`
	AIType            AIType     `json:"aiType"`
}

// GetNovelSeries retrieves a novel series.
func GetNovelSeries(r *http.Request, id string, page int) (*NovelSeriesData, error) {
	var data NovelSeriesData

	seriesResp, err := requests.GetJSONBody(
		r.Context(),
		GetNovelSeriesURL(id),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header,
	)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, seriesResp), &data.NovelSeries)
	if err != nil {
		return nil, err
	}

	data.NovelSeries.Tags = data.NovelSeries.RawTags.ToTags()

	// Get the series contents
	var seriesContentResp novelSeriesContentResponse

	contentResp, err := requests.GetJSONBody(
		r.Context(),
		GetNovelSeriesContentURL(id, page, novelSeriesPageSize),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header,
	)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, contentResp), &seriesContentResp)
	if err != nil {
		return nil, err
	}

	data.NovelSeriesContents = seriesContentResp.Thumbnails.Novel

	for i := range data.NovelSeriesContents {
		novel := data.NovelSeriesContents[i]

		novel.Tags = novel.RawTags.ToTags()
	}

	// Get the user information
	data.User, err = GetUserBasicInformation(r, data.NovelSeries.UserID)
	if err != nil {
		return nil, err
	}

	data.CurrentPage = page
	data.MaxPage = int(math.Ceil(float64(data.NovelSeries.Total) / float64(novelSeriesPageSize)))
	data.Title = data.NovelSeries.Title

	// Process URL fields before returning
	data.NovelSeries.Caption = parseDescriptionURLs(data.NovelSeries.Caption)
	data.User.Comment = parseDescriptionURLs(data.User.Comment)

	for _, novelBrief := range data.NovelSeriesContents {
		if novelBrief != nil {
			novelBrief.Description = parseDescriptionURLs(novelBrief.Description)
		}
	}

	return &data, nil
}
