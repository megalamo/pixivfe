// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/config"
)

const (
	BookmarksPageSize       = 48 // For both illustrations and novels
	UserFollowersPageSize   = 100
	ArtworkCommentsPageSize = 1000
	NovelCommentsPageSize   = 1000
)

var (
	errSearchNameEmpty     = errors.New("search name cannot be empty")
	errSearchCategoryEmpty = errors.New("search category cannot be empty")
)

// GET endpoints

func GetNewestIllustMangaURL(pageSize, workType, r18, lastID string) string {
	base := "https://www.pixiv.net/ajax/illust/new?limit=%s&type=%s&r18=%s&lastId=%s"

	return fmt.Sprintf(base, pageSize, workType, r18, lastID)
}

func GetNewestNovelURL(pageSize, r18, lastID string) string {
	base := "https://www.pixiv.net/ajax/novel/new?limit=%s&r18=%s&lastId=%s"

	return fmt.Sprintf(base, pageSize, r18, lastID)
}

func GetDiscoveryURL(mode string, limit int) string {
	base := "https://www.pixiv.net/ajax/discovery/artworks?mode=%s&limit=%d"

	return fmt.Sprintf(base, mode, limit)
}

func GetDiscoveryNovelURL(mode string, limit int) string {
	base := "https://www.pixiv.net/ajax/discovery/novels?mode=%s&limit=%d"

	return fmt.Sprintf(base, mode, limit)
}

func GetDiscoveryUserURL(limit int) string {
	base := "https://www.pixiv.net/ajax/discovery/users?limit=%d"

	return fmt.Sprintf(base, limit)
}

func GetRankingURL(mode, contentType, date, page string) string {
	params := url.Values{}
	params.Add("mode", mode)
	params.Add("type", contentType)
	params.Add("page", page)

	if date != "" {
		params.Add("date", date)
	}

	return "https://www.pixiv.net/touch/ajax/ranking/illust?" + params.Encode()
}

func GetIllustDetailsManyURL(illustIDs []string) string {
	params := url.Values{}

	for _, id := range illustIDs {
		params.Add("illust_ids[]", id)
	}

	return "https://www.pixiv.net/touch/ajax/illust/details/many?" + params.Encode()
}

func GetRankingCalendarURL(mode string, year, month int) string {
	base := "https://www.pixiv.net/ranking_log.php?mode=%s&date=%d%02d"

	return fmt.Sprintf(base, mode, year, month)
}

func GetUserInformationURL(userID, full string) string {
	base := "https://www.pixiv.net/ajax/user/%s?full=%s"

	return fmt.Sprintf(base, userID, full)
}

func GetUserWorksURL(userID string) string {
	base := "https://www.pixiv.net/ajax/user/%s/profile/all"

	return fmt.Sprintf(base, userID)
}

func GetUserFullArtworkURL(userIDs, illustIDs string) string {
	base := "https://www.pixiv.net/ajax/user/%s/profile/illusts?work_category=illustManga&is_first_page=0&lang=en%s"

	return fmt.Sprintf(base, userIDs, illustIDs)
}

func GetUserFullNovelURL(userID, novelIDs string) string {
	base := "https://www.pixiv.net/ajax/user/%s/profile/novels?is_first_page=0&lang=en%s"

	return fmt.Sprintf(base, userID, novelIDs)
}

func GetUserIllustBookmarksURL(userID, mode string, page int) string {
	base := "https://www.pixiv.net/ajax/user/%s/illusts/bookmarks?tag=&offset=%d&limit=48&rest=%s"

	return fmt.Sprintf(base, userID, page*BookmarksPageSize, mode)
}

func GetUserNovelBookmarksURL(userID, mode string, page int) string {
	base := "https://www.pixiv.net/ajax/user/%s/novels/bookmarks?tag=&offset=%d&limit=48&rest=%s"

	return fmt.Sprintf(base, userID, page*BookmarksPageSize, mode)
}

func GetArtworkFrequentTagsURL(illustIDs string) string {
	base := "https://www.pixiv.net/ajax/tags/frequent/illust?%s"

	return fmt.Sprintf(base, illustIDs)
}

func GetNovelFrequentTagsURL(novelIDs string) string {
	base := "https://www.pixiv.net/ajax/tags/frequent/novel?%s"

	return fmt.Sprintf(base, novelIDs)
}

