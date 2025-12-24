package fragments

import (
	"context"

	"codeberg.org/pixivfe/pixivfe/v3/server/request_context"
	"codeberg.org/pixivfe/pixivfe/v3/server/template/commondata"
)

func CommonData(ctx context.Context) commondata.PageCommonData {
	return request_context.FromContext(ctx).CommonData
}
