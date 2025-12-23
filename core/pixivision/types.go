// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package pixivision

import (
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/core"
)

// NavCategories defines the pixivision categories used in the navigation bar.
var NavCategories = []NavCategory{
	{Name: "Illustrations", Href: "/pixivision/c/illustration", Group: "Explore"},
	{Name: "Manga", Href: "/pixivision/c/manga", Group: "Explore"},
	{Name: "Novels", Href: "/pixivision/c/novels", Group: "Explore"},
	{Name: "Cosplay", Href: "/pixivision/c/cosplay", Group: "Explore"},
	{Name: "Music", Href: "/pixivision/c/music", Group: "Explore"},
	{Name: "Goods", Href: "/pixivision/c/goods", Group: "Explore"},
	{Name: "Tutorials", Href: "/pixivision/c/how-to-draw", Group: "Create"},
	{Name: "Behind the Art", Href: "/pixivision/c/draw-step-by-step", Group: "Create"},
	{Name: "Materials", Href: "/pixivision/c/textures", Group: "Create"},
	{Name: "References", Href: "/pixivision/c/art-references", Group: "Create"},
	{Name: "Other Guides", Href: "/pixivision/c/how-to-make", Group: "Create"},
	// NOTE: pixivision calls this category "Featured" in their frontend,
	// but they also have a selection of 10 articles which are actually "Featured" (see popular.go)
	{Name: "Featured", Href: "/pixivision/c/recommend", Group: "Discover"},
	{Name: "Interviews", Href: "/pixivision/c/interview", Group: "Discover"},
	{Name: "Columns", Href: "/pixivision/c/column", Group: "Discover"},
	{Name: "News", Href: "/pixivision/c/news", Group: "Discover"},
	{Name: "Deskwatch", Href: "/pixivision/c/deskwatch", Group: "Discover"},
	{Name: "We Tried It!", Href: "/pixivision/c/try-out", Group: "Discover"},
}

// NavCategory represents a single category on pixivision.
//
// Used to populate a navigation element.
type NavCategory struct {
	Name  string // The name of the category, e.g., "Illustrations"
	Href  string // The URL path for the category, e.g., "/c/illustration"
	Group string // Optional grouping, e.g., "Explore", "Create"
}

// IndexData defines the data used to render the pixivision homepage.
//
// We fetch these specific categories because they receive regular content updates.
type IndexData struct {
	Title                  string
	IndexArticles          []ArticleTile // Articles from the actual homepage
	InterviewArticles      []ArticleTile // Articles from interview category
	ColumnArticles         []ArticleTile // Articles from column category
	NewsArticles           []ArticleTile // Articles from news category
	MonthlyRankingArticles []ArticleTile // Articles from Monthly Ranking section
	FeaturedArticles       []ArticleTile // Articles from Featured section
	Page                   int
}

// Link represents a generic hyperlink with text and a URL.
type Link struct {
	Text string
	URL  string
}

// ArticleHeader encapsulates the information found in an article's <header> tag.
type ArticleHeader struct {
	Title    string
	Category Link
	Date     time.Time
}

// BodyItem is an interface representing any block-level element within a freeform article's body.
type BodyItem interface {
	isBodyItem() // Marker method to ensure type implementation.
}

// BodyImage represents an image block.
type BodyImage struct {
	Src  string
	Alt  string
	Href string // Optional URL if the image is a link.
}

func (b BodyImage) isBodyItem() {}

// BodyCredit represents the author's credit line.
type BodyCredit struct {
	Text string
}

func (b BodyCredit) isBodyItem() {}

// BodyHeading represents a section subheading (e.g., <h3>).
type BodyHeading struct {
	Text string
	// ID   string
}

func (b BodyHeading) isBodyItem() {}

// BodyRichText represents a block of formatted text, like a paragraph or comment.
type BodyRichText struct {
	HTMLContent string
}

func (b BodyRichText) isBodyItem() {}

// BodyArticleCard represents an embedded card that links to another article.
type BodyArticleCard struct {
	Article ArticleTile
}

func (b BodyArticleCard) isBodyItem() {}

// BodyCaption represents a caption for an image or other element.
type BodyCaption struct {
	Text string
}

func (b BodyCaption) isBodyItem() {}

// BodyQuestion represents a question in an interview format.
//
// ref: https://www.pixivision.net/en/a/10367
type BodyQuestion struct {
	Text string
}

func (b BodyQuestion) isBodyItem() {}

// BodyAnswer represents an answer in an interview format.
//
// ref: https://www.pixivision.net/en/a/10367
type BodyAnswer struct {
	ImageSrc    string // The speaker's icon
	AuthorName  string // The name of the person answering
	HTMLContent string
}

func (b BodyAnswer) isBodyItem() {}

// BodyBoothLink represents an embedded BOOTH product link.
//
// ref: https://www.pixivision.net/en/a/10367
type BodyBoothLink struct {
	AuthorImageSrc string
	AuthorName     string
	AuthorURL      string
	ItemTitle      string
	ItemURL        string
	ItemImageSrc   string
}

