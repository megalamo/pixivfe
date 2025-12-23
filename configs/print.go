// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/rs/zerolog/log"
)

const redactedValue = "[redacted]"

func (cfg *ServerConfig) print() {
	log.Info().
		Str("version", BuildVersion).
		Str("revision", cfg.Build.Revision()).
		Str("cacheid", cfg.Instance.FileServerCacheID).
		Msg("Starting PixivFE")

	// Redact sensitive fields using a shallow copy of the config.
	printableConfig := *cfg

	if len(printableConfig.Basic.Token) > 0 {
		printableConfig.Basic.Token = []string{redactedValue}
	}

	printableConfig.Basic.PasetoSecret = redactedValue
	printableConfig.Limiter.TurnstileSecretKey = redactedValue

	// Marshal the processed config to indented YAML.
	configYAML, err := yaml.MarshalWithOptions(
		printableConfig,
		GetDurationEncoderOption(),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal config to YAML for printing")

		return
	}

	log.Info().
		Msg("Application configuration:")
	fmt.Fprintln(os.Stderr, string(configYAML))
}
