// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/tokenmanager"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/i18n"
)

const (
	// Endpoint URLs.
	GETSettingsSelfURL           = "https://www.pixiv.net/ajax/settings/self"
	POSTSettingsLanguageURL      = "https://www.pixiv.net/ajax/user/language"
	POSTSettingsLocationURL      = "https://www.pixiv.net/ajax/settings/location"
	POSTSettingsUserXRestrictURL = "https://www.pixiv.net/ajax/settings/user_x_restrict"
	POSTSettingsSensitiveViewURL = "https://www.pixiv.net/ajax/settings/sensitive_view_setting"
	POSTSettingsHideAIWorksURL   = "https://www.pixiv.net/ajax/settings/hide_ai_works"
	POSTSettingsReadingStatusURL = "https://www.pixiv.net/ajax/settings/reading_status"

	// PixivAccountSettingsURL is the direct URL to the user's settings on pixiv.net.
	PixivAccountSettingsURL = "https://www.pixiv.net/setting_user.php"

	// Form value keys that are used in the frontend.
	formKeyToken    = "token"
	formKeyCode     = "code"
	formKeyLocation = "location"
	formKeyOptout   = "optout"

	// THE TEST URL IS NSFW!
	// #nosec:G101 -- False positive.
	tokenArtworkURL = "https://www.pixiv.net/en/artworks/115365120"
)

var csrfRegexp = regexp.MustCompile(`\\"token\\":\\"([0-9a-fA-F]+)\\"`)

// SettingsPageData is the data used to render the settings page.
type SettingsPageData struct {
	PixivData     SettingsSelfResponse
	FilterProfile FilterProfile
}

// SettingsSelfResponse represents the structure of the response from the getSettingsSelfURL endpoint.
//
// It contains detailed information about the logged-in user's status and preferences.
type SettingsSelfResponse struct {
	// UserStatus contains all personal settings and status information for the logged-in user.
	UserStatus struct {
		// UserID is the unique numerical ID of the user.
		UserID string `json:"user_id"`

		// UserStatus is a status flag.
		UserStatus string `json:"user_status"`

		// UserAccount is the user's account name.
		UserAccount string `json:"user_account"`

		// UserName is the user's public display name.
		UserName string `json:"user_name"`

		// UserPremium indicates premium status.
		// "0" for regular user, "1" for premium.
		UserPremium string `json:"user_premium"`

		// UserBirth is the user's date of birth in "YYYY-MM-DD" format.
		UserBirth string `json:"user_birth"`

		// UserXRestrict configures R-18/R-18G content display.
		// "0": Hide, "1": Show R-18, "2": Show R-18 and R-18G.
		UserXRestrict string `json:"user_x_restrict"`

		// UserCreateTime is the timestamp when the account was created in "YYYY-MM-DD HH:MM:SS" format.
		UserCreateTime string `json:"user_create_time"`

		// UserMailAddress is the email address associated with the account.
		UserMailAddress string `json:"user_mail_address"`

		// ProfileImg contains the URL for the user's profile image.
		ProfileImg struct {
			// Main is the full URL to the user's main profile picture.
			Main string `json:"main"`
		} `json:"profile_img"`

		// Age is the user's current age, calculated from their birth date.
		Age int `json:"age"`

		// IsLoggedIn indicates whether the user is currently considered logged in.
		IsLoggedIn bool `json:"is_logged_in"`

		// StampSeries is a list of available stamp sets for use in comments.
		StampSeries []struct {
			// Slug is the identifier for the stamp set.
			Slug string `json:"slug"`

			// Name is the display name of the stamp set.
			Name string `json:"name"`

			// Stamps is a list of numerical IDs for each stamp in the set.
			Stamps []int `json:"stamps"`
		} `json:"stamp_series"`

		// EmojiSeries is a list of available emoji sets.
		EmojiSeries []struct {
			// ID is the numerical ID of the emoji.
			ID int `json:"id"`

			// Name is the identifier name of the emoji.
			Name string `json:"name"`
		} `json:"emoji_series"`

		// AdsDisabled is true if the user has a status (e.g., premium) that disables ads.
		AdsDisabled bool `json:"ads_disabled"`

		// ShowAds is true if ads should be displayed to the user.
		ShowAds bool `json:"show_ads"`

		// TwitterAccount is true if a Twitter account is linked to the pixiv account.
		TwitterAccount bool `json:"twitter_account"`

		// IsIllustCreator is true if the user is registered as an illustration creator.
		IsIllustCreator bool `json:"is_illust_creator"`

		// IsNovelCreator is true if the user is registered as a novel creator.
		IsNovelCreator bool `json:"is_novel_creator"`

		// HideAIWorks is true if the user has chosen to hide AI-generated works.
		HideAIWorks bool `json:"hide_ai_works"`

		// ReadingStatusEnabled is true if the novel reading progress feature is enabled.
		ReadingStatusEnabled bool `json:"reading_status_enabled"`

		// IllustMaskRules contains custom illustration masking rules. Usually empty.
		IllustMaskRules []any `json:"illust_mask_rules"`

		// Location is the user's selected country or region, as a two-letter code.
		// TODO: Find out whether it is always a valid ISO 3166-1 alpha-2 code.
		Location string `json:"location"`

		// SensitiveViewSetting configures the display of sensitive (R-15) content.
		// 0: Disabled, 1: Enabled.
		SensitiveViewSetting int `json:"sensitive_view_setting"`
	} `json:"user_status"`
}