func (b BodyBoothLink) isBodyItem() {}

// AuthorProfile represents the author's profile block found within the article body.
type AuthorProfile struct {
	ImageSrc string
	Name     string
	BioHTML  string
	Links    []core.SocialEntry
}

func (b AuthorProfile) isBodyItem() {}

// ArticleFreeformData defines the data used to render a freeform pixivision article,
// such as a column or interview, which is composed of various content blocks.
type ArticleFreeformData struct {
	ID                     string
	Header                 ArticleHeader
	Body                   []BodyItem
	Tags                   []EmbedTag
	NewestTaggedArticles   RelatedArticleGroup
	PopularTaggedArticles  RelatedArticleGroup
	NewestCategoryArticles RelatedArticleGroup
}

// CategoryData defines the data used to render a pixivision category page.
type CategoryData struct {
	Title       string
	ID          string
	CurrentPage int
	Category    Category
}

// TagData defines the data used to render a pixivision tag page.
type TagData struct {
	Tag   *Tag
	Page  int
	ID    string
	Title string
}

// ArticleData represents a single article on pixivision
//
// NOTE: This type should ONLY be used to render a article page.
type ArticleData struct {
	ID                     string
	Title                  string
	Description            []string // Used for structured articles
	IsFreeform             bool     // True if the article is freeform
	BodyHTML               string   // Raw HTML body for freeform articles
	Category               string   // Friendly category name; isn't guaranteed to be a valid category page reference
	CategoryID             string   // Valid category page reference
	Thumbnail              string
	Date                   time.Time
	Items                  []ArticleItem
	Tags                   []EmbedTag
	NewestTaggedArticles   RelatedArticleGroup
	PopularTaggedArticles  RelatedArticleGroup
	NewestCategoryArticles RelatedArticleGroup
}

// ArticleTile represents a single article tile on pixivision
//
// This is a subset of PixivisionArticleData for use in lists.
type ArticleTile struct {
	ID          string
	Title       string
	Thumbnail   string
	Date        time.Time
	Category    string
	CategoryID  string
	Tags        []EmbedTag
	Description []string // Keep description for potential short previews in tiles
}

// EmbedTag represents a tag associated with a pixivision article.
type EmbedTag struct {
	ID   string
	Name string
}

// ArticleItem represents an item (artwork) within a pixivision article.
type ArticleItem struct {
	Username string
	UserID   string
	Title    string
	ID       string
	Avatar   string
	Images   []string
}

// RelatedArticleGroup represents a group of related articles with a common heading link.
type RelatedArticleGroup struct {
	HeadingLink string
	Articles    []ArticleTile
}

// Tag represents a tag page on pixivision.
type Tag struct {
	Title       string
	ID          string
	Description string
	Thumbnail   string
	Articles    []ArticleTile
	Total       int // The total number of articles
}

// Category represents a category page on pixivision.
type Category struct {
	Articles    []ArticleTile
	Thumbnail   string
	Title       string
	Description string
}

const (
	exploreCategory = iota
	createCategory
	discoveryCategory
)

type PixivisionCategory struct {
	Type int
	ID   string
}

// PixivisionCategoryID returns the type of category and the ID of the category from its name.
func PixivisionCategoryID(name string) PixivisionCategory {
	switch name {
	case "Illustrations":
		return PixivisionCategory{Type: exploreCategory, ID: "illustration"}
	case "Manga":
		return PixivisionCategory{Type: exploreCategory, ID: "manga"}
	case "Novels":
		return PixivisionCategory{Type: exploreCategory, ID: "novels"}
	case "Cosplay":
		return PixivisionCategory{Type: exploreCategory, ID: "cosplay"}
	case "Music":
		return PixivisionCategory{Type: exploreCategory, ID: "music"}
	case "Goods":
		return PixivisionCategory{Type: exploreCategory, ID: "goods"}
	case "Tutorials":
		return PixivisionCategory{Type: createCategory, ID: "how-to-draw"}
	case "Behind the Art":
		return PixivisionCategory{Type: createCategory, ID: "draw-step-by-step"}
	case "Materials":
		return PixivisionCategory{Type: createCategory, ID: "textures"}
	case "References":
		return PixivisionCategory{Type: createCategory, ID: "art-references"}
	case "Other Guides":
		return PixivisionCategory{Type: createCategory, ID: "how-to-make"}
	case "Featured":
		return PixivisionCategory{Type: discoveryCategory, ID: "recommend"}
	case "Interviews":
		return PixivisionCategory{Type: discoveryCategory, ID: "interview"}
	case "Columns":
		return PixivisionCategory{Type: discoveryCategory, ID: "column"}
	case "News":
		return PixivisionCategory{Type: discoveryCategory, ID: "news"}
	case "Deskwatch":
		return PixivisionCategory{Type: discoveryCategory, ID: "deskwatch"}
	case "We Tried It!":
		return PixivisionCategory{Type: discoveryCategory, ID: "try-out"}
	}

	return PixivisionCategory{Type: exploreCategory, ID: "illustration"}
}
