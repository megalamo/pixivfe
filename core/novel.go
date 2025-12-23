// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/i18n"
)

type NovelTextLayout int

const (
	NovelTextLayoutUnknown       NovelTextLayout = 0 // returned by japanese works that display as horizontal. not sure what it is though.
	NovelTextLayoutForceVertical NovelTextLayout = 1
	NovelTextLayoutDefault       NovelTextLayout = 2
)

func CalculateNovelTextLayout(mode NovelTextLayout, cookie string) NovelTextLayout {
	if userViewMode := ParseNovelTextLayout(cookie); userViewMode != NovelTextLayoutUnknown {
		mode = userViewMode
	}

	return mode
}

func ParseNovelTextLayout(s string) NovelTextLayout {
	if s == "1" {
		return NovelTextLayoutForceVertical
	}

	if s == "2" {
		return NovelTextLayoutDefault
	}

	return NovelTextLayoutUnknown
}

func (i NovelTextLayout) IsVertical() bool {
	return i == NovelTextLayoutForceVertical
}

// Constants for novel user preferences (PixivFE).
const (
	NovelFontTypeMincho = "mincho" // Serif
	NovelFontTypeGothic = "gothic" // Sans-serif, default

	NovelViewModeNone       = ""  // Don't force, default
	NovelViewModeVertical   = "1" // Force vertical, RTL
	NovelViewModeHorizontal = "2" // Force horizontal, LTR

	// hard-coded value, may change.
	novelRelatedLimit = 180
)

// NovelContentBlock is an interface representing a single content block in a novel.
type NovelContentBlock interface {
	// isNovelContentBlock is a marker method to ensure only our defined block types
	// can implement this interface.
	isNovelContentBlock()
}

// Concrete block types.
type (
	TextBlock  struct{ Content string }
	ImageBlock struct {
		URL      string
		Alt      string
		Link     string
		ErrorMsg string
		IllustID string
	}
)

type (
	ChapterBlock   struct{ Title string }
	PageBreakBlock struct{ PageNumber int }
)

// Marker method implementations.
func (t TextBlock) isNovelContentBlock() {}

func (i ImageBlock) isNovelContentBlock() {}

func (c ChapterBlock) isNovelContentBlock() {}

func (p PageBreakBlock) isNovelContentBlock() {}

type NovelBlockType string

const (
	NovelBlockTypeText      NovelBlockType = "text"
	NovelBlockTypeImage     NovelBlockType = "image"
	NovelBlockTypeChapter   NovelBlockType = "chapter"
	NovelBlockTypePageBreak NovelBlockType = "page_break"
)

var genreMap = map[string]i18n.MsgKey{
	"1":  "Romance",
	"2":  "Isekai fantasy",
	"3":  "Contemporary fantasy",
	"4":  "Mystery",
	"5":  "Horror",
	"6":  "Sci-fi",
	"7":  "Literature",
	"8":  "Drama",
	"9":  "Historical pieces",
	"10": "BL (yaoi)",
	"11": "Yuri",
	"12": "For kids",
	"13": "Poetry",
	"14": "Essays/non-fiction",
	"15": "Screenplays/scripts",
	"16": "Reviews/opinion pieces",
	"17": "Other",
}

// NovelGenre returns the genre name for a given genre ID.
//
// It returns an error if the genre ID is not found.
func NovelGenre(ctx context.Context, s string) string {
	genre, ok := genreMap[s]
	if !ok {
		return fmt.Sprintf("unknown genre %s", s)
	}

	return genre.Tr(ctx)
}

// NovelData holds the data used to render a novel page.
type NovelData struct {
	Novel                    *Novel
	NovelRelated             []*NovelBrief
	NovelSeriesContentTitles []*NovelSeriesContentTitle
	NovelSeriesIDs           []string
	NovelSeriesTitles        []string
	User                     *User
	Title                    string
}

