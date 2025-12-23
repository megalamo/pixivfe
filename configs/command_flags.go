// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import "flag"

// parseCommandLineArgs defines and parses flags, returning the value of the "config" flag.
func parseCommandLineArgs() string {
	var configFilePath string

	if flag.Lookup("config") == nil {
		flag.StringVar(&configFilePath, "config", "./config.yaml", "Path to a PixivFE configuration file in YAML format.")
	}

	if !flag.Parsed() {
		flag.Parse()
	}

	return configFilePath
}
