// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
This package defines the cookie names used by this application.
*/
package cookie

type CookieName string

// Cookie names defined as constants.
//
// NOTE: We don't use the `__Host-` prefix to avoid login issues on non-HTTPS deployments
// where the localhost exemption doesn't apply (ref: https://codeberg.org/PixivFE/PixivFE/issues/132).
const (
	// Authentication and user identity cookies.
	TokenCookie      CookieName = "Token" // #nosec:G101 - false positive
	CSRFCookie       CookieName = "CSRF"
	YUIDBCookie      CookieName = "YUID-B"
	PAbDIDCookie     CookieName = "P-AB-D-ID"
	PAbIDCookie      CookieName = "P-AB-ID"
	PAbID2Cookie     CookieName = "P-AB-ID-2"
	UsernameCookie   CookieName = "Username"
	UserIDCookie     CookieName = "UserID"
	UserAvatarCookie CookieName = "UserAvatar"
	// paseto v4.public token
	AccessCookie CookieName = "Access"

	// User preference and settings cookies.
	ImageProxyCookie             CookieName = "ImageProxy"
	StaticProxyCookie            CookieName = "StaticProxy"
	UgoiraProxyCookie            CookieName = "UgoiraProxy"
	NovelFontTypeCookie          CookieName = "NovelFontType"
	NovelViewModeCookie          CookieName = "NovelViewMode"
	ThumbnailToNewTabCookie      CookieName = "ThumbnailToNewTab"
	SeasonalEffectsEnabledCookie CookieName = "SeasonalEffectsEnabled"
	VisibilityArtR18Cookie       CookieName = "VisibilityArtR18"
	VisibilityArtR18GCookie      CookieName = "VisibilityArtR18G"
	VisibilityArtAICookie        CookieName = "VisibilityArtAI"
	LangCookie                   CookieName = "Lang" // for i18n use
	LogoStyleCookie              CookieName = "LogoStyle"
	BlacklistedArtistsCookie     CookieName = "BlacklistedArtists"
	BlacklistedTagsCookie        CookieName = "BlacklistedTags"
	ThumbnailNavVisibleCookie    CookieName = "ThumbnailNavVisible"
	OpenAllButtonCookie          CookieName = "OpenAllButton"
	SearchDefaultModeCookie      CookieName = "SearchDefaultMode"
	DesktopSidebarHiddenCookie   CookieName = "DesktopSidebarHidden"
	BookmarkDefaultPrivateCookie CookieName = "BookmarkDefaultPrivate"
	FilterProfileCookie          CookieName = "FilterProfile"
)

// AllCookieNames defines all cookies that can be set by the user.
var AllCookieNames = []CookieName{
	TokenCookie,
	CSRFCookie,
	YUIDBCookie,
	PAbDIDCookie,
	PAbIDCookie,
	PAbID2Cookie,
	UsernameCookie,
	UserIDCookie,
	UserAvatarCookie,
	ImageProxyCookie,
	StaticProxyCookie,
	UgoiraProxyCookie,
	NovelFontTypeCookie,
	NovelViewModeCookie,
	ThumbnailToNewTabCookie,
	SeasonalEffectsEnabledCookie,
	VisibilityArtR18Cookie,
	VisibilityArtR18GCookie,
	VisibilityArtAICookie,
	LangCookie,
	LogoStyleCookie,
	BlacklistedArtistsCookie,
	BlacklistedTagsCookie,
	ThumbnailNavVisibleCookie,
	OpenAllButtonCookie,
	SearchDefaultModeCookie,
	DesktopSidebarHiddenCookie,
	BookmarkDefaultPrivateCookie,
	FilterProfileCookie,
}
