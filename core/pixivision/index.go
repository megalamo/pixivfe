// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package pixivision

import (
	"net/http"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"

	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
)

const (
	HomepageDefaultPage = "1"
	HomepageDefaultLang = "en"

	// pixivDatetimeLayout defines the date format used by pixiv.
	pixivDatetimeLayout = "2006-01-02"
)

// GetHomepageWithCategories fetches and parses the pixivision homepage by concurrently
// fetching the actual homepage content and specific categories, returning them as IndexData.
func GetHomepageWithCategories(r *http.Request, page string, lang ...string) (IndexData, error) {
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		return IndexData{}, err
	}

	// For pages other than "1", return early with only homepage data
	if page != "1" {
		indexArticles, err := getHomepage(r, page, lang...)
		if err != nil {
			return IndexData{}, err
		}

		return IndexData{
			Title:         "pixivision",
			IndexArticles: indexArticles,
			Page:          pageInt,
		}, nil
	}

	var data IndexData

	data.Title = "pixivision"
	data.Page = pageInt

	g, ctx := errgroup.WithContext(r.Context())

	// Always fetch the main index articles
	g.Go(func() error {
		articles, err := getHomepage(r.WithContext(ctx), page, lang...)
		if err != nil {
			return err
		}

		data.IndexArticles = articles

		return nil
	})

	// Fetch popular/featured sections since page == "1"
	g.Go(func() error {
		monthlyRanking, featured, err := getPopularArticles(r.WithContext(ctx), lang...)
		if err != nil {
			return err
		}

		data.MonthlyRankingArticles = monthlyRanking
		data.FeaturedArticles = featured

		return nil
	})

	// Fetch other categories since page == "1"
	g.Go(func() error {
		category, err := GetCategory(r.WithContext(ctx), "interview", page, lang...)
		if err != nil {
			return err
		}

		data.InterviewArticles = category.Articles

		return nil
	})

	g.Go(func() error {
		category, err := GetCategory(r.WithContext(ctx), "column", page, lang...)
		if err != nil {
			return err
		}

		data.ColumnArticles = category.Articles

		return nil
	})

	g.Go(func() error {
		category, err := GetCategory(r.WithContext(ctx), "news", page, lang...)
		if err != nil {
			return err
		}

		data.NewsArticles = category.Articles

		return nil
	})

	if err := g.Wait(); err != nil {
		return IndexData{}, err
	}

	return data, nil
}

// getHomepage fetches and parses the pixivision homepage.
func getHomepage(r *http.Request, page string, lang ...string) ([]ArticleTile, error) {
	var articles []ArticleTile

	url := generatePixivisionURL("?p="+page, lang)

	userLang := determineUserLang(url, lang...)

	cookies := map[string]string{
		"user_lang": userLang,
		"PHPSESSID": requests.NoToken,
	}

	resp, err := requests.Get(r.Context(), url, cookies, r.Header)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		return nil, err
	}

	doc.Find("article.spotlight, article._article-card").Each(func(_ int, s *goquery.Selection) {
		var article ArticleTile

		date := s.Find("time._date").AttrOr("datetime", "")

		article.ID = s.Find(`a[data-gtm-action=ClickTitle]`).AttrOr("data-gtm-label", "")
		article.Title = s.Find(`a[data-gtm-action=ClickTitle]`).Text()
		article.Category = s.Find("._category-label").Text()
		article.Thumbnail = parseBackgroundImage(s.Find("._thumbnail").AttrOr("style", ""))
		article.Date, _ = time.Parse(pixivDatetimeLayout, date)

		s.Find("._tag-list a").Each(func(_ int, t *goquery.Selection) {
			var tag EmbedTag

			tag.ID = parseIDFromPixivLink(t.AttrOr("href", ""))
			tag.Name = t.AttrOr("data-gtm-label", "")

			article.Tags = append(article.Tags, tag)
		})

		articles = append(articles, article)
	})

	for i := range articles {
		articles[i].Thumbnail = core.RewriteImageURLs(r, articles[i].Thumbnail)
	}

	return articles, nil
}
