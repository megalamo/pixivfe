// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

func SelfUserPage(w http.ResponseWriter, r *http.Request) error {
	token := untrusted.GetUserToken(r)

	if token == "" {
		return NewUnauthorizedError(utils.GetQueryParam(r, "noAuthReturnPath"), "/self")
	}

	// The left part of the token is the member ID
	userID := strings.Split(token, "_")

	http.Redirect(w, r, "/users/"+userID[0], http.StatusSeeOther)

	return nil
}

func SelfBookmarksPage(w http.ResponseWriter, r *http.Request) error {
	token := untrusted.GetUserToken(r)

	if token == "" {
		return NewUnauthorizedError(utils.GetQueryParam(r, "noAuthReturnPath"), "/self/bookmarks")
	}

	// The left part of the token is the member ID
	userID := strings.Split(token, "_")

	http.Redirect(w, r, "/users/"+userID[0]+"?category=bookmarks", http.StatusSeeOther)

	return nil
}

func SelfFollowingUsersPage(w http.ResponseWriter, r *http.Request) error {
	token := untrusted.GetUserToken(r)

	if token == "" {
		return NewUnauthorizedError(utils.GetQueryParam(r, "noAuthReturnPath"), "/self/followingUsers")
	}

	// The left part of the token is the member ID
	userID := strings.Split(token, "_")

	http.Redirect(w, r, "/users/"+userID[0]+"?category=following", http.StatusSeeOther)

	return nil
}

// SelfFollowingWorksPage is the route handler for the Following works page.
func SelfFollowingWorksPage(w http.ResponseWriter, r *http.Request) error {
	if untrusted.GetUserToken(r) == "" {
		return NewUnauthorizedError(utils.GetQueryParam(r, "noAuthReturnPath"), "/self/followingWorks")
	}

	currentPage := utils.GetQueryParam(r, "page", core.NewestFromFollowingDefaultPageStr)

	currentPageInt, err := strconv.Atoi(currentPage)
	if err != nil {
		return err
	}

	searchMode := core.SearchDefaultMode(r)

	works, err := core.GetNewestFromFollowing(
		r,
		core.NewestFromFollowingDefaultCategory,
		utils.GetQueryParam(r, "mode", searchMode),
		currentPage)
	if err != nil {
		return err
	}

	pageData := core.FollowingData{
		Title:       "Latest works by followed users",
		Data:        works,
		CurrentPage: currentPageInt,
	}

	return views.Following(pageData, searchMode).Render(r.Context(), w)
}
