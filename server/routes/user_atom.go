// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/request_context"
	"codeberg.org/pixivfe/pixivfe/v3/server/template/commondata"
)

// pixivCustomNamespace is the URI for our custom Atom feed extensions.
const pixivCustomNamespace = "http://codeberg.org/pixivfe/pixivfe/ns/v1"

// atomContentTemplate is a pre-parsed template for the HTML content of an Atom entry.
var atomContentTemplate = template.Must(template.New("atomContent").Parse(
	`<div xmlns="http://www.w3.org/1999/xhtml">` +
		`<div><img src="{{.ThumbnailURL}}" alt="{{.Title}} thumbnail"/></div>` +
		`</div>`,
))

// atomLink represents a link in an Atom feed.
type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

// atomAuthor represents an author in an Atom feed.
type atomAuthor struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}

// atomContent represents the content of an Atom entry.
type atomContent struct {
	Type    string `xml:"type,attr"`
	XMLBase string `xml:"xml:base,attr"`
	Content string `xml:",innerxml"`
}

// pixivBookmarkData provides structured bookmark information in the custom namespace.
type pixivBookmarkData struct {
	Bookmarked bool `xml:",chardata"`
	Private    bool `xml:"private,attr,omitempty"`
}

// atomEntry represents an entry in an Atom feed.
type atomEntry struct {
	XMLName xml.Name   `xml:"entry"`
	ID      string     `xml:"id"`
	Link    atomLink   `xml:"link"`
	Updated string     `xml:"updated"`
	Title   string     `xml:"title"`
	Author  atomAuthor `xml:"author"`

	// Custom metadata in its own namespace for machine readability.
	// For artworks
	PixivPages        int                `xml:"pixiv:pages,omitempty"`
	PixivXRestrict    int                `xml:"pixiv:x_restrict,omitempty"`
	PixivAIType       int                `xml:"pixiv:ai_type,omitempty"`
	PixivIllustType   int                `xml:"pixiv:illust_type,omitempty"`
	PixivBookmarkData *pixivBookmarkData `xml:"pixiv:bookmark_data,omitempty"`

	// For novels
	PixivTextCount   int `xml:"pixiv:text_count,omitempty"`
	PixivWordCount   int `xml:"pixiv:word_count,omitempty"`
	PixivReadingTime int `xml:"pixiv:reading_time,omitempty"`

	Content atomContent `xml:"content"`
}

// atomFeed is the root element of an Atom feed.
type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	PixivNS string      `xml:"xmlns:pixiv,attr"` // Declares the custom namespace.
	ID      string      `xml:"id"`
	Links   []atomLink  `xml:"link"`
	Updated string      `xml:"updated"`
	Title   string      `xml:"title"`
	Author  atomAuthor  `xml:"author"`
	Entries []atomEntry `xml:"entry"`
}

// atomFeedBuilder holds the context and logic for building a user's Atom feed.
type atomFeedBuilder struct {
	data        core.UserData
	commonData  commondata.PageCommonData
	userURL     string
	baseAtomURL string
	category    string
}

// UserAtomFeed is the route handler for user atom feeds.
func UserAtomFeed(w http.ResponseWriter, r *http.Request) error {
	data, err := fetchUserData(r)
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

	feed, err := newAtomFeedBuilder(request_context.FromRequest(r).CommonData, data).build()
	if err != nil {
		return fmt.Errorf("failed to build user atom feed: %w", err)
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")

	_, _ = w.Write([]byte(xml.Header))

	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")

	return encoder.Encode(feed)
}

// newAtomFeedBuilder creates and initializes a new builder.
func newAtomFeedBuilder(cd commondata.PageCommonData, data core.UserData) *atomFeedBuilder {
	userURL := fmt.Sprintf("%s/users/%s", cd.BaseURL, data.User.ID)

	return &atomFeedBuilder{
		data:        data,
		commonData:  cd,
		userURL:     userURL,
		baseAtomURL: userURL + "/atom.xml",
		category:    cd.Queries["category"],
	}
}

// build generates the complete atomFeed.
func (b *atomFeedBuilder) build() (*atomFeed, error) {
	entries, err := b.buildEntries()
	if err != nil {
		return nil, err
	}

	feed := &atomFeed{
		PixivNS: pixivCustomNamespace,
		ID:      b.buildFeedID(),
		Updated: time.Now().Format(time.RFC3339),
		Title:   b.buildTitle(),
		Author: atomAuthor{
			Name: b.data.User.Name,
			URI:  b.userURL,
		},
		Links:   b.buildPaginationLinks(),
		Entries: entries,
	}

	return feed, nil
}

// buildTitle constructs the title for the feed based on the category.
func (b *atomFeedBuilder) buildTitle() string {
	// Default to "works" for the default/combined category, or any unknown category.
	categoryName := "works"

	switch b.category {
	case core.UserIllustrationsCategory:
		categoryName = "illustrations"
	case core.UserMangaCategory:
		categoryName = "manga"
	case core.UserNovelsCategory:
		categoryName = "novels"
	case core.UserBookmarksCategory:
		categoryName = "bookmarks"
	case core.UserFollowingCategory:
		categoryName = "followed users"
	}

	return fmt.Sprintf("%s's %s on pixiv", b.data.User.Name, categoryName)
}

// buildBaseQuery creates a url.Values object with the base query parameters for the feed.
func (b *atomFeedBuilder) buildBaseQuery() url.Values {
	q := make(url.Values)
	if b.category != core.UserDefaultCategory && b.category != core.UserArtworksCategory {
		q.Set("category", b.category)
	}

	return q
}

// buildFeedID constructs the canonical ID for the feed, which is the user page URL
// with a category filter if applicable.
func (b *atomFeedBuilder) buildFeedID() string {
	q := b.buildBaseQuery()
	if encodedQuery := q.Encode(); encodedQuery != "" {
		return fmt.Sprintf("%s?%s", b.userURL, encodedQuery)
	}

	return b.userURL
}

// buildPageURL constructs a URL for the Atom feed with optional category and page parameters.
func (b *atomFeedBuilder) buildPageURL(page int) string {
	q := b.buildBaseQuery()

	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}

	if encodedQuery := q.Encode(); encodedQuery != "" {
		return b.baseAtomURL + "?" + encodedQuery
	}

	return b.baseAtomURL
}

