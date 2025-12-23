// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tidwall/gjson"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

const (
	IndexDefaultMode = "all"

	indexRankingContent = "illust"
)

// indexModeToRankingMode maps index page modes to their corresponding ranking modes.
var indexModeToRankingMode = map[string]string{
	"all": "daily",
	"r18": "daily_r18",
}

type IndexData struct {
	Title       string
	Data        IndexArtworks
	NoTokenData RankingData
}

// Pages is a helper struct for parsing pixiv's API.
// Each field should represent a section of the index page.
// Some fields' type can be a separate struct.
// Structs made specifically for this struct must all have the suffix "SN" to distinguish themselves from other structs.
type Pages struct {
	Pixivision       []Pixivision       `json:"pixivision"`
	Follow           []int              `json:"follow"`
	Recommended      RecommendedSN      `json:"recommend"`
	RecommendedByTag []RecommendByTagSN `json:"recommendByTag"`
	RecommendUser    []RecommendUserSN  `json:"recommendUser"`
	TrendingTag      []TrendingTagSN    `json:"trendingTags"`
	Newest           []string           `json:"newPost"`

	// Commented out fields that aren't currently implemented in the frontend
	// EditorRecommended []any `json:"editorRecommend"`
	// RecommendedUsers   []RecommendedUser `json:"recommendUser"`
	// Commission        []any `json:"completeRequestIds"`
	// BoothFollows []BoothFollow `json:"boothFollowItemIds"`
	// OngoingContests []Contest `json:"contestOngoing"`
	// ContestResult []Contest `json:"contestResult"`
	// FavoriteTags []any `json:"myFavoriteTags"`
	// MyPixiv []any `json:"mypixiv"`
	// "ranking",
	// "sketchLiveFollowIds",
	// "sketchLivePopularIds",
	// "tags",
	// "trendingTags",
	// "userEventIds"

	// These aren't included as pages
	// "requests"
	// "illustSeries"
	// SketchLives []SketchLive `json:"sketchLives"`
	// BoothItems []BoothItem `json:"boothItems"`
}

type TrendingTagSN struct {
	Name         string `json:"tag"`
	TrendingRate int    `json:"trendingRate"`
	IDs          []int  `json:"ids"`
}

type RecommendedSN struct {
	IDs []string `json:"ids"`
}

type RecommendByTagSN struct {
	Name string   `json:"tag"`
	IDs  []string `json:"ids"`
}

type RequestSN struct {
	RequestID       string   `json:"requestId"`
	PlanID          string   `json:"planId"`
	CreatorUserID   string   `json:"creatorUserId"`
	RequestTags     []string `json:"requestTags"`
	RequestProposal struct {
		RequestProposalHTML string `json:"requestOriginalProposalHtml"`
	}
	PostWork struct {
		PostWorkID string `json:"postWorkId"`
	} `json:"postWork"`
}

// Pixivision represents a Pixivision article as returned by the landing page endpoint.
//
// Note that this type is less complete than the PixivisionArticle type.
type Pixivision struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnailUrl"`
	URL       string `json:"url"`
}

// RecommendUser represents a recommended user as returned by the landing page endpoint.
//
// This type is distinct from the User type.
type RecommendUserSN struct {
	ID        int      `json:"id"`
	IllustIDs []string `json:"illustIds"`
	NovelIDs  []string `json:"novelIds"` // Unimplemented
}

// RecommendedTags groups artworks under a specific tag recommendation.
type RecommendedTags struct {
	Name     string
	Artworks []ArtworkItem
}

type PopularTag struct {
	Name         string
	TrendingRate int
	IDs          []int
	Artworks     []ArtworkItem
}

type Request struct {
	Description string
	Tags        []string
	Artwork     ArtworkItem
}

// IndexArtworks aggregates various categories of artworks and other related data
// to be displayed on the landing page.
type IndexArtworks struct {
	Commissions     []ArtworkItem
	Following       []ArtworkItem
	Recommended     []ArtworkItem
	Newest          []ArtworkItem
	Rankings        RankingData
	Users           []ArtworkItem
	Pixivision      []Pixivision
	RecommendByTags []RecommendedTags
	RecommendUser   []User
	PopularTag      []PopularTag
	Requests        []Request
}

// GetIndex retrieves and organizes the index page data based on the provided mode.
//
// It fetches raw data from the landing URL, parses the JSON response, and populates the IndexData struct.
func GetIndex(r *http.Request, mode string) (*IndexData, error) {
	var pages Pages

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetLandingURL(mode),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	resp = RewriteEscapedImageURLs(r, resp)

	// Populate thumbnails for each artwork
	artworks := parseArtworks(resp)
	for id, artwork := range artworks {
		if err := artwork.PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for artwork ID %s: %w", id, err)
		}

		artworks[id] = artwork
	}

	if err := json.Unmarshal([]byte(gjson.GetBytes(resp, "page").Raw), &pages); err != nil {
		return nil, err
	}

	landing := IndexArtworks{
		Pixivision: pages.Pixivision,
	}

	// If the user is logged in, populate personalized sections
	if untrusted.GetUserToken(r) != "" {
		landing.Following, err = populateFollowing(r, mode, pages.Follow, artworks)
		if err != nil {
			return nil, err
		}

		landing.RecommendByTags = populateRecommendedByTags(pages.RecommendedByTag, artworks)
		landing.Recommended = populateArtworks(pages.Recommended.IDs, artworks)
		landing.RecommendUser = populateRecommendUsers(pages.RecommendUser, parseUsers(resp), artworks)
		landing.PopularTag = populatePopularTags(pages.TrendingTag, artworks)

		landing.Requests, err = parsePixivRequests(resp, artworks)
		if err != nil {
			return nil, err
		}
	}

	landing.Rankings, err = fetchRankings(r, mode)
	// landing.Newest = populateArtworks(pages.Newest, artworks)
	if err != nil {
		return nil, err
	}

	indexData := &IndexData{
		Title:       "Landing",
		Data:        landing,
		NoTokenData: RankingData{},
	}

	return indexData, nil
}