// SetLanguageRequest represents the request body for setting the user's preferred language.
type SetLanguageRequest struct {
	// Code is the language code to set (e.g., "ja", "en", "ko", "zh-cn", "zh-tw").
	Code string `json:"code"`
}

// SetLocationRequest represents the request body for setting the user's country or region.
// This type is used by `SetLocation`.
type SetLocationRequest struct {
	// Location is the two-letter country code (e.g., "JP").
	// TODO: Find out whether it is always a valid ISO 3166-1 alpha-2 code.
	Location string `json:"location"`
}

// SetSensitiveViewRequest represents the request body for enabling or disabling sensitive content (R-15).
type SetSensitiveViewRequest struct {
	// SensitiveViewSetting sets sensitive content viewing.
	// 0: Disable, 1: Enable.
	//
	// NOTE: Setting this value to 0 will also hide R-18 and R-18G content (UserXRestrict=0).
	SensitiveViewSetting int `json:"sensitiveViewSetting"`
}

// SetXRestrictRequest represents the request body for configuring R-18/R-18G content display.
type SetXRestrictRequest struct {
	// UserXRestrict sets the content restriction level.
	// 0: Hide R-18/R-18G, 1: Show R-18 only, 2: Show both R-18 and R-18G.
	UserXRestrict int `json:"userXRestrict"`
}

// SetAIWorksRequest represents the request body for configuring AI-generated work visibility.
type SetAIWorksRequest struct {
	// HideAIWorks sets AI-generated work visibility.
	// 0: Show, 1: Hide.
	HideAIWorks int `json:"hideAiWorks"`
}

// SetReadingStatusRequest represents the request body for opting in or out of the novel reading progress feature.
type SetReadingStatusRequest struct {
	// Optout sets the reading progress feature.
	// 0: Enable (Opt-in), 1: Disable (Opt-out).
	Optout int `json:"optout"`
}

