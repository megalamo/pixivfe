// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package pixivision

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
)

const (
	TagDefaultLang = "en"
)

var articleCountRegexp = regexp.MustCompile(`(\d+)\s+article\(s\)`)

// GetTag fetches and parses a tag page on pixivision.
func GetTag(r *http.Request, id, page string, lang ...string) (Tag, error) {
	var tag Tag

	URL := generatePixivisionURL(fmt.Sprintf("t/%s/?p=%s", id, page), lang)

	resp, err := requests.Get(r.Context(), URL, map[string]string{
		"user_lang": determineUserLang(URL, lang...),
		"PHPSESSID": requests.NoToken,
	}, r.Header)
	if err != nil {
		return tag, err
	}

	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		return tag, err
	}

	tag.Title = doc.Find(".tdc__header h1").Text()
	if tag.Title == "" {
		tag.Title = doc.Find("li.brc__list-item:nth-child(3)").Text()
	}

	tag.ID = id

	// Extract and process the description
	fullDescription := doc.Find(".tdc__description").Text()
	parts := strings.Split(fullDescription, "pixivision") // Split on the "pixivision currently has ..." boilerplate.

	tag.Description = strings.TrimSpace(parts[0])

	// Extract thumbnail
	tag.Thumbnail = parseBackgroundImage(doc.Find(".tdc__thumbnail").AttrOr("style", ""))
	if strings.HasPrefix(tag.Thumbnail, "https://source.pixiv.net") {
		tag.Thumbnail = strings.ReplaceAll(tag.Thumbnail, "https://source.pixiv.net", "/proxy/source.pixiv.net")
	} else {
		tag.Thumbnail = core.RewriteImageURLs(r, tag.Thumbnail)
	}

	// Extract total number of articles if available.
	if len(parts) > 1 {
		matches := articleCountRegexp.FindStringSubmatch(parts[1])

		if len(matches) > 1 {
			tag.Total, _ = strconv.Atoi(matches[1])
		}
	}

	// Parse each article in the tag page.
	doc.Find("._article-card").Each(func(i int, s *goquery.Selection) {
		var article ArticleTile

		article.ID = s.Find(`a[data-gtm-action="ClickTitle"]`).AttrOr("data-gtm-label", "")
		article.Title = s.Find(`a[data-gtm-action="ClickTitle"]`).Text()
		article.Category = s.Find(".arc__thumbnail-label").Text()
		article.Thumbnail = core.RewriteImageURLs(r, parseBackgroundImage(s.Find("._thumbnail").AttrOr("style", "")))
		article.Date, _ = time.Parse(pixivDatetimeLayout, s.Find("time._date").AttrOr("datetime", ""))

		// Parse tags associated with the article.
		s.Find("._tag-list a").Each(func(i int, t *goquery.Selection) {
			var ttag EmbedTag

			ttag.ID = parseIDFromPixivLink(t.AttrOr("href", ""))
			ttag.Name = t.AttrOr("data-gtm-label", "")
			article.Tags = append(article.Tags, ttag)
		})

		tag.Articles = append(tag.Articles, article)
	})

	return tag, nil
}
