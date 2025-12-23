// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"sort"

	"golang.org/x/text/language"
)

// BaseLocale is the default locale used when no specific locale is set.
const BaseLocale = "en"

// baseTag is the canonical tag for BaseLocale.
var baseTag = language.Make(BaseLocale)

// Languages returns the list of supported language tags derived from
// the loaded gettext catalogs.
//
// The returned slice is a copy, is sorted by tag string, and is safe to retain.
//
// Setup must be called successfully before using Languages; otherwise it panics.
func Languages() []language.Tag {
	if matcher == nil {
		panic("i18n: Setup must be called before calling Languages")
	}

	out := make([]language.Tag, len(supportedTags))
	copy(out, supportedTags)

	// Sort by canonical tag string.
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })

	return out
}
