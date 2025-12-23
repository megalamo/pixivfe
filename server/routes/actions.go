// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/assets/components/fragments"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

const (
	returnPathFormat    = "return_path"
	bookmarkCountFormat = "bookmark_count"
	likeCountFormat     = "like_count"
	artworkIDFormat     = "artwork_id"
	bookmarkIDFormat    = "bookmark_id"
	userIDFormat        = "user_id"
	privateFormat       = "private" // privacy setting for bookmarks and follows
	renderTypeFormat    = "render_type"
)

var errFailedToFollowUser = errors.New("failed to follow user")

// addIllustBookmarkResponse represents the API response for PostAddIllustBookmarkURL.
type addIllustBookmarkResponse struct {
	LastBookmarkID string `json:"last_bookmark_id"`
	StaccStatusID  string `json:"stacc_status_id"`
}

// illustLikeResponse represents the API response for PostIllustLikeURL.
type illustLikeResponse struct {
	IsLiked bool `json:"is_liked"`
}

// followResponse represents the API response for a user follow using the touch API.
type followResponse struct {
	IsSucceed bool `json:"isSucceed"`
}

// authData holds authentication-related data extracted from the request.
type authData struct {
	SessionID  string
	CSRFToken  string
	ReturnPath string
}

// UserID extracts the numeric user ID from the PHPSESSID string.
func (a *authData) UserID() string {
	id, _, _ := strings.Cut(a.SessionID, "_")

	return id
}

/*
When handling htmx requests, we pass all bookmark-related illust data
to the opposite partial (add ↔ delete) to preserve state during client-side swaps.
*/

// AddBookmarkRoute handles both full‑page and HTMX fragment requests for adding
// a bookmark to an illustration.
//
// Quick‑action (thumbnail) requests are always rendered as HTMX fragments.
func AddBookmarkRoute(w http.ResponseWriter, r *http.Request) error {
	auth, err := checkAuthAndTokens(r)
	if err != nil {
		return err
	}

	// Get the illustration ID from the URL.
	illustID := utils.GetPathVar(r, artworkIDFormat)
	if illustID == "" {
		return errors.New("no illustration ID provided.")
	}

	// Build and send the pixiv API request to add the bookmark.
	// The `restrict` flag controls private vs public.
	// We check for "on" since this is what the <input type="checkbox"> provides.
	restrict := "0"
	if r.FormValue(privateFormat) == "on" {
		restrict = "1"
	}

	resp, err := requests.PostJSONBody(
		r.Context(),
		core.PostAddIllustBookmarkURL(),
		fmt.Sprintf(`{"illust_id":"%s","restrict":%s,"comment":"","tags":[]}`, illustID, restrict),
		map[string]string{"PHPSESSID": auth.SessionID},
		auth.CSRFToken,
		"application/json; charset=utf-8",
		r.Header,
	)
	if err != nil {
		return err
	}

	// Parse the response body to get the new bookmark ID.
	var addResp addIllustBookmarkResponse
	if err := json.Unmarshal(resp, &addResp); err != nil {
		return err
	}

	newBookmarkID := addResp.LastBookmarkID

	_, _ = requests.InvalidateURLs([]string{
		"https://www.pixiv.net/ajax/user/" + auth.UserID() + "/illusts/bookmarks",
		"https://www.pixiv.net/ajax/illust/" + illustID,
	})

	renderType := r.FormValue(renderTypeFormat)
	isHtmx := renderType != "" || r.Header.Get("HX-Request") == "true"

	// If this is not an HTMX request, redirect. Otherwise, render a partial.
	if !isHtmx {
		utils.RedirectToWhenceYouCame(w, r, auth.ReturnPath)

		return nil
	}

	switch renderType {
	case "quick":
		// Quick thumbnail swap → render the small "delete" icon only.
		return fragments.QuickDeleteBookmark(
			fragments.DeleteBookmarkData{
				ID:           illustID,
				BookmarkData: &core.BookmarkData{ID: newBookmarkID},
			}).Render(r.Context(), w)
	case "text":
		return fragments.RemoveBookmarkTextButton(fragments.DeleteBookmarkData{
			ID:           illustID,
			BookmarkData: &core.BookmarkData{ID: newBookmarkID},
		}).Render(r.Context(), w)
	default: // "full"
		// Full‑button swap → render the larger button with updated count.
		bookmarkCount, err := strconv.Atoi(r.FormValue(bookmarkCountFormat))
		if err != nil {
			return errors.New("invalid bookmark count.")
		}

		return fragments.DeleteBookmarkPartial(
			core.Illust{
				ID:           illustID,
				BookmarkData: &core.BookmarkData{ID: newBookmarkID},
				Bookmarks:    bookmarkCount + 1,
			}).Render(r.Context(), w)
	}
}

