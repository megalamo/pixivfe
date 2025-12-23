// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core_test

import (
	"errors"
	"testing"

	. "codeberg.org/pixivfe/pixivfe/v3/core"
)

func TestXRestrict_IsNSFWRating(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		x        XRestrict
		expected bool
	}{
		{"Safe", Safe, false},
		{"R-18", R18, true},
		{"R-18G", R18G, true},
		{"All", All, false},
		{"Invalid", -1, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.x.IsNSFWRating(); got != tc.expected {
				t.Errorf("XRestrict(%d).IsNSFWRating() = %v, want %v", tc.x, got, tc.expected)
			}
		})
	}
}

func TestXRestrict_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		x        XRestrict
		expected string
	}{
		{"Safe", Safe, "Safe"},
		{"R-18", R18, "R-18"},
		{"R-18G", R18G, "R-18G"},
		{"All", All, "All"},
		{"Invalid", -1, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.x.Tr(t.Context()); got != tc.expected {
				t.Errorf("XRestrict(%d).String() = %q, want %q", tc.x, got, tc.expected)
			}
		})
	}
}

func TestXRestrict_UnhyphenatedString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		x        XRestrict
		expected string
	}{
		{"Safe", Safe, "Safe"},
		{"R-18", R18, "R18"},
		{"R-18G", R18G, "R18G"},
		{"All", All, "All"},
		{"Invalid", -1, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.x.UnhyphenatedString(); got != tc.expected {
				t.Errorf("XRestrict(%d).UnhyphenatedString() = %q, want %q", tc.x, got, tc.expected)
			}
		})
	}
}

func TestParseXRestrict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  XRestrict
	}{
		{"Safe", "Safe", Safe},
		{"Safe lowercase", "safe", Safe},
		{"R-18 with hyphen", "R-18", R18},
		{"R18 without hyphen", "R18", R18},
		{"R-18 lowercase", "r-18", R18},
		{"R18 lowercase without hyphen", "r18", R18},
		{"R-18G with hyphen", "R-18G", R18G},
		{"R18G without hyphen", "R18G", R18G},
		{"R-18G lowercase", "r-18g", R18G},
		{"R18G lowercase without hyphen", "r18g", R18G},
		{"All lowercase", "all", All},
		{"All capitalized", "All", All},
		{"Invalid value", "something else", -1},
		{"Empty string", "", -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ParseXRestrict(tc.input); got != tc.want {
				t.Errorf("ParseXRestrict(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestAIType_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		a           AIType
		expectedStr string
		expectedErr error
	}{
		{"Unrated", Unrated, "Unrated", nil},
		{"Not AI-generated", NotAIGenerated, "Not AI-generated", nil},
		{"AI-generated", AIGenerated, "AI-generated", nil},
		{"Invalid", -1, "", ErrInvalidAIType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotStr, gotErr := tc.a.Tr(t.Context())

			if gotStr != tc.expectedStr {
				t.Errorf("AIType(%d).String() returned string %q, want %q", tc.a, gotStr, tc.expectedStr)
			}

			if !errors.Is(gotErr, tc.expectedErr) {
				t.Errorf("AIType(%d).String() returned error %v, want %v", tc.a, gotErr, tc.expectedErr)
			}
		})
	}
}

func TestIllustType_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		i           IllustType
		expectedStr string
		expectedErr error
	}{
		{"Illustration", Illustration, "Illustration", nil},
		{"Manga", Manga, "Manga", nil},
		{"Ugoira", Ugoira, "Ugoira", nil},
		{"Novels", Novels, "Novels", nil},
		{"Invalid", -1, "", ErrInvalidIllustType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotStr, gotErr := tc.i.Tr(t.Context())

			if gotStr != tc.expectedStr {
				t.Errorf("IllustType(%d).String() returned string %q, want %q", tc.i, gotStr, tc.expectedStr)
			}

			if !errors.Is(gotErr, tc.expectedErr) {
				t.Errorf("IllustType(%d).String() returned error %v, want %v", tc.i, gotErr, tc.expectedErr)
			}
		})
	}
}

func TestParseIllustType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  IllustType
	}{
		{"Illustration lowercase", "illustration", Illustration},
		{"Illustration capitalized", "Illustration", Illustration},
		{"Illustrations plural", "illustrations", Illustration},
		{"Manga lowercase", "manga", Manga},
		{"Manga capitalized", "Manga", Manga},
		{"Ugoira lowercase", "ugoira", Ugoira},
		{"Ugoira capitalized", "Ugoira", Ugoira},
		{"Novel lowercase", "novel", Novels},
		{"Novels lowercase", "novels", Novels},
		{"Novels capitalized", "Novels", Novels},
		{"Invalid value", "something else", -1},
		{"Empty string", "", -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ParseIllustType(tc.input); got != tc.want {
				t.Errorf("ParseIllustType(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}