// buildEntries creates a slice of atomEntry from the user's works based on category.
func (b *atomFeedBuilder) buildEntries() ([]atomEntry, error) {
	// Handle specific categories first.
	// If the category is not a combined one, we build and return immediately.
	if b.category != core.UserDefaultCategory && b.category != core.UserArtworksCategory {
		categoryData := b.data.User.GetCategory(b.category)
		if categoryData == nil {
			return nil, nil // No data for this category.
		}

		switch b.category {
		case core.UserIllustrationsCategory, core.UserMangaCategory, core.UserBookmarksCategory:
			return b.buildArtworkEntries(categoryData.IllustWorks)
		case core.UserNovelsCategory:
			return b.buildNovelEntries(categoryData.NovelWorks)
		case core.UserFollowingCategory:
			return b.buildUserEntries(categoryData.Users)
		default:
			// Fallback to illustrations for any category not explicitly handled.
			if b.data.User.IllustrationsCategory == nil {
				return nil, nil
			}

			return b.buildArtworkEntries(b.data.User.IllustrationsCategory.IllustWorks)
		}
	}

	// The rest of the function handles the default "home" categories, which combine various works.
	var allEntries []atomEntry

	// Combine illustrations.
	if illustCat := b.data.User.IllustrationsCategory; illustCat != nil && len(illustCat.IllustWorks) > 0 {
		illustEntries, err := b.buildArtworkEntries(illustCat.IllustWorks)
		if err != nil {
			return nil, err
		}

		allEntries = append(allEntries, illustEntries...)
	}

	// Combine manga.
	if mangaCat := b.data.User.MangaCategory; mangaCat != nil && len(mangaCat.IllustWorks) > 0 {
		mangaEntries, err := b.buildArtworkEntries(mangaCat.IllustWorks)
		if err != nil {
			return nil, err
		}

		allEntries = append(allEntries, mangaEntries...)
	}

	// Combine novels.
	if novelCat := b.data.User.NovelsCategory; novelCat != nil && len(novelCat.NovelWorks) > 0 {
		novelEntries, err := b.buildNovelEntries(novelCat.NovelWorks)
		if err != nil {
			return nil, err
		}

		allEntries = append(allEntries, novelEntries...)
	}

	// Sort all combined entries by update time, descending.
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Updated > allEntries[j].Updated
	})

	return allEntries, nil
}