// DeleteBookmarkRoute handles both full‑page and HTMX fragment requests for removing
// a bookmark from an illustration.
//
// Quick‑action (thumbnail) requests are always rendered as HTMX fragments.
func DeleteBookmarkRoute(w http.ResponseWriter, r *http.Request) error {
	auth, err := checkAuthAndTokens(r)
	if err != nil {
		return err
	}

	// Get the bookmark ID from the URL.
	bookmarkID := utils.GetPathVar(r, bookmarkIDFormat)
	if bookmarkID == "" {
		return errors.New("no bookmark ID provided.")
	}

	// The response is just an empty array on success.
	_, err = requests.PostJSONBody(
		r.Context(),
		core.PostDeleteIllustBookmarkURL(),
		"bookmark_id="+bookmarkID,
		map[string]string{"PHPSESSID": auth.SessionID},
		auth.CSRFToken,
		"application/x-www-form-urlencoded; charset=utf-8",
		r.Header,
	)
	if err != nil {
		return err
	}

	// We need the illustration ID back in the form so we can re‑render
	// the button (and also to invalidate its cache entry).
	illustID := r.FormValue(artworkIDFormat)
	if illustID == "" {
		return errors.New("no illustration ID provided in form.")
	}

	_, _ = requests.InvalidateURLs([]string{
		"https://www.pixiv.net/ajax/user/" + auth.UserID() + "/illusts/bookmarks",
		"https://www.pixiv.net/ajax/illust/" + illustID,
	})

	renderType := r.FormValue(renderTypeFormat)
	isHtmx := renderType != "" || r.Header.Get("HX-Request") == "true"

	// If this is not an HTMX request, redirect. Otherwise, render a partial.
	if !isHtmx {
		utils.RedirectToWhenceYouCame(w, r, auth.ReturnPath)

		return nil
	}

	switch renderType {
	case "quick":
		// Quick thumbnail swap → render the small "add" icon only.
		return fragments.QuickAddBookmark(
			fragments.AddBookmarkData{
				ID: illustID,
			}).Render(r.Context(), w)
	case "text":
		return fragments.AddBookmarkTextButton(fragments.AddBookmarkData{
			ID: illustID,
		}).Render(r.Context(), w)
	default: // "full"
		// Full‑button swap → render the larger "add" button with updated count.
		bookmarkCount, err := strconv.Atoi(r.FormValue(bookmarkCountFormat))
		if err != nil {
			return errors.New("invalid bookmark count.")
		}

		return fragments.AddBookmarkPartial(
			core.Illust{
				ID:        illustID,
				Bookmarks: bookmarkCount - 1,
			}).Render(r.Context(), w)
	}
}

// LikeRoute handles both full‑page and HTMX fragment requests for liking an illustration.
func LikeRoute(w http.ResponseWriter, r *http.Request) error {
	auth, err := checkAuthAndTokens(r)
	if err != nil {
		return err
	}

	likeCount, err := strconv.Atoi(r.FormValue(likeCountFormat))
	if err != nil {
		return errors.New("invalid like count.")
	}

	artworkID := utils.GetPathVar(r, artworkIDFormat)
	if artworkID == "" {
		return errors.New("no ID provided.")
	}

	resp, err := requests.PostJSONBody(
		r.Context(),
		core.PostIllustLikeURL(),
		fmt.Sprintf(`{"illust_id": "%s"}`, artworkID),
		map[string]string{"PHPSESSID": auth.SessionID},
		auth.CSRFToken,
		"application/json; charset=utf-8",
		r.Header)
	if err != nil {
		return err
	}

	var likeResp illustLikeResponse
	if err := json.Unmarshal(resp, &likeResp); err != nil {
		return err
	}

	// TODO: This may instead be whether the illustration was liked *before* this request.
	// Find out if this is the case.
	// if !likeResp.IsLiked {
	// 	return errors.New("failed to like illustration.")
	// }

	_, _ = requests.InvalidateURLs([]string{"https://www.pixiv.net/ajax/illust/" + artworkID})

	renderType := r.FormValue(renderTypeFormat)

	// If this is not an HTMX request, redirect. Otherwise, render a partial.
	if r.Header.Get("HX-Request") != "true" {
		utils.RedirectToWhenceYouCame(w, r, auth.ReturnPath)

		return nil
	}

	switch renderType {
	case "text":
		return fragments.UnlikeTextButton().Render(r.Context(), w)
	default: // "full"
		return fragments.UnlikePartial(
			fragments.UnlikeData{
				Likes: likeCount + 1,
			}).Render(r.Context(), w)
	}
}

