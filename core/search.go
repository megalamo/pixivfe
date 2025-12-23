// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/sync/errgroup"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

type SearchCategory = string

const (
	SearchArtworksCategory      SearchCategory = "artworks"
	SearchIllustrationsCategory SearchCategory = "illustrations"
	SearchMangaCategory         SearchCategory = "manga"
	SearchUgoiraCategory        SearchCategory = "ugoira"
	SearchNovelsCategory        SearchCategory = "novels"
	SearchUsersCategory         SearchCategory = "users"
)

type SearchFilterMode string

const (
	SearchFilterModeSafe = "safe"
	SearchFilterModeAll  = "all"
	SearchFilterModeR18  = "r18"
)

type SearchOrder string

const (
	SearchSortNewFirst     SearchOrder = "date_d"
	SearchSortOldFirst     SearchOrder = "date"
	SearchSortPopularFirst SearchOrder = "popular_d"

	// no idea if this exists:
	// SearchSortPopularLast  SearchOrder = "popular"
)

const (
	SearchDefaultCategory = "artworks"
	SearchDefaultOrder    = SearchSortNewFirst
	SearchDefaultPage     = "1"

	SearchMangaKeyword  = "漫画"
	SearchUgoiraKeyword = "うごイラ"

	// searchUsersPageSize defines the number of users returned per page in user search results.
	searchUsersPageSize int = 10
)

func SearchDefaultMode(r *http.Request) string {
	searchMode := untrusted.GetCookie(r, cookie.SearchDefaultModeCookie)
	if searchMode == "" {
		return SearchFilterModeSafe
	}

	return searchMode
}

var (
	SearchAvailableCategories = []SearchCategory{
		SearchArtworksCategory,
		SearchIllustrationsCategory,
		SearchMangaCategory,
		SearchUgoiraCategory,
		SearchNovelsCategory,
		SearchUsersCategory,
	}

	errInvalidCategory = errors.New("invalid category")

	// SearchToolValues defines the possible values for the "tool" parameter.
	SearchToolValues = []string{"", "SAI", "Photoshop", "CLIP STUDIO PAINT", "IllustStudio", "ComicStudio", "Pixia", "AzPainter2", "Painter", "Illustrator", "GIMP", "FireAlpaca", "Oekaki BBS", "AzPainter", "CGillust", "Oekaki Chat", "Tegaki Blog", "MS_Paint", "PictBear", "openCanvas", "PaintShopPro", "EDGE", "drawr", "COMICWORKS", "AzDrawing", "SketchBookPro", "PhotoStudio", "Paintgraphic", "MediBang Paint", "NekoPaint", "Inkscape", "ArtRage", "AzDrawing2", "Fireworks", "ibisPaint", "AfterEffects", "mdiapp", "GraphicsGale", "Krita", "kokuban.in", "RETAS STUDIO", "e-mote", "4thPaint", "ComiLabo", "pixiv Sketch", "Pixelmator", "Procreate", "Expression", "PicturePublisher", "Processing", "Live2D", "dotpict", "Aseprite", "Pastela", "Poser", "Metasequoia", "Blender", "Shade", "3dsMax", "DAZ Studio", "ZBrush", "Comi Po!", "Maya", "Lightwave3D", "Hexagon King", "Vue", "SketchUp", "CINEMA4D", "XSI", "CARRARA", "Bryce", "STRATA", "Sculptris", "modo", "AnimationMaster", "VistaPro", "Sunny3D", "3D-Coat", "Paint 3D", "VRoid Studio", "Mechanical pencil", "Pencil", "Ballpoint pen", "Thin marker", "Colored pencil", "Copic marker", "Dip pen", "Watercolors", "Brush", "Calligraphy pen", "Felt-tip pen", "Magic marker", "Watercolor brush", "Paint", "Acrylic paint", "Fountain pen", "Pastels", "Airbrush", "Color ink", "Crayon", "Oil paint", "Coupy pencil", "Gansai", "Pastel Crayons"}

	// SearchToolLabels defines the labels for the "tool" parameter values.
	SearchToolLabels = []string{"All creation tools", "SAI", "Photoshop", "CLIP STUDIO PAINT", "IllustStudio", "ComicStudio", "Pixia", "AzPainter2", "Painter", "Illustrator", "GIMP", "FireAlpaca", "Oekaki BBS", "AzPainter", "CGillust", "Oekaki Chat", "Tegaki Blog", "MS_Paint", "PictBear", "openCanvas", "PaintShopPro", "EDGE", "drawr", "COMICWORKS", "AzDrawing", "SketchBookPro", "PhotoStudio", "Paintgraphic", "MediBang Paint", "NekoPaint", "Inkscape", "ArtRage", "AzDrawing2", "Fireworks", "ibisPaint", "AfterEffects", "mdiapp", "GraphicsGale", "Krita", "kokuban.in", "RETAS STUDIO", "e-mote", "4thPaint", "ComiLabo", "pixiv Sketch", "Pixelmator", "Procreate", "Expression", "PicturePublisher", "Processing", "Live2D", "dotpict", "Aseprite", "Pastela", "Poser", "Metasequoia", "Blender", "Shade", "3dsMax", "DAZ Studio", "ZBrush", "Comi Po!", "Maya", "Lightwave3D", "Hexagon King", "Vue", "SketchUp", "CINEMA4D", "XSI", "CARRARA", "Bryce", "STRATA", "Sculptris", "modo", "AnimationMaster", "VistaPro", "Sunny3D", "3D-Coat", "Paint 3D", "VRoid Studio", "Mechanical pencil", "Pencil", "Ballpoint pen", "Thin marker", "Colored pencil", "Copic marker", "Dip pen", "Watercolors", "Brush", "Calligraphy pen", "Felt-tip pen", "Magic marker", "Watercolor brush", "Paint", "Acrylic paint", "Fountain pen", "Pastels", "Airbrush", "Color ink", "Crayon", "Oil paint", "Coupy pencil", "Gansai", "Pastel Crayons"}
)

