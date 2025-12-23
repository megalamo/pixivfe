// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
This file provides network-based rate limiting for HTTP requests.

Clients are grouped by their IP network, with shared rate limits
applied depending on the network's history.

clientHistory allows rate limits to be dynamically adjusted based
on the suspicious-to-normal client ratio from the network over time.

Note that the Client type handles assessing individual requests,
while limiterWrapper manages the actual rate limiting for IP networks.
*/
package limiter

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"

	"codeberg.org/pixivfe/pixivfe/v3/config"
)

// Note that the 40% suspiciousRatio gap between RestrictThreshold and RelaxThreshold exists
// to protect against flapping in limiterWrapper state.
const (
	RegularRate             = 2.0             // 120 tokens per minute (2 per second) for a normal network.
	RegularBurst            = 120             // Maximum tokens or a normal network.
	SuspiciousRate          = 0.1             // 6 tokens per minute (0.1 per second) for a suspicious network.
	SuspiciousBurst         = 90              // Maximum tokens for a suspicious network.
	AtomXMLRate             = 1.0             // 1 token per second for atom.xml routes.
	AtomXMLBurst            = 90              // Maximum tokens for atom.xml routes.
	LimiterExpiryDuration   = time.Hour       // How long to keep limiters in memory before cleanup.
	CleanupInterval         = 5 * time.Minute // Interval between limiter cleanup runs.
	MaxNetworkClientHistory = 60              // Max. number of client histories to track per network.
	RestrictThreshold       = 0.6             // Ratio of suspicious clients that triggers suspicious rate limits.
	RelaxThreshold          = 0.2             // Ratio of suspicious clients that triggers normal rate limits.
)

var (
	limiters sync.Map   // In-memory storage for rate limiters.
	timeNow  = time.Now // Wrapper for time.Now, which allows us to mock it in tests.
)

// clientHistory represents a circular buffer of client suspicious statuses.
type clientHistory struct {
	statuses   []bool // true = suspicious, false = not suspicious
	index      int    // Current index for insertion
	count      int    // Count of items in the buffer
	suspicious int    // Count of suspicious clients
}

// limiterWrapper holds a rate limiter and additional metadata.
//
// Limiters are associated with an IP network and persist in the limiters sync.Map.
type limiterWrapper struct {
	limiter      *rate.Limiter
	network      string        // Associated network identifier
	lastAccess   time.Time     // Last time limiter was accessed
	mu           sync.Mutex    // mutex for operations on this limiter
	history      clientHistory // History of client suspicious statuses
	isSuspicious bool          // Current limiter suspicious status
}

// serializableLimiter is a representation of a limiterWrapper that can be
// safely serialized to and from JSON. It excludes non-serializable fields
// like mutexes and reconstructs the rate.Limiter from its parameters.
type serializableLimiter struct {
	Network      string        `json:"network"`
	LastAccess   time.Time     `json:"last_access"`
	History      clientHistory `json:"history"`
	IsSuspicious bool          `json:"is_suspicious"`
	Rate         float64       `json:"rate"`
	Burst        int           `json:"burst"`
}

// Save serializes the current state of all limiters to the provided writer.
//
// This function iterates through the in-memory limiters, converts them to a
// serializable format, and writes them as a JSON array.
func Save(w io.Writer) error {
	var stateToSave []*serializableLimiter

	limiters.Range(func(key, value any) bool {
		limWrapper, ok := value.(*limiterWrapper)
		if !ok {
			log.Warn().Any("key", key).
				Msg("Skipping invalid limiter type during state save")

			return true // continue iteration
		}

		limWrapper.mu.Lock()

		sl := &serializableLimiter{
			Network:      limWrapper.network,
			LastAccess:   limWrapper.lastAccess,
			History:      limWrapper.history,
			IsSuspicious: limWrapper.isSuspicious,
			Rate:         float64(limWrapper.limiter.Limit()),
			Burst:        limWrapper.limiter.Burst(),
		}

		stateToSave = append(stateToSave, sl)

		limWrapper.mu.Unlock()

		return true
	})

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ") // Pretty-print for readability

	if err := encoder.Encode(stateToSave); err != nil {
		log.Error().Err(err).Msg("Failed to encode limiter state")

		return err
	}

	log.Info().Int("count", len(stateToSave)).Msg("Successfully saved limiter state")

	return nil
}

