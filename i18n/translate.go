// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"bytes"
	"context"
	"log"
	"strings"
	"sync"
	"text/template"

	"github.com/leonelquinteros/gotext"
	"golang.org/x/text/language"
)

// templateCache caches compiled templates per unique template text.
var templateCache sync.Map // key: text, value: *template.Template

type Vars map[string]any

// NewUserError creates a new UserFacingError.
func NewUserError(ctx context.Context, msgid string, kv ...any) *UserError {
	return &UserError{
		msgid: Tr(ctx, msgid, kv...),
		kv:    kv,
	}
}

// UserError is an error type whose message is a translated string.
// It is intended for errors that can be shown directly to the end user.
type UserError struct {
	msgid string
	kv    []any
}

// Error returns the translated error message.
func (e *UserError) Error() string {
	return e.msgid
}

// Tr returns the translated string for a source message id (msgid), which should
// be the original English UI text. If key-value pairs are provided, the translation
// is formatted using text/template-style named placeholders.
//
// If a translation is not found, Tr returns the msgid unchanged, or visibly wrapped
// if strict mode is enabled.
func Tr(ctx context.Context, msgid string, kv ...any) string {
	return translate(ctx, "", msgid, "", 0, false, v(kv...))
}

// TrC translates a source message id (msgid) with an explicit disambiguating
// context, similar to gettext's pgettext. If key-value pairs are provided,
// the translation is formatted using named placeholders.
func TrC(ctx context.Context, contextKey, msgid string, kv ...any) string {
	return translate(ctx, contextKey, msgid, "", 0, false, v(kv...))
}

// TrN translates a singular or plural message depending on n. If a translation
// is missing, we choose singular when n == 1, otherwise plural. If key-value pairs
// are provided, the translation is formatted using named placeholders.
func TrN(ctx context.Context, singular, plural string, n int, kv ...any) string {
	return translate(ctx, "", singular, plural, n, true, v(kv...))
}

// TrNC is the contextual variant of TrN, similar to gettext's npgettext.
// It disambiguates plural forms under a context key and formats the result
// using any provided key-value pairs.
func TrNC(ctx context.Context, contextKey, singular, plural string, n int, kv ...any) string {
	return translate(ctx, contextKey, singular, plural, n, true, v(kv...))
}

// translate performs the underlying lookup and formatting.
func translate(
	ctx context.Context,
	contextKey, singular, plural string,
	n int,
	pluralMode bool,
	vars Vars,
) string {
	loc, matched := resolveLocale(TagFrom(ctx))

	// Fallback message
	base := singular
	if pluralMode && n != 1 {
		base = plural
	}

	finalText := base
	found := false

	if loc != nil {
		switch {
		case pluralMode && contextKey != "":
			found = loc.IsTranslatedNDC(poDomain, singular, n, contextKey)
			if found {
				finalText = loc.GetNDC(poDomain, singular, plural, n, contextKey)
			}
		case pluralMode:
			found = loc.IsTranslatedND(poDomain, singular, n)
			if found {
				finalText = loc.GetND(poDomain, singular, plural, n)
			}
		case contextKey != "":
			found = loc.IsTranslatedDC(poDomain, singular, contextKey)
			if found {
				finalText = loc.GetDC(poDomain, singular, contextKey)
			}
		default:
			found = loc.IsTranslatedD(poDomain, singular)
			if found {
				finalText = loc.GetD(poDomain, singular)
			}
		}
	}

	if !found && strictMissingKeys() {
		logMissingOnce(strippedTagString(matched), buildLogKey(contextKey, singular))

		finalText = "⟦" + base + "⟧"
	}

	return render(matched, finalText, vars)
}

// render formats s as a text/template using the provided data.
func render(locale language.Tag, s string, data Vars) string {
	// If no data is provided, skip template execution unless the string
	// contains template markers, in which case `missingkey=error` surface the issue.
	if len(data) == 0 && !strings.Contains(s, "{{") {
		return s
	}

	if !strings.Contains(s, "{{") {
		return s
	}

	key := s

	var tmpl *template.Template
	if t, ok := templateCache.Load(key); ok {
		tmpl = t.(*template.Template)
	} else {
		var err error

		tmpl, err = template.New("msg").Option("missingkey=error").Parse(s)
		if err != nil {
			if strictMissingKeys() {
				return "⟦" + s + "⟧"
			}

			log.Printf("i18n: template parse error for locale %s: %v, text: %q", locale, err, s)

			return s
		}

		templateCache.Store(key, tmpl)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]any(data)); err != nil {
		if strictMissingKeys() {
			return "⟦" + s + "⟧"
		}

		log.Printf("i18n: template execute error for locale %s: %v, text: %q", locale, err, s)

		return s
	}

	return buf.String()
}

// resolveLocale matches t to one of the loaded locales and returns the
// corresponding gotext.Locale and the matched tag.
// If no matcher or no locale is found, it returns nil and baseTag.
func resolveLocale(t language.Tag) (*gotext.Locale, language.Tag) {
	if matcher == nil {
		return nil, baseTag
	}

	matched, _ := language.MatchStrings(matcher, t.String())

	return localesByTag[matched.String()], matched
}

// v builds Vars from alternating key, value pairs.
// Panics on programmer error.
func v(kv ...any) Vars {
	if len(kv)%2 != 0 {
		panic("i18n.V: odd number of arguments, want key, value pairs")
	}

	m := make(Vars, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			panic("i18n.V: key must be string")
		}

		m[k] = kv[i+1]
	}

	return m
}