// buildArtworkEntries creates a slice of atomEntry from artworks.
func (b *atomFeedBuilder) buildArtworkEntries(artworks []core.ArtworkItem) ([]atomEntry, error) {
	entries := make([]atomEntry, 0, len(artworks))

	for _, artwork := range artworks {
		contentHTML, err := buildAtomContentHTML(artwork.Thumbnail, artwork.Title)
		if err != nil {
			return nil, fmt.Errorf("artwork %s: %w", artwork.ID, err)
		}

		var bookmarkData *pixivBookmarkData
		if artwork.BookmarkData != nil {
			bookmarkData = &pixivBookmarkData{
				Bookmarked: true,
				Private:    artwork.BookmarkData.Private,
			}
		}

		artworkURL := fmt.Sprintf("%s/artworks/%s", b.commonData.BaseURL, artwork.ID)
		entry := atomEntry{
			ID:      artworkURL,
			Link:    atomLink{Rel: "alternate", Href: artworkURL},
			Updated: artwork.CreateDate.Format(time.RFC3339),
			Title:   artwork.Title,
			Author: atomAuthor{
				Name: artwork.UserName,
				URI:  fmt.Sprintf("%s/users/%s", b.commonData.BaseURL, artwork.UserID),
			},
			PixivPages:        artwork.Pages,
			PixivXRestrict:    int(artwork.XRestrict),
			PixivAIType:       int(artwork.AIType),
			PixivIllustType:   artwork.IllustType,
			PixivBookmarkData: bookmarkData,
			Content: atomContent{
				Type:    "xhtml",
				XMLBase: b.commonData.BaseURL,
				Content: contentHTML,
			},
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// buildNovelEntries creates a slice of atomEntry from novels.
func (b *atomFeedBuilder) buildNovelEntries(novels []*core.NovelBrief) ([]atomEntry, error) {
	entries := make([]atomEntry, 0, len(novels))

	for _, novel := range novels {
		contentHTML, err := buildAtomContentHTML(novel.CoverURL, novel.Title)
		if err != nil {
			return nil, fmt.Errorf("novel %s: %w", novel.ID, err)
		}

		var bookmarkData *pixivBookmarkData
		if novel.BookmarkData != nil {
			bookmarkData = &pixivBookmarkData{
				Bookmarked: true,
				Private:    novel.BookmarkData.Private,
			}
		}

		novelURL := fmt.Sprintf("%s/novels/%s", b.commonData.BaseURL, novel.ID)
		entry := atomEntry{
			ID:      novelURL,
			Link:    atomLink{Rel: "alternate", Href: novelURL},
			Updated: novel.CreateDate.Format(time.RFC3339),
			Title:   novel.Title,
			Author: atomAuthor{
				Name: novel.UserName,
				URI:  fmt.Sprintf("%s/users/%s", b.commonData.BaseURL, novel.UserID),
			},
			PixivTextCount:    novel.TextCount,
			PixivWordCount:    novel.WordCount,
			PixivReadingTime:  novel.ReadingTime,
			PixivBookmarkData: bookmarkData,
			Content: atomContent{
				Type:    "xhtml",
				XMLBase: b.commonData.BaseURL,
				Content: contentHTML,
			},
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// buildUserEntries creates a slice of atomEntry from followed users.
func (b *atomFeedBuilder) buildUserEntries(users []*core.User) ([]atomEntry, error) {
	entries := make([]atomEntry, 0, len(users))

	for _, user := range users {
		contentHTML, err := buildAtomContentHTML(user.Avatar, user.Name)
		if err != nil {
			return nil, fmt.Errorf("user %s: %w", user.ID, err)
		}

		userURL := fmt.Sprintf("%s/users/%s", b.commonData.BaseURL, user.ID)
		entry := atomEntry{
			ID:   userURL,
			Link: atomLink{Rel: "alternate", Href: userURL},
			// NOTE: The API doesn't provide a "followed at" timestamp, so we use the current time.
			Updated: time.Now().Format(time.RFC3339),
			Title:   user.Name,
			Author: atomAuthor{
				Name: user.Name,
				URI:  userURL,
			},
			Content: atomContent{
				Type:    "xhtml",
				XMLBase: b.commonData.BaseURL,
				Content: contentHTML,
			},
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// buildPaginationLinks creates the navigation links for the atom feed.
func (b *atomFeedBuilder) buildPaginationLinks() []atomLink {
	links := []atomLink{
		{Rel: "self", Href: b.commonData.CurrentPath},
		// The "alternate" link points to the HTML version of the feed's content.
		{Rel: "alternate", Href: b.buildFeedID()},
		{Rel: "first", Href: b.buildPageURL(1)},
		{Rel: "last", Href: b.buildPageURL(b.data.MaxPage)},
	}

	if b.data.CurrentPage > 1 {
		links = append(links, atomLink{
			Rel: "previous", Href: b.buildPageURL(b.data.CurrentPage - 1),
		})
	}

	if b.data.CurrentPage < b.data.MaxPage {
		links = append(links, atomLink{
			Rel: "next", Href: b.buildPageURL(b.data.CurrentPage + 1),
		})
	}

	return links
}

// buildAtomContentHTML generates the simplified XHTML content for an Atom entry.
func buildAtomContentHTML(thumbnailURL, title string) (string, error) {
	data := struct {
		ThumbnailURL string
		Title        string
	}{
		ThumbnailURL: thumbnailURL,
		Title:        title,
	}

	var contentBuilder strings.Builder
	if err := atomContentTemplate.Execute(&contentBuilder, data); err != nil {
		return "", err
	}

	return contentBuilder.String(), nil
}
