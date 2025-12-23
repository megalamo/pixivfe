// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"runtime/debug"
	"strings"
)

// BuildVersion is the latest tagged release of PixivFE.
const BuildVersion string = "v3.0.1"

type buildInfo struct {
	VcsRevision string
	VcsTime     string
	VcsModified bool
}

func (b *buildInfo) Revision() string {
	if b.VcsRevision == "" {
		return "unknown"
	}

	s := strings.Split(b.VcsTime, "T")[0] + "-" + b.VcsRevision[:8]
	if b.VcsModified {
		s += "+dirty"
	}

	return s
}

func (b *buildInfo) load() {
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		b.VcsRevision = getBuildSetting(buildInfo.Settings, "vcs.revision")
		b.VcsTime = getBuildSetting(buildInfo.Settings, "vcs.time")
		b.VcsModified = getBuildSetting(buildInfo.Settings, "vcs.modified") == "true"
	}
}

func getBuildSetting(settings []debug.BuildSetting, key string) string {
	for _, kv := range settings {
		if key == kv.Key {
			return kv.Value
		}
	}

	return ""
}
