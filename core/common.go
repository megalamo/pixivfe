// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/i18n"
)

// comments and rankings use UTC+9.
const PixivTimeOffset = time.Hour * 9

// PixivTimeLayout is the date format used by *some* pixiv API endpoints.
const PixivTimeLayout = "2006-01-02 15:04:05"

// KnownAITags is a list of tags that are associated with AI-generated content.
//
// Useful for heuristically identifying AI-generated content that may have an incorrect ai_type.
var KnownAITags = []string{
	"ai-assisted",
	"ai_generated",
	"aiartwork",
	"aigenerated",
	"aigirl",
	"aiイラスト",
	"ai作品",
	"ai生成",
	"ai生成イラスト",
	"ai生成作品",
	"ai画像",
	"ai绘画",
	"novelai",
	"novelaidiffusion",
	"stablediffusion",
}

// knownAITagsSet is a set for fast lookups of known AI tags.
var knownAITagsSet = buildAITagsSet()

// IsKnownAITag checks if a tag is a known AI tag. The check is case-insensitive.
func IsKnownAITag(tag string) bool {
	_, ok := knownAITagsSet[strings.ToLower(tag)]

	return ok
}

// buildAITagsSet creates a set from the KnownAITags slice.
func buildAITagsSet() map[string]struct{} {
	set := make(map[string]struct{}, len(KnownAITags))
	for _, tag := range KnownAITags {
		// All tags in KnownAITags are already lowercase.
		set[tag] = struct{}{}
	}

	return set
}

// Errors.
var (
	ErrInvalidAIType     = errors.New("invalid AIType value")
	ErrInvalidIllustType = errors.New("invalid IllustType value")
)

// pixiv returns 0, 1, 2 to filter SFW and/or NSFW artworks.
// Those values are saved in `XRestrict`.
//
// Note the hyphen in the canonical string representation;
// Go does not allow hyphens in identifiers.
type XRestrict int

const (
	Safe XRestrict = 0
	R18  XRestrict = 1
	R18G XRestrict = 2
	All  XRestrict = 3 // All is a custom value to represent all ratings.

)

// IsNSFWRating returns true if the rating is R18 or R18G.
func (x XRestrict) IsNSFWRating() bool {
	switch x {
	case R18, R18G:
		return true
	default:
		return false
	}
}

// Note the hyphen in the canonical string representation;
// Go does not allow hyphens in identifiers.
func (x XRestrict) Tr(ctx context.Context) string {
	switch x {
	case Safe:
		return i18n.Tr(ctx, "Safe")
	case R18:
		return i18n.Tr(ctx, "R-18")
	case R18G:
		return i18n.Tr(ctx, "R-18G")
	case All:
		return i18n.Tr(ctx, "All")
	}

	return ""
}

// UnhyphenatedString returns the unhyphenated string representation of the rating.
func (x XRestrict) UnhyphenatedString() string {
	switch x {
	case Safe:
		return "Safe"
	case R18:
		return "R18"
	case R18G:
		return "R18G"
	case All:
		return "All"
	}

	return ""
}

// ParseXRestrict converts a string into its corresponding XRestrict value.
//
// No error is returned for invalid XRestrict values
// because handling errors in templates is a meme.
func ParseXRestrict(s string) XRestrict {
	switch strings.ToLower(s) {
	case "safe":
		return Safe
	case "r-18", "r18":
		return R18
	case "r-18g", "r18g":
		return R18G
	case "all":
		return All
	}

	return -1
}

// pixiv returns 0, 1, 2 to filter SFW and/or NSFW artworks..
// Those values are saved in `aiType`.
type AIType int

const (
	Unrated        AIType = 0
	NotAIGenerated AIType = 1
	AIGenerated    AIType = 2
)

func (x AIType) Tr(ctx context.Context) (string, error) {
	switch x {
	case Unrated:
		return i18n.Tr(ctx, "Unrated"), nil
	case NotAIGenerated:
		return i18n.Tr(ctx, "Not AI-generated"), nil
	case AIGenerated:
		return i18n.Tr(ctx, "AI-generated"), nil
	}

	return "", fmt.Errorf("%w: %d", ErrInvalidAIType, int(x))
}

