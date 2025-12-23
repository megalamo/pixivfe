// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

var (
	ArtworkRecentLimit = 20 // Limit for recent artworks

	artworkRelatedLimit = 180 // Limit for related artworks
)

// BookmarkData is a custom type to handle the following API response formats:
//
// Type 1, bookmarked:
//
//	"bookmarkData": {
//	  "id": "1234",
//	  "private": false
//	},
//
// Type 2, not bookmarked:
//
//	"bookmarkData": null
type BookmarkData struct {
	ID      string `json:"id"`
	Private bool   `json:"private"`
}

// ArtworkItem is an illustration or manga that appears in a collection/list.
type ArtworkItem struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	UserID       string        `json:"userId"`
	UserName     string        `json:"userName"`
	UserAvatar   string        `json:"profileImageUrl"`
	Thumbnail    string        `json:"url"`
	Pages        int           `json:"pageCount"`
	XRestrict    XRestrict     `json:"xRestrict"`
	SanityLevel  SanityLevel   `json:"sl"`
	CreateDate   time.Time     `json:"createDate"` // used for user atom feeds
	AIType       AIType        `json:"aiType"`
	BookmarkData *BookmarkData `json:"bookmarkData"`
	IllustType   int           `json:"illustType"`
	Tags         []string      `json:"tags"`     // used by core/popular_search
	SeriesID     string        `json:"seriesId"` // used by core/mangaseries
	SeriesTitle  string        `json:"seriesTitle"`
	Thumbnails   Thumbnails
	Width        int
	Height       int
	Rank         int // Used for ranking data
}

// ShouldHide reports whether the artwork should be hidden according to the
// visibility and blacklist settings stored in the supplied filter profile.
func (work *ArtworkItem) ShouldHide(cookies map[cookie.CookieName]string) bool {
	// A nil artwork has no fields – nothing to hide.
	if work == nil {
		return false
	}

	profile := ReadFilterProfile(cookies[cookie.FilterProfileCookie])

	// AI-generated works.
	if profile.AI == FilterHide && work.AIType == AIGenerated {
		return true
	}

	// Restricted works (R-18 / R-18G).
	switch work.XRestrict {
	case R18:
		if profile.R18 == FilterHide {
			return true
		}
	case R18G:
		if profile.R18G == FilterHide {
			return true
		}
	}

	// Blacklisted user.
	if len(profile.BlacklistedArtists) > 0 {
		if slices.Contains(profile.BlacklistedArtists, work.UserID) {
			return true
		}
	}

	// Blacklisted tags.
	if len(profile.BlacklistedTags) > 0 {
		for _, workTag := range work.Tags {
			if slices.ContainsFunc(
				profile.BlacklistedTags,
				func(tag string) bool {
					return strings.EqualFold(tag, workTag)
				},
			) {
				return true
			}
		}
	}

	// Nothing matched – keep the work visible.
	return false
}

