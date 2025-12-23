// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package pixivision

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
)

const (
	ArticleDefaultLang = "en"

	questionPrefix1 string = "── "
	questionPrefix2 string = "── "
)

// ParseArticle fetches and parses a single article on pixivision.
//
// It acts as a dispatcher, returning either ArticleData for structured
// articles or ArticleFreeformData for freeform articles.
//
// The caller should use a type switch to handle the returned interface{}.
func ParseArticle(r *http.Request, id string, lang []string) (any, error) {
	doc, err := fetchArticle(r, id, lang)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pixivision article for parsing: %w", err)
	}

	// Check if the article is freeform or structured
	isFreeform := doc.Find("._feature-article-body__pixiv_illust").Length() == 0

	if isFreeform {
		return parseFreeformArticleData(doc, r, id)
	}

	return parseStructuredArticleData(doc, r, id)
}

// parseStructuredArticleData parses an artwork-based ("structured") article.
func parseStructuredArticleData(doc *goquery.Document, r *http.Request, id string) (ArticleData, error) {
	var (
		article      ArticleData
		descParseErr error
	)

	article.ID = id
	article.IsFreeform = false

	// Parse metadata (common to all article types)
	article.Title = doc.Find("h1.am__title").Text()
	article.Category = doc.Find(".am__categoty-pr ._category-label").Text()
	// NOTE: parse error for time is intentionally ignored
	article.Date, _ = time.Parse(pixivDatetimeLayout, doc.Find("time._date").AttrOr("datetime", ""))

	// Extract the category ID from the href attribute
	categoryHref := doc.Find(".am__categoty-pr a").AttrOr("href", "")
	parts := strings.Split(categoryHref, "/c/")

	if len(parts) > 1 && len(parts[1]) > 0 {
		idAndRest := strings.Split(parts[1], "/")

		article.CategoryID = idAndRest[0]
	}

	// Find the article thumbnail and proxy it
	article.Thumbnail = core.RewriteImageURLs(r,
		strings.ReplaceAll(doc.Find(".aie__image").AttrOr("src", ""), "https://embed.pixiv.net", "/proxy/embed.pixiv.net"))

	// Parse description paragraphs
	doc.Find(".fab__paragraph p").EachWithBreak(func(_ int, pSelection *goquery.Selection) bool {
		if strings.TrimSpace(pSelection.Text()) == "" {
			return true
		}

		innerHTML, err := pSelection.Html()
		if err != nil {
			descParseErr = err

			return false
		}

		article.Description = append(article.Description, innerHTML)

		return true
	})

	if descParseErr != nil {
		return ArticleData{}, fmt.Errorf("failed to render description paragraph: %w", descParseErr)
	}

	// Parse artworks featured in the article
	doc.Find("._feature-article-body__pixiv_illust").Each(func(i int, artworkSelection *goquery.Selection) {
		var item ArticleItem

		titleLinkSelection := artworkSelection.Find(".am__work__title a.inner-link")
		userLinkSelection := artworkSelection.Find(".am__work__user-name a.inner-link")

		item.Title = titleLinkSelection.Text()
		item.ID = parseIDFromPixivLink(titleLinkSelection.AttrOr("href", ""))
		item.Username = userLinkSelection.Text()
		item.UserID = parseIDFromPixivLink(userLinkSelection.AttrOr("href", ""))

		// NOTE: the "uesr" typo is per the source HTML
		avatarSrc := artworkSelection.Find(".am__work__user-icon-container img.am__work__uesr-icon").AttrOr("src", "")

		item.Avatar = core.RewriteImageURLs(r, avatarSrc)

		artworkSelection.Find("img.am__work__illust").Each(func(_ int, imageSelection *goquery.Selection) {
			imgSrc := imageSelection.AttrOr("src", "")
			finalImgSrc := core.RewriteImageURLs(r, imgSrc)

			item.Images = append(item.Images, finalImgSrc)
		})

		article.Items = append(article.Items, item)
	})

	// Parse tags and related articles
	article.Tags = parseArticleTags(doc)

	article.NewestTaggedArticles, article.PopularTaggedArticles, article.NewestCategoryArticles = parseAllRelatedArticles(doc, r)

	return article, nil
}

