// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package idgen

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// Make makes a short ID with a 6 byte timestamp and 3 bytes of entropy.
func Make() string {
	entropy := [3]byte{'a', 'a', 'a'} // debug

	_, _ = rand.Read(entropy[:])

	return maketime(time.Now()) + base64.RawURLEncoding.EncodeToString(entropy[:])
}

func maketime(t time.Time) string {
	return t.Format("150405")
}