type Illust struct {
	ID               string                    `json:"id"`
	Title            string                    `json:"title"`
	Description      string                    `json:"description"`
	UserID           string                    `json:"userId"`
	UserName         string                    `json:"userName"`
	UserAccount      string                    `json:"userAccount"`
	RawRecentWorkIDs OptionalIntMap[*struct{}] `json:"userIllusts"` // We only want the IDs
	RecentWorkIDs    []int
	Date             time.Time `json:"uploadDate"`
	Images           []Thumbnails
	Tags             struct {
		AuthorID string `json:"authorId"`
		IsLocked bool   `json:"isLocked"`
		Tags     Tags   `json:"tags"`
		Writable bool   `json:"writable"`
	} `json:"tags"`
	Pages         int           `json:"pageCount"`
	Bookmarks     int           `json:"bookmarkCount"`
	Likes         int           `json:"likeCount"`
	Comments      int           `json:"commentCount"`
	Views         int           `json:"viewCount"`
	CommentOff    int           `json:"commentOff"`
	SanityLevel   SanityLevel   `json:"sl"`
	XRestrict     XRestrict     `json:"xRestrict"`
	AIType        AIType        `json:"aiType"`
	BookmarkData  *BookmarkData `json:"bookmarkData"`
	Liked         bool          `json:"likeData"`
	SeriesNavData struct {
		SeriesType  string `json:"seriesType"`
		SeriesID    string `json:"seriesId"`
		Title       string `json:"title"`
		IsWatched   bool   `json:"isWatched"`
		IsNotifying bool   `json:"isNotifying"`
		Order       int    `json:"order"`
		Next        struct {
			Title string `json:"title"`
			Order int    `json:"order"`
			ID    string `json:"id"`
		} `json:"next"`
		Prev struct {
			Title string `json:"title"`
			Order int    `json:"order"`
			ID    string `json:"id"`
		} `json:"prev"`
	} `json:"seriesNavData"`
	User         *User
	RecentWorks  []ArtworkItem
	RelatedWorks []ArtworkItem
	CommentsData *CommentsData
	IllustType   IllustType `json:"illustType"`

	// The following are used on the /search route only
	Urls struct {
		Mini     string `json:"mini"`
		Thumb    string `json:"thumb"`
		Small    string `json:"small"`
		Regular  string `json:"regular"`
		Original string `json:"original"`
	} `json:"urls"`
	Thumbnails Thumbnails
	Width      int `json:"width"`
	Height     int `json:"height"`
}

// FastIllustParams encapsulates basic artwork data required
// to call GetArtworkByIDFast, available through Artwork-* request headers.
type FastIllustParams struct {
	ID         string
	UserID     string
	Username   string
	IllustType IllustType
	Pages      int
}

type imageResponse struct {
	Width  int               `json:"width"`
	Height int               `json:"height"`
	Urls   map[string]string `json:"urls"`
}

// artworkRelatedResponse represents the API response for GetArtworkRelatedURL.
type artworkRelatedResponse struct {
	Illusts []ArtworkItem `json:"illusts"`
	NextIDs []string      `json:"nextIds"`
	Details OptionalStrMap[struct {
		Methods         []string      `json:"methods"`
		Score           float64       `json:"score"`
		SeedIllustIDs   []json.Number `json:"seedIllustIds"`
		BanditInfo      string        `json:"banditInfo"`
		RecommendListID string        `json:"recommendListId"`
	}] `json:"details"`
}

// GetArtwork retrieves information about a specific artwork.
func GetArtwork(w http.ResponseWriter, r *http.Request, artworkID string) (*Illust, error) {
	return getAndProcessArtwork(w, r, artworkID, nil)
}

// GetArtworkFast returns an Illust quicker than GetArtwork, but
// requires FastIllustParams to be known beforehand and does not fetch
// related works, recent works, or comments.
func GetArtworkFast(w http.ResponseWriter, r *http.Request, params FastIllustParams) (*Illust, error) {
	return getAndProcessArtwork(w, r, params.ID, &params)
}

// GetBasicArtwork fetches and processes basic artwork data.
func GetBasicArtwork(r *http.Request, artworkID string, illust *Illust) error {
	resp, err := requests.GetJSONBody(
		r.Context(),
		GetArtworkInformationURL(artworkID),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header,
	)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(RewriteEscapedImageURLs(r, resp), illust); err != nil {
		return err
	}

	thumbnails, err := PopulateThumbnailsFor(illust.Urls.Small)
	if err != nil {
		return err
	}

	originalURL, err := url.Parse(illust.Urls.Original)
	if err != nil {
		return fmt.Errorf("failed to parse original URL '%s': %w", illust.Urls.Original, err)
	}

	thumbnails.Download = utils.GetProxyBase(untrusted.GetUgoiraProxy(r)) + "/pximg" + originalURL.Path
	illust.Thumbnails = thumbnails

	recentWorkIDs, _ := illust.RawRecentWorkIDs.ExtractIDs()
	sort.Sort(sort.Reverse(sort.IntSlice(recentWorkIDs)))

	if len(recentWorkIDs) > ArtworkRecentLimit {
		recentWorkIDs = recentWorkIDs[:ArtworkRecentLimit]
	}

	illust.RecentWorkIDs = recentWorkIDs

	return nil
}