// parseFreeformArticleData parses a column/interview ("freeform") article.
func parseFreeformArticleData(doc *goquery.Document, r *http.Request, id string) (ArticleFreeformData, error) {
	var article ArticleFreeformData

	article.ID = id

	// Parse header
	article.Header.Title = doc.Find("h1.am__title").Text()
	article.Header.Date, _ = time.Parse(pixivDatetimeLayout, doc.Find("time._date").AttrOr("datetime", ""))

	categoryLink := doc.Find(".am__categoty-pr a")

	article.Header.Category = Link{
		Text: categoryLink.Find("span._category-label").Text(),
		URL:  normalizeHeadingLink(categoryLink.AttrOr("href", "")),
	}

	// Parse the structured body
	bodyItems, err := parseFreeformBody(doc, r)
	if err != nil {
		return ArticleFreeformData{}, fmt.Errorf("failed to parse freeform article body: %w", err)
	}

	article.Body = bodyItems

	// Parse tags and related articles
	article.Tags = parseArticleTags(doc)
	article.NewestTaggedArticles, article.PopularTaggedArticles, article.NewestCategoryArticles = parseAllRelatedArticles(doc, r)

	return article, nil
}

// parseFreeformBody iterates over block items in a freeform article and parses them into BodyItem structs.
func parseFreeformBody(doc *goquery.Document, r *http.Request) ([]BodyItem, error) {
	var (
		bodyItems   []BodyItem
		parseErr    error
		speakerName string
	)

	doc.Find("div.am__body .article-item").EachWithBreak(func(_ int, itemSelection *goquery.Selection) bool {
		var (
			item BodyItem
			err  error
		)

		switch {
		case itemSelection.HasClass("_feature-article-body__heading"):
			item = BodyHeading{
				Text: strings.TrimSpace(itemSelection.Find("h3").Text()),
			}
		case itemSelection.HasClass("_feature-article-body__image"):
			imgEl := itemSelection.Find("img")
			image := BodyImage{
				Src: core.RewriteImageURLs(r, imgEl.AttrOr("src", "")),
				Alt: imgEl.AttrOr("alt", ""),
			}

			if linkEl := itemSelection.Find("a"); linkEl.Length() > 0 {
				image.Href = linkEl.AttrOr("href", "")
			}

			item = image
		case itemSelection.HasClass("_feature-article-body__credit"):
			item = BodyCredit{
				Text: strings.TrimSpace(itemSelection.Find("p.fab__credit").Text()),
			}
		case itemSelection.HasClass("_feature-article-body__question"):
			questionText := strings.TrimSpace(itemSelection.Find(".fab__paragraph.question p").Text())

			questionText = strings.TrimPrefix(questionText, questionPrefix1)
			questionText = strings.TrimPrefix(questionText, questionPrefix2)
			item = BodyQuestion{
				Text: questionText,
			}
		case itemSelection.HasClass("_feature-article-body__answer"):
			htmlContent, htmlErr := itemSelection.Find(".answer-text._medium-editor-text").Html()
			if htmlErr != nil {
				parseErr = fmt.Errorf("failed to render answer block HTML: %w", htmlErr)

				return false
			}

			rewrittenContent := core.RewriteImageURLs(r, htmlContent)

			item = BodyAnswer{
				ImageSrc:    core.RewriteImageURLs(r, itemSelection.Find("img").AttrOr("src", "")),
				AuthorName:  speakerName,
				HTMLContent: rewrittenContent,
			}
		case itemSelection.HasClass("_feature-article-body__paragraph"), itemSelection.HasClass("_feature-article-body__link"):
			item, err = parseBodyRichText(itemSelection, r)
		case itemSelection.HasClass("_feature-article-body__article_card"):
			item = parseBodyArticleCard(itemSelection, r)
		case itemSelection.HasClass("_feature-article-body__booth_link"):
			item = parseBodyBoothLink(itemSelection, r)
		case itemSelection.HasClass("_feature-article-body__caption"):
			item = BodyCaption{
				Text: strings.TrimSpace(itemSelection.Find(".fab__caption p").Text()),
			}
		case itemSelection.HasClass("_feature-article-body__profile"):
			item, err = parseBodyProfile(itemSelection, r)
			if err == nil {
				if profile, ok := item.(AuthorProfile); ok {
					speakerName = profile.Name
				}
			}
		default:
			return true
		}

		if err != nil {
			parseErr = fmt.Errorf("failed to parse article body item: %w", err)

			return false
		}

		if item != nil {
			bodyItems = append(bodyItems, item)
		}

		return true
	})

	if parseErr != nil {
		return nil, parseErr
	}

	return bodyItems, nil
}