// InitFile deserializes limiter state from the provided reader.
//
// This function reads a JSON array of serialized limiters, reconstructs them
// into fully functional limiterWrapper instances, and populates the in-memory
// limiters map. Note that this will overwrite any existing limiters in memory
// with the loaded state.
func InitFile(r io.Reader) error {
	var loadedState []*serializableLimiter

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&loadedState); err != nil {
		// An empty file is not an error, just means we start fresh.
		if err == io.EOF {
			log.Info().Msg("Limiter state file is empty, starting fresh")

			return nil
		}

		log.Error().Err(err).Msg("Failed to decode limiter state")

		return err
	}

	// For a clean load, first clear the existing limiters.
	// This prevents merging old and new state in unexpected ways.
	limiters.Range(func(key, _ any) bool {
		limiters.Delete(key)

		return true
	})

	for _, sl := range loadedState {
		// Reconstruct the limiterWrapper from the serializable struct
		limWrapper := &limiterWrapper{
			limiter:      rate.NewLimiter(rate.Limit(sl.Rate), sl.Burst),
			network:      sl.Network,
			lastAccess:   sl.LastAccess,
			isSuspicious: sl.IsSuspicious,
			history:      sl.History,
			// mu is a zero-value sync.Mutex, which is ready to use
		}
		limiters.Store(sl.Network, limWrapper)
	}

	log.Info().Int("count", len(loadedState)).Msg("Successfully loaded limiter state")

	return nil
}

func Init() {
	log.Info().Msg("Limiter enabled, attempting to load state")

	limiterStateFile := config.Global.Limiter.StateFilepath

	// Attempt to load the limiter state from a file.
	file, err := os.Open(limiterStateFile) // #nosec:G304
	if err != nil {
		if os.IsNotExist(err) {
			// This is expected on first run; we'll start with a fresh state.
			log.Info().Str("file", limiterStateFile).
				Msg("Limiter state file not found, starting with a fresh state")
		} else {
			// For other errors (e.g., permissions), log a warning and continue.
			log.Warn().Err(err).Str("file", limiterStateFile).
				Msg("Could not open limiter state file; starting with a fresh state")
		}

		return
	}
	defer file.Close()

	if err := InitFile(file); err != nil {
		// This can happen if the file is corrupt. limiter.Load handles empty
		// files gracefully, so this catches other decode errors.
		log.Warn().Err(err).Str("file", limiterStateFile).
			Msg("Could not parse limiter state file; starting with a fresh state")
	}
}

func Fini() {
	limiterStateFile := config.Global.Limiter.StateFilepath

	// Save limiter state on graceful shutdown.
	log.Info().Str("file", limiterStateFile).Msg("Saving limiter state...")

	file, err := os.Create(limiterStateFile) // #nosec:G304
	if err != nil {
		// This is not fatal for shutdown, but should be logged.
		log.Warn().Err(err).Str("file", limiterStateFile).
			Msg("Failed to create limiter state file for saving")
	} else {
		defer file.Close()

		if err := Save(file); err != nil {
			log.Warn().Err(err).Str("file", limiterStateFile).
				Msg("Failed to write limiter state")
		}
	}
}

// checkRateLimit attempts to consume 1 token from the limiterWrapper.
//
// Returns an empty string if the request is allowed, or a non-empty string with
// the reason if the request is blocked due to rate limiting.
func checkRateLimit(limiter *limiterWrapper, networkStr string) string {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// Update last access time
	limiter.lastAccess = timeNow()

	// Try to allow 1 request
	if !limiter.limiter.Allow() {
		log.Warn().
			Str("ip", networkStr).
			Msg("Rate limit exceeded")

		return "Rate limit exceeded"
	}

	return ""
}

// getOrCreateLimiter returns a limiterWrapper for the given network.
//
// If a limiter already exists in memory, it is returned as-is (the suspicious parameter is ignored).
// Otherwise, if suspicious is true, a limiter with reduced rate/burst is created; if false,
// a regular rate limiter is created.
func getOrCreateLimiter(networkStr string, suspicious bool) *limiterWrapper {
	// Try to load existing limiter from memory
	if limWrapper, found := loadLimiterFromMemory(networkStr); found {
		return limWrapper
	}

	// Create new limiter with appropriate rate and burst
	var limWrapper *limiterWrapper
	if suspicious {
		limWrapper = newLimiterWrapper(SuspiciousRate, SuspiciousBurst, networkStr, true)
	} else {
		limWrapper = newLimiterWrapper(RegularRate, RegularBurst, networkStr, false)
	}

	// Store the new limiter
	limiters.Store(networkStr, limWrapper)

	return limWrapper
}

// getOrCreateAtomXMLLimiter returns a limiterWrapper specifically configured for atom.xml routes.
//
// This limiter uses unconditional intermediate token bucket config of 1 token/second with max 90.
// It uses a separate key space (networkStr + ":atom") to avoid conflicts with regular limiters.
func getOrCreateAtomXMLLimiter(networkStr string) *limiterWrapper {
	atomKey := networkStr + ":atom"

	// Try to load existing atom.xml limiter from memory
	if limWrapper, found := loadLimiterFromMemory(atomKey); found {
		return limWrapper
	}

	// Create new atom.xml limiter with fixed rate and burst
	limWrapper := newLimiterWrapper(AtomXMLRate, AtomXMLBurst, atomKey, false)

	// Store the new limiter
	limiters.Store(atomKey, limWrapper)

	return limWrapper
}

