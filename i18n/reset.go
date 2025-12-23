// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build test

/*
This file is included only when built with '-tags test'.
It provides a reset hook for unit tests. It is not part of production builds.
*/

package i18n

import (
	"sync"

	"golang.org/x/text/language"

	"codeberg.org/pixivfe/pixivfe/v3/i18n/tags"
)

// ResetForTests clears global state so tests can exercise Setup multiple times.
//
// Usage:
//
//	go test -tags test ./...
//
// Concurrency: only call from tests before spinning up any goroutines that
// use this package. After resetting, call Setup again to initialize.
func ResetForTests() {
	// Clear missing translation dedupe state.
	missingKeyOnce = sync.Map{}

	// Clear loaded locales and matcher.
	localesByTag = nil
	supportedTags = nil
	matcher = nil

	// Ensure baseTag remains correct.
	baseTag = language.Make(BaseLocale)

	// Reset tag translations subpackage.
	tags.ResetForTests()
}
