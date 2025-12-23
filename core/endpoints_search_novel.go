package core

import (
	"fmt"
	"net/url"
)

type NovelSearchParams struct {
	Order SearchOrder
	Mode  SearchFilterMode
	Page  int
}

type NovelSearchURLs struct {
	ByTag        string
	BySeriesName string
	// by work title and work description
	ByTitleDesc string
}

// Search novels by tag, title&description, series
//
// Only title&description has page, XRestrict and other settings. the two other endpoints have no setting.
func GetNovelSearchURLs(searchTerm string, params NovelSearchParams) NovelSearchURLs {
	if params.Order == "" {
		params.Order = SearchDefaultOrder
	}

	if params.Mode == "" {
		params.Mode = "all"
	}

	if params.Page == 0 {
		params.Page = 1
	}

	return NovelSearchURLs{
		ByTag: fmt.Sprintf(
			`https://www.pixiv.net/ajax/search/tags/%s?lang=ja`,
			url.PathEscape(searchTerm)),
		BySeriesName: fmt.Sprintf(
			`https://www.pixiv.net/ajax/stories/tag_stories?tag=%s&lang=ja`,
			url.QueryEscape(searchTerm)),
		ByTitleDesc: fmt.Sprintf(
			`https://www.pixiv.net/ajax/search/novels/%s?word=%s&order=%s&mode=%s&p=%d&csw=0&s_mode=s_tag&gs=1&lang=ja`,
			url.PathEscape(searchTerm),
			url.QueryEscape(searchTerm),
			params.Order,
			params.Mode,
			params.Page),
	}
}
