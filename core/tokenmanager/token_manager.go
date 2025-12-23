// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
Package tokenmanager provides functionality for managing and rotating API tokens.
*/
package tokenmanager

import (
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	yuidbChars  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	yuidbLength = 7

	// abIDUpperBound is the upper bound for generating p_ab_id and p_ab_id_2 cookie values,
	// producing a single-digit integer [0-9].
	abIDUpperBound = 10
)

// Possible tokenStatus values.
const (
	Good     tokenStatus = iota // Token is in a good state and can be used
	TimedOut                    // Token is currently timed out and should not be used
)

// tokenStatus represents the current state of a token.
type tokenStatus int

// Token represents an individual API token with its associated metadata.
type Token struct {
	Value string // The actual token value

	YUIDB string // A "yuid_b" cookie value

	// ab cookies
	PAbDID string // A "p_ab_d_id" cookie value
	PAbID  string // A "p_ab_id" cookie value
	PAbID2 string // A "p_ab_id_2" cookie value

	status              tokenStatus   // Current status of the token
	timeoutUntil        time.Time     // Time until which the token is timed out
	failureCount        int           // Number of consecutive failures
	lastUsed            time.Time     // Last time the token was used
	baseTimeoutDuration time.Duration // Base duration for timeout calculations
}

// TokenManager handles a collection of tokens and provides methods for token selection and management.
type TokenManager struct {
	tokens              []*Token      // Slice of available tokens
	maxRetries          int           // Maximum nber of retries before considering a request failed
	baseTimeout         time.Duration // Base timeout duration for requests
	maxBackoffTime      time.Duration // Maximum allowed backoff time
	loadBalancingMethod string        // Method used for load balancing (e.g., "round-robin", "random")
	currentIndex        int           // Current index for round-robin selection
	mu                  sync.Mutex
}

// NewTokenManager creates and initializes a new TokenManager with the given parameters.
func NewTokenManager(
	tokenValues []string,
	maxRetries int,
	baseTimeout, maxBackoffTime time.Duration,
	loadBalancingMethod string,
) *TokenManager {
	tokens := make([]*Token, len(tokenValues))
	// #nosec:G404 - ab cookie generation doesn't need to be cryptographically secure.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i, value := range tokenValues {
		yuidb, pAbDID, pAbID, pAbID2 := GenerateABCookies(r)

		tokens[i] = &Token{
			Value:               value,
			YUIDB:               yuidb,
			PAbDID:              pAbDID,
			PAbID:               pAbID,
			PAbID2:              pAbID2,
			status:              Good,
			baseTimeoutDuration: baseTimeout,
		}
	}

	return &TokenManager{
		tokens:              tokens,
		maxRetries:          maxRetries,
		baseTimeout:         baseTimeout,
		maxBackoffTime:      maxBackoffTime,
		loadBalancingMethod: loadBalancingMethod,
		currentIndex:        0,
	}
}

// CreateRandomToken generates an arbitrary Token with a random 33-character
// lowercase string value, and associated ab cookie values.
//
// ref: https://codeberg.org/kita/px-api-docs/src/commit/92a71331bb/README.md#authorization
func CreateRandomToken() *Token {
	const (
		letters = "abcdefghijklmnopqrstuvwxyz"
		length  = 33
	)

	// #nosec:G404 - ab cookie generation doesn't need to be cryptographically secure.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	builder := strings.Builder{}
	builder.Grow(length)

	for range length {
		builder.WriteByte(letters[r.Intn(len(letters))])
	}

	yuidb, pAbDID, pAbID, pAbID2 := GenerateABCookies(r)

	return &Token{
		Value:  builder.String(),
		YUIDB:  yuidb,
		PAbDID: pAbDID,
		PAbID:  pAbID,
		PAbID2: pAbID2,
		status: Good,
		// baseTimeoutDuration is not needed for a one-off random token.
	}
}

// GetToken selects and returns a token.
func (tm *TokenManager) GetToken() *Token {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	healthyTokens := tm.getHealthyTokens()

	if len(healthyTokens) == 0 {
		return tm.getFallbackToken(now)
	}

	var selectedToken *Token

	switch tm.loadBalancingMethod {
	case "round-robin":
		selectedToken = tm.roundRobinSelection(healthyTokens)
	case "random":
		selectedToken = tm.randomSelection(healthyTokens)
	case "least-recently-used":
		selectedToken = tm.leastRecentlyUsedSelection(healthyTokens)
	default:
		selectedToken = tm.roundRobinSelection(healthyTokens)
	}

	selectedToken.lastUsed = now

	return selectedToken
}

