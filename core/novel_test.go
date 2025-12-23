// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// renderBlocksToHTML is a test helper that simulates the rendering of NovelContentBlocks
// into an HTML string, mimicking the logic from the novel.templ file.
// This allows us to test the final output in a predictable way.
func renderBlocksToHTML(blocks []NovelContentBlock) string {
	var parts []string

	for _, block := range blocks {
		switch b := block.(type) {
		case TextBlock:
			parts = append(parts, fmt.Sprintf(`<p>%s</p>`, b.Content))
		case ImageBlock:
			// NOTE: Simplified rendering for testing, using horizontal layout classes.
			imgTag := fmt.Sprintf(`<img src="%s" alt="%s" class="size-full rounded-lg max-w-xl my-4 mx-auto" />`, b.URL, b.Alt)
			if b.Link != "" {
				imgTag = fmt.Sprintf(`<a href="%s" class="contents">%s</a>`, b.Link, imgTag)
			}

			if b.ErrorMsg != "" {
				imgTag = fmt.Sprintf(`<p class="text-red-400 italic">%s</p>`, b.ErrorMsg)
			}

			parts = append(parts, imgTag)
		case ChapterBlock:
			parts = append(parts, fmt.Sprintf(`<h2 class="text-xl font-bold mt-4">%s</h2>`, b.Title))
		case PageBreakBlock:
			parts = append(parts, fmt.Sprintf(`<hr id="novel_section_%d" class="my-8" />`, b.PageNumber))
		}
	}

	return strings.Join(parts, "\n")
}

