// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package limiter

import (
	"time"

	"github.com/rs/zerolog/log"
)

var lastCleanupAt time.Time

// TODO: fix this. Remove mutex. use phony.
func DoCleanup() {
	now := time.Now()
	if lastCleanupAt.IsZero() {
		lastCleanupAt = now
		return
	}

	if now.Sub(lastCleanupAt) < CleanupInterval {
		return
	}

	lastCleanupAt = now

	go func() {
		cleanupExpiredLimiters()
		globalTokenStorage.cleanupExpiredLinkTokens()

		dur := time.Since(now)
		log.Info().Time("start", now).Dur("dur", dur).Msg("limiter cleanup")
	}()
}
