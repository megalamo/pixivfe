// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
Package i18n provides internationalisation utilities backed by GNU gettext
.po catalogues. It translates source message IDs (msgids) across locales
and supports both context and plural forms.

# Quick start

Use the original English UI text as the msgid; do not invent keys.

Translate strings with calls such as:

	i18n.Tr(ctx, "Are you sure you want to quit?")
	i18n.TrC(ctx, "menu", "Open") // disambiguation via context
	i18n.TrN(ctx, "{{.Count}} file", "{{.Count}} files", n, "Count", n)
	i18n.TrNC(ctx, "menu", "{{.Count}} item", "{{.Count}} items", n, "Count", n)

Translations can be used directly in templ templates:

	{ i18n.Tr(ctx, "Settings") }

To insert raw HTML, use:

	@templ.Raw(i18n.Tr(ctx, ...))
	@i18n.MsgKey("Settings")

# Missing translations

By default, missing translations return the msgid unchanged. When
StrictMissingKeys is enabled, missing lookups are logged once
per locale+key and the returned text is visibly wrapped as "⟦...⟧".

# Formatting

Translations can include placeholders that are processed by Go's standard
text/template package. Provide substitutions as alternating key-value pairs
to any of the Tr functions:

	i18n.Tr(ctx, "Welcome, {{.Name}}!", "Name", user.Name)

Numbers are not localised automatically; convert values to strings
yourself if you need locale-specific presentation.

# Content tag translations

Translations for user-generated content tags (e.g. artwork tags) live in
subpackage i18n/tags.

# Further reading

Contributor and translator guidance are available in docs/dev/i18n.md.
*/
package i18n
