// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/sync/errgroup"

	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

// FilterMode specifies how a content category is treated in the local profile.
type FilterMode string

// The FilterMode values represent allowed treatments for a category in the local profile.
const (
	FilterShow   FilterMode = "show"   // display
	FilterCensor FilterMode = "censor" // render in page structure, but hide visuals
	FilterHide   FilterMode = "hide"   // do not display
)

// Form field names for the four categories. These keys are read from the
// request when updating a profile via [HandleContentFilters].
const (
	FormR15         = "mode_r15"
	FormR18         = "mode_r18"
	FormR18G        = "mode_r18g"
	FormAI          = "mode_ai"
	FormSyncToPixiv = "sync_to_pixiv"
)

// Label keys for the four categories.
const (
	LabelKeyR15  = "R-15 content"
	LabelKeyR18  = "R-18 content"
	LabelKeyR18G = "R-18G content"
	LabelKeyAI   = "AI-generated content"
)

// UnifiedContentLevel summarizes category choices into a single level that
// mirrors pixiv's global sensitivity setting.
// Used to reconcile local preferences with account-level restrictions and
// compute values for the pixiv settings API.
type UnifiedContentLevel int

// UnifiedContentLevel values describe the maximum category that may be shown.
const (
	LevelHideAll   UnifiedContentLevel = iota // no restricted content
	LevelR15                                  // permit R-15 content
	LevelR15AndR18                            // permit R-15 and R-18 content
	LevelAllowAll                             // permits R-15, R-18, and R-18G content
)

// String returns a human readable label for l.
func (l UnifiedContentLevel) String() string {
	switch l {
	case LevelHideAll:
		return "Hide all"
	case LevelR15:
		return "Allow R-15"
	case LevelR15AndR18:
		return "Allow R-15 and R-18"
	case LevelAllowAll:
		return "Allow R-15, R-18 and R-18G"
	default:
		return "Unknown"
	}
}

// FilterProfile records per-category content modes and optional extras for
// local preference management.
//
// A profile has explicit fields for each category, [FilterProfile.R15],
// [FilterProfile.R18], [FilterProfile.R18G], and [FilterProfile.AI]. The
// profile is serialised to and from JSON and stored in a browser cookie.
//
// The zero value does not represent a valid choice of modes.
// Call [ReadFilterProfile] to obtain a fully initialised profile.
type FilterProfile struct {
	Version int `json:"v"` // the schema version of the profile JSON

	R15  FilterMode `json:"r15"`  // the local mode for R-15 content
	R18  FilterMode `json:"r18"`  // the local mode for R-18 content
	R18G FilterMode `json:"r18g"` // the local mode for R-18G content
	AI   FilterMode `json:"ai"`   // the local mode for AI-generated content

	DefaultSearchMode  string   `json:"default_search_mode,omitempty"` // the default search scope ("", "all", "safe", or "r18")
	BlacklistedTags    []string `json:"blacklisted_tags,omitempty"`    // list of tags to exclude
	BlacklistedArtists []string `json:"blacklisted_artists,omitempty"` // list of artist user IDs to exclude
}

const filterProfileVersion = 1

func defaultFilterProfile() FilterProfile {
	return FilterProfile{
		Version: filterProfileVersion,
		R15:     FilterShow,
		R18:     FilterShow,
		R18G:    FilterShow,
		AI:      FilterShow,
	}
}

func isValidMode(m FilterMode) bool {
	return m == FilterShow || m == FilterCensor || m == FilterHide
}

// normalize ensures version and valid modes for all four categories.
func (fp *FilterProfile) normalize() {
	fp.Version = filterProfileVersion

	for _, m := range []*FilterMode{&fp.R15, &fp.R18, &fp.R18G, &fp.AI} {
		if !isValidMode(*m) {
			*m = FilterShow
		}
	}
}

