// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package limiter

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/config"
)

// testConfigMutex serializes tests that mutate global package state.
var testConfigMutex sync.Mutex

// mockTimeProvider implements the timeProvider interface
// and maintains a controllable current time for testing.
type mockTimeProvider struct {
	currentTime time.Time
}

// newMockTimeProvider creates a new mockTimeProvider initialized with the given time.
// This allows tests to start from a specific point in time.
func newMockTimeProvider(initialTime time.Time) *mockTimeProvider {
	return &mockTimeProvider{
		currentTime: initialTime,
	}
}

// Now returns the current mock time.
func (m *mockTimeProvider) Now() time.Time {
	return m.currentTime
}

// Sleep advances the mock current time by the specified duration.
func (m *mockTimeProvider) Sleep(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// setupLimiterTest prepares a test environment with a mock time provider
// and properly configured limiter settings for testing.
//
// It returns the mock time provider.
//
// The original time function and config are restored when the test completes.
//
// NOTE: Acquire the limiter test lock/time hook once for the entire test.
// Do not call setupLimiterTest again in subtests; it's guarded by a global mutex
// and re-entering it would deadlock.
func setupLimiterTest(t *testing.T) *mockTimeProvider {
	t.Helper()

	// Acquire the global test lock and configure defaults first so that any
	// mutation of package-level state (including timeNow) happens under the lock.
	setupTestConfig(t)

	mockTime := newMockTimeProvider(time.Now())

	// Hook the mock time provider to the token bucket's timeNow while the lock is held.
	origTimeNow := timeNow // Save original for restoring after tests.

	timeNow = func() time.Time {
		return mockTime.Now()
	}

	// Clear limiters to ensure test isolation.
	limiters = sync.Map{}

	// Register cleanup to restore the original timeNow. Because this cleanup is
	// registered after setupTestConfig's cleanup, it will run first and thus
	// revert timeNow before the test lock is released.
	t.Cleanup(func() {
		timeNow = origTimeNow
		// Clear limiters again on cleanup to prevent test contamination.
		limiters = sync.Map{}
	})

	return mockTime
}

// setupTestConfig configures the global config with test-appropriate values.
// It also acquires a global lock for the duration of the test to serialize
// modifications to package-level state.
func setupTestConfig(t *testing.T) {
	t.Helper()
	testConfigMutex.Lock()

	// Save original config.
	origConfig := config.Global

	// Set up test configuration with basic defaults.
	config.Global.Limiter.Enabled = true
	config.Global.Limiter.IPv4Prefix = 24                 // Default /24 for IPv4.
	config.Global.Limiter.IPv6Prefix = 64                 // Default /64 for IPv6.
	config.Global.Limiter.PassIPs = []string{"127.0.0.1"} // Basic pass IP for other tests.
	config.Global.Limiter.BlockIPs = []string{"10.0.0.1"} // Basic block IP for other tests.
	config.Global.Limiter.FilterLocal = false
	config.Global.Limiter.CheckHeaders = true

	config.PasetoValidator.LoadSecretKeyFromHex(config.Global.Basic.PasetoSecret)

	// Restore original config on cleanup and release the global test lock.
	t.Cleanup(func() {
		config.Global = origConfig

		testConfigMutex.Unlock()
	})
}

// setTestPassIPs is a helper to set pass IPs.
// It should be called only while the test lock acquired by setupTestConfig/setupLimiterTest is held.
func setTestPassIPs(ips []string) {
	config.Global.Limiter.PassIPs = ips
}

// setTestBlockIPs is a helper to set block IPs.
// It should be called only while the test lock acquired by setupTestConfig/setupLimiterTest is held.
func setTestBlockIPs(ips []string) {
	config.Global.Limiter.BlockIPs = ips
}

// createTestPingCookie creates a valid ping cookie.
func createTestPingCookie(r *http.Request) *http.Cookie {
	return createAccessTokenCookie(r)
}
