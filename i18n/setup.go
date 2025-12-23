// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/leonelquinteros/gotext"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/language"

	"codeberg.org/pixivfe/pixivfe/v3/i18n/tags"
	"codeberg.org/pixivfe/pixivfe/v3/server/assets"
)

var (
	// poDomain is the gettext domain to load under each locale.
	poDomain = "pixivfe"

	// localesByTag maps canonical BCP 47 tags, for example
	// "en", "ja", "pt-BR", to their loaded gotext.Locale.
	localesByTag map[string]*gotext.Locale

	// supportedTags holds the list of BCP 47 tags for which a locale was successfully loaded.
	supportedTags []language.Tag

	// matcher is a private [language.Matcher] derived from the loaded locales.
	matcher language.Matcher
)

// Setup initialises package i18n by loading gettext catalogues from embedded assets
// and constructing a language matcher.
//
// It scans the embedded assets for .po files in the "po" directory and loads the "pixivfe" gettext
// domain. The expected layout is:
//
//	po/<locale>.po
//
// The <locale> filename part may use hyphens or underscores, for example "pt-BR.po" or "pt_BR.po",
// and is normalised to a canonical BCP 47 language tag for matching. The template file, "po/pixivfe.pot",
// is ignored. The base locale, specified by BaseLocale, is always included and acts as the default fallback.
//
// Calling Setup again replaces the previously loaded locales and matcher.
//
// On success Setup returns nil. It returns an error if embedded assets cannot be read
// or if tag translation data cannot be loaded.
func Setup() error {
	Logger = log.With().Str("sys", "i18n").Logger()

	localesByTag = make(map[string]*gotext.Locale)
	supportedTags = nil
	matcher = nil

	entries, err := fs.ReadDir(assets.FS, "po")
	if err != nil {
		return fmt.Errorf("failed to read po directory: %w", err)
	}

	var tagsList []language.Tag

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".po") {
			continue
		}

		fileName := entry.Name()
		if fileName == poDomain+".pot" {
			continue
		}

		localeName := strings.TrimSuffix(fileName, ".po")

		// Accept both underscore and hyphen.
		// Convert to a canonical BCP 47 string for matching and display.
		t, err := language.Parse(strings.ReplaceAll(localeName, "_", "-"))
		if err != nil {
			Logger.Warn().Err(err).Str("file", fileName).Msg("Skipping invalid locale file")
			continue
		}

		canonical := t.String()

		po := gotext.NewPoFS(assets.FS)
		po.ParseFile(path.Join("po", fileName))

		loc := gotext.NewLocale("", canonical) // Base path is unused when manually adding translators.
		loc.AddTranslator(poDomain, po)

		localesByTag[canonical] = loc

		tagsList = append(tagsList, t)

		Logger.Info().
			Str("locale", canonical).
			Str("domain", poDomain).
			Msg("Loaded locale")
	}

	// Build a private matcher from the loaded languages.
	// baseTag is first to make it the default fallback for matching.
	all := make([]language.Tag, 0, len(tagsList)+1)

	all = append(all, baseTag)

	// Sort loaded tags by their canonical string.
	sort.Slice(tagsList, func(i, j int) bool { return tagsList[i].String() < tagsList[j].String() })

	for _, t := range tagsList {
		if t == baseTag {
			continue
		}

		all = append(all, t)
	}

	matcher = language.NewMatcher(all)
	supportedTags = all

	if err := loadTagTranslations(); err != nil {
		return err
	}

	return nil
}

func loadTagTranslations() error {
	file, err := assets.FS.Open("i18n/tags/data/tag_translations.yaml")
	if err != nil {
		return fmt.Errorf("failed to open tag translations file: %w", err)
	}
	defer file.Close()

	var newTagTranslations map[string]string
	if err := yaml.NewDecoder(file).Decode(&newTagTranslations); err != nil {
		return fmt.Errorf("failed to decode tag translations file: %w", err)
	}

	// Install into subpackage.
	tags.SetTranslations(newTagTranslations)

	Logger.Info().Int("count", len(newTagTranslations)).Msg("Loaded tag translations")

	return nil
}