// ReadFilterProfile reads a [FilterProfile] from a map of cookies.
// If the cookie is missing, malformed, or uses an unexpected version, it returns
// a default profile. The returned profile is normalized to a valid
// combination of modes and carries the current schema version.
func ReadFilterProfile(cookie string) FilterProfile {
	if cookie == "" {
		return defaultFilterProfile()
	}

	var fp FilterProfile
	if err := json.Unmarshal([]byte(cookie), &fp); err != nil {
		return defaultFilterProfile()
	}

	if fp.Version != filterProfileVersion {
		return defaultFilterProfile()
	}

	fp.normalize()

	return fp
}

// ComputePixivLevel returns the pixiv account's global [UnifiedContentLevel]
// as reflected by the caller's [SettingsSelfResponse].
// If s is nil or the user is not logged in, it returns [LevelAllowAll].
func ComputePixivLevel(s *SettingsSelfResponse) UnifiedContentLevel {
	if s == nil || !s.UserStatus.IsLoggedIn {
		return LevelAllowAll
	}

	if s.UserStatus.SensitiveViewSetting == 0 {
		return LevelHideAll
	}

	switch s.UserStatus.UserXRestrict {
	case "0":
		return LevelR15
	case "1":
		return LevelR15AndR18
	case "2":
		return LevelAllowAll
	default:
		return LevelR15
	}
}

// Treat Censor as not allowed when summarizing local choices into a pixiv-style level.
func coerceLocalToPixivLevel(fp FilterProfile) UnifiedContentLevel {
	r15 := fp.R15 == FilterShow
	r18 := fp.R18 == FilterShow
	r18g := fp.R18G == FilterShow

	switch {
	case r18g:
		return LevelAllowAll
	case r18:
		return LevelR15AndR18
	case r15:
		return LevelR15
	default:
		return LevelHideAll
	}
}

// ComputeEffectiveLevel returns the effective [UnifiedContentLevel] obtained
// by combining the local profile and the pixiv account level.
// The result is the minimum of the two levels.
// When summarizing local modes, Censor is treated as not allowed.
func ComputeEffectiveLevel(fp FilterProfile, s *SettingsSelfResponse) UnifiedContentLevel {
	pix := ComputePixivLevel(s)

	local := coerceLocalToPixivLevel(fp)
	if pix < local {
		return pix
	}

	return local
}

func pixivSettingsFromLevel(l UnifiedContentLevel) (sensitiveViewSetting, userXRestrict int) {
	switch l {
	case LevelHideAll:
		return 0, 0
	case LevelR15:
		return 1, 0
	case LevelR15AndR18:
		return 1, 1
	case LevelAllowAll:
		return 1, 2
	default:
		return 1, 0
	}
}

// ComputeSyncSettings derives pixiv setting values from a local [FilterProfile].
// When summarizing, Censor is treated as not allowed for the R‑15/R‑18/R‑18G level.
// For AI, pixiv only exposes a hide toggle, so AI is synced as hidden only when
// the local mode is Hide.
func ComputeSyncSettings(fp FilterProfile) (sensitive, xrestrict, hideAI int) {
	sensitive, xrestrict = pixivSettingsFromLevel(coerceLocalToPixivLevel(fp))
	if fp.AI == FilterHide {
		hideAI = 1
	}

	return
}

func parseFilterMode(val string, cur FilterMode) FilterMode {
	switch FilterMode(val) {
	case FilterShow, FilterCensor, FilterHide:
		return FilterMode(val)
	default:
		return cur
	}
}

