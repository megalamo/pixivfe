// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"sync"

	"github.com/leonelquinteros/gotext"
	"github.com/rs/zerolog"
	"golang.org/x/text/language"

	"codeberg.org/pixivfe/pixivfe/v3/config"
)

var (
	// Logger is the logger used by package i18n.
	Logger zerolog.Logger

	// missingKeyOnce deduplicates WARN logs for missing msgids in strict mode.
	// The key is locale+"\x00"+msgid.
	missingKeyOnce sync.Map
)

func strictMissingKeys() bool {
	return config.Global.Internationalization.StrictMissingKeys
}

// logMissingOnce logs a missing translation warning once per (locale, msgid) pair
// when strict mode is enabled.
func logMissingOnce(locale, key string) {
	if !strictMissingKeys() {
		return
	}

	id := locale + "\x00" + key
	if _, loaded := missingKeyOnce.LoadOrStore(id, struct{}{}); !loaded {
		Logger.Warn().
			Str("locale", locale).
			Str("key", key).
			Msg("Missing i18n translation")
	}
}

// strippedTagString removes variants to form a stable key using base, script and region only.
func strippedTagString(tag language.Tag) string {
	b, s, r := tag.Raw()
	stripped, _ := language.Compose(b, s, r)

	return stripped.String()
}

// buildLogKey composes the logging key like gettext "ctx<sep>msgid" when context is present.
func buildLogKey(ctxKey, id string) string {
	if ctxKey != "" {
		return ctxKey + gotext.EotSeparator + id
	}

	return id
}