// pixiv returns 0, 1, 2 to indicate the type of illustration.
// Those values are saved in `illustType`.
type IllustType int

const (
	Illustration IllustType = 0
	Manga        IllustType = 1
	Ugoira       IllustType = 2
	Novels       IllustType = 3 // Novels is a custom value to represent novels.

)

func (i IllustType) Tr(ctx context.Context) (string, error) {
	switch i {
	case Illustration:
		return i18n.Tr(ctx, "Illustration"), nil
	case Manga:
		return i18n.Tr(ctx, "Manga"), nil
	case Ugoira:
		return i18n.Tr(ctx, "Ugoira"), nil
	case Novels:
		return i18n.Tr(ctx, "Novels"), nil
	}

	return "", fmt.Errorf("%w: %d", ErrInvalidIllustType, int(i))
}

// ParseIllustType converts a string into its corresponding IllustType value.
//
// Normalizes the string to be case-insensitive before parsing. No error is
// returned for invalid IllustType values because handling errors in templates
// is a meme.
func ParseIllustType(s string) IllustType {
	switch strings.ToLower(s) {
	case "illustration", "illustrations":
		return Illustration
	case "manga":
		return Manga
	case "ugoira":
		return Ugoira
	case "novel", "novels":
		return Novels
	}

	return -1
}

// SanityLevel represents pixiv's content rating system for artworks.
// It is more reliable and granular for authorization control than XRestrict.
//
// SanityLevel values:
//
//	0: Unreviewed - Typically seen on newly uploaded works
//	2: Safe       - Reviewed and unrestricted content
//	4: R-15       - Reviewed, mild age restriction
//	6: R-18/R-18G - Reviewed, strict age restriction
//	                (Maps to XRestrict values 1 and 2 respectively)
//
// Notes:
//   - Content with SanityLevel > 4 requires user authorization, but
//     appear to be intermittently enforced by the API.
//   - Novel routes lack SanityLevel data.
type SanityLevel int

const (
	SLUnreviewed SanityLevel = 0
	SLSafe       SanityLevel = 2
	SLR15        SanityLevel = 4
	SLR18        SanityLevel = 6
)