// SearchData defines the data used to render the search page.
type SearchData struct {
	workSearchResponse

	Title string
	Tag   tagSearchResult
	Users struct {
		Data     []*User
		Total    int
		LastPage int
	}
	RelatedTags          Tags // RelatedTags is processed from RawRelatedTags
	Total                int
	CurrentPage          int
	LastPage             int
	PopularSearchEnabled bool
}

// KeywordCompletions represents a keyword and its associated tag completions.
type KeywordCompletions struct {
	tagCompletionsResponse

	Keyword string
	IsLast  bool // IsLast indicates if this keyword is the last item in the tags slice
}

// tagCompletionsResponse represents the API response for tag completions (/rpc/cps.php).
type tagCompletionsResponse struct {
	Candidates []struct {
		TagName        string `json:"tag_name"`
		AccessCount    string `json:"access_count"`
		Type           string `json:"type"`            // "romaji" or "prefix"
		TagTranslation string `json:"tag_translation"` // Populated when type is "romaji"
	} `json:"candidates"`
}

// WorkSearchSettings defines the settings for searches
// when the chosen category is a work type (i.e., "artworks", "illustrations", "manga", or "novels").
type WorkSearchSettings struct {
	Name     string // Keywords to search for. Used in the URL path and the 'word' query param.
	Category string // Filter by type (e.g., "illustrations", "manga"). Used in the URL path.
	Order    string // Sort by date.
	Mode     string // Safe, R18 or both.
	Ratio    string // Landscape, portrait, or squared.
	Page     string // Page number.
	Smode    string // Exact match, partial match, or match with title.
	Wlt      string // Minimum image width.
	Wgt      string // Maximum image width.
	Hlt      string // Minimum image height.
	Hgt      string // Maximum image height.
	Tool     string // Filter by production tools (e.g. Photoshop).
	Scd      string // After this date.
	Ecd      string // Before this date.
}

// tagSearchResult is a custom type that extends tagSearchResponse
// to include cover artwork information.
type tagSearchResult struct {
	tagSearchResponse

	CoverArtwork Illust // Custom field to store extended info about the cover artwork
}

// tagSearchResponse defines the API response structure for /ajax/search/tags.
type tagSearchResponse struct {
	Name            string `json:"tag"`
	AlternativeName string `json:"word"`
	Metadata        struct {
		Detail string      `json:"abstract"`
		Image  string      `json:"image"`
		Name   string      `json:"tag"`
		ID     json.Number `json:"id"`
	} `json:"pixpedia"`
}

