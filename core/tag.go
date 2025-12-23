package core

import (
	"bytes"
	"encoding/json"

	"codeberg.org/pixivfe/pixivfe/v3/i18n/tags"
)

/*
pixiv returns tags in four distinct formats in the standard desktop API:
	- Array-based with metadata, enclosed inside a tags array (type 1)
	- Map-based with language translations, enclosed inside a tagTranslation object (type 2)
	- Array-based with no metadata, enclosed inside a body array (type 3)
	- Simple string array, enclosed inside a tags array (type 4)

Tags as type 1 can be unmarshalled directly as a []Tag, but code interfacing with endpoints
that return the type 2 format should call TagTranslationsToTag to allow for easier template
reuse in the frontend

# Peak pixiv moment

tagTranslation will be returned as an empty *array* rather than
an empty object when there are no tagTranslations

this is handled by TagTranslationWrapper which uses a custom UnmarshalJSON method

*do not* create a map[string]TagTranslations directly; Go will be unhappy when it
hits this edge case

# Response examples

Type 1 example:
	{
		"tags": [
			{
				"tag": "漫画",
				"locked": true,
				"deletable": false,
				"userId": "67084033",
				"romaji": "mannga",
				"translation": {
					"en": "manga"
				},
				"userName": "postcard"
			}
		]
	}

Type 2 example:
	{
		"tagTranslation": {
			"漫画": {
				"en": "manga",
				"ko": "만화",
				"zh": "",
				"zh_tw": "漫畫",
				"romaji": "mannga"
			}
		}
	}

Type 2 example (no data):
	{
		"tagTranslation": []
	}

Type 3 example:
	{
		"body": [
			{
				"tag": "足裏",
				"tag_translation": "sole"
			}
		]
	}

Type 4 example:
	{
		"tags": ["東方", "おすそわけシリーズ", "チルノ", "カービィ"]
	}
*/

// Tag models tags formatted as Type 1.
type Tag struct {
	Name            string          `json:"tag"`         // Name of the tag
	Locked          bool            `json:"locked"`      // Whether the tag can be edited by other users
	Deletable       bool            `json:"deletable"`   // Whether the tag can be removed
	UserID          string          `json:"userId"`      // userId of the user that added the tag
	Romaji          string          `json:"romaji"`      // Japanese romanization of the tag
	TagTranslations TagTranslations `json:"translation"` //
	UserName        string          `json:"userName"`    // userName of the user that added the tag
}

// Tags models a slice of tags formatted as Type 1.
//
// Prefer using this type over creating a slice of Tag directly.
type Tags []Tag

// TagTranslations models tags formatted as Type 2.
//
// Can be standalone or embedded in Type 1.
type TagTranslations struct {
	En     string `json:"en"`     // English translation
	Ko     string `json:"ko"`     // Korean translation
	Zh     string `json:"zh"`     // Simplified Chinese translation
	ZhTw   string `json:"zh_tw"`  // Traditional Chinese translation
	Romaji string `json:"romaji"` // Japanese romanization, use Tag.Romaji instead of this field
}

// TagTranslationWrapper is a custom type that safely handles tags formatted as Type 2.
//
// It preserves the order of keys as they appear in the JSON.
type TagTranslationWrapper struct {
	data  map[string]TagTranslations
	order []string
}

// ToTags converts TagTranslationWrapper (Type 2) into the standard Tag format (Type 1).
//
// If tagNames is provided (non-nil and non-empty), the returned slice preserves that order and filters duplicates.
// Otherwise, the result is built by iterating over the keys in their original JSON order.
func (t *TagTranslationWrapper) ToTags(tagNames []string) []Tag {
	// When tagNames is not provided, build []Tag from tagTranslations using preserved order.
	if len(tagNames) == 0 {
		result := make([]Tag, 0, len(t.data))

		for _, name := range t.order {
			if translation, ok := t.data[name]; ok {
				result = append(result, Tag{
					Name:            name,
					TagTranslations: translation,
					Romaji:          translation.Romaji,
				})
			}
		}

		return result
	}

	// When tagNames is provided, iterate over it and ignore any duplicates.
	seen := make(map[string]bool, len(tagNames))
	result := make([]Tag, 0, len(tagNames))

	for _, name := range tagNames {
		if seen[name] {
			continue
		}

		seen[name] = true

		tag := Tag{Name: name}
		if translation, ok := t.data[name]; ok {
			tag.TagTranslations = translation
			tag.Romaji = translation.Romaji
		}

		result = append(result, tag)
	}

	return result
}