// Retrieves the users followed by a given user.
//
// The mode parameter controls whether follows that are public ("show")
// or private ("hide") are retrieved.
//
// Attempting to retrieve private follows for a user other than the one
// for which a PHPSESSID is provided returns their public follows instead.
func GetUserFollowingURL(userID string, page, limit int, mode string) string {
	base := "https://www.pixiv.net/ajax/user/%s/following?offset=%d&limit=%d&rest=%s"

	return fmt.Sprintf(base, userID, page*limit, limit, mode)
}

// Retrieves the users following a given user.
//
// Attempting to retrieve followers for a user other than the one
// for which a PHPSESSID is provided returns HTTP 403 for the API request.
func GetUserFollowersURL(userID string, page int) string {
	base := "https://www.pixiv.net/ajax/user/%s/followers?offset=%d&limit=100"

	return fmt.Sprintf(base, userID, page*UserFollowersPageSize)
}

func GetNewestFromFollowingURL(contentType, mode, page string) string {
	base := "https://www.pixiv.net/ajax/follow_latest/%s?mode=%s&p=%s"

	// TODO: Recheck this URL.
	return fmt.Sprintf(base, contentType, mode, page)
}

func GetArtworkInformationURL(illustID string) string {
	base := "https://www.pixiv.net/ajax/illust/%s"

	return fmt.Sprintf(base, illustID)
}

func GetArtworkImagesURL(illustID string) string {
	base := "https://www.pixiv.net/ajax/illust/%s/pages"

	return fmt.Sprintf(base, illustID)
}

func GetArtworkRelatedURL(illustID string, limit int) string {
	base := "https://www.pixiv.net/ajax/illust/%s/recommend/init?limit=%d"

	return fmt.Sprintf(base, illustID, limit)
}

// Retrieves the comments for a given illustration ID.
//
// Unlike other endpoints, the limit parameter doesn't seem to have a maximum.
func GetArtworkCommentsURL(illustID string, page int) string {
	base := "https://www.pixiv.net/ajax/illusts/comments/roots?illust_id=%s&offset=%d&limit=1000"

	return fmt.Sprintf(base, illustID, page*ArtworkCommentsPageSize)
}

// Retrieves the replies for a given comment ID.
//
// Unsure what the page parameter does given the lack of a limit parameter.
func GetArtworkCommentRepliesURL(illustID string, page int) string {
	base := "https://www.pixiv.net/ajax/illusts/comments/replies?comment_id=%s&page=%d"

	return fmt.Sprintf(base, illustID, page)
}

// Retrieves the comments for a given novel ID.
//
// Unlike other endpoints, the limit parameter doesn't seem to have a maximum.
func GetNovelCommentsURL(novelID string, page int) string {
	base := "https://www.pixiv.net/ajax/novels/comments/roots?novel_id=%s&offset=%d&limit=1000"

	return fmt.Sprintf(base, novelID, page*NovelCommentsPageSize)
}

// Retrieves the replies for a given comment ID.
//
// Unsure what the page parameter does given the lack of a limit parameter.
func GetNovelCommentRepliesURL(novelID string, page int) string {
	base := "https://www.pixiv.net/ajax/novels/comments/replies?comment_id=%s&page=%d"

	return fmt.Sprintf(base, novelID, page)
}

func GetTagDetailURL(unescapedTag string) string {
	base := "https://www.pixiv.net/ajax/search/tags/%s"

	unescapedTag = url.PathEscape(unescapedTag)

	return fmt.Sprintf(base, unescapedTag)
}

func GetTagCompletionURL(keyword string) string {
	var base string
	if config.Global.Feature.FastTagSuggestions {
		// Use Vercel proxy for lower latency suggestions via cache
		base = "https://tag-suggestions.vercel.app/api/proxy?keyword=%s&lang=en"
	} else {
		// Use default Pixiv endpoint
		base = "https://www.pixiv.net/rpc/cps.php?keyword=%s&lang=en"
	}

	return fmt.Sprintf(base, url.QueryEscape(keyword))
}

