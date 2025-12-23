// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/assets/components/partials"
	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// recentType represents the content type for which to retrieve recent items.
type recentType string

const (
	recentTypeArtwork recentType = "artwork"
	recentTypeNovel   recentType = "novel"
)

var (
	errNovelRelatedContentNotSupported = errors.New("novel related content not yet supported")
	errUnsupportedRelatedType          = errors.New("unsupported related type")
	errUnsupportedCommentType          = errors.New("unsupported comment type")
)

type recentParams struct {
	Type          recentType
	UserID        string
	RecentWorkIDs []int
}

// relatedType represents the content type for which to retrieve related items.
type relatedType string

const (
	relatedTypeArtwork relatedType = "artwork"
	relatedTypeNovel   relatedType = "novel"
)

type relatedParams struct {
	Type  relatedType
	ID    string
	Limit int
}

// commentType represents the content type for which to retrieve comments.
type commentType string

const (
	commentTypeArtwork commentType = "artwork"
	commentTypeNovel   commentType = "novel"
)

type commentsParams struct {
	Type        commentType
	ID          string
	UserID      string
	SanityLevel core.SanityLevel // For type=artworks
	XRestrict   core.XRestrict   // For type=novel
}

func ArtworkPartial(w http.ResponseWriter, r *http.Request) error {
	illustType, err := strconv.Atoi(utils.GetQueryParam(r, "illusttype"))
	if err != nil {
		return err
	}

	pageCount, err := strconv.Atoi(utils.GetQueryParam(r, "pages"))
	if err != nil {
		return err
	}

	data, err := core.GetArtworkFast(w, r, core.FastIllustParams{
		ID:         utils.GetQueryParam(r, "id"),
		UserID:     utils.GetQueryParam(r, "userid"),
		Username:   utils.GetQueryParam(r, "username"),
		IllustType: core.IllustType(illustType),
		Pages:      pageCount,
	})
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

	return views.ArtworkFullContent(*data).Render(r.Context(), w)
}

func RecentPartial(w http.ResponseWriter, r *http.Request) error {
	var illust core.Illust

	recentWorkIDs, err := parseWorkIDs(utils.GetQueryParam(r, "recentworkids"))
	if err != nil {
		return err
	}

	params := recentParams{
		Type:          recentType(utils.GetQueryParam(r, "type")),
		UserID:        utils.GetQueryParam(r, "userid"),
		RecentWorkIDs: recentWorkIDs,
	}

	switch params.Type {
	case recentTypeArtwork:
		data, err := core.PopulateArtworkRecent(r, params.UserID, params.RecentWorkIDs)
		if err != nil {
			return err
		}

		illust.RecentWorks = data

	case recentTypeNovel:
		return errNovelRelatedContentNotSupported

	default:
		return fmt.Errorf("%w: %s", errUnsupportedRelatedType, params.Type)
	}

	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	return partials.ArtworkRecentWorks(illust).Render(r.Context(), w)
}

func RelatedPartial(w http.ResponseWriter, r *http.Request) error {
	var illust core.Illust

	params := relatedParams{
		Type: relatedType(utils.GetQueryParam(r, "type")),
		ID:   utils.GetQueryParam(r, "id"),
	}

	switch params.Type {
	case relatedTypeArtwork:
		data, err := core.GetArtworkRelated(r, params.ID)
		if err != nil {
			return err
		}

		illust.RelatedWorks = data

	case relatedTypeNovel:
		return errNovelRelatedContentNotSupported

	default:
		return fmt.Errorf("%w: %s", errUnsupportedRelatedType, params.Type)
	}

	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	return partials.ArtworkRelatedWorks(illust).Render(r.Context(), w)
}

