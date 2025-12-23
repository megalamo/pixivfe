// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package idgen

import (
	"strings"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	now := time.Now()

	if strings.ReplaceAll(now.Format("15:04:05"), ":", "") != maketime(now) {
		t.Error("time part incorrect")
	}
}