type Novel struct {
	Bookmarks      int       `json:"bookmarkCount"`
	CommentCount   int       `json:"commentCount"`
	MarkerCount    int       `json:"markerCount"`
	CreateDate     time.Time `json:"createDate"`
	UploadDate     time.Time `json:"uploadDate"`
	Description    string    `json:"description"`
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Likes          int       `json:"likeCount"`
	Pages          int       `json:"pageCount"`
	UserID         string    `json:"userId"`
	UserName       string    `json:"userName"`
	Views          int       `json:"viewCount"`
	IsOriginal     bool      `json:"isOriginal"`
	IsBungei       bool      `json:"isBungei"`
	XRestrict      XRestrict `json:"xRestrict"`
	Restrict       int       `json:"restrict"`
	Content        string    `json:"content"`
	CoverURL       string    `json:"coverUrl"`
	IsBookmarkable bool      `json:"isBookmarkable"`
	BookmarkData   any       `json:"bookmarkData"`
	LikeData       bool      `json:"likeData"`
	PollData       any       `json:"pollData"`
	Marker         any       `json:"marker"`
	Tags           struct {
		AuthorID string `json:"authorId"`
		IsLocked bool   `json:"isLocked"`
		Tags     []struct {
			Name string `json:"tag"`
		} `json:"tags"`
		Writable bool `json:"writable"`
	} `json:"tags"`
	SeriesNavData struct {
		SeriesType    string                `json:"seriesType"`
		SeriesID      int                   `json:"seriesId"`
		Title         string                `json:"title"`
		IsConcluded   bool                  `json:"isConcluded"`
		IsReplaceable bool                  `json:"isReplaceable"`
		IsWatched     bool                  `json:"isWatched"`
		IsNotifying   bool                  `json:"isNotifying"`
		Order         int                   `json:"order"`
		Next          NovelAdjacentInSeries `json:"next"`
		Prev          NovelAdjacentInSeries `json:"prev"`
	} `json:"seriesNavData"`
	HasGlossary bool `json:"hasGlossary"`
	IsUnlisted  bool `json:"isUnlisted"`
	// seen values: zh-cn, ja
	Language       string `json:"language"`
	CommentOff     int    `json:"commentOff"`
	CharacterCount int    `json:"characterCount"`
	WordCount      int    `json:"wordCount"`
	UseWordCount   bool   `json:"useWordCount"`
	ReadingTime    int    `json:"readingTime"`
	AIType         AIType `json:"aiType"`
	Genre          string `json:"genre"`
	Settings       struct {
		ViewMode NovelTextLayout `json:"viewMode"`
		// ...
	} `json:"suggestedSettings"`
	TextEmbeddedImages map[string]struct {
		NovelImageID string `json:"novelImageId"`
		SanityLevel  string `json:"sl"`
		Urls         struct {
			Two40Mw     string `json:"240mw"`
			Four80Mw    string `json:"480mw"`
			One200X1200 string `json:"1200x1200"`
			One28X128   string `json:"128x128"`
			Original    string `json:"original"`
		} `json:"urls"`
	} `json:"textEmbeddedImages"`
	CommentsData  *CommentsData
	UserNovels    map[string]*NovelBrief `json:"userNovels"`
	ContentBlocks []NovelContentBlock    `json:"-"` // Parsed content blocks for rendering
}

type NovelAdjacentInSeries struct {
	Title     string `json:"title"`
	Order     int    `json:"order"`
	ID        string `json:"id"`
	Available bool   `json:"available"`
}

type NovelBrief struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	XRestrict      XRestrict     `json:"xRestrict"`
	Restrict       int           `json:"restrict"`
	CoverURL       string        `json:"url"`
	Tags           Tags          `json:"-"` // Processed from RawTags
	RawTags        StringTags    `json:"tags"`
	UserID         string        `json:"userId"`
	UserName       string        `json:"userName"`
	UserAvatar     string        `json:"profileImageUrl"`
	TextCount      int           `json:"textCount"`
	WordCount      int           `json:"wordCount"`
	ReadingTime    int           `json:"readingTime"`
	Description    string        `json:"description"`
	IsBookmarkable bool          `json:"isBookmarkable"`
	BookmarkData   *BookmarkData `json:"bookmarkData"`
	Bookmarks      int           `json:"bookmarkCount"`
	IsOriginal     bool          `json:"isOriginal"`
	CreateDate     time.Time     `json:"createDate"`
	UpdateDate     time.Time     `json:"updateDate"`
	IsMasked       bool          `json:"isMasked"`
	SeriesID       string        `json:"seriesId"`
	SeriesTitle    string        `json:"seriesTitle"`
	IsUnlisted     bool          `json:"isUnlisted"`
	AIType         AIType        `json:"aiType"`
	Genre          string        `json:"genre"`
}