// GetYUIDB selects and returns a YUIDB value.
func (tm *TokenManager) GetYUIDB() string {
	return tm.GetToken().YUIDB
}

// GetPAbDID selects and returns a PAbDID value.
func (tm *TokenManager) GetPAbDID() string {
	return tm.GetToken().PAbDID
}

// GetPAbID selects and returns a PAbID value.
func (tm *TokenManager) GetPAbID() string {
	return tm.GetToken().PAbID
}

// GetPAbID2 selects and returns a PAbID2 value.
func (tm *TokenManager) GetPAbID2() string {
	return tm.GetToken().PAbID2
}

// MarkTokenStatus updates the status of a token and handles timeout logic.
func (tm *TokenManager) MarkTokenStatus(token *Token, status tokenStatus) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	token.status = status
	if status == TimedOut {
		token.failureCount++
		// Calculate timeout duration using exponential backoff with a maximum limit
		const exponentialBase = 2

		timeoutDuration := time.Duration(math.Min(
			float64(tm.baseTimeout)*math.Pow(exponentialBase, float64(token.failureCount-1)),
			float64(tm.maxBackoffTime),
		))

		token.timeoutUntil = time.Now().Add(timeoutDuration)
	} else {
		// Reset failure count when marked as Good
		token.failureCount = 0
	}
}

// ResetAllTokens resets all tokens to their initial good state.
func (tm *TokenManager) ResetAllTokens() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, token := range tm.tokens {
		token.status = Good
		token.failureCount = 0
	}
}

// getHealthyTokens returns a slice of tokens that are currently in a good state.
func (tm *TokenManager) getHealthyTokens() []*Token {
	healthyTokens := make([]*Token, 0)

	for _, token := range tm.tokens {
		if token.status == Good {
			healthyTokens = append(healthyTokens, token)
		}
	}

	return healthyTokens
}

// getFallbackToken attempts to find a timed-out token that can be reset and used.
func (tm *TokenManager) getFallbackToken(now time.Time) *Token {
	var bestToken *Token
	for _, token := range tm.tokens {
		if token.status == TimedOut && (bestToken == nil || token.timeoutUntil.Before(bestToken.timeoutUntil)) {
			bestToken = token
		}
	}

	if bestToken != nil && now.After(bestToken.timeoutUntil) {
		bestToken.status = Good
		bestToken.lastUsed = now

		return bestToken
	}

	return bestToken
}

// roundRobinSelection implements round-robin token selection strategy.
func (tm *TokenManager) roundRobinSelection(healthyTokens []*Token) *Token {
	if tm.currentIndex >= len(healthyTokens) {
		tm.currentIndex = 0
	}

	selectedToken := healthyTokens[tm.currentIndex]
	tm.currentIndex++

	return selectedToken
}

// randomSelection implements random token selection.
//
// #nosec:G404 - token selection doesn't need to be cryptographically secure.
func (tm *TokenManager) randomSelection(healthyTokens []*Token) *Token {
	return healthyTokens[rand.Intn(len(healthyTokens))]
}

// leastRecentlyUsedSelection implements least recently used token selection.
func (tm *TokenManager) leastRecentlyUsedSelection(healthyTokens []*Token) *Token {
	sort.Slice(healthyTokens, func(i, j int) bool {
		return healthyTokens[i].lastUsed.Before(healthyTokens[j].lastUsed)
	})

	return healthyTokens[0]
}

// GenerateABCookies generates the yuid_b and three ab cookie values using the provided random source.
func GenerateABCookies(r *rand.Rand) (string, string, string, string) {
	yuidbBuilder := strings.Builder{}

	yuidbBuilder.Grow(yuidbLength)

	for range yuidbLength {
		yuidbBuilder.WriteByte(yuidbChars[r.Intn(len(yuidbChars))])
	}

	return yuidbBuilder.String(),
		strconv.FormatInt(int64(r.Int31()), 10), // A 32-bit integer.
		strconv.Itoa(r.Intn(abIDUpperBound)),
		strconv.Itoa(r.Intn(abIDUpperBound))
}