// loadLimiterFromMemory tries to load from memory a limiterWrapper
// for a given network.
//
// Returns the limiter wrapper if found and true, or nil and false if no data was found.
func loadLimiterFromMemory(network string) (*limiterWrapper, bool) {
	if value, ok := limiters.Load(network); ok {
		limWrapper, ok := value.(*limiterWrapper)
		if !ok {
			return nil, false
		}

		limWrapper.mu.Lock()

		limWrapper.lastAccess = timeNow()
		limWrapper.mu.Unlock()

		return limWrapper, true
	}

	return nil, false
}

// newLimiterWrapper creates a new limiterWrapper with the given parameters.
func newLimiterWrapper(rateLim float64, burstLim int, network string, isSuspicious bool) *limiterWrapper {
	now := timeNow()

	return &limiterWrapper{
		limiter:      rate.NewLimiter(rate.Limit(rateLim), burstLim),
		network:      network,
		lastAccess:   now,
		isSuspicious: isSuspicious,
		history: clientHistory{
			statuses: make([]bool, MaxNetworkClientHistory),
		},
	}
}

// updateNetworkHistory adds a client's suspicious status to the limiterWrapper's history.
//
// The rate and burst of the associated limiter may be updated depending on the history.
func updateNetworkHistory(limiter *limiterWrapper, networkStr string, isSuspicious bool) {
	if limiter == nil {
		return
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// Add this client's status to the history
	addClientToHistory(&limiter.history, isSuspicious)

	// Check if we need to upgrade or downgrade the limiter
	shouldUpgrade, shouldDowngrade := evaluateLimiterChange(limiter.history)

	if shouldUpgrade && limiter.isSuspicious {
		// Upgrade: change from suspicious to regular
		limiter.limiter.SetLimit(rate.Limit(RegularRate))
		limiter.limiter.SetBurst(RegularBurst)

		limiter.isSuspicious = false

		log.Info().
			Str("network", networkStr).
			Msg("Upgraded rate limiter for network")
	} else if shouldDowngrade && !limiter.isSuspicious {
		// Downgrade: change from regular to suspicious
		limiter.limiter.SetLimit(rate.Limit(SuspiciousRate))
		limiter.limiter.SetBurst(SuspiciousBurst)

		limiter.isSuspicious = true

		log.Warn().
			Str("network", networkStr).
			Msg("Downgraded rate limiter for network")
	}
}

// addClientToHistory adds a client's suspicious status to the clientHistory.
func addClientToHistory(history *clientHistory, isSuspicious bool) {
	// Initialize history if needed
	if history.statuses == nil {
		history.statuses = make([]bool, MaxNetworkClientHistory)
	}

	// If we're replacing an existing entry, adjust suspicious count
	if history.count == MaxNetworkClientHistory {
		if history.statuses[history.index] {
			history.suspicious--
		}
	} else {
		history.count++
	}

	// Add new entry
	history.statuses[history.index] = isSuspicious
	if isSuspicious {
		history.suspicious++
	}

	// Move index for next insertion
	history.index = (history.index + 1) % MaxNetworkClientHistory
}

// evaluateLimiterChange determines if a limiter should be upgraded or downgraded
// based on the client history.
func evaluateLimiterChange(history clientHistory) (bool, bool) {
	// Only make decisions when the buffer is full
	if history.count < MaxNetworkClientHistory {
		// Keep the initial limiter configuration until we have enough data
		return false, false
	}

	suspiciousRatio := float64(history.suspicious) / float64(history.count)

	// Determine if we should upgrade or downgrade
	upgrade := suspiciousRatio <= RelaxThreshold
	downgrade := suspiciousRatio >= RestrictThreshold

	return upgrade, downgrade
}

// cleanupExpiredLimiters removes limiters that haven't been accessed for the expiry duration.
func cleanupExpiredLimiters() {
	now := timeNow()

	var (
		expiredCount int
		keysToDelete []any
	)

	// Collect keys to delete in a slice to avoid deleting during Range()
	limiters.Range(func(key, value any) bool {
		limWrapper, ok := value.(*limiterWrapper)
		if !ok {
			log.Warn().Any("key", key).
				Msg("Found invalid limiter type in map")

			keysToDelete = append(keysToDelete, key)

			return true
		}

		limWrapper.mu.Lock()

		lastAccess := limWrapper.lastAccess
		limWrapper.mu.Unlock()

		if now.Sub(lastAccess) > LimiterExpiryDuration {
			keysToDelete = append(keysToDelete, key)
		}

		return true
	})

	// Delete expired or invalid limiters
	for _, key := range keysToDelete {
		limiters.Delete(key)

		expiredCount++
	}

	if expiredCount > 0 {
		log.Info().Int("count", expiredCount).
			Msg("Cleaned up expired limiters")
	}
}