// workSearchResponse defines the API response structure for /ajax/search/(artworks|illustrations|manga|novels).
type workSearchResponse struct {
	IllustManga   workResults `json:"illustManga"` // Populated when category is "artworks"
	Illustrations workResults `json:"illust"`      // Populated when category is "illustrations"
	Manga         workResults `json:"manga"`       // Populated when category is "manga"
	Novels        struct {
		Data     []*NovelBrief `json:"data"`
		Total    int           `json:"total"`
		LastPage int           `json:"lastPage"`
	} `json:"novel"` // Populated when category is "novel"
	Popular struct {
		Permanent []ArtworkItem `json:"permanent"`
		Recent    []ArtworkItem `json:"recent"`
	} `json:"popular"`
	TagTranslation TagTranslationWrapper `json:"tagTranslation"`
	RawRelatedTags []string              `json:"relatedTags"`
}

// userSearchResponse defines the API response structure for /ajax/search/users.
type userSearchResponse struct {
	Data []any `json:"data"` // NOTE: this was an empty array in the response analyzed
	Page struct {
		UserIDs []int          `json:"userIds"`
		WorkIDs WorkIDsWrapper `json:"workIds"` // Key is userID (string)
		Total   int            `json:"total"`
	} `json:"page"` // Page holds user IDs and their associated work IDs.
	TagTranslation TagTranslationWrapper `json:"tagTranslation"`
	Thumbnails     struct {
		Illust      []ArtworkItem `json:"illust"`
		Novel       []*NovelBrief `json:"novel"`
		NovelSeries []any         `json:"novelSeries"` // TODO: Our NovelSeries type might be correct, not sure.
		NovelDraft  []any         `json:"novelDraft"`  // We don't have a type
		Collection  []any         `json:"collection"`  // We don't have a type
	} `json:"thumbnails"`
	IllustSeries []IllustSeries `json:"illustSeries"`
	Requests     []any          `json:"requests"` // We don't have a type
	Users        []*User        `json:"users"`
	// ZoneConfig   any            `json:"zoneConfig"` // NOTE: not implemented, these are advertisements
}

type workResults struct {
	Data     []ArtworkItem `json:"data"`
	Total    int           `json:"total"`
	LastPage int           `json:"lastPage"`
}

