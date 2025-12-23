// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	maxEnvironmentKeyValueParts = 2
	minQuotedValueLength        = 2
)

var (
	errExpectedPointerToStruct = errors.New("expected a pointer to a struct")
	errUnsupportedSliceType    = errors.New("unsupported slice type")
	errUnsupportedFieldType    = errors.New("unsupported field type")
)

// readEnv populates the provided ServerConfig struct with values from
// environment variables.
//
// Returns an error if any required environment variables are missing or invalid.
func readEnv(spec any) error {
	structValue := reflect.ValueOf(spec)
	if structValue.Kind() != reflect.Ptr {
		return fmt.Errorf("%w, got %s", errExpectedPointerToStruct, structValue.Kind())
	}

	structValue = structValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return fmt.Errorf("%w, got a pointer to %s", errExpectedPointerToStruct, structValue.Kind())
	}

	structType := structValue.Type()

	for fieldIndex := range structValue.NumField() {
		field := structValue.Field(fieldIndex)
		fieldType := structType.Field(fieldIndex)

		if fieldType.Anonymous {
			if field.Kind() == reflect.Struct {
				if err := readEnv(field.Addr().Interface()); err != nil {
					return err
				}
			}

			continue
		}

		tag := fieldType.Tag.Get("env")
		if tag == "" {
			if field.Kind() == reflect.Struct {
				if err := readEnv(field.Addr().Interface()); err != nil {
					return err
				}
			}

			continue
		}

		parts := strings.Split(tag, ",")
		envVarName := parts[0]
		overwrite := slices.Contains(parts[1:], "overwrite")

		envValue, exists := os.LookupEnv(envVarName)
		if !exists {
			// If the environment variable is not set, skip this field.
			// Default values should be handled by loadFromDefaults.
			continue
		}

		if !field.CanSet() {
			continue
		}

		// If not overwrite and field already has a non-zero/non-empty value, skip.
		if !overwrite && !isZero(field) {
			continue
		}

		if err := setFieldValue(field, fieldType, envVarName, envValue); err != nil {
			return err
		}
	}

	return nil
}

// setFieldValue sets the field value based on its type.
func setFieldValue(
	field reflect.Value,
	fieldType reflect.StructField,
	envVarName, envValue string,
) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(envValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			parsedDuration, err := time.ParseDuration(envValue)
			if err != nil {
				return fmt.Errorf(
					"failed to parse duration for %s from env var %s (%s): %w",
					fieldType.Name, envVarName, envValue, err)
			}

			field.SetInt(int64(parsedDuration))
		} else {
			intValue, err := strconv.ParseInt(envValue, 10, 64)
			if err != nil {
				return fmt.Errorf(
					"failed to parse int for %s from env var %s (%s): %w",
					fieldType.Name, envVarName, envValue, err)
			}

			field.SetInt(intValue)
		}
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(envValue)
		if err != nil {
			return fmt.Errorf(
				"failed to parse bool for %s from env var %s (%s): %w",
				fieldType.Name, envVarName, envValue, err)
		}

		field.SetBool(boolValue)
	case reflect.Slice:
		// All slices used in configuration are of strings
		if field.Type().Elem().Kind() == reflect.String {
			values := strings.Split(envValue, ",")
			trimmedValues := make([]string, 0, len(values))

			for _, value := range values {
				trimmed := strings.TrimSpace(value)
				if trimmed != "" {
					trimmedValues = append(trimmedValues, trimmed)
				}
			}

			field.Set(reflect.ValueOf(trimmedValues))
		} else {
			return fmt.Errorf("%w for field %s", errUnsupportedSliceType, fieldType.Name)
		}
	case reflect.Struct:
		// For nested structs, recursively call loadFromEnv
		if err := readEnv(field.Addr().Interface()); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w for field %s: %s", errUnsupportedFieldType, fieldType.Name, field.Kind())
	}

	return nil
}

// isZero checks if a reflect.Value is its zero value.
func isZero(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Slice:
		return value.Len() == 0
	case reflect.Struct:
		// For structs, simply consider it zero if all its fields are zero.
		for fieldIndex := range value.NumField() {
			if !isZero(value.Field(fieldIndex)) {
				return false
			}
		}

		return true
	}

	return false
}

// useDotEnv loads environment variables from a .env file, checking
// the current working directory, then the directory of the binary.
//
// This function soft fails if the .env file doesn't exist in either location.
func useDotEnv() error {
	// Try to load from current working directory first
	cwd, err := os.Getwd()
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Could not get current working directory")
	} else { // Fallback to trying executable directory if Getwd fails, though unlikely
		envPath := filepath.Join(cwd, ".env")
		if err := tryLoadDotEnv(envPath); err == nil {
			// Successfully loaded from CWD
			return nil
		} else if !os.IsNotExist(err) {
			// Log error if it's not a "file not found" error
			log.Warn().
				Err(err).
				Str("path", envPath).
				Msg("Error trying to load .env file")
		}
	}

	// Fallback: Determine directory of the running binary
	dir := "." // Default to current dir if os.Executable() fails
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Dir(exe)
	}

	envPath := filepath.Join(dir, ".env")

	return tryLoadDotEnv(envPath)
}

// tryLoadDotEnv attempts to load and parse a .env file from the given path.
//
// It returns nil on success or if the file doesn't exist.
//
// It returns an error for other issues (e.g., permission denied, malformed content).
func tryLoadDotEnv(envPath string) error {
	// #nosec G304 - envPath is controlled and comes from known safe sources
	data, err := os.ReadFile(envPath)
	if os.IsNotExist(err) {
		log.Info().
			Str("path", envPath).
			Msg("No .env file found, skipping")

		return nil
	}

	if err != nil {
		log.Warn().
			Err(err).
			Str("path", envPath).
			Msg("Could not read .env file")

		return nil
	}

	// Parse and set variables
	for lineNumber, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", maxEnvironmentKeyValueParts)
		if len(parts) != maxEnvironmentKeyValueParts {
			log.Warn().
				Str("path", envPath).
				Int("line", lineNumber+1).
				Str("content", line).
				Msg("Invalid format in .env file")

			continue
		}

		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		// Strip matching quotes
		if len(value) > minQuotedValueLength && value[0] == value[len(value)-1] && (value[0] == '"' || value[0] == '\'') {
			value = value[1 : len(value)-1]
		}
		// Only set if not already defined
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				log.Warn().
					Err(err).
					Str("key", key).
					Msg("Could not set environment variable")
			}
		}
	}

	log.Info().
		Str("path", envPath).
		Msg("Loaded configuration from .env file")

	return nil
}
