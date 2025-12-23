// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/rs/zerolog/log"
)

func (cfg *ServerConfig) readYAML(configFilePath string) error {
	if configFilePath == "" {
		return nil
	}

	_, err := os.Stat(configFilePath)
	if os.IsNotExist(err) {
		log.Info().
			Str("path", configFilePath).
			Msg("No YAML configuration file found, skipping")

		return nil
	}

	yamlCfg, err := os.ReadFile(configFilePath) // #nosec G304 -- Only loading a config file
	if err != nil {
		return fmt.Errorf("failed to read configuration file %s: %w", configFilePath, err)
	}

	if err := yaml.Unmarshal(yamlCfg, cfg); err != nil {
		return fmt.Errorf("failed to parse YAML from %s: %w", configFilePath, err)
	}

	log.Info().
		Str("path", configFilePath).
		Msg("Successfully loaded configuration")

	return nil
}
