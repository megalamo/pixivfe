// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

const (
	userFollowingPageSize = 12 // API uses limit=12
	userWorksPageSize     = 30
)

var (
	errUnsupportedCategory = errors.New("unsupported category")
	errInvalidPageNumber   = errors.New("invalid page number")
)

// userWorkCollections is a temporary struct to hold populated work categories
// before they are merged into the main User struct. This allows fetching user info
// and user works in parallel.
type userWorkCollections struct {
	Illustrations *workCategory
	Manga         *workCategory
	Bookmarks     *workCategory
	Novels        *workCategory
	Following     *workCategory
	Followers     *workCategory
}

// GetUserProfile retrieves the user profile, including counts, artworks/bookmarks, and social data.
//
// Goroutines are used to avoid blocking on network requests.
func GetUserProfile(r *http.Request, id, category, mode string, currentPage int) (UserData, error) {
	if _, err := strconv.Atoi(id); err != nil {
		return UserData{}, err
	}

	if !slices.Contains(validCategories, category) {
		return UserData{}, fmt.Errorf(`Invalid work category: %#v.`, category)
	}

	var (
		userInfo *User
		works    userWorkCollections
		errGroup errgroup.Group
	)

	// Fetch basic user information
	errGroup.Go(func() error {
		var err error

		userInfo, err = GetUserBasicInformation(r, id)
		if err != nil {
			return err
		}

		// Populate parsed prefecture if available
		if userInfo.Region.Prefecture != "" {
			if prefName, ok := prefectures[userInfo.Region.Prefecture]; ok {
				userInfo.Region.ParsedPrefecture = prefName
			}
		}

		// Set original avatar URL
		userInfo.AvatarOriginal = GetOriginalAvatarURL(userInfo.Avatar)

		// Parse social data
		userInfo.parseSocial()

		// Add webpage as social entry if available
		if webpageEntry := userInfo.webpageToSocialEntry(); webpageEntry != nil {
			// Check for duplicate platform
			isDuplicate := false

			for _, entry := range userInfo.Social {
				if entry.Platform == webpageEntry.Platform {
					isDuplicate = true

					break
				}
			}

			// Only append if not a duplicate
			if !isDuplicate {
				userInfo.Social = append(userInfo.Social, *webpageEntry)
			}
		}

		// Sort social entries
		userInfo.sortSocial()

		// Set background image if available
		if userInfo.Background != nil {
			backgroundURL, ok := userInfo.Background["url"].(string)
			if ok {
				// We have a valid background URL
				userInfo.BackgroundImage = backgroundURL

				// Default to current URL for original in case parsing fails
				userInfo.BackgroundImageOriginal = backgroundURL

				// Try to create the original URL by parsing
				parsedURL, err := url.Parse(backgroundURL)
				if err == nil {
					// Remove the /c/.../ segment from the path
					originalPath := sizeQualityRe.ReplaceAllString(parsedURL.Path, "/")

					parsedURL.Path = originalPath
					userInfo.BackgroundImageOriginal = parsedURL.String()
				}
			}
		}

		return nil
	})

	// Fetch works and populate categories
	errGroup.Go(func() error {
		var err error

		works, err = getPopulatedWorks(r, id, category, currentPage, mode)
		if err != nil {
			return err
		}

		return nil
	})

	if err := errGroup.Wait(); err != nil {
		return UserData{}, err
	}

	// Merge userInfo and work collections into the final user struct
	user := userInfo

	user.IllustrationsCategory = works.Illustrations
	user.MangaCategory = works.Manga
	user.BookmarksCategory = works.Bookmarks
	user.NovelsCategory = works.Novels
	user.FollowingCategory = works.Following
	user.FollowersCategory = works.Followers

	user.Comment = parseDescriptionURLs(user.Comment)
	user.CommentHTML = parseDescriptionURLs(user.CommentHTML)

	// Populate personal fields and workspace items directly on the user object
	user.PersonalFields = user.personalFields()
	user.WorkspaceItems = user.workspaceItems()

	return UserData{
		Title:       user.Name,
		User:        user,
		MaxPage:     user.GetCategory(category).MaxPage,
		CurrentPage: currentPage,
	}, nil
}

