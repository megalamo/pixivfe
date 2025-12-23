// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/audit"
)

const (
	envOutputFile  = "deploy/.env.example"
	yamlOutputFile = "deploy/config.yaml.example"
	filePerm       = 0o644

	placeholderToken = "123456_arstdhnei"

	envFileHeader = `# PixivFE configuration (via environment variables)
#
# Copy this file to .env and customize the values below.
#
# Refer to https://pixivfe-docs.pages.dev/hosting/configuration-options/ for more information.
#
# This file was auto-generated using go run ./cmd/genconfig.

`
	yamlFileHeader = `# PixivFE configuration (via configuration file)
#
# Copy this file to config.yaml and customize the values below.
#
# Refer to https://pixivfe-docs.pages.dev/hosting/configuration-options/ for more information.
#
# This file was auto-generated using go run ./cmd/genconfig.
`
	proxySettingsComment = `
## Network proxy settings
## ref: https://pkg.go.dev/net/http#ProxyFromEnvironment
# HTTPS_PROXY=
# HTTP_PROXY=`

	tokenYAMLComment = `  # -- This field uses the value of your pixiv account's PHPSESSID cookie
  # ref: https://pixivfe-docs.pages.dev/hosting/api-authentication/`
)

func main() {
	audit.SetDefaultLogger()
	generateEnvFile()
	generateYAMLFile()
}

// generateEnvFile generates the deploy/.env.example file.
func generateEnvFile() {
	cfg := &config.ServerConfig{}
	cfg.SetDefaults()

	var sb strings.Builder
	sb.WriteString(envFileHeader)

	val := reflect.ValueOf(*cfg)
	typ := val.Type()

	// Iterate over the top-level struct fields.
	for i := range typ.NumField() {
		structField := typ.Field(i)
		structValue := val.Field(i)

		if structValue.Kind() != reflect.Struct || structField.Name == "Build" {
			continue
		}

		fmt.Fprintf(&sb, "## %s\n", structField.Name)

		// Iterate over the fields of the nested struct.
		innerTyp := structValue.Type()
		for j := range innerTyp.NumField() {
			field := innerTyp.Field(j)
			value := structValue.Field(j)

			tag, ok := field.Tag.Lookup("env")
			if !ok {
				continue
			}

			envVarName := strings.Split(tag, ",")[0]

			switch envVarName {
			case "PIXIVFE_TOKEN":
				// Use a commented placeholder for the token.
				fmt.Fprintf(&sb, "# %s=\"%s\"\n", envVarName, placeholderToken)
			case "PIXIVFE_PORT", "PIXIVFE_HOST":
				// Uncomment essential fields.
				fmt.Fprintf(&sb, "%s=\"%v\"\n", envVarName, value.Interface())
			default:
				// For other fields, comment them out. If the value is a slice
				// or an empty string, omit the value to prompt user input.
				if value.Kind() == reflect.Slice || (value.Kind() == reflect.String && value.Len() == 0) {
					fmt.Fprintf(&sb, "# %s=\n", envVarName)
				} else {
					fmt.Fprintf(&sb, "# %s=%v\n", envVarName, value.Interface())
				}
			}
		}

		sb.WriteString("\n")
	}

	sb.WriteString(strings.TrimSpace(proxySettingsComment) + "\n\n")

	if err := os.WriteFile(envOutputFile, []byte(sb.String()), filePerm); err != nil {
		log.Fatal().Err(err).Str("path", envOutputFile).Msg("Failed to write .env.example file")
	}

	log.Info().Str("path", envOutputFile).Msg("Successfully generated .env.example")
}

// generateYAMLFile generates the deploy/config.yaml file.
func generateYAMLFile() {
	cfg := &config.ServerConfig{}
	cfg.SetDefaults()

	cfg.Basic.Token = []string{placeholderToken}

	var yamlContent strings.Builder
	// Marshal the config to YAML.
	encoderOpts := []yaml.EncodeOption{
		config.GetDurationEncoderOption(),
		yaml.Indent(2),
	}
	if err := yaml.NewEncoder(&yamlContent, encoderOpts...).Encode(cfg); err != nil {
		log.Fatal().Err(err).Msg("Failed to marshal config to YAML")
	}

	var sb strings.Builder
	sb.WriteString(yamlFileHeader)

	// Process the marshaled YAML line-by-line to create a clean template.
	for line := range strings.SplitSeq(yamlContent.String(), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Top-level keys (e.g., "basic:") are treated as section headers.
		if !strings.HasPrefix(line, " ") {
			fmt.Fprintf(&sb, "\n%s\n", line)
			continue
		}

		// Keep the token field and its special comments uncommented.
		if strings.HasPrefix(trimmed, "token:") {
			sb.WriteString(tokenYAMLComment + "\n")
			sb.WriteString(line + "\n")

			continue
		}

		if strings.HasPrefix(trimmed, "- "+placeholderToken) {
			sb.WriteString(line + "\n")
			continue
		}

		// By default, comment out the line.
		indentSize := len(line) - len(strings.TrimLeft(line, " "))
		fmt.Fprintf(&sb, "%s# %s\n", strings.Repeat(" ", indentSize), trimmed)
	}

	if err := os.WriteFile(yamlOutputFile, []byte(sb.String()), filePerm); err != nil {
		log.Fatal().Err(err).Str("path", yamlOutputFile).Msg("Failed to write config file")
	}

	log.Info().Str("path", yamlOutputFile).Msg("Successfully generated config.yaml.example")
}