func CommentsPartial(w http.ResponseWriter, r *http.Request) error {
	var commentsData *core.CommentsData

	// use most relaxed constraint by default
	sanityLevel := core.SLR18

	if sl := utils.GetQueryParam(r, "sanitylevel"); sl != "" {
		x, err := strconv.Atoi(sl)
		if err != nil {
			return err
		} else {
			sanityLevel = core.SanityLevel(x)
		}
	}

	// use most relaxed constraint by default
	xRestrict := core.R18G

	if xr := utils.GetQueryParam(r, "xrestrict"); xr != "" {
		x, err := strconv.Atoi(xr)
		if err != nil {
			return err
		} else {
			xRestrict = core.XRestrict(x)
		}
	}

	params := commentsParams{
		Type:        commentType(utils.GetQueryParam(r, "type")),
		ID:          utils.GetQueryParam(r, "id"),
		UserID:      utils.GetQueryParam(r, "userid"),
		SanityLevel: sanityLevel,
		XRestrict:   xRestrict,
	}

	switch params.Type {
	case commentTypeArtwork:
		artworkParams := core.ArtworkCommentsParams{
			ID:          params.ID,
			UserID:      params.UserID,
			SanityLevel: params.SanityLevel,
		}

		data, _, err := core.GetArtworkComments(r, artworkParams)
		if err != nil {
			return err
		}

		commentsData = data
	case commentTypeNovel:
		artworkParams := core.NovelCommentsParams{
			ID:        params.ID,
			UserID:    params.UserID,
			XRestrict: params.XRestrict,
		}

		data, _, err := core.GetNovelComments(r, artworkParams)
		if err != nil {
			return err
		}

		commentsData = data
	default:
		return fmt.Errorf("%w: %s", errUnsupportedCommentType, params.Type)
	}

	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	return partials.CommentsModal(partials.CommentsModalProps{CommentsData: commentsData}).Render(r.Context(), w)
}

func DiscoveryPartial(w http.ResponseWriter, r *http.Request) error {
	mode := utils.GetQueryParam(r, "mode", core.SearchDefaultMode(r))

	data, err := core.GetDiscoveryArtworks(r, mode)
	if err != nil {
		return err
	}

	w.Header().Set("Cache-Control", "no-store")

	return partials.DiscoveryArtwork(data, mode).Render(r.Context(), w)
}

const candidatesLim = 9

// TagCompletionsPartial handles tag completion requests.
// Cache for one week; longer term caching is delegated to Vercel's Edge Cache.
func TagCompletionsPartial(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "public, max-age=604800")

	keywords := utils.GetQueryParam(r, "name")
	if keywords == "" {
		// If no keywords are provided, don't error
		return partials.TagCompletionsNoContent(nil).Render(r.Context(), w)
	}

	completions, err := core.GetTagCompletions(r, keywords)
	if err != nil {
		return err
	}

	if completions == nil || len(completions.Candidates) == 0 {
		return partials.TagCompletionsNoContent(&core.KeywordCompletions{}).Render(r.Context(), w)
	}

	// Limit candidates to [candidatesLim].
	if len(completions.Candidates) > candidatesLim {
		completions.Candidates = completions.Candidates[:candidatesLim]
	}

	return partials.TagCompletions(completions, utils.GetQueryParam(r, "category")).Render(r.Context(), w)
}

func StreetPartial(w http.ResponseWriter, r *http.Request) error {
	if untrusted.GetUserToken(r) == "" {
		return NewUnauthorizedError("/", "/street")
	}

	w.Header().Set("Cache-Control", "no-cache")

	// Parse query parameters from GET request
	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	contentIndex, _ := strconv.Atoi(query.Get("content_index_prev"))

	pageData, err := core.GetStreet(r, core.StreetParams{
		Page:             page,
		ContentIndexPrev: contentIndex,
		K:                query.Get("k"),
		Vhi:              query.Get("vhi"),
		Vhm:              query.Get("vhm"),
		Vhn:              query.Get("vhn"),
	})
	if err != nil {
		return err
	}

	// Return only the items and the next loader, not the main page wrapper.
	return partials.StreetItems(*pageData).Render(r.Context(), w)
}

func parseWorkIDs(s string) ([]int, error) {
	if strings.TrimSpace(s) == "" {
		return []int{}, nil
	}

	strIDs := strings.Split(s, ",")
	ids := make([]int, 0, len(strIDs))

	// Convert each string to int and trim any whitespace
	for _, str := range strIDs {
		str = strings.TrimSpace(str)

		id, err := strconv.Atoi(str)
		if err != nil {
			return nil, fmt.Errorf("invalid work ID '%s': %w", str, err)
		}

		ids = append(ids, id)
	}

	return ids, nil
}
