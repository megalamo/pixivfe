// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package pixivision

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
)

const (
	CategoryDefaultPage = "1"
	CategoryDefaultLang = "en"
)

// GetCategory fetches and parses a category page on pixivision.
func GetCategory(r *http.Request, id, page string, lang ...string) (Category, error) {
	var category Category

	URL := generatePixivisionURL(fmt.Sprintf("c/%s/?p=%s", id, page), lang)

	userLang := determineUserLang(URL, lang...)

	cookies := map[string]string{
		"user_lang": userLang,
		"PHPSESSID": requests.NoToken,
	}

	resp, err := requests.Get(r.Context(), URL, cookies, r.Header)
	if err != nil {
		return Category{}, err
	}

	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		return Category{}, err
	}

	category.Thumbnail = parseBackgroundImage(doc.Find(".ssc__eyecatch-container").AttrOr("style", ""))
	category.Thumbnail = strings.ReplaceAll(category.Thumbnail,
		"https://s.pximg.net", "/proxy/s.pximg.net")

	category.Title = doc.Find(".ssc__name").Text()
	if category.Title == "" {
		category.Title = doc.Find(".sscs__header").Text()
	}

	category.Description = doc.Find(".ssc__descriotion").Text() // NOTE: This is a typo in the original HTML

	// Parse each article in the category page
	doc.Find("._article-card").Each(func(i int, s *goquery.Selection) {
		var article ArticleTile

		// article.ID = s.Find(".arc__title a").AttrOr("data-gtm-label", "")
		// article.Title = s.Find(".arc__title a").Text()

		article.ID = s.Find(`a[data-gtm-action="ClickTitle"]`).AttrOr("data-gtm-label", "")
		article.Title = s.Find(`a[data-gtm-action="ClickTitle"]`).Text()
		article.Category = s.Find(".arc__thumbnail-label").Text()
		article.Thumbnail = parseBackgroundImage(s.Find("._thumbnail").AttrOr("style", ""))

		date := s.Find("time._date").AttrOr("datetime", "")

		article.Date, _ = time.Parse(pixivDatetimeLayout, date)

		// Proxy the thumbnail URL for the article
		article.Thumbnail = core.RewriteImageURLs(r, article.Thumbnail)

		s.Find("._tag-list a").Each(func(i int, s *goquery.Selection) {
			var tag EmbedTag

			tag.ID = parseIDFromPixivLink(s.AttrOr("href", ""))
			tag.Name = s.AttrOr("data-gtm-label", "")
			article.Tags = append(article.Tags, tag)
		})

		category.Articles = append(category.Articles, article)
	})

	return category, nil
}