// GetUserBasicInformation retrieves basic information for a given user ID.
func GetUserBasicInformation(r *http.Request, id string) (*User, error) {
	var user *User

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetUserInformationURL(id, "1"),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, resp), &user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// getPopulatedWorks fetches then populates information for a user's works, as well as handling their
// frequently used tags and series data.
//
// Goroutines are used to avoid blocking on network requests.
func getPopulatedWorks(r *http.Request, id, currentCategoryValue string, page int, mode string) (userWorkCollections, error) {
	works, err := fetchWorkIDsAndSeriesData(r, id, currentCategoryValue, page)
	if err != nil {
		return works, err
	}

	// This block handles categories that aren't in the main user/works endpoint
	if currentCategoryValue == UserFollowingCategory && works.Following == nil {
		works.Following = &workCategory{}
	}

	if currentCategoryValue == UserFollowersCategory && works.Followers == nil {
		works.Followers = &workCategory{}
	}

	var g errgroup.Group

	// Illustrations
	if works.Illustrations != nil && works.Illustrations.WorkIDs != "" {
		g.Go(func() error {
			artworks, err := populateArtworkIDs(r, id, works.Illustrations.WorkIDs)
			if err != nil {
				return err
			}

			works.Illustrations.IllustWorks = artworks

			return nil
		})

		g.Go(func() error {
			tags, err := fetchFrequentTags(r, works.Illustrations.WorkIDs, UserIllustrationsCategory)
			if err != nil {
				return err
			}

			works.Illustrations.FrequentTags = tags

			return nil
		})
	}

	// Manga
	if works.Manga != nil && works.Manga.WorkIDs != "" {
		g.Go(func() error {
			artworks, err := populateArtworkIDs(r, id, works.Manga.WorkIDs)
			if err != nil {
				return err
			}

			works.Manga.IllustWorks = artworks

			return nil
		})

		g.Go(func() error {
			tags, err := fetchFrequentTags(r, works.Manga.WorkIDs, UserMangaCategory)
			if err != nil {
				return err
			}

			works.Manga.FrequentTags = tags

			return nil
		})
	}

	// Novels
	if works.Novels != nil && works.Novels.WorkIDs != "" {
		g.Go(func() error {
			novels, err := populateNovelIDs(r, id, works.Novels.WorkIDs)
			if err != nil {
				return err
			}

			works.Novels.NovelWorks = novels

			return nil
		})

		g.Go(func() error {
			tags, err := fetchFrequentTags(r, works.Novels.WorkIDs, UserNovelsCategory)
			if err != nil {
				return err
			}

			works.Novels.FrequentTags = tags

			return nil
		})
	}

	// Bookmarks
	g.Go(func() error {
		bookmarks, total, err := populateIllustBookmarks(r, id, mode, page)
		if err != nil {
			return err
		}

		works.Bookmarks.IllustWorks = bookmarks
		works.Bookmarks.TotalWorks = total
		works.Bookmarks.MaxPage = int(math.Ceil(float64(total) / BookmarksPageSize))

		return nil
	})

	// Following
	if currentCategoryValue == UserFollowingCategory {
		g.Go(func() error {
			users, total, err := populateUserFollowing(r, id, mode, page)
			if err != nil {
				return err
			}

			works.Following.Users = users
			works.Following.TotalWorks = total
			works.Following.MaxPage = int(math.Ceil(float64(total) / float64(userFollowingPageSize)))

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return works, err
	}

	return works, nil
}

// fetchWorkIDsAndSeriesData retrieves artwork IDs and series information for a user.
//
// It populates UserWorkCategory structs for each category.
func fetchWorkIDsAndSeriesData(r *http.Request, id, currentCategory string, page int) (userWorkCollections, error) {
	var (
		works userWorkCollections
		resp  userWorksResponse
	)

	rawResp, err := requests.GetJSONBody(
		r.Context(),
		GetUserWorksURL(id),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return works, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, rawResp), &resp)
	if err != nil {
		return works, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	// Process illustrations
	illustCat := &workCategory{}
	illustIDs, count := resp.Illusts.ExtractIDs()

	illustCat.TotalWorks = count
	illustCat.WorkIDs = buildIDString(illustIDs, page, currentCategory, UserIllustrationsCategory, illustCat)
	works.Illustrations = illustCat

	// Process manga
	mangaCat := &workCategory{}
	mangaIDs, count := resp.Manga.ExtractIDs()

	mangaCat.TotalWorks = count
	mangaCat.WorkIDs = buildIDString(mangaIDs, page, currentCategory, UserMangaCategory, mangaCat)
	mangaCat.MangaSeries = resp.MangaSeries
	works.Manga = mangaCat

	// Process novels
	novelCat := &workCategory{}
	novelIDs, count := resp.Novels.ExtractIDs()

	novelCat.TotalWorks = count
	novelCat.WorkIDs = buildIDString(novelIDs, page, currentCategory, UserNovelsCategory, novelCat)
	novelCat.NovelSeries = resp.NovelSeries
	works.Novels = novelCat

	// Create an empty bookmarks category
	//
	// We don't build an ID string here as fetchBookmarks populates IllustWorks without the need for a WorkIDs string
	// (which is also why we can't call fetchFrequentTags for the "bookmarks" category, though extracting the IDs from
	// ArtworkBrief would be possible)
	works.Bookmarks = &workCategory{}

	return works, nil
}

// buildIDString builds the ID string for API requests and sets the MaxPage for the category.
func buildIDString(ids []int, page int, currentCategory, catValue string, cat *workCategory) string {
	sort.Sort(sort.Reverse(sort.IntSlice(ids)))

	// We only use the actual page number for the current category being viewed
	// and default the page to 1 for other categories.
	//
	// This is so that we don't attempt to paginate categories that don't have enough
	// items, which would raise a spurious error from computeSliceBounds regarding an
	// invalid page number.
	//
	// NOTE: A different approach will be needed if we require maxPages for inactive categories.
	effectivePage := page
	if currentCategory != catValue {
		effectivePage = 1
	}

	start, end, maxPage, err := computeSliceBounds(effectivePage, userWorksPageSize, len(ids))
	if err != nil {
		return ""
	}

	if currentCategory == catValue {
		cat.MaxPage = maxPage
	}

	var idsBuilder strings.Builder
	for _, k := range ids[start:end] {
		idsBuilder.WriteString(fmt.Sprintf("&ids[]=%d", k))
	}

	return idsBuilder.String()
}

// fetchFrequentTags fetches a user's frequently used tags, based on category.
func fetchFrequentTags(r *http.Request, ids, categoryValue string) (Tags, error) {
	var (
		simpleTags []SimpleTag
		url        string
	)

	switch categoryValue {
	case UserIllustrationsCategory, UserMangaCategory:
		url = GetArtworkFrequentTagsURL(ids)
	case UserNovelsCategory:
		url = GetNovelFrequentTagsURL(ids)
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedCategory, categoryValue)
	}

	// Return early if there are no IDs
	//
	// NOTE: theoretically, this check should never evaluate as true since fetchUserWorks doesn't call
	// getUserFrequentTags when len(tagsIDs) == 0, instead returning an empty []Tag
	if ids == "" {
		return nil, nil
	}

	resp, err := requests.GetJSONBody(
		r.Context(),
		url, map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(resp, &simpleTags)
	if err != nil {
		return nil, err
	}

	// Convert SimpleTag to Tag
	return SimpleTags(simpleTags).ToTags(), nil
}

// populateUserFollowing retrieves and populates the users followed by a given user ID.
func populateUserFollowing(r *http.Request, id, mode string, page int) ([]*User, int, error) {
	var resp userFollowingResponse

	page--

	if mode == "all" {
		mode = "show"
	}

	rawResp, err := requests.GetJSONBody(
		r.Context(),
		GetUserFollowingURL(id, page, userFollowingPageSize, mode),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, 0, err
	}

	if err := json.Unmarshal(RewriteEscapedImageURLs(r, rawResp), &resp); err != nil {
		return nil, 0, err
	}

	users := make([]*User, len(resp.Users))
	for i, apiUser := range resp.Users {
		users[i] = &User{
			ID:             apiUser.UserID,
			Name:           apiUser.UserName,
			Avatar:         apiUser.ProfileImageURL,
			AvatarOriginal: GetOriginalAvatarURL(apiUser.ProfileImageURL),
			Comment:        apiUser.UserComment,
			IsFollowed:     apiUser.Following,
			FollowedBack:   apiUser.Followed,
			IsBlocking:     apiUser.IsBlocking,
			IsMyPixiv:      apiUser.IsMypixiv,
			Artworks:       apiUser.Illusts,
		}

		// Populate thumbnails for each artwork
		for j := range users[i].Artworks {
			if err := users[i].Artworks[j].PopulateThumbnails(); err != nil {
				return nil, 0, err
			}
		}
	}

	return users, resp.Total, nil
}

// populateWorkIDs populates a []T for a given set of work IDs based on
// the "works" field of the JSON response from the provided URL.
//
// The URL should include work IDs in the format `&ids[]=123456`.
func populateWorkIDs[T ArtworkItem | NovelBrief](r *http.Request, url string) ([]T, error) {
	rawResp, err := requests.GetJSONBody(
		r.Context(),
		url,
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Works map[int]T `json:"works"`
	}

	if err := json.Unmarshal(RewriteEscapedImageURLs(r, rawResp), &resp); err != nil {
		return nil, err
	}

	works := make([]T, 0, len(resp.Works))
	for _, work := range resp.Works {
		works = append(works, work)
	}

	return works, nil
}

// populateArtworkIDs populates a []ArtworkBrief for a given set of artwork IDs.
func populateArtworkIDs(r *http.Request, id, ids string) ([]ArtworkItem, error) {
	works, err := populateWorkIDs[ArtworkItem](r, GetUserFullArtworkURL(id, ids))
	if err != nil {
		return nil, err
	}

	// Sort the works based on ID in descending order
	sort.Slice(works, func(i, j int) bool {
		return numberGreaterThan(works[i].ID, works[j].ID)
	})

	// Populate thumbnails for each artwork
	for idx := range works {
		artwork := &works[idx]
		if err := artwork.PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for artwork ID %s: %w", artwork.ID, err)
		}
	}

	return works, nil
}

// populateNovelIDs populates a []*NovelBrief for a given set of novel IDs.
func populateNovelIDs(r *http.Request, id, ids string) ([]*NovelBrief, error) {
	works, err := populateWorkIDs[NovelBrief](r, GetUserFullNovelURL(id, ids))
	if err != nil {
		return nil, err
	}

	// Sort the works based on ID in descending order
	sort.Slice(works, func(i, j int) bool {
		return numberGreaterThan(works[i].ID, works[j].ID)
	})

	// Convert to []*NovelBrief and process RawTags to Tags for each novel
	novelPointers := make([]*NovelBrief, len(works))

	for i := range works {
		works[i].Tags = works[i].RawTags.ToTags()
		novelPointers[i] = &works[i]
	}

	return novelPointers, nil
}

// populateIllustBookmarks populates a []ArtworkBrief for a given set of bookmarked work IDs.
//
// This function cannot be neatly refactored to use getWorkIDs due to having
// a different API response structure.
func populateIllustBookmarks(r *http.Request, id, mode string, page int) ([]ArtworkItem, int, error) {
	page--

	if mode == "all" {
		mode = "show"
	}

	rawResp, err := requests.GetJSONBody(
		r.Context(),
		GetUserIllustBookmarksURL(id, mode, page),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, -1, err
	}

	var resp userIllustBookmarks

	err = json.Unmarshal(RewriteEscapedImageURLs(r, rawResp), &resp)
	if err != nil {
		return nil, -1, err
	}

	artworks := make([]ArtworkItem, len(resp.Artworks))

	for index, rawResp := range resp.Artworks {
		var artwork ArtworkItem

		err = json.Unmarshal(rawResp, &artwork)
		if err != nil {
			artworks[index] = ArtworkItem{
				ID:        "#",
				Title:     "Deleted or private",
				UserName:  "Deleted or private",
				Thumbnail: "https://s.pximg.net/common/images/limit_unknown_360.png",
			}

			continue
		}

		// Populate thumbnails
		if err := artwork.PopulateThumbnails(); err != nil {
			return nil, -1, fmt.Errorf("failed to populate thumbnails for artwork ID %s: %w", id, err)
		}

		artworks[index] = artwork
	}

	return artworks, resp.Total, nil
}

// computeSliceBounds is a utility function to compute slice bounds safely.
//
// It calculates the start and end indices for slicing based on pagination parameters.
//
// Parameters:
//   - page: The current page number (1-based)
//   - worksPerPage: Number of items to display per page
//   - totalItems: Total number of items available
//
// Returns:
//   - int: Start index for slicing (inclusive)
//   - int: End index for slicing (exclusive)
//   - int: Total number of pages available
//   - error: Error if page number is invalid (less than 1 or greater than page limit)
//     or nil if calculation succeeds
//
// If totalItems is 0, returns (0, 0, 0, nil) indicating no items to slice.
func computeSliceBounds(page int, worksPerPage float64, totalItems int) (int, int, int, error) {
	if totalItems == 0 {
		return 0, 0, 0, nil
	}

	maxPage := int(math.Ceil(float64(totalItems) / worksPerPage))

	if page < 1 || page > maxPage {
		return 0, 0, 0, errInvalidPageNumber
	}

	start := (page - 1) * int(worksPerPage)
	end := min(start+int(worksPerPage), totalItems)

	return start, end, maxPage, nil
}

func numberGreaterThan(l, r string) bool {
	if len(l) > len(r) {
		return true
	}

	if len(l) < len(r) {
		return false
	}

	return l > r
}
