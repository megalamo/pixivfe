// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"slices"
	"time"

	"golang.org/x/sync/errgroup"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// ArtworkCommentsParams holds the parameters required to fetch artwork comments.
type ArtworkCommentsParams struct {
	// ID is the ID of the artwork for which to fetch comments.
	ID string

	// UserID is the ID of the user who posted the artwork.
	UserID string

	// SanityLevel determines the content filtering level for artworks.
	SanityLevel SanityLevel
}

// NovelCommentsParams holds the parameters required to fetch novel comments.
type NovelCommentsParams struct {
	// ID is the ID of the novel for which to fetch comments.
	ID string

	// UserID is the ID of the user who posted the novel.
	UserID string

	// XRestrict determines the content filtering level for novels.
	XRestrict XRestrict
}

// CommentsData is a container for the fetched comments and their total count.
type CommentsData struct {
	// Comments is a slice of root-level comments, each potentially containing replies.
	Comments []*Comment

	// Count is the total number of comments, including replies.
	Count int
}

// Comment represents a single comment or reply on a work.
type Comment struct {
	UserID        string `json:"userId"`
	Username      string `json:"userName"`
	IsDeletedUser bool   `json:"isDeletedUser"`
	Img           string `json:"img"`
	ID            string `json:"id"`
	// Comment contains the text content of the comment.
	// It may be replaced with an HTML img tag if the comment is a stamp.
	Comment string `json:"comment"`
	StampID string `json:"stampId"`
	// StampLink is a field returned by pixiv for replies.
	// Not used by us as we need to generate the stamp URL ourselves using GetStaticProxy.
	StampLink   string    `json:"-"`
	CommentDate time.Time `json:"commentDate"`
	// TODO: Find out what this field is for.
	// Only present for comments that are replies.
	CommentRootID   string `json:"commentRootId"`
	CommentParentID string `json:"commentParentId"` // TODO: Find out what this field is for.
	CommentUserID   string `json:"commentUserId"`
	ReplyToUserID   string `json:"replyToUserId"` // Only present for comments that are replies.
	ReplyToUsername string `json:"replyToUserName"`
	Editable        bool   `json:"editable"`
	HasReplies      bool   `json:"hasReplies"`

	// Replies is an internal field to hold fetched replies to this comment.
	Replies []*Comment

	// WorkUserID is an internal field holding the ID of the work's author.
	WorkUserID string
}

