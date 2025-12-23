// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"context"
	"net/http"
	"strings"

	"golang.org/x/text/language"

	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

type contextKeyType struct{}

var tagKey = contextKeyType{}

// LangParam is the name of the URL query parameter used by HTTP helpers to read
// a preferred UI language as a BCP 47 tag. The cookie counterpart is [cookie.LangCookie].
const LangParam = "lang"

// WithTag stores t in ctx and returns a derived context that carries it.
//
// The returned context should be passed to downstream code that performs
// translations. Passing the zero value of [language.Tag] clears any existing value.
//
// The ctx must not be nil.
func WithTag(ctx context.Context, t language.Tag) context.Context {
	return context.WithValue(ctx, tagKey, t)
}

// TagFrom returns the language tag stored in ctx, or the tag for [BaseLocale]
// if none is present. It never returns the zero value of [language.Tag].
//
// Setup must be called successfully before using [Tr] or [Languages], otherwise
// those functions will panic. TagFrom itself does not panic and simply returns
// the base language tag when no tag is found in ctx or ctx is nil.
func TagFrom(ctx context.Context) language.Tag {
	if ctx != nil {
		if t, _ := ctx.Value(tagKey).(language.Tag); t != (language.Tag{}) {
			return t
		}
	}

	return baseTag
}

// FromRequest returns the best language tag for r by inspecting user preferences
// in priority order:
// 1) query parameter [LangParam]
// 2) cookie [cookie.LangCookie]
// 3) Accept-Language header
//
// Special case: if [LangParam] is "auto" (case-insensitive), the cookie is ignored
// and only the Accept-Language header is considered.
//
// If r is nil, or if Setup has not been called, FromRequest returns the tag for [BaseLocale].
func FromRequest(r *http.Request) language.Tag {
	// If r is nil, or if Setup has not been called, fall back to baseTag.
	if r == nil || matcher == nil {
		return baseTag
	}

	// Highest priority: explicit query parameter.
	q := r.URL.Query().Get(LangParam)
	auto := strings.EqualFold(q, "auto")

	preferred := make([]string, 0, 3)
	if q != "" && !auto {
		preferred = append(preferred, q)
	}

	// Next: cookie (skipped if "auto" was explicitly requested).
	if !auto {
		if c := untrusted.GetCookie(r, cookie.LangCookie); c != "" {
			preferred = append(preferred, c)
		}
	}

	// Finally: Accept-Language header.
	if al := r.Header.Get("Accept-Language"); al != "" {
		preferred = append(preferred, al)
	}

	// Match the user's preferences.
	tag, _ := language.MatchStrings(matcher, preferred...)

	return tag
}

// WithRequest resolves the language from r using [FromRequest] and installs the
// matched tag in the returned context. It is equivalent to:
//
//	WithTag(ctx, FromRequest(r))
//
// If r is nil or Setup has not been called, the tag for [BaseLocale] is installed.
// The ctx must not be nil.
func WithRequest(ctx context.Context, r *http.Request) context.Context {
	return WithTag(ctx, FromRequest(r))
}
