// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

func setImageServer(w http.ResponseWriter, r *http.Request) (string, error) {
	customProxy := r.FormValue("custom_image_proxy")
	selectedProxy := r.FormValue("image_proxy")

	switch {
	case selectedProxy == "custom" && customProxy != "":
		return handleCustomProxy(w, r, customProxy)
	case selectedProxy != "":
		return handleSelectedProxy(w, r, selectedProxy)
	default:
		untrusted.ClearCookie(w, r, cookie.ImageProxyCookie)

		return "Image proxy server cleared. Using default proxy.", nil
	}
}

func handleCustomProxy(w http.ResponseWriter, r *http.Request, customProxy string) (string, error) {
	var proxyURL string

	if customProxy == config.BuiltInImageProxyPath {
		proxyURL = config.BuiltInImageProxyPath
	} else {
		parsedURL, err := utils.ParseURL(customProxy, "Custom image proxy")
		if err != nil {
			return "", err
		}

		proxyURL = parsedURL.String()
	}

	untrusted.SetCookie(w, r, cookie.ImageProxyCookie, proxyURL)

	return fmt.Sprintf("Image proxy server set successfully to: %s", proxyURL), nil
}

func handleSelectedProxy(w http.ResponseWriter, r *http.Request, selectedProxy string) (string, error) {
	proxyURL := selectedProxy

	untrusted.SetCookie(w, r, cookie.ImageProxyCookie, proxyURL)

	return fmt.Sprintf("Image proxy server set successfully to: %s", proxyURL), nil
}

func setVisualEffects(w http.ResponseWriter, r *http.Request) (string, error) {
	var isSuccessful bool

	seasonalEffects := r.FormValue("seasonal_effects")

	if seasonalEffects == "on" {
		untrusted.SetCookie(w, r, cookie.SeasonalEffectsEnabledCookie, "true")

		isSuccessful = true
	} else {
		untrusted.SetCookie(w, r, cookie.SeasonalEffectsEnabledCookie, "false")

		isSuccessful = true
	}

	if isSuccessful {
		return "Visual effects preference updated successfully.", nil
	}

	return "", errors.New("Invalid visual effects preference.")
}

func setNovelFontType(w http.ResponseWriter, r *http.Request) (string, error) {
	fontType := r.FormValue("font_type")
	if fontType == core.NovelFontTypeMincho || fontType == core.NovelFontTypeGothic {
		untrusted.SetCookie(w, r, cookie.NovelFontTypeCookie, fontType)

		return "Novel font type updated successfully.", nil
	}

	return "", errors.New("Invalid font type.")
}

func setNovelViewMode(w http.ResponseWriter, r *http.Request) (string, error) {
	viewMode := r.FormValue("view_mode")
	if viewMode == core.NovelViewModeHorizontal || viewMode == core.NovelViewModeVertical || viewMode == core.NovelViewModeNone {
		untrusted.SetCookie(w, r, cookie.NovelViewModeCookie, viewMode)

		return "Novel view mode updated successfully.", nil
	}

	return "", errors.New("Invalid view mode.")
}

//nolint:unparam
func setThumbnailToNewTab(w http.ResponseWriter, r *http.Request) (string, error) {
	ttnt := r.FormValue("ttnt")
	if ttnt == "_blank" {
		untrusted.SetCookie(w, r, cookie.ThumbnailToNewTabCookie, ttnt)

		return "Thumbnails will now open in a new tab.", nil
	}

	untrusted.SetCookie(w, r, cookie.ThumbnailToNewTabCookie, "_self")

	return "Thumbnails will now open in the same tab.", nil
}

//nolint:unparam
func setLogout(w http.ResponseWriter, r *http.Request) (string, error) {
	// Clear-Site-Data header with wildcard to clear everything
	w.Header().Set("Clear-Site-Data", "*")

	// Cookie clearing as fallback
	untrusted.ClearCookie(w, r, cookie.TokenCookie)
	untrusted.ClearCookie(w, r, cookie.CSRFCookie)
	untrusted.ClearCookie(w, r, cookie.PAbDIDCookie)
	untrusted.ClearCookie(w, r, cookie.PAbIDCookie)
	untrusted.ClearCookie(w, r, cookie.PAbID2Cookie)
	untrusted.ClearCookie(w, r, cookie.UsernameCookie)
	untrusted.ClearCookie(w, r, cookie.UserIDCookie)
	untrusted.ClearCookie(w, r, cookie.UserAvatarCookie)

	return "Successfully logged out.", nil
}

func setCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	key := r.FormValue("key")
	value := r.FormValue("value")

	for _, cookieName := range cookie.AllCookieNames {
		if string(cookieName) == key {
			untrusted.SetCookie(w, r, cookieName, value)

			return fmt.Sprintf("Cookie %s set successfully.", key), nil
		}
	}

	return "", fmt.Errorf("Invalid Cookie Name: %s", key)
}

func clearCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	key := r.FormValue("key")

	for _, cookieName := range cookie.AllCookieNames {
		if string(cookieName) == key {
			untrusted.ClearCookie(w, r, cookieName)

			return fmt.Sprintf("Cookie %s cleared successfully.", key), nil
		}
	}

	return "", fmt.Errorf("Invalid Cookie Name: %s", key)
}