// UnmarshalJSON implements custom JSON unmarshaling for TagTranslationWrapper.
//
// It preserves the order of keys as they appear in the JSON.
// This is required as the pixiv API returns tagTranslation keys in a meaningful order.
func (t *TagTranslationWrapper) UnmarshalJSON(data []byte) error {
	// Handle empty array case (pixiv returns [] instead of {} when empty)
	if bytes.Equal(data, []byte("[]")) {
		t.data = make(map[string]TagTranslations)
		t.order = nil

		return nil
	}

	// Parse as map to preserve key order
	var asMap map[string]TagTranslations
	if err := json.Unmarshal(data, &asMap); err != nil {
		return err
	}

	// To preserve order, we need to parse the JSON manually to extract key order
	// Use a simple approach: decode into json.RawMessage map first
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// Extract keys in order by parsing the JSON byte sequence
	order := make([]string, 0, len(asMap))
	decoder := json.NewDecoder(bytes.NewReader(data))

	// Read opening brace
	if _, err := decoder.Token(); err != nil {
		return err
	}

	// Read key-value pairs in order
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}

		if key, ok := token.(string); ok {
			order = append(order, key)
			// Skip the value
			var value json.RawMessage
			if err := decoder.Decode(&value); err != nil {
				return err
			}
		}
	}

	t.data = asMap
	t.order = order

	return nil
}

// SimpleTag models tags formatted as Type 3.
type SimpleTag struct {
	Name        string `json:"tag"`
	Translation string `json:"tag_translation"`
}

// SimpleTags models a slice of tags formatted as Type 3.
//
// Prefer using this type over creating a slice of SimpleTag directly.
type SimpleTags []SimpleTag

// ToTags converts SimpleTags (Type 3) into the standard Tag format (Type 1).
func (st SimpleTags) ToTags() []Tag {
	result := make([]Tag, len(st))

	for i, simpleTag := range st {
		result[i] = Tag{
			Name: simpleTag.Name,
			TagTranslations: TagTranslations{
				En: simpleTag.Translation,
			},
		}
	}

	return result
}

// StringTags models tags formatted as Type 4.
type StringTags []string

// ToTags converts StringTags (Type 4) into the standard Tag format (Type 1).
func (st StringTags) ToTags() []Tag {
	result := make([]Tag, len(st))

	for i, name := range st {
		tag := Tag{
			Name: name,
		}
		if translated := tags.TrToEn(name); translated != name {
			tag.TagTranslations.En = translated
		}

		result[i] = tag
	}

	return result
}

// StreetTag models tags formatted as they appear in the street response.
type StreetTag struct {
	Name           string `json:"name"`           // Name of the tag
	TranslatedName any    `json:"translatedName"` // Translation of the tag
	IsEmphasized   bool   `json:"isEmphasized"`   // Whether the tag is emphasized in the UI
}

// StreetTags models a slice of tags formatted as they appear in the street response.
//
// Prefer using this type over creating a slice of StreetTag directly.
type StreetTags []StreetTag

// ToTags converts StreetTags into the standard Tag format (Type 1).
//
// Note: IsEmphasized field is discarded during conversion as the Type 1 format
// does not include emphasis information.
func (st StreetTags) ToTags() []Tag {
	result := make([]Tag, len(st))

	for i, streetTag := range st {
		tag := Tag{
			Name: streetTag.Name,
		}

		// When a tag translation is not available, the TranslatedName field is null, not an empty string.
		if streetTag.TranslatedName != nil {
			if translated, ok := streetTag.TranslatedName.(string); ok && translated != "" {
				tag.TagTranslations.En = translated
			}
		}

		result[i] = tag
	}

	return result
}