// UnmarshalJSON implements custom JSON unmarshaling for the Comment type.
//
// This is necessary to handle the date format provided by the pixiv API.
func (c *Comment) UnmarshalJSON(data []byte) error {
	type alias Comment

	aux := &struct {
		*alias

		Date string `json:"commentDate"`
	}{
		alias: (*alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// The pixiv API returns commentDate in UTC+9,
	// so we need to correct it back to UTC for relative dates to
	// display correctly in the front end.
	parsedTime, err := time.Parse("2006-01-02 15:04", aux.Date)
	if err != nil {
		// We intentionally ignore date parsing errors and just use zero time;
		// a failed time parse doesn't warrant a failed page render.
		c.CommentDate = time.Time{}

		return nil //nolint:nilerr
	}

	parsedTime = parsedTime.Add(-PixivTimeOffset)

	// If the parsed time is a future date, cap it to the current time.
	now := time.Now()
	if parsedTime.After(now) {
		parsedTime = now
	}

	c.CommentDate = parsedTime.UTC()

	return nil
}

// processStamp converts a stamp comment into an HTML img tag.
//
// If the comment's  StampID field is not empty, this method overwrites the Comment field
// with an `<img>` element pointing to the corresponding stamp image.
func (c *Comment) processStamp(r *http.Request) {
	if c.StampID == "" {
		return
	}

	stampURL := utils.GetProxyBase(untrusted.GetStaticProxy(r)) + "/common/images/stamp/generated-stamps/" + c.StampID + "_s.jpg"

	c.Comment = `<img src="` + stampURL + `" class="stamp" loading="lazy" />`
}

var (
	// emojiList is a map of emoji shortcodes to their corresponding image IDs.
	emojiList = map[string]string{
		"normal":        "101",
		"surprise":      "102",
		"serious":       "103",
		"heaven":        "104",
		"happy":         "105",
		"excited":       "106",
		"sing":          "107",
		"cry":           "108",
		"normal2":       "201",
		"shame2":        "202",
		"love2":         "203",
		"interesting2":  "204",
		"blush2":        "205",
		"fire2":         "206",
		"angry2":        "207",
		"shine2":        "208",
		"panic2":        "209",
		"normal3":       "301",
		"satisfaction3": "302",
		"surprise3":     "303",
		"smile3":        "304",
		"shock3":        "305",
		"gaze3":         "306",
		"wink3":         "307",
		"happy3":        "308",
		"excited3":      "309",
		"love3":         "310",
		"normal4":       "401",
		"surprise4":     "402",
		"serious4":      "403",
		"love4":         "404",
		"shine4":        "405",
		"sweat4":        "406",
		"shame4":        "407",
		"sleep4":        "408",
		"heart":         "501",
		"teardrop":      "502",
		"star":          "503",
	}

	// emojiShortcodeRegexp matches emoji shortcodes.
	emojiShortcodeRegexp = regexp.MustCompile(`\(([^)]+)\)`)
)

// parseEmojis replaces emoji shortcodes in a string with corresponding image tags.
//
// #nosec:G203 -- Input is escaped with html.EscapeString() BEFORE any replacements are made, which are constructed from a hardcoded, trusted map.
func parseEmojis(s string) string {
	return emojiShortcodeRegexp.ReplaceAllStringFunc(html.EscapeString(s),
		func(match string) string {
			// Extract the shortcode from inside the parentheses, e.g., "happy" from "(happy)".
			shortcode := match[1 : len(match)-1]

			// Check if the shortcode is a valid, known emoji.
			emojiID, found := emojiList[shortcode]
			if !found {
				// If it's not a known emoji, return the original escaped match.
				// e.g., "(not-an-emoji)" remains as is.
				return match
			}

			return fmt.Sprintf(`<img src="%s/common/images/emoji/%s.png" alt="(%s)" class="emoji" />`,
				utils.GetProxyBase(config.Global.ContentProxies.Static), emojiID, shortcode)
		})
}

// commentsRootsResponse represents the structure of a single page of comments from the pixiv API.
type commentsRootsResponse struct {
	// Comments is the list of comments on the current page.
	Comments []*Comment `json:"comments"`

	// HasNext indicates if there is a subsequent page of comments.
	HasNext bool `json:"hasNext"`
}

// urlFunc is a function type that generates a URL for a specific page of comments or replies.
type urlFunc func(id string, page int) string

// GetArtworkComments fetches and processes all comments for a given artwork.
//
// It returns the structured comment data, performance timings, and any error encountered.
func GetArtworkComments(r *http.Request, params ArtworkCommentsParams) (*CommentsData, []utils.Timing, error) {
	noToken := params.SanityLevel <= SLSafe

	comments, commentTimings, err := getComments(r, params.ID, noToken, params.UserID, GetArtworkCommentsURL, GetArtworkCommentRepliesURL)
	if err != nil {
		return nil, nil, err
	}

	return &CommentsData{
		Comments: comments,
		Count:    countCommentsAndReplies(comments),
	}, commentTimings, nil
}

// GetNovelComments fetches and processes all comments for a given novel.
//
// It returns the structured comment data, performance timings, and any error encountered.
func GetNovelComments(r *http.Request, params NovelCommentsParams) (*CommentsData, []utils.Timing, error) {
	noToken := params.XRestrict < 1

	comments, commentTimings, err := getComments(r, params.ID, noToken, params.UserID, GetNovelCommentsURL, GetNovelCommentRepliesURL)
	if err != nil {
		return nil, nil, err
	}

	return &CommentsData{
		Comments: comments,
		Count:    countCommentsAndReplies(comments),
	}, commentTimings, nil
}

// getComments provides the generic logic for fetching and processing comments.
//
// It first fetches all pages of root comments. Then, for each root comment
// that has replies, it concurrently fetches all pages of its replies.
func getComments(
	r *http.Request,
	workID string,
	noToken bool,
	workUserID string,
	getCommentsURL urlFunc,
	getRepliesURL urlFunc,
) ([]*Comment, []utils.Timing, error) {
	timings := make([]utils.Timing, 0)
	start := time.Now()

	defer func() {
		totalDuration := time.Since(start)

		timings = append(timings, utils.Timing{
			Name:        "comments-total",
			Duration:    totalDuration,
			Description: "Total comments fetch time",
		})
	}()

	// Fetch all root comments.
	rootFetchStart := time.Now()

	allComments, err := fetchPaginatedComments(r, workID, noToken, getCommentsURL, 0)
	if err != nil {
		return nil, timings, err
	}

	timings = append(timings, utils.Timing{
		Name:        "comments-root-fetch",
		Duration:    time.Since(rootFetchStart),
		Description: "Root comments fetch",
	})

	// Concurrently process all root comments and fetch their replies.
	processingStart := time.Now()
	g, ctx := errgroup.WithContext(r.Context())

	for _, rootComment := range allComments {
		g.Go(func() error {
			// Process the root comment itself.
			rootComment.WorkUserID = workUserID
			rootComment.Comment = parseEmojis(rootComment.Comment)
			rootComment.processStamp(r)

			// If the root comment has replies, fetch and process them.
			if !rootComment.HasReplies {
				return nil
			}

			requestWithCtx := r.WithContext(ctx)
			// Reply pages are 1-indexed on pixiv's API, so we start fetching from page 1.
			replies, err := fetchPaginatedComments(requestWithCtx, rootComment.ID, noToken, getRepliesURL, 1)
			if err != nil {
				return err
			}

			// Process all fetched replies.
			for _, reply := range replies {
				reply.WorkUserID = workUserID
				reply.Comment = parseEmojis(reply.Comment)
				reply.processStamp(r)
			}

			// Reverse the replies for chronological display, as the API returns them newest-first.
			slices.Reverse(replies)

			rootComment.Replies = replies

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, timings, err
	}

	timings = append(timings, utils.Timing{
		Name:        "comments-process-all",
		Duration:    time.Since(processingStart),
		Description: "Concurrent processing of all comments and replies",
	})

	return allComments, timings, nil
}

// fetchPaginatedComments fetches all pages of comments from a given endpoint.
//
// It starts from the page number specified by startPage and continues to fetch
// subsequent pages until the API response indicates that no more pages are
// available.
func fetchPaginatedComments(
	r *http.Request,
	id string,
	noToken bool,
	urlFn urlFunc,
	startPage int,
) ([]*Comment, error) {
	var comments []*Comment

	page := startPage
	hasNext := true

	for hasNext {
		var (
			data    commentsRootsResponse
			cookies map[string]string
		)

		if noToken {
			cookies = map[string]string{"PHPSESSID": requests.NoToken}
		} else {
			cookies = map[string]string{"PHPSESSID": untrusted.GetUserToken(r)}
		}

		resp, err := requests.GetJSONBody(
			r.Context(),
			urlFn(id, page),
			cookies,
			r.Header,
		)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(RewriteEscapedImageURLs(r, resp), &data)
		if err != nil {
			return nil, err
		}

		comments = append(comments, data.Comments...)
		hasNext = data.HasNext
		page++
	}

	return comments, nil
}

// countCommentsAndReplies calculates the total number of comments, including
// all nested replies within a slice of root comments.
func countCommentsAndReplies(comments []*Comment) int {
	total := len(comments)

	for _, comment := range comments {
		total += len(comment.Replies)
	}

	return total
}
