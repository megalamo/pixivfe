// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build test

package tags

// ResetForTests clears translations.
func ResetForTests() {
	translations = map[string]string{}
}