func GetArtworkRelated(r *http.Request, artworkID string) ([]ArtworkItem, error) {
	var data artworkRelatedResponse

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetArtworkRelatedURL(artworkID, artworkRelatedLimit),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(RewriteEscapedImageURLs(r, resp), &data); err != nil {
		return nil, err
	}

	for i, artwork := range data.Illusts {
		if err := artwork.PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for artwork ID %s: %w", artwork.ID, err)
		}

		data.Illusts[i] = artwork
	}

	return data.Illusts, nil
}

func PopulateArtworkRecent(r *http.Request, userID string, recentWorkIDs []int) ([]ArtworkItem, error) {
	if len(recentWorkIDs) == 0 {
		return nil, nil
	}

	var idsString string
	for _, id := range recentWorkIDs {
		idsString += fmt.Sprintf("&ids[]=%d", id)
	}

	recent, err := populateArtworkIDs(r, userID, idsString)
	if err != nil {
		return nil, err
	}

	for i, artwork := range recent {
		if err := artwork.PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for artwork ID %s: %w", artwork.ID, err)
		}

		artwork.Thumbnail = artwork.Thumbnails.MasterWebp_1200
		recent[i] = artwork
	}

	sort.Slice(recent, func(i, j int) bool {
		return numberGreaterThan(recent[i].ID, recent[j].ID)
	})

	return recent, nil
}

