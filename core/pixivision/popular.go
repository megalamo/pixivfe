// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package pixivision

import (
	"net/http"

	"github.com/PuerkitoBio/goquery"

	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
)

// getPopularArticles fetches and parses the pixivision popular articles page (/p).
//
// This page contains "Monthly Ranking" and "Featured" sections.
//
// Returns monthlyRankingArticles and featuredArticles as separate slices of ArticleTile.
func getPopularArticles(r *http.Request, lang ...string) ([]ArticleTile, []ArticleTile, error) {
	var (
		monthlyRankingArticles []ArticleTile
		featuredArticles       []ArticleTile
	)

	pageURL := generatePixivisionURL("p", lang)
	userLang := determineUserLang(pageURL, lang...)

	cookies := map[string]string{
		"user_lang": userLang,
		"PHPSESSID": requests.NoToken,
	}

	resp, err := requests.Get(r.Context(), pageURL, cookies, r.Header)
	if err != nil {
		return nil, nil, err
	}

	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		return nil, nil, err
	}

	// Find each major section, e.g., "Monthly Ranking", "Featured"
	doc.Find("div._articles-list-card-container").Each(func(_ int, sectionSelection *goquery.Selection) {
		sectionTitle := sectionSelection.Find("h1.alc__heading").Text()

		var currentArticles []ArticleTile

		// Find each article within the section
		sectionSelection.Find("article._article-summary-card").Each(func(_ int, articleSelection *goquery.Selection) {
			var article ArticleTile

			titleLink := articleSelection.Find("a.gtm__act-ClickTitle")

			article.ID = parseIDFromPixivLink(titleLink.AttrOr("href", ""))
			article.Title = titleLink.Find("h2.asc__title").Text()

			article.Category = articleSelection.Find("span._category-label").Text()

			styleAttr := articleSelection.Find("div._thumbnail").AttrOr("style", "")

			rawThumbURL := parseBackgroundImage(styleAttr)
			if rawThumbURL != "" {
				article.Thumbnail = core.RewriteImageURLs(r, rawThumbURL)
			}

			currentArticles = append(currentArticles, article)
		})

		// Assign to appropriate slice based on section title
		if len(currentArticles) > 0 {
			switch sectionTitle {
			case "Monthly Ranking":
				monthlyRankingArticles = currentArticles
			case "Featured":
				featuredArticles = currentArticles
			}
		}
	})

	return monthlyRankingArticles, featuredArticles, nil
}
