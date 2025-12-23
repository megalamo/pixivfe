// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
Types for core/user.go
*/
package core

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"
)

const (
	UserDefaultCategory       = "" // Should be treated identically to UserArtworksCategory
	UserArtworksCategory      = "artworks"
	UserIllustrationsCategory = "illustrations"
	UserMangaCategory         = "manga"
	UserNovelsCategory        = "novels"
	UserBookmarksCategory     = "bookmarks"
	UserFollowingCategory     = "following"
	UserFollowersCategory     = "followers"
)

var (
	validCategories = []string{
		UserDefaultCategory,
		UserArtworksCategory,
		UserIllustrationsCategory,
		UserMangaCategory,
		UserNovelsCategory,
		UserBookmarksCategory,
		UserFollowingCategory,
		UserFollowersCategory,
	}

	prefectures = map[string]string{
		"1":  "Hokkaido",
		"2":  "Aomori",
		"3":  "Iwate",
		"4":  "Miyagi",
		"5":  "Akita",
		"6":  "Yamagata",
		"7":  "Fukushima",
		"8":  "Ibaraki",
		"9":  "Tochigi",
		"10": "Gunma",
		"11": "Saitama",
		"12": "Chiba",
		"13": "Tokyo",
		"14": "Kanagawa",
		"15": "Niigata",
		"16": "Toyama",
		"17": "Ishikawa",
		"18": "Fukui",
		"19": "Yamanashi",
		"20": "Nagano",
		"21": "Gifu",
		"22": "Shizuoka",
		"23": "Aichi",
		"24": "Mie",
		"25": "Shiga",
		"26": "Kyoto",
		"27": "Osaka",
		"28": "Hyogo",
		"29": "Nara",
		"30": "Wakayama",
		"31": "Tottori",
		"32": "Shimane",
		"33": "Okayama",
		"34": "Hiroshima",
		"35": "Yamaguchi",
		"36": "Tokushima",
		"37": "Kagawa",
		"38": "Ehime",
		"39": "Kochi",
		"40": "Fukuoka",
		"41": "Saga",
		"42": "Nagasaki",
		"43": "Kumamoto",
		"44": "Oita",
		"45": "Miyazaki",
		"46": "Kagoshima",
		"47": "Okinawa",
	}

	// trackingParams are common URL parameters used for tracking purposes.
	trackingParams = []string{
		"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
		"fbclid", "gclid", "msclkid", "mc_cid", "mc_eid",
	}
)

// UserData defines the data used to render the User page.
type UserData struct {
	Title       string
	User        *User
	MaxPage     int
	CurrentPage int
	MetaImage   string
}

// UserAtomFeedData defines the data used to render user atom feeds.
type UserAtomFeedData struct {
	Title     string
	User      *User
	Updated   string
	PageLimit int
	Page      int
}

// SocialEntry represents a single social media entry with platform and URL.
type SocialEntry struct {
	Platform string
	URL      string
}

// CleanURL removes UTM parameters and other tracking parameters from the URL.
func (s *SocialEntry) CleanURL() {
	if s.URL == "" {
		return
	}

	parsedURL, err := url.Parse(s.URL)
	if err != nil {
		return
	}

	query := parsedURL.Query()
	for _, param := range trackingParams {
		query.Del(param)
	}

	parsedURL.RawQuery = query.Encode()
	s.URL = parsedURL.String()
}

// workCategory represents a unified structure for handling different work categories.
type workCategory struct {
	MaxPage      int            // Maximum number of pages for pagination
	FrequentTags Tags           // Frequently used tags within the category
	IllustWorks  []ArtworkItem  // Populated if the work type requires ArtworkBrief
	NovelWorks   []*NovelBrief  // Populated if the work type requires NovelBrief
	WorkIDs      string         // Concatenated string of work IDs
	TotalWorks   int            // Number of works for the category
	MangaSeries  []IllustSeries // Populated for the "manga" category
	NovelSeries  []NovelSeries  // Populated for the "novels" category
	Users        []*User        // Populated for the "following" and "followers" category
}

// personalField represents a key/value pair for a personal field.
type personalField struct {
	Key   string
	Value string
}

// workspaceItem represents a key/value pair for workspace details.
type workspaceItem struct {
	Key   string
	Value string
}

// User represents a user.
type User struct {
	ID             string                            `json:"userId"`
	Name           string                            `json:"name"`
	Image          string                            `json:"image"`    // Unimplemented
	Avatar         string                            `json:"imageBig"` // Higher resolution avatar
	AvatarOriginal string                            // Original resolution avatar
	Premium        bool                              `json:"premium"`    // Unimplemented
	IsFollowed     bool                              `json:"isFollowed"` // Whether the logged-in user is following this user
	IsMyPixiv      bool                              `json:"isMypixiv"`  // Unimplemented
	IsBlocking     bool                              `json:"isBlocking"` // Unimplemented
	Background     map[string]any                    `json:"background"`
	SketchLiveID   string                            `json:"sketchLiveId"` // Unimplemented
	Partial        int                               `json:"partial"`      // Unimplemented
	SketchLives    []any                             `json:"sketchLives"`  // Unimplemented
	Commission     any                               `json:"commission"`   // Unimplemented
	Following      int                               `json:"following"`
	MyPixiv        int                               `json:"mypixivCount"`
	FollowedBack   bool                              `json:"followedBack"` // Unimplemented
	Comment        string                            `json:"comment"`      // Biography
	CommentHTML    string                            `json:"commentHtml"`  // HTML-formatted biography
	Webpage        string                            `json:"webpage"`
	SocialRaw      OptionalStrMap[map[string]string] `json:"social"`
	CanSendMessage bool                              `json:"canSendMessage"` // Unimplemented
	Region         struct {
		Name             string
		Region           string
		Prefecture       string
		ParsedPrefecture string
		PrivacyLevel     string
	} `json:"region"`
	Age struct {
		Name         string // terrible naming, should be `Value`, not `Name`; pixiv moment
		PrivacyLevel string
	} `json:"age"`
	BirthDay struct {
		Name         string
		PrivacyLevel string
	} `json:"birthDay"`
	Gender struct {
		Name         string
		PrivacyLevel string
	} `json:"gender"`
	Job struct {
		Name         string
		PrivacyLevel string
	} `json:"job"`
	Workspace struct {
		UserWorkspacePc     string
		UserWorkspaceTool   string
		UserWorkspaceTablet string
		UserWorkspaceMouse  string
	} `json:"workspace"`
	Official bool `json:"official"`
	Group    any  `json:"group"`

	// The following fields are internal to PixivFE
	Social                  []SocialEntry
	BackgroundImage         string
	BackgroundImageOriginal string        // Original resolution background image
	Artworks                []ArtworkItem // Populated on user discovery and following/follower pages
	Novels                  []*NovelBrief // Populated on user discovery and following/follower pages
	PersonalFields          []personalField
	WorkspaceItems          []workspaceItem

	// Work categories
	IllustrationsCategory *workCategory
	MangaCategory         *workCategory
	BookmarksCategory     *workCategory
	NovelsCategory        *workCategory
	FollowingCategory     *workCategory
	FollowersCategory     *workCategory
}