// insertIllustsResponse models the response from /ajax/novel/.../insert_illusts
//
// NOTE: this is a simplified version of the actual response structure.
type insertIllustsResponse map[string]struct {
	Illust struct {
		Images struct {
			Original string `json:"original"`
		} `json:"images"`
	} `json:"illust"`
}

// novelImageData models an embedded image in the novel content.
type novelImageData struct {
	URL      string
	Alt      string
	Link     string
	IllustID string
	ErrorMsg string
}

func GetNovelPageData(r *http.Request, id string) (*NovelData, error) {
	// Validate the ID
	if _, err := strconv.Atoi(id); err != nil {
		return nil, fmt.Errorf("invalid ID: %s", id)
	}

	var (
		novel         *Novel
		related       []*NovelBrief
		contentTitles []*NovelSeriesContentTitle
		user          *User
		commentsData  *CommentsData
	)

	var g errgroup.Group

	// Fetch novel
	g.Go(func() error {
		var err error

		novel, err = getNovelByID(r, id)
		if err != nil {
			return err
		}

		// Fetch series content titles if novel is part of a series
		if novel.SeriesNavData.SeriesID != 0 {
			g.Go(func() error {
				var err error

				contentTitles, err = getNovelSeriesContentTitlesByID(r, novel.SeriesNavData.SeriesID)

				return err
			})
		}

		// Fetch comments if they are not disabled
		if novel.CommentOff != 1 {
			g.Go(func() error {
				params := NovelCommentsParams{
					ID:        id,
					UserID:    novel.UserID,
					XRestrict: novel.XRestrict,
				}

				var err error

				commentsData, _, err = GetNovelComments(r, params)

				return err
			})
		}

		// Fetch user information
		g.Go(func() error {
			var err error

			user, err = GetUserBasicInformation(r, novel.UserID)

			return err
		})

		return nil
	})

	// Fetch related novels
	g.Go(func() error {
		var err error

		related, err = getNovelRelated(r, id)

		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	novel.CommentsData = commentsData

	// Construct the title
	title := novel.Title
	if novel.SeriesNavData.SeriesID != 0 {
		title = fmt.Sprintf("#%d %s | %s", novel.SeriesNavData.Order, novel.Title, novel.SeriesNavData.Title)
	}

	// Construct series arrays
	novelSeriesIDs := make([]string, len(contentTitles))
	novelSeriesTitles := make([]string, len(contentTitles))

	for i, ct := range contentTitles {
		novelSeriesIDs[i] = ct.ID
		novelSeriesTitles[i] = fmt.Sprintf("#%d %s", i+1, ct.Title)
	}

	// Process URL fields before returning
	novel.Description = parseDescriptionURLs(novel.Description)
	user.Comment = parseDescriptionURLs(user.Comment)

	for _, novelBrief := range related {
		if novelBrief != nil {
			novelBrief.Description = parseDescriptionURLs(novelBrief.Description)
		}
	}

	return &NovelData{
		Novel:                    novel,
		NovelRelated:             related,
		User:                     user,
		NovelSeriesContentTitles: contentTitles,
		NovelSeriesIDs:           novelSeriesIDs,
		NovelSeriesTitles:        novelSeriesTitles,
		Title:                    title,
	}, nil
}

func getNovelByID(r *http.Request, id string) (*Novel, error) {
	var novel *Novel

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetNovelURL(id),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return novel, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, resp), &novel)
	if err != nil {
		return novel, err
	}

	// Clean up UserNovels map by removing null entries
	if novel.UserNovels != nil {
		cleanedUserNovels := make(map[string]*NovelBrief)

		for id, novelBrief := range novel.UserNovels {
			if novelBrief != nil {
				cleanedUserNovels[id] = novelBrief
			}
		}

		novel.UserNovels = cleanedUserNovels
	}

	// Process the novel content into structured blocks
	novel.ContentBlocks = processNovelContent(r, novel)

	return novel, nil
}