/*
NOTE: we're using the mobile API for FollowRoute since it's an actual AJAX API
			instead of some weird php thing for the usual desktop routes (/bookmark_add.php and /rpc_group_setting.php)

			the desktop routes return HTML for the pixiv SPA when they feel like it and don't return helpful responses
			when you send a request that doesn't perfectly meet their specifications, making troubleshooting a nightmare

			for comparison, the mobile API worked first try without any issues

			interestingly enough, replicating the requests for the desktop routes via cURL worked fine but a Go implementation
			just refused to work
*/

// FollowRoute handles both full‑page and HTMX fragment requests for following and unfollowing a user.
func FollowRoute(w http.ResponseWriter, r *http.Request) error {
	auth, err := checkAuthAndTokens(r)
	if err != nil {
		return err
	}

	followUserID := r.FormValue(userIDFormat)
	if followUserID == "" {
		return errors.New("no user ID provided.")
	}

	action := strings.ToLower(r.FormValue("action"))
	if action == "" {
		return errors.New("no action provided.")
	}

	var (
		mode       string
		isFollowed bool
	)

	switch action {
	case "follow", "add":
		mode = "add_bookmark_user"
		isFollowed = true
	case "unfollow", "delete":
		mode = "delete_bookmark_user"
		isFollowed = false
	default:
		return errors.New("invalid action provided.")
	}

	requestData := map[string]string{
		"mode":    mode,
		"user_id": followUserID,
	}
	// The 'restrict' parameter is only added for follow actions.
	if isFollowed {
		restrict := "0"

		privateVal := r.FormValue(privateFormat)
		if privateVal == "true" || privateVal == "on" {
			restrict = "1"
		}

		requestData["restrict"] = restrict
	}

	rawResp, err := requests.PostJSONBody(
		r.Context(),
		core.PostTouchAPI(),
		requestData,
		map[string]string{"PHPSESSID": auth.SessionID},
		auth.CSRFToken,
		"",
		r.Header)
	if err != nil {
		return err
	}

	var resp followResponse
	if err := json.Unmarshal(rawResp, &resp); err != nil {
		return fmt.Errorf("failed to parse follow response: %w", err)
	}

	if !resp.IsSucceed {
		actionStr := "unfollow"
		if isFollowed {
			actionStr = "follow"
		}

		return fmt.Errorf("%w: %s", errFailedToFollowUser, actionStr)
	}

	_, _ = requests.InvalidateURLs([]string{
		"https://www.pixiv.net/ajax/user/" + auth.UserID() + "/following",
		"https://www.pixiv.net/ajax/user/" + auth.UserID() + "?full=",
		"https://www.pixiv.net/ajax/user/" + followUserID + "?full=",
	})

	// If this is not an HTMX request, redirect. Otherwise, render a partial.
	if r.Header.Get("HX-Request") != "true" {
		utils.RedirectToWhenceYouCame(w, r, auth.ReturnPath)

		return nil
	}

	return fragments.FollowButtons(
		followUserID,
		isFollowed,
		"filled").Render(r.Context(), w)
}

// checkAuthAndTokens extracts and validates session token, CSRF token, and return path.
//
// If authentication fails, it calls UnauthorizedPage and returns an error.
// If successful, it returns the AuthData struct.
func checkAuthAndTokens(r *http.Request) (*authData, error) {
	sessionID := untrusted.GetUserToken(r)
	csrfToken := untrusted.GetCookie(r, cookie.CSRFCookie)
	returnPath := r.FormValue(returnPathFormat)

	if sessionID == "" || csrfToken == "" {
		return nil, NewUnauthorizedError(returnPath, returnPath)
	}

	return &authData{
		SessionID:  sessionID,
		CSRFToken:  csrfToken,
		ReturnPath: returnPath,
	}, nil
}