func parseBodyRichText(sel *goquery.Selection, r *http.Request) (BodyItem, error) {
	paragraphSel := sel.Find(".fab__paragraph")

	var (
		sb       strings.Builder
		parseErr error
	)

	paragraphSel.Children().EachWithBreak(func(_ int, child *goquery.Selection) bool {
		// Skip empty paragraph tags that don't contain an image.
		if child.Is("p") && child.Find("img").Length() == 0 && strings.TrimSpace(child.Text()) == "" {
			return true
		}

		// Normalize internal pixivision links before extracting HTML
		child.Find("a[href*='pixivision.net']").Each(func(_ int, linkSel *goquery.Selection) {
			if href, exists := linkSel.Attr("href"); exists {
				if parsedURL, err := url.Parse(href); err == nil {
					linkSel.SetAttr("href", normalizeHeadingLink(parsedURL.Path))
				}
			}
		})

		html, err := goquery.OuterHtml(child)
		if err != nil {
			parseErr = fmt.Errorf("could not render rich text child HTML: %w", err)

			return false
		}

		sb.WriteString(html)

		return true
	})

	if parseErr != nil {
		return nil, parseErr
	}

	content := sb.String()
	if content == "" {
		// Return nil if no content was generated, so an empty item is not added.
		//nolint:nilnil
		return nil, nil
	}

	rewrittenContent := core.RewriteImageURLs(r, content)

	return BodyRichText{HTMLContent: rewrittenContent}, nil
}

func parseBodyArticleCard(sel *goquery.Selection, r *http.Request) BodyArticleCard {
	cardSelection := sel.Find("article._article-card")

	var item ArticleTile

	thumbLink := cardSelection.Find(".arc__thumbnail-container > a")

	item.ID = thumbLink.AttrOr("data-gtm-label", "")

	style := thumbLink.Find("._thumbnail").AttrOr("style", "")
	rawThumbURL := parseBackgroundImage(style)

	item.Thumbnail = core.RewriteImageURLs(r, rawThumbURL)

	item.Category = cardSelection.Find("span.arc__thumbnail-label").Text()

	categoryLink := cardSelection.Find(".arc__thumbnail-label").Parent().AttrOr("href", "")
	catParts := strings.Split(categoryLink, "/c/")

	if len(catParts) > 1 {
		item.CategoryID = strings.Split(catParts[1], "/")[0]
	}

	titleLink := cardSelection.Find("h2.arc__title > a")

	item.Title = titleLink.Text()

	if item.ID == "" {
		item.ID = titleLink.AttrOr("data-gtm-label", "")
	}

	if item.ID == "" {
		item.ID = parseIDFromPixivLink(titleLink.AttrOr("href", ""))
	}

	dateStr := cardSelection.Find("time._date").AttrOr("datetime", "")

	item.Date, _ = time.Parse("2006-01-02", dateStr)

	cardSelection.Find("._tag-list a").Each(func(_ int, tagSel *goquery.Selection) {
		item.Tags = append(item.Tags, EmbedTag{
			ID:   parseIDFromPixivLink(tagSel.AttrOr("href", "")),
			Name: strings.TrimSpace(tagSel.Text()),
		})
	})

	return BodyArticleCard{Article: item}
}

func parseBodyBoothLink(sel *goquery.Selection, r *http.Request) BodyBoothLink {
	authorLink := sel.Find(".author-img-container")
	titleLink := sel.Find(".illust-title > a")

	item := BodyBoothLink{
		AuthorImageSrc: core.RewriteImageURLs(r, authorLink.Find("img").AttrOr("src", "")),
		AuthorURL:      authorLink.AttrOr("href", ""),
		AuthorName:     sel.Find("a.author").Text(),
		ItemTitle:      titleLink.Text(),
		ItemURL:        titleLink.AttrOr("href", ""),
		ItemImageSrc:   core.RewriteImageURLs(r, sel.Find(".illust-wrap img").AttrOr("src", "")),
	}

	return item
}

