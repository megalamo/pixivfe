// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
The link token is embedded in HTML pages as /limiter/{token}, which is fetched by real browsers
as a simple proof that they are not naive bots (i.e., bots that can pass our basic HTTP request
header checks, but cannot fetch CSS resources linked to by the HTML and attach the generated
ping cookie to their future requests).

A link token is:
  - client-specific (tied to a fingerprint);
  - single-use;
  - and short-lived.

After a link token is used to generate a ping cookie, it is immediately invalidated to prevent replay attacks.

This approach obviously won't prevent any bots that are remotely sophisticated that can properly mimic browser
behavior (think properly tuned playwright or selenium), but should stop some dude flooding an instance using
python-requests; if this actually becomes relevant in our threat model, we could use Cloudflare Turnstile for
more robust challenges, but this of course disadvantages users without JavaScript enabled.
*/
package limiter

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// linkTokenLiveSeconds is the TTL for the link token.
	//
	// Short-lived as a link token should be consumed soon after being generated.
	linkTokenLiveSeconds = 60

	// linkTokenTTL is the computed duration for the link token's lifetime.
	linkTokenTTL = time.Second * time.Duration(linkTokenLiveSeconds)

	tokenBytesLength int = 24
)

// globalTokenStorage is a global instance of linkTokenStorage.
var globalTokenStorage = &TokenStorage{
	tokens: make(map[string]tokenEntry),
}

var errInvalidOrExpiredToken = errors.New("invalid or expired token")

// tokenEntry represents metadata for a single token.
type tokenEntry struct {
	// expiresAt is when this token expires.
	expiresAt time.Time

	// clientFingerprint is the associated client fingerprint.
	clientFingerprint string
}

// TokenStorage holds tokens in memory, each associated with a client fingerprint.
type TokenStorage struct {
	// tokens is a map of token to token entries.
	tokens map[string]tokenEntry
	mu     sync.Mutex
}

// createLinkToken creates a new link token for the provided client fingerprint.
func (s *TokenStorage) createLinkToken(fingerprint string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenBytes := make([]byte, tokenBytesLength)

	_, err := rand.Read(tokenBytes)
	if err != nil {
		log.Err(err).
			Msg("Failed to generate a secure link token")

		tokenBytes = fmt.Appendf(tokenBytes, "%d-%s", timeNow().UnixNano(), fingerprint)
	}

	// Encode as URL-safe base64
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store the new token
	s.tokens[token] = tokenEntry{
		expiresAt:         timeNow().Add(linkTokenTTL),
		clientFingerprint: fingerprint,
	}

	return token
}

// consumeLinkToken validates and removes a token if it matches the provided fingerprint.
func (s *TokenStorage) consumeLinkToken(token, fingerprint string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.tokens[token]
	if !exists || timeNow().After(entry.expiresAt) {
		delete(s.tokens, token)

		return false
	}

	// Check if the fingerprint matches
	if entry.clientFingerprint != fingerprint {
		log.Warn().
			Str("link-token", token).
			Str("expected", entry.clientFingerprint).
			Str("received", fingerprint).
			Msg("Fingerprint mismatch for link token")

		return false
	}

	// Token is valid and matches fingerprint - consume it by removing
	delete(s.tokens, token)

	return true
}

// cleanupExpiredLinkTokens removes all expired link tokens from storage.
func (s *TokenStorage) cleanupExpiredLinkTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := timeNow()
	expiredCount := 0

	for token, entry := range s.tokens {
		if now.After(entry.expiresAt) {
			delete(s.tokens, token)

			expiredCount++
		}
	}

	if expiredCount > 0 {
		log.Info().
			Int("count", expiredCount).
			Int("remaining", len(s.tokens)).
			Msg("Cleaned up expired tokens")
	}
}

// GetOrCreateLinkToken creates a new client-specific link token.
//
// A fresh token is always generated for each request, scoped to the client's fingerprint.
func GetOrCreateLinkToken(r *http.Request) (string, error) {
	client, err := newClientInfo(r)
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	return globalTokenStorage.createLinkToken(client.fingerprint), nil
}