// GetArtworkSearchURL constructs a search URL from the given settings.
func GetArtworkSearchURL(settings WorkSearchSettings) (string, error) {
	if settings.Name == "" {
		return "", errSearchNameEmpty
	}

	if settings.Category == "" {
		return "", errSearchCategoryEmpty
	}

	u := &url.URL{
		Scheme: "https",
		Host:   "www.pixiv.net",
	}

	// Path segments must be escaped. url.PathEscape is insufficient as it allows
	// characters like '&' and '+', which the target API expects to be escaped.
	// We use url.QueryEscape and replace its '+' encoding for spaces with '%20'
	// to produce the required path segment encoding.
	rawPath := fmt.Sprintf("/ajax/search/%s/%s",
		strings.ReplaceAll(url.QueryEscape(settings.Category), "+", "%20"),
		strings.ReplaceAll(url.QueryEscape(settings.Name), "+", "%20"),
	)

	// To use our manually-encoded path, we must set both RawPath and its decoded
	// equivalent in Path. The URL.String method will then prefer our RawPath.
	// We can ignore the error from PathUnescape as we just built a valid string.
	u.RawPath = rawPath
	u.Path, _ = url.PathUnescape(rawPath)

	// Build query parameters. The 'word' parameter is always required.
	query := url.Values{}
	query.Set("word", settings.Name)

	// Add all optional query parameters.
	params := map[string]string{
		"p":      settings.Page,
		"order":  settings.Order,
		"mode":   settings.Mode,
		"ratio":  settings.Ratio,
		"s_mode": settings.Smode,
		"wlt":    settings.Wlt,
		"wgt":    settings.Wgt,
		"hlt":    settings.Hlt,
		"hgt":    settings.Hgt,
		"tool":   settings.Tool,
		"scd":    settings.Scd,
		"ecd":    settings.Ecd,
	}

	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}

	u.RawQuery = query.Encode()

	return u.String(), nil
}

// TODO: i=1 is Creator accounts only. i=0 returns all accounts.
func GetUserSearchURL(query, page string) string {
	baseURL, _ := url.Parse("https://www.pixiv.net/ajax/search/users")

	params := fmt.Sprintf("nick=%s&s_mode=s_usr&i=0&lang=en", url.QueryEscape(query))

	// Add page parameter if it exists
	if page != "" {
		params += "&p=" + url.QueryEscape(page)
	}

	// Set the raw query string
	baseURL.RawQuery = params

	return baseURL.String()
}

func GetLandingURL(mode string) string {
	base := "https://www.pixiv.net/ajax/top/illust?mode=%s"

	return fmt.Sprintf(base, mode)
}

func GetNovelURL(novelID string) string {
	base := "https://www.pixiv.net/ajax/novel/%s"

	return fmt.Sprintf(base, novelID)
}

func GetNovelRelatedURL(novelID string, limit int) string {
	base := "https://www.pixiv.net/ajax/novel/%s/recommend/init?limit=%d"

	return fmt.Sprintf(base, novelID, limit)
}

func GetNovelSeriesURL(seriesID string) string {
	base := "https://www.pixiv.net/ajax/novel/series/%s"

	return fmt.Sprintf(base, seriesID)
}

func GetNovelSeriesContentURL(seriesID string, page, perPage int) string {
	base := "https://www.pixiv.net/ajax/novel/series_content/%s?limit=%d&last_order=%d&order_by=asc"

	return fmt.Sprintf(base, seriesID, perPage, perPage*(page-1))
}

func GetNovelSeriesContentTitlesURL(seriesID int) string {
	base := "https://www.pixiv.net/ajax/novel/series/%d/content_titles"

	return fmt.Sprintf(base, seriesID)
}

func GetInsertIllustURL(novelID, id string) string {
	base := "https://www.pixiv.net/ajax/novel/%s/insert_illusts?id[]=%s"

	return fmt.Sprintf(base, novelID, id)
}

func GetMangaSeriesContentURL(seriesID string, page int) string {
	base := "https://www.pixiv.net/ajax/series/%s?p=%d"

	return fmt.Sprintf(base, seriesID, page)
}

func GetPixivSettingsURL() string {
	base := "https://www.pixiv.net/ajax/settings"

	return base
}

func GetStreetURL() string {
	base := "https://www.pixiv.net/ajax/street/v2/main"

	return base
}

// POST endpoints

func PostAddIllustBookmarkURL() string {
	return "https://www.pixiv.net/ajax/illusts/bookmarks/add"
}

func PostDeleteIllustBookmarkURL() string {
	return "https://www.pixiv.net/ajax/illusts/bookmarks/delete"
}

func PostIllustLikeURL() string {
	return "https://www.pixiv.net/ajax/illusts/like"
}

func PostTouchAPI() string {
	return "https://www.pixiv.net/touch/ajax_api/ajax_api.php"
}
