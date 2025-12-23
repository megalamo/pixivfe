// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/pixivision"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

var errUnexpectedArticleDataType = errors.New("unexpected article data type")

func PixivisionHomePage(w http.ResponseWriter, r *http.Request) error {
	data, err := pixivision.GetHomepageWithCategories(
		r,
		utils.GetQueryParam(r, "p", pixivision.HomepageDefaultPage),
		pixivision.HomepageDefaultLang)
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

	return views.PixivisionIndex(data).Render(r.Context(), w)
}

func PixivisionArticlePage(w http.ResponseWriter, r *http.Request) error {
	data, err := pixivision.ParseArticle(
		r,
		utils.GetPathVar(r, "id"),
		[]string{pixivision.ArticleDefaultLang})
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

	switch articleData := data.(type) {
	case pixivision.ArticleFreeformData:
		return views.PixivisionArticleFreeform(articleData).Render(r.Context(), w)
	case pixivision.ArticleData:
		return views.PixivisionArticle(articleData).Render(r.Context(), w)
	default:
		return errUnexpectedArticleDataType
	}
}

func PixivisionCategoryPage(w http.ResponseWriter, r *http.Request) error {
	id := utils.GetPathVar(r, "id")
	page := utils.GetQueryParam(r, "p", pixivision.CategoryDefaultPage)

	pageInt, err := strconv.Atoi(page)
	if err != nil {
		return err
	}

	data, err := pixivision.GetCategory(r, id, page, pixivision.CategoryDefaultLang)
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

	pageData := pixivision.CategoryData{
		Category:    data,
		CurrentPage: pageInt,
		ID:          id,
		Title:       data.Title,
	}

	return views.PixivisionCategory(pageData).Render(r.Context(), w)
}

func PixivisionTagPage(w http.ResponseWriter, r *http.Request) error {
	id := utils.GetPathVar(r, "id")
	page := utils.GetQueryParam(r, "p", "1")

	pageInt, err := strconv.Atoi(page)
	if err != nil {
		return err
	}

	data, err := pixivision.GetTag(r, id, page, pixivision.TagDefaultLang)
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

	pageData := pixivision.TagData{
		Tag:   &data,
		Page:  pageInt,
		ID:    id,
		Title: data.Title,
	}

	return views.PixivisionTag(pageData).Render(r.Context(), w)
}