// parseArtworks extracts artwork information from the "thumbnails.illust" field
// of the JSON response and maps them by their ID.
func parseArtworks(resp []byte) map[string]ArtworkItem {
	artworks := make(map[string]ArtworkItem)

	gjson.GetBytes(resp, "thumbnails.illust").ForEach(func(_, value gjson.Result) bool {
		var artwork ArtworkItem
		if err := json.Unmarshal([]byte(value.Raw), &artwork); err != nil {
			return false
		}

		if artwork.ID != "" {
			artworks[artwork.ID] = artwork
		}

		return true
	})

	return artworks
}

// parseUsers extracts user information from the "users" field
// of the JSON response and maps them by their ID.
func parseUsers(resp []byte) map[string]User {
	users := make(map[string]User)

	gjson.GetBytes(resp, "users").ForEach(func(_, value gjson.Result) bool {
		var user User
		if err := json.Unmarshal([]byte(value.String()), &user); err != nil {
			return false
		}

		if user.ID != "" {
			users[user.ID] = user
		}

		return true
	})

	return users
}

// populateArtworks is a generic helper function that maps a slice of string IDs
// to their corresponding ArtworkBrief objects.
func populateArtworks(ids []string, artworks map[string]ArtworkItem) []ArtworkItem {
	populated := make([]ArtworkItem, 0, len(ids))

	for _, id := range ids {
		if artwork, exists := artworks[id]; exists {
			populated = append(populated, artwork)
		}
	}

	return populated
}

// populateFollowing converts int IDs to strings and uses the generic helper.
//
// If followIDs is empty, it attempts population by calling GetNewestFromFollowing instead (pixiv moment).
func populateFollowing(r *http.Request, mode string, followIDs []int, artworks map[string]ArtworkItem) ([]ArtworkItem, error) {
	if len(followIDs) == 0 {
		data, err := GetNewestFromFollowing(r, NewestFromFollowingDefaultCategory, mode, NewestFromFollowingDefaultPageStr)
		if err != nil {
			return nil, err
		}

		return data.Thumbnails.Illust, nil
	}

	stringIDs := make([]string, len(followIDs))
	for i, id := range followIDs {
		stringIDs[i] = strconv.Itoa(id)
	}

	return populateArtworks(stringIDs, artworks), nil
}

// populateRecommendedByTags uses the generic helper for each tag's IDs.
func populateRecommendedByTags(recommends []RecommendByTagSN, artworks map[string]ArtworkItem) []RecommendedTags {
	recommendByTags := make([]RecommendedTags, 0, len(recommends))

	for _, recommend := range recommends {
		artworksList := populateArtworks(recommend.IDs, artworks)

		recommendByTags = append(recommendByTags, RecommendedTags{
			Name:     recommend.Name,
			Artworks: artworksList,
		})
	}

	return recommendByTags
}

// populateRecommendUsers is a generic helper function.
func populateRecommendUsers(recommendUsers []RecommendUserSN, users map[string]User, artworks map[string]ArtworkItem) []User {
	populated := make([]User, 0, len(recommendUsers))

	for _, recommendUser := range recommendUsers {
		if user, exists := users[strconv.Itoa(recommendUser.ID)]; exists {
			// Populate the user's artworks using their illustIds
			user.Artworks = populateArtworks(recommendUser.IllustIDs, artworks)
			// Process user comment URLs
			user.Comment = parseDescriptionURLs(user.Comment)
			populated = append(populated, user)
		}
	}

	return populated
}

func populatePopularTags(tags []TrendingTagSN, artworks map[string]ArtworkItem) []PopularTag {
	populated := make([]PopularTag, 0, len(tags))

	for _, tag := range tags {
		const maxPopularTagArtworks = 3

		ids := make([]string, 0, maxPopularTagArtworks)
		for _, workID := range tag.IDs {
			ids = append(ids, strconv.Itoa(workID))
		}

		artworksList := populateArtworks(ids, artworks)

		populated = append(populated, PopularTag{
			Name:         tag.Name,
			TrendingRate: tag.TrendingRate,
			Artworks:     artworksList,
		})
	}

	return populated
}

func parsePixivRequests(resp []byte, artworks map[string]ArtworkItem) ([]Request, error) {
	var requests []RequestSN

	if err := json.Unmarshal([]byte(gjson.GetBytes(resp, "requests").Raw), &requests); err != nil {
		return nil, err
	}

	populated := make([]Request, 0, len(requests))

	for _, request := range requests {
		desc := request.RequestProposal.RequestProposalHTML
		tags := request.RequestTags
		artwork := artworks[request.PostWork.PostWorkID]

		populated = append(populated, Request{
			Description: desc,
			Tags:        tags,
			Artwork:     artwork,
		})
	}

	return populated, nil
}

// fetchRankings retrieves the current rankings based on the selected mode.
//
// It maps the landing page mode to the appropriate ranking mode and fetches the ranking data.
func fetchRankings(r *http.Request, mode string) (RankingData, error) {
	rankingMode, exists := indexModeToRankingMode[mode]
	if !exists {
		rankingMode = RankingDefaultMode
	}

	return GetRanking(r, rankingMode, indexRankingContent, RankingDefaultDate, RankingDefaultPage)
}
