package searxng

import (
	"context"
)

func (s *SearchClient) SearchPixivNovel(ctx context.Context, title string, pageno int) (*SearchResponse, error) {
	return s.Search(ctx, SearchRequest{
		Q:      "site:pixiv.net/novel " + title,
		Pageno: pageno,
	})
}