// regionList is a list of all ISO 3166-1 alpha-2 codes except South Sudan
// (as it is not included in pixiv's list).
//
// 248 regions in total.
//
// Magic pattern:
//
//	id="(\w+)".*\[\[ISO.*\]\] \|\| \[\[(\w+ ?\w+ ?\w+ ?\w+ ?\w+ ?)
var regionList = [][]string{
	{"AF", "Afghanistan"},
	{"AL", "Albania"},
	{"DZ", "Algeria"},
	{"AS", "American Samoa"},
	{"AD", "Andorra"},
	{"AO", "Angola"},
	{"AI", "Anguilla"},
	{"AQ", "Antarctica"},
	{"AG", "Antigua and Barbuda"},
	{"AR", "Argentina"},
	{"AM", "Armenia"},
	{"AW", "Aruba"},
	{"AU", "Australia"},
	{"AT", "Austria"},
	{"AZ", "Azerbaijan"},
	{"BH", "Bahrain"},
	{"GG", "Bailiwick of Guernsey"},
	{"BD", "Bangladesh"},
	{"BB", "Barbados"},
	{"BY", "Belarus"},
	{"BE", "Belgium"},
	{"BZ", "Belize"},
	{"BJ", "Benin"},
	{"BM", "Bermuda"},
	{"BT", "Bhutan"},
	{"BO", "Bolivia"},
	{"BA", "Bosnia and Herzegovina"},
	{"BW", "Botswana"},
	{"BV", "Bouvet Island"},
	{"BR", "Brazil"},
	{"IO", "British Indian Ocean Territory"},
	{"VG", "British Virgin Islands"},
	{"BN", "Brunei"},
	{"BG", "Bulgaria"},
	{"BF", "Burkina Faso"},
	{"BI", "Burundi"},
	{"KH", "Cambodia"},
	{"CM", "Cameroon"},
	{"CA", "Canada"},
	{"CV", "Cape Verde"},
	{"BQ", "Caribbean Netherlands"},
	{"KY", "Cayman Islands"},
	{"CF", "Central African Republic"},
	{"CL", "Chile"},
	{"CN", "China"},
	{"CX", "Christmas Island"},
	{"CC", "Cocos "},
	{"MF", "Collectivity of Saint Martin"},
	{"CO", "Colombia"},
	{"KM", "Comoros"},
	{"CK", "Cook Islands"},
	{"CR", "Costa Rica"},
	{"HR", "Croatia"},
	{"CY", "Cyprus"},
	{"CZ", "Czech Republic"},
	{"CD", "Democratic Republic of the Congo"},
	{"DK", "Denmark"},
	{"DJ", "Djibouti"},
	{"DM", "Dominica"},
	{"DO", "Dominican Republic"},
	{"TL", "East Timor"},
	{"EC", "Ecuador"},
	{"EG", "Egypt"},
	{"SV", "El Salvador"},
	{"GQ", "Equatorial Guinea"},
	{"ER", "Eritrea"},
	{"EE", "Estonia"},
	{"SZ", "Eswatini"},
	{"ET", "Ethiopia"},
	{"FK", "Falkland Islands"},
	{"FO", "Faroe Islands"},
	{"FM", "Federated States of Micronesia"},
	{"FI", "Finland"},
	{"FR", "France"},
	{"GF", "French Guiana"},
	{"PF", "French Polynesia"},
	{"TF", "French Southern and Antarctic Lands"},
	{"GA", "Gabon"},
	{"GE", "Georgia "},
	{"DE", "Germany"},
	{"GH", "Ghana"},
	{"GI", "Gibraltar"},
	{"GR", "Greece"},
	{"GL", "Greenland"},
	{"GD", "Grenada"},
	{"GP", "Guadeloupe"},
	{"GT", "Guatemala"},
	{"GN", "Guinea"},
	{"GW", "Guinea"},
	{"GY", "Guyana"},
	{"HT", "Haiti"},
	{"HM", "Heard Island and McDonald Islands"},
	{"HN", "Honduras"},
	{"HK", "Hong Kong"},
	{"HU", "Hungary"},
	{"IS", "Iceland"},
	{"IN", "India"},
	{"ID", "Indonesia"},
	{"IM", "Isle of Man"},
	{"IL", "Israel"},
	{"IT", "Italy"},
	{"JM", "Jamaica"},
	{"JP", "Japan"},
	{"JE", "Jersey"},
	{"JO", "Jordan"},
	{"KZ", "Kazakhstan"},
	{"KE", "Kenya"},
	{"NL", "Kingdom of the Netherlands"},
	{"KI", "Kiribati"},
	{"KW", "Kuwait"},
	{"KG", "Kyrgyzstan"},
	{"LV", "Latvia"},
	{"LB", "Lebanon"},
	{"LS", "Lesotho"},
	{"LR", "Liberia"},
	{"LY", "Libya"},
	{"LI", "Liechtenstein"},
	{"LT", "Lithuania"},
	{"LU", "Luxembourg"},
	{"MO", "Macau"},
	{"MG", "Madagascar"},
	{"MW", "Malawi"},
	{"MY", "Malaysia"},
	{"MV", "Maldives"},
	{"MT", "Malta"},
	{"MH", "Marshall Islands"},
	{"MQ", "Martinique"},
	{"MR", "Mauritania"},
	{"MU", "Mauritius"},
	{"YT", "Mayotte"},
	{"MX", "Mexico"},
	{"MD", "Moldova"},
	{"MC", "Monaco"},
	{"MN", "Mongolia"},
	{"ME", "Montenegro"},
	{"MS", "Montserrat"},
	{"MA", "Morocco"},
	{"MZ", "Mozambique"},
	{"MM", "Myanmar"},
	{"NA", "Namibia"},
	{"NR", "Nauru"},
	{"NP", "Nepal"},
	{"NC", "New Caledonia"},
	{"NZ", "New Zealand"},
	{"NI", "Nicaragua"},
	{"NE", "Niger"},
	{"NG", "Nigeria"},
	{"NF", "Norfolk Island"},
	{"MP", "Northern Mariana Islands"},
	{"MK", "North Macedonia"},
	{"NO", "Norway"},
	{"PK", "Pakistan"},
	{"PW", "Palau"},
	{"PA", "Panama"},
	{"PG", "Papua New Guinea"},
	{"PY", "Paraguay"},
	{"PH", "Philippines"},
	{"PN", "Pitcairn Islands"},
	{"PL", "Poland"},
	{"PT", "Portugal"},
	{"PR", "Puerto Rico"},
	{"QA", "Qatar"},
	{"IE", "Republic of Ireland"},
	{"CG", "Republic of the Congo"},
	{"RO", "Romania"},
	{"RU", "Russia"},
	{"RW", "Rwanda"},
	{"BL", "Saint Barth"},
	{"SH", "Saint Helena"},
	{"KN", "Saint Kitts and Nevis"},
	{"LC", "Saint Lucia"},
	{"PM", "Saint Pierre and Miquelon"},
	{"VC", "Saint Vincent and the Grenadines"},
	{"WS", "Samoa"},
	{"SM", "San Marino"},
	{"SA", "Saudi Arabia"},
	{"SN", "Senegal"},
	{"RS", "Serbia"},
	{"SC", "Seychelles"},
	{"SL", "Sierra Leone"},
	{"SG", "Singapore"},
	{"SX", "Sint Maarten"},
	{"SK", "Slovakia"},
	{"SI", "Slovenia"},
	{"SB", "Solomon Islands"},
	{"SO", "Somalia"},
	{"ZA", "South Africa"},
	{"GS", "South Georgia and the South "},
	{"KR", "South Korea"},
	// {"SS", "South Sudan"},
	{"ES", "Spain"},
	{"LK", "Sri Lanka"},
	{"SD", "Sudan"},
	{"SR", "Suriname"},
	{"SJ", "Svalbard and Jan Mayen"},
	{"SE", "Sweden"},
	{"CH", "Switzerland"},
	{"SY", "Syria"},
	{"TW", "Taiwan"},
	{"TJ", "Tajikistan"},
	{"TZ", "Tanzania"},
	{"TH", "Thailand"},
	{"BS", "The Bahamas"},
	{"GM", "The Gambia"},
	{"TK", "Tokelau"},
	{"TO", "Tonga"},
	{"TT", "Trinidad and Tobago"},
	{"TN", "Tunisia"},
	{"TR", "Turkey"},
	{"TM", "Turkmenistan"},
	{"TC", "Turks and Caicos Islands"},
	{"TV", "Tuvalu"},
	{"UG", "Uganda"},
	{"UA", "Ukraine"},
	{"AE", "United Arab Emirates"},
	{"GB", "United Kingdom"},
	{"US", "United States"},
	{"UM", "United States Minor Outlying Islands"},
	{"VI", "United States Virgin Islands"},
	{"UY", "Uruguay"},
	{"UZ", "Uzbekistan"},
	{"VU", "Vanuatu"},
	{"VA", "Vatican City"},
	{"VE", "Venezuela"},
	{"VN", "Vietnam"},
	{"WF", "Wallis and Futuna"},
	{"EH", "Western Sahara"},
	{"YE", "Yemen"},
	{"ZM", "Zambia"},
	{"ZW", "Zimbabwe"},
}

// RegionList returns the region list.
func RegionList() [][]string {
	return regionList
}

// associateContentWithUsers associates artworks and novels with their respective users.
func associateContentWithUsers(users []*User, artworks []ArtworkItem, novels []*NovelBrief) {
	userMap := make(map[string]*User, len(users))

	for i := range users {
		user := users[i]

		userMap[user.ID] = user
	}

	// Associate artworks with users
	for _, artwork := range artworks {
		if user, exists := userMap[artwork.UserID]; exists {
			user.Artworks = append(user.Artworks, artwork)
		}
	}

	// Associate novels with users
	for _, novel := range novels {
		if user, exists := userMap[novel.UserID]; exists {
			user.Novels = append(user.Novels, novel)
		}
	}
}
