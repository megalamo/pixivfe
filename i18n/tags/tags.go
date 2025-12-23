// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package tags

// translations holds tag-to-English mappings.
var translations = map[string]string{}

// SetTranslations replaces the in-memory tag translations map.
// The provided map is used as-is and not copied.
func SetTranslations(m map[string]string) {
	translations = m
}

// TrToEn returns an English translation for tag.
//
// No normalization is performed on input tags (for example, case folding,
// NFKC, or whitespace trimming).
//
// If no translation is found, TrToEn returns the original tag.
func TrToEn(tag string) string {
	if t, ok := translations[tag]; ok {
		return t
	}

	return tag
}