// SetToken attempts to log in a user using a provided PHPSESSID.
func SetToken(w http.ResponseWriter, r *http.Request) (string, error) {
	//  Get and validate the input token from the form.
	token := r.FormValue(formKeyToken)
	if token == "" {
		return "", i18n.NewUserError(r.Context(), "Empty token submitted.")
	}

	cookies := map[string]string{"PHPSESSID": token}

	// Validate the token by making an API call.
	// We only care if it succeeds, not what it returns.
	_, err := requests.GetJSONBody(
		r.Context(),
		GetNewestFromFollowingURL("illust", "all", "1"),
		cookies,
		r.Header,
	)
	if err != nil {
		return "", i18n.NewUserError(r.Context(), "Session token validation failed.")
	}

	// Fetch an artwork page to extract the CSRF token and ab cookies.
	artworkResp, _, err := requests.Do(r.Context(), requests.RequestOptions{
		Method:          http.MethodGet,
		URL:             tokenArtworkURL,
		Cookies:         cookies,
		IncomingHeaders: r.Header,
	})
	if err != nil {
		return "", i18n.NewUserError(r.Context(), "Session initialization failed.")
	}
	defer artworkResp.Body.Close()

	if artworkResp.StatusCode != http.StatusOK {
		return "", i18n.NewUserError(r.Context(), "Cannot authorize with the supplied token.")
	}

	// Parse the page body and cookies.
	artworkData, err := io.ReadAll(artworkResp.Body)
	if err != nil {
		return "", i18n.NewUserError(r.Context(), "Internal error during session setup.")
	}

	// Extract CSRF token from the HTML.
	csrfMatches := csrfRegexp.FindStringSubmatch(string(artworkData))

	const expectedCSRFMatches = 2
	if len(csrfMatches) < expectedCSRFMatches {
		return "", i18n.NewUserError(r.Context(), "Session initialization failed.")
	}

	csrfToken := csrfMatches[1]

	// Extract ab cookies and yuid_b from the response.
	foundCookies := make(map[string]string)

	for _, cookie := range artworkResp.Cookies() {
		switch cookie.Name {
		case "yuid_b", "p_ab_d_id", "p_ab_id", "p_ab_id_2":
			foundCookies[cookie.Name] = cookie.Value
		}
	}

	// NOTE: yuid_b seems to only appear for AJAX requests
	yuidb := foundCookies["yuid_b"]
	pAbDID := foundCookies["p_ab_d_id"]
	pAbID := foundCookies["p_ab_id"]
	pAbID2 := foundCookies["p_ab_id_2"]

	// If yuid_b or any of the ab cookies are missing, fail gracefully and generate new values for the empty ones.
	if yuidb == "" || pAbDID == "" || pAbID == "" || pAbID2 == "" {
		// #nosec:G404 - generation doesn't need to be cryptographically secure.
		randSrc := rand.New(rand.NewSource(time.Now().UnixNano()))
		genYUIDB, genPAbDID, genPAbID, genPAbID2 := tokenmanager.GenerateABCookies(randSrc)

		if yuidb == "" {
			yuidb = genYUIDB
		}

		if pAbDID == "" {
			pAbDID = genPAbDID
		}

		if pAbID == "" {
			pAbID = genPAbID
		}

		if pAbID2 == "" {
			pAbID2 = genPAbID2
		}
	}

	untrusted.SetCookie(w, r, cookie.YUIDBCookie, yuidb)
	untrusted.SetCookie(w, r, cookie.PAbDIDCookie, pAbDID)
	untrusted.SetCookie(w, r, cookie.PAbIDCookie, pAbID)
	untrusted.SetCookie(w, r, cookie.PAbID2Cookie, pAbID2)

	// Fetch user information using the validated token via settings/self
	selfResp, err := requests.GetJSONBody(
		r.Context(),
		GETSettingsSelfURL,
		cookies,
		r.Header,
	)
	if err != nil {
		return "", i18n.NewUserError(r.Context(), "Could not fetch user information.")
	}

	var settingsResult SettingsSelfResponse
	if err := json.Unmarshal(RewriteEscapedImageURLs(r, selfResp), &settingsResult); err != nil {
		return "", i18n.NewUserError(r.Context(), "Error processing user data.")
	}

	// Persist all required session data in cookies.
	untrusted.SetCookie(w, r, cookie.TokenCookie, token)
	untrusted.SetCookie(w, r, cookie.CSRFCookie, csrfToken)
	untrusted.SetCookie(w, r, cookie.UsernameCookie, settingsResult.UserStatus.UserName)
	untrusted.SetCookie(w, r, cookie.UserIDCookie, settingsResult.UserStatus.UserID)
	untrusted.SetCookie(w, r, cookie.UserAvatarCookie, settingsResult.UserStatus.ProfileImg.Main)

	return i18n.Tr(r.Context(), "Successfully logged in."), nil
}