func parseBodyProfile(sel *goquery.Selection, r *http.Request) (AuthorProfile, error) {
	profileSel := sel.Find(".making-profile .profile-wrapper")
	imgSrc := profileSel.Find("img").AttrOr("src", "")

	profile := AuthorProfile{
		ImageSrc: core.RewriteImageURLs(r, imgSrc),
		Name:     profileSel.Find(".profile-contents > ul > li").First().Text(),
	}

	bioHTML, err := profileSel.Find("._medium-editor-text").Html()
	if err != nil {
		return AuthorProfile{}, fmt.Errorf("could not find author bio: %w", err)
	}

	profile.BioHTML = core.RewriteImageURLs(r, bioHTML)

	profileSel.Find(".profile-contents > ul > li").Last().Find("a").Each(func(_ int, linkSel *goquery.Selection) {
		entry := core.SocialEntry{
			Platform: strings.ToLower(linkSel.Text()),
			URL:      linkSel.AttrOr("href", ""),
		}
		entry.CleanURL()

		profile.Links = append(profile.Links, entry)
	})

	return profile, nil
}

// Common parsers

// parseArticleTags parses tags associated with the article.
func parseArticleTags(doc *goquery.Document) []EmbedTag {
	var tags []EmbedTag

	doc.Find(".am__tags ._tag-list a").Each(func(i int, tagSelection *goquery.Selection) {
		tags = append(tags, EmbedTag{
			ID:   parseIDFromPixivLink(tagSelection.AttrOr("href", "")),
			Name: tagSelection.AttrOr("data-gtm-label", ""),
		})
	})

	return tags
}

// parseAllRelatedArticles finds and parses all "related articles" sections on the page.
func parseAllRelatedArticles(doc *goquery.Document, r *http.Request) (RelatedArticleGroup, RelatedArticleGroup, RelatedArticleGroup) {
	var newestTagged, popularTagged, newestCategory RelatedArticleGroup

	doc.Find("div._related-articles[data-gtm-category='Related Article Latest']").Each(func(i int, section *goquery.Selection) {
		if section.Find("ul.rla__list-group").Length() > 0 {
			newestTagged = parseRelatedArticleSection(section, r)
		}
	})

	doc.Find("div._related-articles[data-gtm-category='Related Article Popular']").Each(func(i int, section *goquery.Selection) {
		if section.Find("ul.rla__list-group").Length() > 0 {
			popularTagged = parseRelatedArticleSection(section, r)
		}
	})

	doc.Find("div._related-articles[data-gtm-category='Article Latest']").Each(func(i int, section *goquery.Selection) {
		if section.Find("ul.rla__list-group").Length() > 0 {
			newestCategory = parseRelatedArticleSection(section, r)
		}
	})

	return newestTagged, popularTagged, newestCategory
}

// fetchArticle fetches the pixivision article page and returns it as a goquery.Document.
func fetchArticle(r *http.Request, id string, lang []string) (*goquery.Document, error) {
	URL := generatePixivisionURL("a/"+id, lang)
	userLang := determineUserLang(URL, lang...)

	cookies := map[string]string{
		"user_lang": userLang,
		"PHPSESSID": requests.NoToken,
	}

	// Fetch the article page
	resp, err := requests.Get(r.Context(), URL, cookies, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch article page: %w", err)
	}

	// Parse HTML response
	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return doc, nil
}

// parseRelatedArticleSection parses a list of related articles from a div._related-articles selection.
func parseRelatedArticleSection(sectionSelection *goquery.Selection, r *http.Request) RelatedArticleGroup {
	var group RelatedArticleGroup

	group.HeadingLink = normalizeHeadingLink(sectionSelection.Find("h3.rla__heading a").AttrOr("href", ""))

	sectionSelection.Find("ul.rla__list-group li.rla__list-item article._article-summary-card-related").Each(func(i int, s *goquery.Selection) {
		var item ArticleTile

		thumbLinkSelection := s.Find("a.ascr__thumbnail-container")

		item.ID = thumbLinkSelection.AttrOr("data-gtm-label", "")

		styleAttr := thumbLinkSelection.Find("div._thumbnail").AttrOr("style", "")
		rawThumbnailURL := parseBackgroundImage(styleAttr)

		item.Thumbnail = core.RewriteImageURLs(r, rawThumbnailURL)

		item.Category = s.Find("div.ascr__category-pr a span._category-label").Text()

		titleElement := s.Find("div.ascr__title-container a h4.ascr__title")

		item.Title = titleElement.Text()

		if item.ID == "" {
			item.ID = parseIDFromPixivLink(titleElement.Parent().AttrOr("href", ""))
		}

		group.Articles = append(group.Articles, item)
	})

	return group
}