// GetCategory returns the UserWorkCategory for a given category name string.
func (u *User) GetCategory(name string) *workCategory {
	switch name {
	case "illustrations", "artworks", "":
		return u.IllustrationsCategory
	case "manga":
		return u.MangaCategory
	case "novels":
		return u.NovelsCategory
	case "bookmarks":
		return u.BookmarksCategory
	case "following":
		return u.FollowingCategory
	case "followers":
		return u.FollowersCategory
	default:
		// Default to illustrations for any other unexpected value
		return u.IllustrationsCategory
	}
}

// personalFields returns a slice of personal fields for a user.
func (u *User) personalFields() []personalField {
	return []personalField{
		{"Age", u.Age.Name},
		{"Birthday", u.BirthDay.Name}, // Renamed to "Birthday" for the UI
		{"Gender", u.Gender.Name},
		{"Job", u.Job.Name},
	}
}

// workspaceItems returns a slice of workspace items for a user.
func (u *User) workspaceItems() []workspaceItem {
	return []workspaceItem{
		{"PC", u.Workspace.UserWorkspacePc},
		{"Tool", u.Workspace.UserWorkspaceTool},
		{"Tablet", u.Workspace.UserWorkspaceTablet},
		{"Mouse", u.Workspace.UserWorkspaceMouse},
	}
}

// parseSocial parses the social data for a user.
func (u *User) parseSocial() {
	// Convert to sorted slice
	u.Social = make([]SocialEntry, 0, len(u.SocialRaw))
	for platform, data := range u.SocialRaw {
		entry := SocialEntry{
			Platform: platform,
			URL:      data["url"],
		}
		entry.CleanURL()

		u.Social = append(u.Social, entry)
	}
}

// sortSocial sorts the social data for a user.
func (u *User) sortSocial() {
	sort.Slice(u.Social, func(i, j int) bool {
		return u.Social[i].Platform < u.Social[j].Platform
	})
}

// webpageToSocialEntry converts a webpage URL to a SocialEntry by extracting
// the second-level domain as the platform name.
func (u *User) webpageToSocialEntry() *SocialEntry {
	if u.Webpage == "" {
		return nil
	}

	// Ensure URL has protocol prefix.
	urlStr := u.Webpage
	if !strings.HasPrefix(strings.ToLower(urlStr), "http://") &&
		!strings.HasPrefix(strings.ToLower(urlStr), "https://") {
		urlStr = "https://" + urlStr
	}

	// Parse the URL.
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}

	// Split the host into parts.
	parts := strings.Split(parsedURL.Host, ".")

	const minDomainParts = 2
	if len(parts) < minDomainParts {
		return nil
	}

	// Extract second-level domain (last two parts).
	const domainOffset = 2

	domainIndex := len(parts) - domainOffset
	if domainIndex < 0 {
		return nil
	}

	// Create and return SocialEntry.
	return &SocialEntry{
		Platform: parts[domainIndex], // Use second-level domain.
		URL:      u.Webpage,          // Use original URL.
	}
}

// userIllustBookmarks represents the response structure for /ajax/user/{id}/illusts/bookmarks.
type userIllustBookmarks struct {
	Artworks []json.RawMessage `json:"works"`
	Total    int               `json:"total"`
}

// userWorksResponse represents the response structure for /ajax/user/{id}/profile/all.
type userWorksResponse struct {
	Illusts     OptionalIntMap[*struct{}] `json:"illusts"`
	Manga       OptionalIntMap[*struct{}] `json:"manga"`
	MangaSeries []IllustSeries            `json:"mangaSeries"`
	Novels      OptionalIntMap[*struct{}] `json:"novels"`
	NovelSeries []NovelSeries             `json:"novelSeries"`
}

// userFollowingResponse represents the API response for GetUserFollowingURL.
type userFollowingResponse struct {
	Users []struct {
		UserID          string        `json:"userId"`
		UserName        string        `json:"userName"`
		ProfileImageURL string        `json:"profileImageUrl"`
		UserComment     string        `json:"userComment"`
		Following       bool          `json:"following"`
		Followed        bool          `json:"followed"`
		IsBlocking      bool          `json:"isBlocking"`
		IsMypixiv       bool          `json:"isMypixiv"`
		Illusts         []ArtworkItem `json:"illusts"`
	} `json:"users"`
	Total int `json:"total"`
}