func TestParseNovelContent(t *testing.T) {
	t.Parallel()

	// Mock image data for testing image tag parsing
	mockImageData := map[string]*novelImageData{
		"[uploadedimage:123]": {
			URL: "https://example.com/uploaded_123.jpg",
			Alt: "[uploadedimage:123]",
		},
		"[pixivimage:456-1]": {
			URL:  "https://example.com/pixiv_456_p1.jpg",
			Alt:  "[pixivimage:456-1]",
			Link: "/artworks/456",
		},
		// Image with an error message
		"[pixivimage:789]": {
			ErrorMsg: "Cannot insert illust: 789",
		},
		// Data for the real-world test case
		"[pixivimage:123456-1]": {
			URL:      "https://example.com/pixiv_123456_p1.jpg",
			Alt:      "[pixivimage:123456-1]",
			Link:     "/artworks/123456",
			IllustID: "123456-1",
		},
		"[pixivimage:123456-2]": {
			URL:      "https://example.com/pixiv_123456_p2.jpg",
			Alt:      "[pixivimage:123456-2]",
			Link:     "/artworks/123456",
			IllustID: "123456-2",
		},
	}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic cases
		{
			name:     "Empty input",
			input:    "",
			expected: ``,
		},
		{
			name:     "Simple paragraph with single newline",
			input:    "Hello world.\nThis is a new line.",
			expected: `<p>Hello world.<br />This is a new line.</p>`,
		},
		{
			name:     "Multiple paragraphs with different newlines",
			input:    "Paragraph 1.\nLine 1.1.\n\nParagraph 2.\r\n\r\nParagraph 3.",
			expected: "<p>Paragraph 1.<br />Line 1.1.</p>\n<p>Paragraph 2.</p>\n<p>Paragraph 3.</p>",
		},
		{
			name:     "Indentation preservation",
			input:    "Line 1.\n  Indented Line 2.\n    More indentation.",
			expected: `<p>Line 1.<br />  Indented Line 2.<br />    More indentation.</p>`,
		},

		// Markup tag cases
		{
			name:     "Furigana within a paragraph",
			input:    "Text with [[rb: 漢字 > かんじ]].\nAnother line.",
			expected: `<p>Text with <ruby>漢字<rp>(</rp><rt>かんじ</rt><rp>)</rp></ruby>.<br />Another line.</p>`,
		},
		{
			name:     "Chapter tag on its own line creates a standalone h2 element",
			input:    "Some introductory text.\n\n[chapter: Introduction]\n\nThis is the first paragraph after the chapter.",
			expected: "<p>Some introductory text.</p>\n<h2 class=\"text-xl font-bold mt-4\">Introduction</h2>\n<p>This is the first paragraph after the chapter.</p>",
		},
		{
			name:     "Inline chapter tag is treated as plain text",
			input:    "Text before [chapter: Inline Chapter] text after.",
			expected: `<p>Text before [chapter: Inline Chapter] text after.</p>`,
		},
		{
			name:     "Jump URI and Jump Page with correct attributes",
			input:    "Click [[jumpuri: Google > https://google.com]].\nOr [jump: 5].",
			expected: `<p>Click <a href="https://google.com" target="_blank" rel="noopener noreferrer" class="text-blue-400 hover:underline">Google</a>.<br />Or <a href="#novel_section_5" class="text-blue-400 hover:underline">To page 5</a>.</p>`,
		},

		// Newpage cases
		// TODO

		// Image cases
		{
			name:     "Text with uploaded image",
			input:    "Here is an image: [uploadedimage:123].",
			expected: "<p>Here is an image: </p>\n<img src=\"https://example.com/uploaded_123.jpg\" alt=\"[uploadedimage:123]\" class=\"size-full rounded-lg max-w-xl my-4 mx-auto\" />\n<p>.</p>",
		},
		{
			name:     "Text with linked pixiv image",
			input:    "Check out [pixivimage:456-1] this artwork.",
			expected: "<p>Check out </p>\n<a href=\"/artworks/456\" class=\"contents\"><img src=\"https://example.com/pixiv_456_p1.jpg\" alt=\"[pixivimage:456-1]\" class=\"size-full rounded-lg max-w-xl my-4 mx-auto\" /></a>\n<p> this artwork.</p>",
		},
		{
			name:     "Image tag with error message",
			input:    "An error occurred: [pixivimage:789]",
			expected: "<p>An error occurred: </p>\n<p class=\"text-red-400 italic\">Cannot insert illust: 789</p>",
		},

		// Complex cases
		{
			name: "Real-world complex content",
			input: "[pixivimage:123456-1]\r\n\r\n\r\n\r\n" +
				"Zlorgify hexx thronple,\r\n" +
				"Binplox wazzle grumpf.\r\n" +
				"\r\n" +
				"SpeakerA: \"Thronk ploxify hexx. Binplox wazzle grumpf zlorgify.. Hexx thronple binplox wazzle!? Grumpf ploxify thronk hexx..?\"\r\n" +
				"\r\n" +
				"SpeakerB: \"Wazzle grumpf, hexx binplox thronple zlorgify ploxify\"\r\n" +
				"\r\n" +
				"SpeakerC: \"Ploxify hexx, binplox wazzle, grumpf thronple! Zlorgify hexx binplox wazzle\"\r\n" +
				"\r\n" +
				"SpeakerD: \"Binplox wazzle, grumpf hexx thronple zlorgify..\"\r\n" +
				"\r\n" +
				"Hexx binplox wazzle grumpf,\r\n" +
				"Thronple zlorgify ploxify hexx.\r\n" +
				"\r\n" +
				"SpeakerD: \"Grumpf hexx, binplox wazzle, thronple zlorgify ploxify hexx grumpf\"\r\n" +
				"   \"Binplox wazzle grumpf thronple zlorgify ploxify, hexx binplox wazzle\"\r\n" +
				"   \r\n" +
				"   \"Grumpf zlorgify thronple hexx binplox, wazzle grumpf thronple zlorgify ploxify\"\r\n" +
				"\r\n\r\n" +
				"Hexx binplox (wazzle grumpf) thronple zlorgify ploxify hexx binplox,\r\n" +
				"Wazzle grumpf (thronple zlorgify) ploxify hexx, binplox wazzle grumpf thronple zlorgify ploxify.\r\n" +
				"\r\n" +
				"SpeakerB: \"Thronple hexx binplox wazzle, grumpf zlorgify ploxify.. Thronple hexx binplox wazzle\"\r\n" +
				"    \r\n" +
				"\r\n" +
				"SpeakerC: \"Binplox wazzle! Grumpf thronple zlorgify ploxify\"\r\n" +
				"\r\n" +
				"SpeakerB: \"Hexx binplox wazzle! Grumpf, thronple zlorgify ploxify hexx?\"\r\n" +
				"\r\n" +
				"SpeakerD: \"Wazzle grumpf thronple zlorgify ploxify hexx binplox, wazzle grumpf thronple zlorgify\"\r\n" +
				"   \r\n" +
				"   \"Hexx binplox wazzle grumpf thronple, zlorgify ploxify hexx. Binplox wazzle grumpf thronple zlorgify\"\r\n" +
				"   \"Grumpf thronple zlorgify ploxify hexx? ..Binplox wazzle grumpf thronple\"\r\n" +
				"\r\n" +
				"SpeakerE: \"Zlorgify ploxify\"\r\n" +
				"\r\n" +
				"SpeakerF: \"Hexx..\"\r\n" +
				"    \"Binplox wazzle grumpf, thronple zlorgify ploxify\"\r\n" +
				"\r\n" +
				"[newpage]\r\n" +
				"\r\n" +
				"[pixivimage:123456-2]\r\n" +
				"\r\n\r\n" +
				"Grumpf thronple zlorgify ploxify hexx, binplox wazzle grumpf thronple zlorgify.\r\n" +
				"\r\n" +
				"SpeakerG: \"Hexx.. binplox WAZZLE.. Grumpf thronple zlorgify ploxify hexx binplox WAZZLE?\"\r\n" +
				"\r\n" +
				"SpeakerH: \"Grumpf thronple zlorgify ploxify hexx..\"\r\n" +
				"\r\n" +
				"SpeakerI: \"..Binplox wazzle grumpf thronple zlorgify ploxify hexx\"\r\n" +
				"  \"Grumpf thronple zlorgify 'ploxify hexx binplox' 'wazzle grumpf thronple' ..zlorgify\"\r\n" +
				"\r\n" +
				"SpeakerG: \"Hexx binplox wazzle grumpf WAZZLE.. *thronple zlorgify*\"\r\n" +
				"\r\n" +
				"SpeakerI: \"Ploxify, Hexx, binplox wazzle grumpf thronple zlorgify ploxify hexx?\"\r\n" +
				"  \"Binplox wazzle grumpf thronple zlorgify ploxify?\"\r\n" +
				"\r\n" +
				"SpeakerH: \"Hexx, binplox wazzle grumpf thronple zlorgify\"\r\n" +
				"\r\n" +
				"SpeakerI: \"Binplox wazzle Hexx, grumpf Ploxify, thronple zlorgify ploxify hexx binplox?\"\r\n" +
				"\r\n" +
				"SpeakerG: \"GRUMPF!? Thronple zlorgify WAZZLE!?\"\r\n" +
				"\r\n" +
				"SpeakerI: \".....Ploxify hexx binplox? ...Wazzle grumpf? ^^\"\r\n" +
				"\r\n" +
				"SpeakerG: \"..Thronple.. WAZZLE..\"\r\n" +
				"\r\n" +
				"Zlorgify ploxify hexx binplox wazzle grumpf, thronple zlorgify ploxify hexx binplox wazzle grumpf?\r\n" +
				"\r\n\r\n" +
				"Thronple zlorgify ploxify",
			expected: "<a href=\"/artworks/123456\" class=\"contents\"><img src=\"https://example.com/pixiv_123456_p1.jpg\" alt=\"[pixivimage:123456-1]\" class=\"size-full rounded-lg max-w-xl my-4 mx-auto\" /></a>\n" +
				"<p><br /></p>\n" +
				"<p><br /></p>\n" +
				"<p>Zlorgify hexx thronple,<br />Binplox wazzle grumpf.</p>\n" +
				"<p>SpeakerA: \"Thronk ploxify hexx. Binplox wazzle grumpf zlorgify.. Hexx thronple binplox wazzle!? Grumpf ploxify thronk hexx..?\"</p>\n" +
				"<p>SpeakerB: \"Wazzle grumpf, hexx binplox thronple zlorgify ploxify\"</p>\n" +
				"<p>SpeakerC: \"Ploxify hexx, binplox wazzle, grumpf thronple! Zlorgify hexx binplox wazzle\"</p>\n" +
				"<p>SpeakerD: \"Binplox wazzle, grumpf hexx thronple zlorgify..\"</p>\n" +
				"<p>Hexx binplox wazzle grumpf,<br />Thronple zlorgify ploxify hexx.</p>\n" +
				"<p>SpeakerD: \"Grumpf hexx, binplox wazzle, thronple zlorgify ploxify hexx grumpf\"<br />   \"Binplox wazzle grumpf thronple zlorgify ploxify, hexx binplox wazzle\"<br />   <br />   \"Grumpf zlorgify thronple hexx binplox, wazzle grumpf thronple zlorgify ploxify\"</p>\n" +
				"<p><br /></p>\n" +
				"<p>Hexx binplox (wazzle grumpf) thronple zlorgify ploxify hexx binplox,<br />Wazzle grumpf (thronple zlorgify) ploxify hexx, binplox wazzle grumpf thronple zlorgify ploxify.</p>\n" +
				"<p>SpeakerB: \"Thronple hexx binplox wazzle, grumpf zlorgify ploxify.. Thronple hexx binplox wazzle\"<br />    </p>\n" +
				"<p>SpeakerC: \"Binplox wazzle! Grumpf thronple zlorgify ploxify\"</p>\n" +
				"<p>SpeakerB: \"Hexx binplox wazzle! Grumpf, thronple zlorgify ploxify hexx?\"</p>\n" +
				"<p>SpeakerD: \"Wazzle grumpf thronple zlorgify ploxify hexx binplox, wazzle grumpf thronple zlorgify\"<br />   <br />   \"Hexx binplox wazzle grumpf thronple, zlorgify ploxify hexx. Binplox wazzle grumpf thronple zlorgify\"<br />   \"Grumpf thronple zlorgify ploxify hexx? ..Binplox wazzle grumpf thronple\"</p>\n" +
				"<p>SpeakerE: \"Zlorgify ploxify\"</p>\n" +
				"<p>SpeakerF: \"Hexx..\"<br />    \"Binplox wazzle grumpf, thronple zlorgify ploxify\"</p>\n" +
				"<hr id=\"novel_section_2\" class=\"my-8\" />\n" +
				"<p><br /></p>\n" +
				"<a href=\"/artworks/123456\" class=\"contents\"><img src=\"https://example.com/pixiv_123456_p2.jpg\" alt=\"[pixivimage:123456-2]\" class=\"size-full rounded-lg max-w-xl my-4 mx-auto\" /></a>\n" +
				"<p><br /></p>\n" +
				"<p>Grumpf thronple zlorgify ploxify hexx, binplox wazzle grumpf thronple zlorgify.</p>\n" +
				"<p>SpeakerG: \"Hexx.. binplox WAZZLE.. Grumpf thronple zlorgify ploxify hexx binplox WAZZLE?\"</p>\n" +
				"<p>SpeakerH: \"Grumpf thronple zlorgify ploxify hexx..\"</p>\n" +
				"<p>SpeakerI: \"..Binplox wazzle grumpf thronple zlorgify ploxify hexx\"<br />  \"Grumpf thronple zlorgify 'ploxify hexx binplox' 'wazzle grumpf thronple' ..zlorgify\"</p>\n" +
				"<p>SpeakerG: \"Hexx binplox wazzle grumpf WAZZLE.. *thronple zlorgify*\"</p>\n" +
				"<p>SpeakerI: \"Ploxify, Hexx, binplox wazzle grumpf thronple zlorgify ploxify hexx?\"<br />  \"Binplox wazzle grumpf thronple zlorgify ploxify?\"</p>\n" +
				"<p>SpeakerH: \"Hexx, binplox wazzle grumpf thronple zlorgify\"</p>\n" +
				"<p>SpeakerI: \"Binplox wazzle Hexx, grumpf Ploxify, thronple zlorgify ploxify hexx binplox?\"</p>\n" +
				"<p>SpeakerG: \"GRUMPF!? Thronple zlorgify WAZZLE!?\"</p>\n" +
				"<p>SpeakerI: \".....Ploxify hexx binplox? ...Wazzle grumpf? ^^\"</p>\n" +
				"<p>SpeakerG: \"..Thronple.. WAZZLE..\"</p>\n" +
				"<p>Zlorgify ploxify hexx binplox wazzle grumpf, thronple zlorgify ploxify hexx binplox wazzle grumpf?</p>\n" +
				"<p><br /></p>\n" +
				"<p>Thronple zlorgify ploxify</p>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := renderBlocksToHTML(parseNovelContent(tc.input, mockImageData))
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("parseNovelContent() test case failed: %q\nInput:\n`%s`\n\nGot:\n`%s`\n\nWant:\n`%s`", tc.name, tc.input, result, tc.expected)
			}
		})
	}
}