// getAndProcessArtwork orchestrates fetching artwork data, handling both
// standard (full) and fast (partial) retrieval paths.
func getAndProcessArtwork(w http.ResponseWriter, r *http.Request, artworkID string, fastParams *FastIllustParams) (*Illust, error) {
	start := time.Now()
	timings := utils.NewTimings()

	var (
		illust     Illust
		g          errgroup.Group
		userID     string
		pages      int
		illustType IllustType
	)

	// 1. Basic artwork fetch
	//    - Standard path : blocking
	//    - Fast path     : scheduled as a goroutine (parameters are pre-known)
	isFastPath := fastParams != nil
	if isFastPath {
		userID = fastParams.UserID
		pages = fastParams.Pages
		illustType = fastParams.IllustType

		g.Go(func() error {
			t0 := time.Now()

			if err := GetBasicArtwork(r, artworkID, &illust); err != nil {
				return fmt.Errorf("basic data fetch failed: %w", err)
			}

			timings.Append("artwork-basic-fetch", time.Since(t0), "Basic artwork data fetch and process")

			return nil
		})
	} else {
		t0 := time.Now()

		if err := GetBasicArtwork(r, artworkID, &illust); err != nil {
			return nil, fmt.Errorf("basic data fetch failed: %w", err)
		}

		timings.Append("artwork-basic-fetch", time.Since(t0), "Basic artwork data fetch and process")

		userID = illust.UserID
		pages = illust.Pages
		illustType = illust.IllustType
	}

	// Shared concurrent fetches
	g.Go(func() error {
		t0 := time.Now()

		userInfo, err := GetUserBasicInformation(r, userID)
		if err != nil {
			return fmt.Errorf("user info fetch failed: %w", err)
		}

		illust.User = userInfo

		timings.Append("artwork-user-fetch", time.Since(t0), "User info fetch")

		return nil
	})

	if pages > 1 {
		g.Go(func() error {
			t0 := time.Now()

			images, err := getArtworkImages(r, artworkID, illustType)
			if err != nil {
				return fmt.Errorf("artwork images fetch failed: %w", err)
			}

			illust.Images = images

			timings.Append("artwork-images-fetch", time.Since(t0), "Images fetch")

			return nil
		})
	}

	// Full (non-fast) path extras
	if !isFastPath {
		g.Go(func() error {
			t0 := time.Now()

			related, err := GetArtworkRelated(r, artworkID)
			if err != nil {
				return fmt.Errorf("related artworks fetch failed: %w", err)
			}

			illust.RelatedWorks = related

			timings.Append("artwork-related-fetch", time.Since(t0), "Related artworks fetch")

			return nil
		})

		g.Go(func() error {
			t0 := time.Now()

			recent, err := PopulateArtworkRecent(r, userID, illust.RecentWorkIDs)
			if err != nil {
				return fmt.Errorf("recent works fetch failed: %w", err)
			}

			illust.RecentWorks = recent

			timings.Append("artwork-recent-fetch", time.Since(t0), "Recent works fetch")

			return nil
		})

		if illust.CommentOff != 1 {
			g.Go(func() error {
				params := ArtworkCommentsParams{
					ID:          artworkID,
					UserID:      userID,
					SanityLevel: illust.SanityLevel,
				}

				commentsData, commentTimings, err := GetArtworkComments(r, params)
				if err != nil {
					return fmt.Errorf("comments fetch failed: %w", err)
				}

				illust.CommentsData = commentsData

				for _, t := range commentTimings {
					timings.Append(t.Name, t.Duration, t.Description)
				}

				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Post-processing
	if illust.Images == nil {
		illust.Images = make([]Thumbnails, illust.Pages)

		thumbs, err := PopulateThumbnailsFor(illust.Urls.Small)
		if err != nil {
			return nil, fmt.Errorf("failed to generate thumbnails for image: %w", err)
		}

		illust.Images[0] = thumbs
		illust.Images[0].Width = illust.Width
		illust.Images[0].Height = illust.Height
		illust.Images[0].Original = illust.Urls.Original
		illust.Images[0].IllustType = illust.IllustType

		orig, err := url.Parse(illust.Urls.Original)
		if err != nil {
			return nil, fmt.Errorf("failed to parse original URL '%s': %w", illust.Urls.Original, err)
		}

		illust.Images[0].Download = utils.GetProxyBase(untrusted.GetUgoiraProxy(r)) + "/pximg" + orig.Path
	}

	if illust.IllustType == Ugoira && len(illust.Images) > 0 {
		proxy := utils.GetProxyBase(untrusted.GetUgoiraProxy(r))

		illust.Images[0].Video = proxy + "/ugoira/" + illust.ID
	}

	// Process description URLs before returning
	illust.Description = parseDescriptionURLs(illust.Description)

	timings.WriteHeaders(w)
	utils.AddServerTimingHeader(w, "artwork-total", time.Since(start), "Total artwork fetch time")

	return &illust, nil
}

// getArtworkImages retrieves the images for an artwork.
func getArtworkImages(r *http.Request, workID string, illustType IllustType) ([]Thumbnails, error) {
	resp, err := requests.GetJSONBody(
		r.Context(),
		GetArtworkImagesURL(workID),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header,
	)
	if err != nil {
		var apiErr *requests.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("your pixiv settings may have filtered out this content (see https://www.pixiv.net/settings/viewing): %w", err)
		}

		// TODO: How to make the error message better?
		// the user xrestrict setting is inside the initial HTML page, __NEXT_DATA__, \"xRestrict\":1
		// Response of /ajax/illust/* contains
		//   .body.restrict is always 0
		//   .body.xRestrict is the art's explicit level
		// pixiv.net doesn't even fetch /ajax/illust/*/pages in the case that the content filter blocks the image
		// how do we meaningfully get the user xrestrict level? note that the code should work for novels and more as well.

		return nil, err
	}

	var apiImages []imageResponse
	if err := json.Unmarshal(RewriteEscapedImageURLs(r, resp), &apiImages); err != nil {
		return nil, err
	}

	thumbnails := make([]Thumbnails, 0, len(apiImages))

	for _, img := range apiImages {
		smallURL := img.Urls["small"]

		thumb, err := PopulateThumbnailsFor(smallURL)
		if err != nil {
			return nil, fmt.Errorf("failed to generate thumbnails for image: %w", err)
		}

		thumb.Original = img.Urls["original"]
		thumb.Width = img.Width
		thumb.Height = img.Height
		thumb.IllustType = illustType

		orig, err := url.Parse(thumb.Original)
		if err != nil {
			return nil, fmt.Errorf("failed to parse original URL '%s': %w", thumb.Original, err)
		}

		thumb.Download = utils.GetProxyBase(untrusted.GetUgoiraProxy(r)) + "/pximg" + orig.Path

		thumbnails = append(thumbnails, thumb)
	}

	return thumbnails, nil
}
