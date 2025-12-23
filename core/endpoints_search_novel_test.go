package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/pixivfe/pixivfe/v3/core"
)

func TestSearchNovelSeries(t *testing.T) {
	urls := core.GetNovelSearchURLs("君を救うまで", core.NovelSearchParams{})
	assert.Equal(t, urls, core.NovelSearchURLs{
		ByTag:        `https://www.pixiv.net/ajax/search/tags/%E5%90%9B%E3%82%92%E6%95%91%E3%81%86%E3%81%BE%E3%81%A7?lang=ja`,
		ByTitleDesc:  `https://www.pixiv.net/ajax/search/novels/%E5%90%9B%E3%82%92%E6%95%91%E3%81%86%E3%81%BE%E3%81%A7?word=%E5%90%9B%E3%82%92%E6%95%91%E3%81%86%E3%81%BE%E3%81%A7&order=date_d&mode=all&p=1&csw=0&s_mode=s_tag&gs=1&lang=ja`,
		BySeriesName: `https://www.pixiv.net/ajax/stories/tag_stories?tag=%E5%90%9B%E3%82%92%E6%95%91%E3%81%86%E3%81%BE%E3%81%A7&lang=ja`,
	})
}
