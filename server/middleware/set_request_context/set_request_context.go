// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package set_request_context

import (
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/server/middleware/limiter"
	"codeberg.org/pixivfe/pixivfe/v3/server/request_context"
)

// WithRequestContext is a middleware that attaches a RequestContext to each HTTP request.
func WithRequestContext(w http.ResponseWriter, r *http.Request, next http.Handler) {
	next.ServeHTTP(w, r.WithContext(request_context.WithRequestContext(r.Context(), r, limiter.GetOrCreateLinkToken)))
}
