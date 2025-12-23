// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package pixivision

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// TestParseFreeformArticleData tests the parseFreeformArticleData function with testFreeformArticle data.
func TestParseFreeformArticleData(t *testing.T) {
	t.Parallel()

	// Parse the test HTML data
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(testFreeformArticle))
	if err != nil {
		t.Fatalf("Failed to parse test HTML: %v", err)
	}

	// Create a simple HTTP request for testing
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create test request: %v", err)
	}

	// Test the function
	testID := "test-article-123"

	result, err := parseFreeformArticleData(doc, req, testID)
	if err != nil {
		t.Fatalf("parseFreeformArticleData returned error: %v", err)
	}

	// Basic validation - check that we got expected data
	if result.ID != testID {
		t.Errorf("Expected ID %q, got %q", testID, result.ID)
	}

	// Test Header fields
	expectedTitle := "Generic Article Title"
	if result.Header.Title != expectedTitle {
		t.Errorf("Expected title %q, got %q", expectedTitle, result.Header.Title)
	}

	expectedCategoryText := "Sample Category"
	if result.Header.Category.Text != expectedCategoryText {
		t.Errorf("Expected category text %q, got %q", expectedCategoryText, result.Header.Category.Text)
	}

	if result.Header.Category.URL == "" {
		t.Error("Expected non-empty category URL")
	}

	expectedDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !result.Header.Date.Equal(expectedDate) {
		t.Errorf("Expected date %v, got %v", expectedDate, result.Header.Date)
	}

	// Test Body items
	if len(result.Body) == 0 {
		t.Error("Expected at least one body item")
	}

	// Verify we have different types of body items
	var (
		hasBodyImage       bool
		hasBodyCredit      bool
		hasBodyHeading     bool
		hasBodyRichText    bool
		hasBodyArticleCard bool
		hasBodyCaption     bool
		hasAuthorProfile   bool
	)

	for _, item := range result.Body {
		switch item.(type) {
		case BodyImage:
			hasBodyImage = true
		case BodyCredit:
			hasBodyCredit = true
		case BodyHeading:
			hasBodyHeading = true
		case BodyRichText:
			hasBodyRichText = true
		case BodyArticleCard:
			hasBodyArticleCard = true
		case BodyCaption:
			hasBodyCaption = true
		case AuthorProfile:
			hasAuthorProfile = true
		}
	}

	if !hasBodyImage {
		t.Error("Expected at least one BodyImage in body items")
	}

	if !hasBodyCredit {
		t.Error("Expected at least one BodyCredit in body items")
	}

	if !hasBodyHeading {
		t.Error("Expected at least one BodyHeading in body items")
	}

	if !hasBodyRichText {
		t.Error("Expected at least one BodyRichText in body items")
	}

	if !hasBodyArticleCard {
		t.Error("Expected at least one BodyArticleCard in body items")
	}

	if !hasBodyCaption {
		t.Error("Expected at least one BodyCaption in body items")
	}

	if !hasAuthorProfile {
		t.Error("Expected at least one AuthorProfile in body items")
	}

	// Test Tags
	if len(result.Tags) == 0 {
		t.Error("Expected at least one tag")
	}

	// Look for expected tags from test data
	expectedTags := []string{"Author name", "Series name"}

	foundTags := make(map[string]bool)
	for _, tag := range result.Tags {
		foundTags[tag.Name] = true
	}

	for _, expectedTag := range expectedTags {
		if !foundTags[expectedTag] {
			t.Errorf("Expected to find tag %q", expectedTag)
		}
	}

	// Test RelatedArticleGroup fields
	if result.NewestTaggedArticles.HeadingLink == "" {
		t.Error("Expected non-empty NewestTaggedArticles.HeadingLink")
	}

	if len(result.NewestTaggedArticles.Articles) == 0 {
		t.Error("Expected at least one article in NewestTaggedArticles")
	}

	if result.PopularTaggedArticles.HeadingLink == "" {
		t.Error("Expected non-empty PopularTaggedArticles.HeadingLink")
	}

	if len(result.PopularTaggedArticles.Articles) == 0 {
		t.Error("Expected at least one article in PopularTaggedArticles")
	}

	if result.NewestCategoryArticles.HeadingLink == "" {
		t.Error("Expected non-empty NewestCategoryArticles.HeadingLink")
	}

	if len(result.NewestCategoryArticles.Articles) == 0 {
		t.Error("Expected at least one article in NewestCategoryArticles")
	}
}
