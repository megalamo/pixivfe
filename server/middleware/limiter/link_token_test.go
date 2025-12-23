// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

//nolint:paralleltest // These tests modify shared global state and must be run serially.
package limiter

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// resetTokenStorage is a helper to reset the global token storage between tests.
func resetTokenStorage() {
	globalTokenStorage = &TokenStorage{
		tokens: make(map[string]tokenEntry),
	}
}

func TestGetOrCreateLinkToken(t *testing.T) {
	resetTokenStorage()

	t.Run("Generate new token", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Real-IP", "1.1.1.1")
		r.Header.Set("User-Agent", "test-agent")

		token, err := GetOrCreateLinkToken(r)
		if err != nil {
			t.Fatalf("Error generating token: %v", err)
		}

		if token == "" {
			t.Error("Expected non-empty token")
		}

		// Ensure token is in storage by trying to consume it
		client, err := newClientInfo(r)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		valid := globalTokenStorage.consumeLinkToken(token, client.fingerprint)
		if !valid {
			t.Error("Token should be valid and consumable")
		}
	})

	t.Run("Always generate fresh tokens", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Real-IP", "1.1.1.1")
		r.Header.Set("User-Agent", "test-agent")

		token1, err := GetOrCreateLinkToken(r)
		if err != nil {
			t.Fatalf("Error generating first token: %v", err)
		}

		token2, err := GetOrCreateLinkToken(r)
		if err != nil {
			t.Fatalf("Error generating second token: %v", err)
		}

		if token1 == token2 {
			t.Error("Expected different tokens for each call, got the same value")
		}
	})
}

func TestLinkTokenStorage(t *testing.T) {
	mockTime := setupLimiterTest(t)

	resetTokenStorage()

	t.Run("Token storage and consumption", func(t *testing.T) {
		fingerprint := "test-fingerprint"

		// Create token
		token := globalTokenStorage.createLinkToken(fingerprint)
		if token == "" {
			t.Error("Expected non-empty token")
		}

		// Consume token
		valid := globalTokenStorage.consumeLinkToken(token, fingerprint)
		if !valid {
			t.Error("Expected token to be valid")
		}

		// Try to consume again - should fail
		valid = globalTokenStorage.consumeLinkToken(token, fingerprint)
		if valid {
			t.Error("Token should be consumed and no longer valid")
		}
	})

	t.Run("Token expiry", func(t *testing.T) {
		resetTokenStorage()

		fingerprint := "expire-test"
		token := globalTokenStorage.createLinkToken(fingerprint)

		// Advance time beyond expiry
		mockTime.Sleep(time.Duration(linkTokenLiveSeconds+1) * time.Second)

		// Should not be consumable
		valid := globalTokenStorage.consumeLinkToken(token, fingerprint)
		if valid {
			t.Error("Expired token should not be valid")
		}
	})

	t.Run("Concurrent token operations", func(t *testing.T) {
		resetTokenStorage()

		// Test concurrent creation and consumption
		const numGoroutines = 10

		done := make(chan bool, numGoroutines)

		for i := range numGoroutines {
			go func(id int) {
				defer func() { done <- true }()

				fingerprint := fmt.Sprintf("concurrent-test-%d", id)
				token := globalTokenStorage.createLinkToken(fingerprint)

				// Immediately consume
				valid := globalTokenStorage.consumeLinkToken(token, fingerprint)
				if !valid {
					t.Errorf("Token should be valid for goroutine %d", id)
				}
			}(i)
		}

		// Wait for all goroutines
		for range numGoroutines {
			<-done
		}
	})
}

func TestLinkTokenCleanup(t *testing.T) {
	mockTime := setupLimiterTest(t)

	resetTokenStorage()

	t.Run("Cleanup expired tokens", func(t *testing.T) {
		// Create some tokens
		fingerprint1 := "cleanup-test-1"
		fingerprint2 := "cleanup-test-2"

		token1 := globalTokenStorage.createLinkToken(fingerprint1)
		token2 := globalTokenStorage.createLinkToken(fingerprint2)

		// Advance time to expire tokens
		mockTime.Sleep(time.Duration(linkTokenLiveSeconds+1) * time.Second)

		// Run cleanup
		globalTokenStorage.cleanupExpiredLinkTokens()

		// Both tokens should now be invalid
		valid1 := globalTokenStorage.consumeLinkToken(token1, fingerprint1)
		valid2 := globalTokenStorage.consumeLinkToken(token2, fingerprint2)

		if valid1 || valid2 {
			t.Error("Expired tokens should be cleaned up and invalid")
		}
	})
}

func TestLinkTokenEdgeCases(t *testing.T) {
	setupLimiterTest(t)
	resetTokenStorage()

	t.Run("Empty fingerprint", func(t *testing.T) {
		token := globalTokenStorage.createLinkToken("")

		// Should still work with empty fingerprint
		valid := globalTokenStorage.consumeLinkToken(token, "")
		if !valid {
			t.Error("Token with empty fingerprint should still be valid")
		}
	})

	t.Run("Wrong fingerprint", func(t *testing.T) {
		token := globalTokenStorage.createLinkToken("correct-fingerprint")

		// Try to consume with wrong fingerprint
		valid := globalTokenStorage.consumeLinkToken(token, "wrong-fingerprint")
		if valid {
			t.Error("Token should not be valid with wrong fingerprint")
		}

		// Should still be consumable with correct fingerprint
		valid = globalTokenStorage.consumeLinkToken(token, "correct-fingerprint")
		if !valid {
			t.Error("Token should still be valid with correct fingerprint")
		}
	})

	t.Run("Non-existent token", func(t *testing.T) {
		valid := globalTokenStorage.consumeLinkToken("non-existent-token", "any-fingerprint")
		if valid {
			t.Error("Non-existent token should not be valid")
		}
	})
}