func getNovelRelated(r *http.Request, id string) ([]*NovelBrief, error) {
	var data struct {
		List []*NovelBrief `json:"novels"`
	}

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetNovelRelatedURL(id, novelRelatedLimit),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return data.List, err
	}

	err = json.Unmarshal(RewriteEscapedImageURLs(r, resp), &data)
	if err != nil {
		return data.List, err
	}

	// Convert RawTags to Tags for each novel
	for i := range data.List {
		data.List[i].Tags = data.List[i].RawTags.ToTags()
	}

	return data.List, nil
}

func getNovelSeriesContentTitlesByID(r *http.Request, id int) ([]*NovelSeriesContentTitle, error) {
	var data []*NovelSeriesContentTitle

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetNovelSeriesContentTitlesURL(id),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return data, err
	}

	err = json.Unmarshal(resp, &data)
	if err != nil {
		return data, err
	}

	return data, nil
}

// Regular expressions for finding embedded illustrations in novel content.
var (
	pixivImageRegexp    = regexp.MustCompile(`\[pixivimage:\d+(-\d+)?\]`)
	novelImageTagRegexp = regexp.MustCompile(`\[pixivimage:\d+(-\d+)?\]|\[uploadedimage:\d+\]`)
	idWithPageRegexp    = regexp.MustCompile(`\d+(-\d+)?`)
	idRegexp            = regexp.MustCompile(`\d+`)
)