// HandleContentFilters applies content filter updates from an HTTP form,
// persists the result in a cookie, and optionally synchronises the profile to
// the pixiv account provided by the caller.
//
// Form input keys are [FormR15], [FormR18], [FormR18G], and [FormAI].
// Each key accepts "show", "censor", or "hide". If form field values are absent,
// the previous modes are preserved. When the form field [FormSyncToPixiv] equals "1"
// and the request has a user token, the function updates the account's settings on pixiv.
//
// It returns a short user-facing message and an error.
func HandleContentFilters(w http.ResponseWriter, r *http.Request) (string, error) {
	fp := ReadFilterProfile(untrusted.GetCookie(r, cookie.FilterProfileCookie))

	updates := []struct {
		key string
		dst *FilterMode
	}{
		{FormR15, &fp.R15},
		{FormR18, &fp.R18},
		{FormR18G, &fp.R18G},
		{FormAI, &fp.AI},
	}

	for _, u := range updates {
		*u.dst = parseFilterMode(r.FormValue(u.key), *u.dst)
	}

	fp.normalize()

	b, err := json.Marshal(fp)
	if err != nil {
		return "", err
	}

	untrusted.SetCookie(w, r, cookie.FilterProfileCookie, string(b))

	// Optional sync to pixiv.
	if r.FormValue(FormSyncToPixiv) == "1" && untrusted.GetUserToken(r) != "" {
		sensitive, xrestrict, hideAI := ComputeSyncSettings(fp)

		var g errgroup.Group
		g.Go(func() error {
			return PerformSettingUpdate(r, POSTSettingsSensitiveViewURL, SetSensitiveViewRequest{SensitiveViewSetting: sensitive})
		})

		if sensitive != 0 {
			g.Go(func() error {
				return PerformSettingUpdate(r, POSTSettingsUserXRestrictURL, SetXRestrictRequest{UserXRestrict: xrestrict})
			})
		}

		g.Go(func() error {
			return PerformSettingUpdate(r, POSTSettingsHideAIWorksURL, SetAIWorksRequest{HideAIWorks: hideAI})
		})

		if err := g.Wait(); err != nil {
			return "Local preferences saved. Could not update pixiv account settings.", nil //nolint:nilerr
		}

		return "Preferences updated and synced with pixiv.", nil
	}

	return "Local preferences updated successfully.", nil
}

// stringToSlice cleans and splits a newline-separated string into a string slice.
func stringToSlice(s string) []string {
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// HandleDefaultSearchMode applies the default search mode update from an HTTP
// form and persists it in the filter profile cookie.
//
// Form input key is "default_search_mode".
func HandleDefaultSearchMode(w http.ResponseWriter, r *http.Request) (string, error) {
	fp := ReadFilterProfile(untrusted.GetCookie(r, cookie.FilterProfileCookie))

	mode := r.FormValue("default_search_mode")

	switch SearchFilterMode(mode) {
	case "", SearchFilterModeAll, SearchFilterModeSafe, SearchFilterModeR18:
		fp.DefaultSearchMode = mode
	default:
		return "", fmt.Errorf("invalid search mode: %s", mode)
	}

	b, err := json.Marshal(fp)
	if err != nil {
		return "", err
	}

	untrusted.SetCookie(w, r, cookie.FilterProfileCookie, string(b))

	return "Default search mode updated successfully.", nil
}

// HandleBlacklistedTags applies the tag blacklist update from an HTTP
// form and persists it in the filter profile cookie.
//
// Form input key is "tags". Tags are newline-separated and converted to lowercase.
func HandleBlacklistedTags(w http.ResponseWriter, r *http.Request) (string, error) {
	fp := ReadFilterProfile(untrusted.GetCookie(r, cookie.FilterProfileCookie))

	tags := r.FormValue("tags")

	fp.BlacklistedTags = stringToSlice(tags)

	for i, tag := range fp.BlacklistedTags {
		fp.BlacklistedTags[i] = strings.ToLower(tag)
	}

	b, err := json.Marshal(fp)
	if err != nil {
		return "", err
	}

	untrusted.SetCookie(w, r, cookie.FilterProfileCookie, string(b))

	return "Tag blacklist updated successfully.", nil
}

// HandleBlacklistedArtists applies the artist blacklist update from an HTTP
// form and persists it in the filter profile cookie.
//
// Form input key is "artists". Artists are newline-separated user IDs.
func HandleBlacklistedArtists(w http.ResponseWriter, r *http.Request) (string, error) {
	fp := ReadFilterProfile(untrusted.GetCookie(r, cookie.FilterProfileCookie))

	artists := r.FormValue("artists")

	fp.BlacklistedArtists = stringToSlice(artists)

	b, err := json.Marshal(fp)
	if err != nil {
		return "", err
	}

	untrusted.SetCookie(w, r, cookie.FilterProfileCookie, string(b))

	return "Artist blacklist updated successfully.", nil
}