// workItem represents a single work item (illust or novel) associated with a user.
type workItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`       // NOTE: observed values were "illust" and "novel"
	CreatedAt string `json:"created_at"` // NOTE: parseable as time.Time
}

// WorkIDsWrapper is a custom type that safely handles workIds formatted as either map or array.
type WorkIDsWrapper map[string][]workItem

// UnmarshalJSON implements custom JSON unmarshaling for WorkIDsWrapper.
func (w *WorkIDsWrapper) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as map first
	var asMap map[string][]workItem
	if err := json.Unmarshal(data, &asMap); err == nil {
		*w = WorkIDsWrapper(asMap)

		return nil
	}

	// If that fails, try as array
	var asArray []any
	if err := json.Unmarshal(data, &asArray); err == nil {
		// If it's an array, return empty map
		*w = make(WorkIDsWrapper)

		return nil
	}

	// If both fail, try one more time as map to get original error
	var finalTry map[string][]workItem

	return json.Unmarshal(data, &finalTry)
}

// GetTagData retrieves tag data from the pixiv API based on the provided tag name.
func GetTagData(r *http.Request, name string) (tagSearchResult, error) {
	var tag tagSearchResult

	rawResp, err := requests.GetJSONBody(
		r.Context(),
		GetTagDetailURL(name),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return tag, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, rawResp), &tag)
	if err != nil {
		return tag, err
	}

	return tag, nil
}

// GetSearch delegates the search operation to either getPopularSearch or getStandardSearch based on settings.Order.
//
// For non-user searches, Tag data is also populated.
func GetSearch(r *http.Request, settings WorkSearchSettings) (*SearchData, error) {
	var (
		result *SearchData
		tag    tagSearchResult
		g      errgroup.Group
	)

	originalName := settings.Name
	originalCategory := settings.Category

	// The pixiv API doesn't support searches in a "ugoira" category natively,
	// but we can roll our own by appending the "うごイラ" keyword to the search query
	// and making a search in the "illustrations" category.
	if originalCategory == SearchUgoiraCategory {
		if settings.Name != SearchUgoiraKeyword {
			settings.Name += " " + SearchUgoiraKeyword
		}

		settings.Category = SearchIllustrationsCategory
	}

	// Fetch search results and tag data concurrently
	g.Go(func() error {
		var err error
		if strings.ToLower(settings.Order) == "popular" {
			result, err = getPopularSearch(r, settings)
		} else {
			result, err = getStandardSearch(r, settings)
		}

		return err
	})

	g.Go(func() error {
		var err error

		tag, err = GetTagData(r, originalName)
		if err != nil {
			return err
		}

		// Fetch cover artwork for tag if available
		id := tag.Metadata.ID.String()
		if id != "" {
			var illust Illust
			if err := GetBasicArtwork(r, id, &illust); err == nil {
				tag.CoverArtwork = illust
			}
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// If we made a search in the "ugoira" category,
	// we need to trim the "うごイラ" keyword from the Tag.Metadata.Name
	// for consistency in the UI.
	if originalCategory == SearchUgoiraCategory &&
		strings.HasSuffix(tag.Metadata.Name, SearchUgoiraKeyword) {
		tag.Metadata.Name = strings.TrimSpace(strings.TrimSuffix(tag.Metadata.Name, SearchUgoiraKeyword))
	}

	// Set tag data and metadata in struct field order
	result.Title = "Results for " + originalName
	result.Tag = tag
	result.PopularSearchEnabled = config.Global.Feature.PopularSearch

	return result, nil
}

// GetSearchUsers retrieves user search results and converts to SearchData format.
//
// Note: the Tag field is intentionally NOT populated for user searches.
func GetSearchUsers(r *http.Request, settings WorkSearchSettings) (*SearchData, error) {
	resp, err := requests.GetJSONBody(
		r.Context(),
		GetUserSearchURL(settings.Name, settings.Page),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	var userResult userSearchResponse
	if err := json.Unmarshal(RewriteEscapedImageURLs(r, resp), &userResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user search response: %w", err)
	}

	// Process thumbnails for users' works
	for i := range userResult.Thumbnails.Illust {
		if err := userResult.Thumbnails.Illust[i].PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for user artwork %d: %w", i, err)
		}
	}

	// Create users array
	users := make([]*User, len(userResult.Users))
	copy(users, userResult.Users)

	// Associate artworks and novels with their respective users
	associateContentWithUsers(users, userResult.Thumbnails.Illust, userResult.Thumbnails.Novel)

	// Calculate last page based on total count
	const pageRoundingOffset = 9

	lastPage := (userResult.Page.Total + pageRoundingOffset) / searchUsersPageSize

	// Create the SearchData struct
	result := &SearchData{
		Title: "Results for " + settings.Name,
		// Tag field intentionally not populated for user searches
		workSearchResponse: workSearchResponse{
			TagTranslation: userResult.TagTranslation,
		},
		Users: struct {
			Data     []*User
			Total    int
			LastPage int
		}{
			Data:     users,
			Total:    userResult.Page.Total,
			LastPage: lastPage,
		},
		// RelatedTags not applicable for user searches
		Total:                userResult.Page.Total,
		CurrentPage:          1, // Will be set by caller
		LastPage:             lastPage,
		PopularSearchEnabled: false, // Not applicable for user searches
	}

	return result, nil
}

// getStandardSearch handles the standard search logic.
func getStandardSearch(r *http.Request, settings WorkSearchSettings) (*SearchData, error) {
	url, err := GetArtworkSearchURL(settings)
	if err != nil {
		return nil, err
	}

	resp, err := requests.GetJSONBody(
		r.Context(),
		url,
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	var result workSearchResponse
	if err := json.Unmarshal(RewriteEscapedImageURLs(r, resp), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search response: %w", err)
	}

	// Create SearchData struct
	searchData := &SearchData{
		workSearchResponse: result,
	}

	// Convert string tags to Tag objects with translations
	searchData.RelatedTags = searchData.TagTranslation.ToTags(searchData.RawRelatedTags)

	// Process thumbnails for popular artworks
	for i := range searchData.Popular.Permanent {
		if err := searchData.Popular.Permanent[i].PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for popular permanent artwork %d: %w", i, err)
		}
	}

	for i := range searchData.Popular.Recent {
		if err := searchData.Popular.Recent[i].PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for popular recent artwork %d: %w", i, err)
		}
	}

	// Process data based on category and set top-level Total and LastPage
	switch settings.Category {
	case SearchArtworksCategory:
		// Process thumbnails for artworks
		for i := range searchData.IllustManga.Data {
			if err := searchData.IllustManga.Data[i].PopulateThumbnails(); err != nil {
				return nil, fmt.Errorf("failed to populate thumbnails for artwork %d: %w", i, err)
			}
		}

		searchData.Total = searchData.IllustManga.Total
		searchData.LastPage = searchData.IllustManga.LastPage

	case SearchIllustrationsCategory:
		// Process thumbnails for illustrations
		for i := range searchData.Illustrations.Data {
			if err := searchData.Illustrations.Data[i].PopulateThumbnails(); err != nil {
				return nil, fmt.Errorf("failed to populate thumbnails for illustration %d: %w", i, err)
			}
		}

		searchData.Total = searchData.Illustrations.Total
		searchData.LastPage = searchData.Illustrations.LastPage

	case SearchMangaCategory:
		// Process thumbnails for manga
		for i := range searchData.Manga.Data {
			if err := searchData.Manga.Data[i].PopulateThumbnails(); err != nil {
				return nil, fmt.Errorf("failed to populate thumbnails for manga %d: %w", i, err)
			}
		}

		searchData.Total = searchData.Manga.Total
		searchData.LastPage = searchData.Manga.LastPage

	case SearchNovelsCategory:
		// Process tags for novels
		for i := range searchData.Novels.Data {
			searchData.Novels.Data[i].Tags = searchData.Novels.Data[i].RawTags.ToTags()
		}

		searchData.Total = searchData.Novels.Total
		searchData.LastPage = searchData.Novels.LastPage

	default:
		return nil, fmt.Errorf("%w: %s", errInvalidCategory, settings.Category)
	}

	return searchData, nil
}

// GetTagCompletions retrieves tag completion suggestions for the last keyword in a search query.
func GetTagCompletions(r *http.Request, keywords string) (*KeywordCompletions, error) {
	// Split keywords by spaces and filter out empty strings.
	tags := strings.Fields(strings.TrimSpace(keywords))
	if len(tags) == 0 {
		return nil, nil
	}

	// We only need completions for the last tag in the user's input.
	lastTag := tags[len(tags)-1]

	var tagCompletionResponse tagCompletionsResponse

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetTagCompletionURL(lastTag),
		map[string]string{"PHPSESSID": requests.NoToken},
		r.Header)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(resp, &tagCompletionResponse); err != nil {
		return nil, err
	}

	return &KeywordCompletions{
		Keyword:                lastTag,
		IsLast:                 true,
		tagCompletionsResponse: tagCompletionResponse,
	}, nil
}

// getPopularSearch handles the popular search logic.
func getPopularSearch(r *http.Request, settings WorkSearchSettings) (*SearchData, error) {
	// Check if popular search is enabled
	if !config.Global.Feature.PopularSearch {
		return nil, fmt.Errorf("Popular search is disabled by server configuration.")
	}

	// Perform popular search
	searchArtworks, err := searchPopular(r.Context(), r, settings)
	if err != nil {
		return nil, err
	}

	// Create SearchData
	apiResponse := workSearchResponse{
		IllustManga: searchArtworks,
	}

	searchData := &SearchData{
		workSearchResponse: apiResponse,
		Total:              searchArtworks.Total,
		LastPage:           searchArtworks.LastPage,
		// TODO: Populate Popular (the regular one) and RelatedTags.
	}

	// Populate thumbnails for each artwork
	for id, artwork := range searchData.IllustManga.Data {
		if err := artwork.PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for artwork ID %d: %w", id, err)
		}

		searchData.IllustManga.Data[id] = artwork
	}

	return searchData, nil
}