// fetchIllustsForNovel fetches illusts for the given novel content.
func fetchIllustsForNovel(r *http.Request, novel *Novel) (map[string]insertIllustsResponse, error) {
	results := make(map[string]insertIllustsResponse)

	// Find all [pixivimage:...] matches in the content
	matches := pixivImageRegexp.FindAllString(novel.Content, -1)
	if len(matches) == 0 {
		return results, nil
	}

	// Extract unique illust IDs
	illustIDs := make(map[string]bool)

	for _, match := range matches {
		illustID := idWithPageRegexp.FindString(match)
		if illustID != "" {
			illustIDs[illustID] = true
		}
	}

	// If no illust IDs found, return early
	if len(illustIDs) == 0 {
		return results, nil
	}

	// Capture request context before starting goroutines
	ctx := r.Context()

	var (
		mu sync.Mutex
		g  errgroup.Group
	)

	// Fetch each illust concurrently
	for illustID := range illustIDs {
		g.Go(func() error {
			resp, err := requests.GetJSONBody(
				ctx,
				GetInsertIllustURL(novel.ID, illustID),
				map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
				r.Header)
			if err != nil {
				return fmt.Errorf("failed to fetch illust %s: %w", illustID, err)
			}

			var illustData insertIllustsResponse
			if err := json.Unmarshal(resp, &illustData); err != nil {
				return fmt.Errorf("failed to unmarshal illust %s: %w", illustID, err)
			}

			mu.Lock()

			results[illustID] = illustData

			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// processNovelContent parses raw novel text into a slice of structured blocks.
func processNovelContent(r *http.Request, novel *Novel) []NovelContentBlock {
	// Nothing to do if there's no content.
	if novel.Content == "" {
		return nil
	}

	// Pre-compute data required by the parser.
	imageData := prepareImageData(r, novel)

	// Convert the text (with the help of the image map) into blocks.
	return parseNovelContent(novel.Content, imageData)
}

// prepareImageData scans the novel text for image tags and returns a map that
// turns those raw tags into fully-realised novelImageData objects.
func prepareImageData(r *http.Request, novel *Novel) map[string]*novelImageData {
	imageData := make(map[string]*novelImageData)

	// Collect every candidate image tag once up-front.
	rawTags := novelImageTagRegexp.FindAllString(novel.Content, -1)
	if len(rawTags) == 0 {
		return imageData
	}

	// We might need illust information for pixiv images. Fetch it once.
	illustsByID, illustErr := fetchIllustsForNovel(r, novel)

	for _, tag := range rawTags {
		switch {
		// 1. Uploaded images - super-simple lookup.
		case strings.HasPrefix(tag, "[uploadedimage:"):
			if id := idRegexp.FindString(tag); id != "" {
				if img, ok := novel.TextEmbeddedImages[id]; ok {
					imageData[tag] = &novelImageData{
						URL: img.Urls.Original,
						Alt: tag,
					}
				}
			}

		// 2. pixiv illusts - a little more bookkeeping.
		case strings.HasPrefix(tag, "[pixivimage:"):
			illustID := idWithPageRegexp.FindString(tag)
			if illustID == "" {
				// Malformed tag - nothing we can do.
				continue
			}

			// If the illust fetch failed overall OR this particular illust is
			// missing, record an error and move on.
			data, ok := illustsByID[illustID]
			if illustErr != nil || !ok {
				imageData[tag] = &novelImageData{
					ErrorMsg: "Cannot insert illust: " + illustID,
				}

				continue
			}

			illust, ok := data[illustID]
			if !ok || illust.Illust.Images.Original == "" {
				imageData[tag] = &novelImageData{
					ErrorMsg: "Invalid image URL for: " + illustID,
				}

				continue
			}

			// Happy path.
			imageData[tag] = &novelImageData{
				URL:      RewriteImageURLs(r, illust.Illust.Images.Original),
				Alt:      tag,
				Link:     "/artworks/" + strings.Split(illustID, "-")[0],
				IllustID: illustID,
			}
		}
	}

	return imageData
}

var (
	// # Structural splitters
	// These regular expressions are used to split the novel content into structural blocks.

	// newPageRegexp matches the [newpage] tag.
	// We don't include surrounding whitespace (`\s*`), to prevent it from
	// consuming newlines that should be treated as paragraph breaks.
	newPageRegexp = regexp.MustCompile(`\[newpage\]`)

	// lineSplitRegexp splits a string by single newlines to create line breaks
	// within a paragraph. It is used by `processTextMarkup`.
	lineSplitRegexp = regexp.MustCompile(`\r\n|\r|\n`)

	// # Markup tag parsers
	// These regular expressions find and replace specific pixiv markup tags with HTML.

	// blockChapterRegexp matches a line containing only a chapter tag. This allows
	// it to be treated as a distinct block-level element during parsing.
	blockChapterRegexp = regexp.MustCompile(`(?m)^\s*\[chapter:.+?\]\s*$`)

	// chapterRegexp extracts the title from a [chapter: ...] tag.
	chapterRegexp = regexp.MustCompile(`\[chapter:\s*(.+?)\s*\]`)

	// furiganaRegexp matches the [[rb: ... > ...]] tag for ruby text (furigana).
	furiganaRegexp = regexp.MustCompile(`\[\[rb:\s*(.+?)\s*>\s*(.+?)\s*\]\]`)

	// jumpURIRegexp matches the [[jumpuri: ... > ...]] tag for external links.
	jumpURIRegexp = regexp.MustCompile(`\[\[jumpuri:\s*(.+?)\s*>\s*(.+?)\s*\]\]`)

	// jumpPageRegexp matches the [jump: ...] tag for jumping to a specific page.
	jumpPageRegexp = regexp.MustCompile(`\[jump:\s*(\d+?)\s*\]`)
)

// parseNovelContent splits the text on [newpage] tags and delegates each
// segment to the line-oriented parser.
func parseNovelContent(content string, imageData map[string]*novelImageData) []NovelContentBlock {
	if content == "" {
		return nil
	}

	var blocks []NovelContentBlock

	segments := newPageRegexp.Split(content, -1)
	for pageIdx, segment := range segments {
		segBlocks := parseSegmentIntoBlocks(segment, imageData)
		if len(segBlocks) == 0 {
			continue
		}

		// Every segment after the first gets an explicit page break.
		if pageIdx > 0 {
			blocks = append(blocks, PageBreakBlock{PageNumber: pageIdx + 1})
		}

		blocks = append(blocks, segBlocks...)
	}

	return blocks
}

// parseParagraph converts a single paragraph string into a slice of blocks.
//
// A "paragraph" is a contiguous block of text that may contain a stand-alone
// chapter tag or inline text with image tags.
func parseParagraph(paragraph string, imageData map[string]*novelImageData) []NovelContentBlock {
	// Early-out for blank / whitespace-only paragraphs.
	if strings.TrimSpace(paragraph) == "" {
		return nil
	}

	// 1. Stand-alone chapter tag ─ return immediately.
	if m := chapterRegexp.FindStringSubmatch(paragraph); m != nil && blockChapterRegexp.MatchString(paragraph) {
		return []NovelContentBlock{
			ChapterBlock{Title: strings.TrimSpace(m[1])},
		}
	}

	// 2. No inline image tags ─ fast-path.
	imgMatches := novelImageTagRegexp.FindAllStringIndex(paragraph, -1)
	if len(imgMatches) == 0 {
		return []NovelContentBlock{
			TextBlock{Content: processTextMarkup(paragraph)},
		}
	}

	// 3. Mixed text / images.
	var (
		blocks []NovelContentBlock
		cursor int // start index of the yet-to-be-processed substring
	)

	for _, loc := range imgMatches {
		start, end := loc[0], loc[1]
		tag := paragraph[start:end]

		// Flush text that precedes this tag.
		if start > cursor {
			blocks = append(blocks, TextBlock{
				Content: processTextMarkup(paragraph[cursor:start]),
			})
		}

		// Valid image tag?
		if img, ok := imageData[tag]; ok {
			blocks = append(blocks, ImageBlock{
				URL:      img.URL,
				Alt:      img.Alt,
				Link:     img.Link,
				IllustID: img.IllustID,
				ErrorMsg: img.ErrorMsg,
			})
			cursor = end

			continue
		}

		// Unknown / malformed tag: leave it untouched so that it becomes part
		// of the next text flush.
		cursor = start
	}

	// Flush any trailing text that comes after the final (valid) image tag.
	if cursor < len(paragraph) {
		blocks = append(blocks, TextBlock{
			Content: processTextMarkup(paragraph[cursor:]),
		})
	}

	return blocks
}

// parseSegmentIntoBlocks converts a single page-segment into blocks, handling
// paragraphs, blank runs, chapter headers and inline images.
func parseSegmentIntoBlocks(segment string, imageData map[string]*novelImageData) []NovelContentBlock {
	// Nothing to do if there's no content.
	if segment == "" {
		return nil
	}

	// Normalise newline conventions up-front so the rest of the function can
	// assume `\n`` is the only line-separator we will ever see.
	segment = strings.ReplaceAll(segment, "\r\n", "\n")
	segment = strings.ReplaceAll(segment, "\r", "\n")

	lines := strings.Split(segment, "\n")

	// Remove *trailing* blank lines to mirror the official renderer.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Segment is nothing but whitespace; return.
	if len(lines) == 0 {
		return nil
	}

	// Working state while scanning the lines.
	var (
		blocks                []NovelContentBlock
		currentParagraphLines []string
		inBlankRun            bool
	)

	// flushParagraph appends the gathered paragraph to `blocks` and resets the
	// accumulator slice.
	flushParagraph := func() {
		if len(currentParagraphLines) == 0 {
			return
		}

		paragraph := strings.Join(currentParagraphLines, "\n")

		blocks = append(blocks, parseParagraph(paragraph, imageData)...)
		currentParagraphLines = currentParagraphLines[:0]
	}

	for _, line := range lines {
		// Fast-path the common case: non-blank lines.
		if line != "" {
			inBlankRun = false

			currentParagraphLines = append(currentParagraphLines, line)

			continue
		}

		// Blank line.
		if inBlankRun {
			// Second (or later) consecutive blank line ⇒ explicit <br>.
			blocks = append(blocks, TextBlock{Content: "<br />"})

			continue
		}

		flushParagraph()

		inBlankRun = true
	}

	flushParagraph() // Any trailing paragraph not yet flushed.

	return blocks
}

// processTextMarkup converts pixiv markup in plain text to its HTML equivalent
// and inserts <br/> for single newlines while preserving indentation.
func processTextMarkup(text string) string {
	if text == "" {
		return ""
	}

	text = furiganaRegexp.ReplaceAllString(text,
		`<ruby>$1<rp>(</rp><rt>$2</rt><rp>)</rp></ruby>`)
	text = jumpURIRegexp.ReplaceAllString(text,
		`<a href="$2" target="_blank" rel="noopener noreferrer" class="text-blue-400 hover:underline">$1</a>`)
	text = jumpPageRegexp.ReplaceAllString(text,
		`<a href="#novel_section_$1" class="text-blue-400 hover:underline">To page $1</a>`)

	// Preserve author formatting by converting raw newlines to <br/>.
	return strings.Join(lineSplitRegexp.Split(text, -1), "<br />")
}