// setRawCookie processes a multi-line string of key=value pairs
// from the "raw" form value and sets corresponding valid session cookies.
//
// It returns a status message indicating success, including skipped count if any.
//
//nolint:unparam
func setRawCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	raw := r.FormValue("raw")

	lines := strings.Split(raw, "\n")

	var (
		appliedCount int // Tracks settings that were actually applied
		skippedCount int // Tracks skipped lines
	)

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines or lines that are just comments,
		// but don't count them as skipped entries
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		// Split into exactly two parts: key and value
		const expectedParts = 2

		parts := strings.SplitN(trimmedLine, "=", expectedParts)
		if len(parts) != expectedParts {
			// Malformed lines are skipped and counted as such
			skippedCount++

			continue
		}

		// Trim potential whitespace around key and value
		keyStr := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Convert key name
		name := cookie.CookieName(keyStr)

		// Check if the cookie name is allowed
		if !slices.Contains(cookie.AllCookieNames, name) {
			skippedCount++

			continue
		}

		// Set the cookie
		untrusted.SetCookie(w, r, name, value)

		appliedCount++
	}

	// Applied count is the base message
	msgApplied := ""

	if appliedCount > 0 {
		// Manually handle plurals since fmt.Sprintf doesn't
		if appliedCount == 1 {
			msgApplied = "Applied 1 setting successfully"
		} else {
			msgApplied = fmt.Sprintf("Applied %d settings successfully", appliedCount)
		}
	}

	// Skipped count is an optional addition
	msgSkipped := ""
	if skippedCount > 0 {
		msgSkipped = fmt.Sprintf("Skipped %d invalid or unknown entries", skippedCount)
	}

	// Combine the parts conditionally
	switch {
	case appliedCount > 0 && skippedCount > 0:
		// Both applied and skipped: Combine with a period and space.
		return fmt.Sprintf("%s. %s.", msgApplied, msgSkipped), nil
	case appliedCount > 0:
		// Only applied
		return msgApplied + ".", nil
	case skippedCount > 0:
		// Only skipped (implies appliedCount is 0)
		return fmt.Sprintf("No valid settings found. %s.", msgSkipped), nil
	default: // appliedCount == 0 && skippedCount == 0
		// Neither applied nor skipped
		return "No valid settings found in the input.", nil
	}
}

//nolint:unparam
func resetAll(w http.ResponseWriter, r *http.Request) (string, error) {
	// Clear-Site-Data header with wildcard to clear everything
	w.Header().Set("Clear-Site-Data", "*")

	// Cookie clearing as fallback
	untrusted.ClearAllCookies(w, r)

	return "All preferences have been reset to default values.", nil
}

func SettingsPage(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "no-store")

	var profile core.SettingsSelfResponse

	if untrusted.GetUserToken(r) != "" {
		// TODO: Handle error appropriately, maybe show an error page or log
		p, _ := core.GetSettingsSelf(r)
		if p != nil {
			profile = *p
		}
	}

	return views.Settings(core.SettingsPageData{
		PixivData:     profile,
		FilterProfile: core.ReadFilterProfile(untrusted.GetCookie(r, cookie.FilterProfileCookie)),
	}).Render(r.Context(), w)
}

var actions = map[string]func(http.ResponseWriter, *http.Request) (string, error){
	"image_server":         setImageServer,
	"logout":               setLogout,
	"reset_all":            resetAll,
	"novel_font_type":      setNovelFontType,
	"novel_view_mode":      setNovelViewMode,
	"thumbnail_to_new_tab": setThumbnailToNewTab,
	"visual_effects":       setVisualEffects,
	"set_cookie":           setCookie,
	"clear_cookie":         clearCookie,
	"raw":                  setRawCookie,
	"token":                core.SetToken,
	"language":             core.SetLanguage,
	"location":             core.SetLocation,
	"reading_status":       core.SetReadingStatus,
	"content_filters":      core.HandleContentFilters,
	"default_search_mode":  core.HandleDefaultSearchMode,
	"blacklisted_tags":     core.HandleBlacklistedTags,
	"blacklisted_artists":  core.HandleBlacklistedArtists,
}

func SettingsPOST(w http.ResponseWriter, r *http.Request) error {
	// If a component return is requested, hand off the request immediately.
	if r.FormValue("return_component") == "true" {
		handleComponentAction(w, r)

		return nil
	}

	var (
		err     error
		message string
	)

	if action, ok := actions[utils.GetPathVar(r, "action")]; ok {
		message, err = action(w, r)
	} else {
		err = errors.New("No such setting is available.")
	}

	returnPath := r.FormValue("returnPath")

	if r.Header.Get("HX-Request") == "true" {
		switch {
		case err != nil:
			handleAJAXResponse(w, r, "", err)
		case returnPath != "":
			http.Redirect(w, r, returnPath, http.StatusSeeOther)
		default:
			handleAJAXResponse(w, r, message, nil)
		}

		return nil
	}

	if err != nil {
		return err
	}

	http.Redirect(w, r, returnPath, http.StatusSeeOther)

	return nil
}