// GET handlers

// GetSettingsSelf fetches personal settings and user status.
func GetSettingsSelf(r *http.Request) (*SettingsSelfResponse, error) {
	resp, err := requests.GetJSONBody(
		r.Context(),
		GETSettingsSelfURL,
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	var settingsResult SettingsSelfResponse
	if err := json.Unmarshal(RewriteEscapedImageURLs(r, resp), &settingsResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal personal settings: %w", err)
	}

	return &settingsResult, nil
}

// Settings POST handlers

// SetLanguage updates the user's preferred language on pixiv.
func SetLanguage(_ http.ResponseWriter, r *http.Request) (string, error) {
	langCode := r.FormValue(formKeyCode)
	if langCode == "" {
		return "", i18n.NewUserError(r.Context(), "Language selection is required.")
	}

	err := PerformSettingUpdate(
		r,
		POSTSettingsLanguageURL,
		SetLanguageRequest{Code: langCode})
	if err != nil {
		return "", i18n.NewUserError(r.Context(), "Failed to update language.")
	}

	return i18n.Tr(r.Context(), "Language updated successfully."), nil
}

// SetLocation updates the user's country or region setting on pixiv.
func SetLocation(_ http.ResponseWriter, r *http.Request) (string, error) {
	location := r.FormValue(formKeyLocation)
	if location == "" {
		return "", i18n.NewUserError(r.Context(), "Location is required.")
	}

	err := PerformSettingUpdate(
		r,
		POSTSettingsLocationURL,
		SetLocationRequest{Location: location})
	if err != nil {
		return "", i18n.NewUserError(r.Context(), "Failed to update location.")
	}

	return i18n.Tr(r.Context(), "Location updated successfully."), nil
}

// SetReadingStatus handles opting in or out of the feature that tracks reading progress for novels.
//
// The pixiv API uses inverted logic:
// - 0 = Enable reading progress tracking (opt-in).
// - 1 = Disable reading progress tracking (opt-out).
//
// The HTML form behavior:
// - When "Save reading progress" checkbox is checked: sends formKeyOptout="0".
// - When unchecked: formKeyOptout is not sent (empty string).
//
// Translation logic:
// - Checkbox checked (value "0") -> Enable tracking -> API value: 0.
// - Checkbox unchecked (empty) -> Disable tracking -> API value: 1.
func SetReadingStatus(_ http.ResponseWriter, r *http.Request) (string, error) {
	optout := 1 // Default: disable tracking (opt-out)

	// If checkbox was checked, enable tracking
	if r.FormValue(formKeyOptout) == "0" {
		optout = 0 // Enable tracking (opt-in)
	}

	err := PerformSettingUpdate(
		r,
		POSTSettingsReadingStatusURL,
		SetReadingStatusRequest{Optout: optout})
	if err != nil {
		return "", i18n.NewUserError(r.Context(), "Failed to update reading status.")
	}

	return i18n.Tr(r.Context(), "Reading status updated successfully."), nil
}

// PerformSettingUpdate is a helper function to handle the common logic for POSTing a setting update to pixiv.
func PerformSettingUpdate(r *http.Request, url string, payload any) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal settings payload: %w", err)
	}

	_, err = requests.PostJSONBody(
		r.Context(),
		url,
		string(jsonPayload),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		untrusted.GetCookie(r, cookie.CSRFCookie),
		"application/json",
		r.Header)
	if err != nil {
		return err
	}

	_, _ = requests.InvalidateURLs([]string{"https://www.pixiv.net/ajax/settings/self"})

	return nil
}
