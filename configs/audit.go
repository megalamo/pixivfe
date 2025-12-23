// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/core/audit"
)

const (
	responseDirPermissions = 0o700
	logFilePermissions     = 0o666
)

// setupAudit initializes the auditor with the provided configuration.
func (cfg *ServerConfig) setupAudit() {
	if !cfg.Development.InDevelopment {
		switch cfg.Log.Level {
		case "debug":
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		case "info":
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		case "warn":
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
		case "error":
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		default:
		}
	}

	writers := []io.Writer{}

	if len(cfg.Log.Outputs) == 0 {
		writers = append(writers, ConsoleWriter(os.Stderr))
	} else {
		for _, output := range cfg.Log.Outputs {
			var w io.Writer

			switch output {
			case "/dev/stdout":
				w = ConsoleWriter(os.Stdout)
			case "/dev/stderr":
				w = ConsoleWriter(os.Stderr)
			default:
				file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFilePermissions) // #nosec:G302,G304
				if err != nil {
					// If opening the file fails, we simply don't add it to the writers.
					// The loop will continue to the next output.
					fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v\n", output, err)

					continue
				}

				if cfg.Log.Format == "json" {
					w = file
				} else {
					w = ConsoleWriter(file)
				}
			}

			writers = append(writers, w)
		}
	}

	log.Logger = log.Output(zerolog.MultiLevelWriter(writers...))

	audit.SaveResponses = cfg.Development.SaveResponses
	audit.ResponseDirectory = cfg.Development.ResponseSaveLocation

	if audit.SaveResponses {
		if err := os.MkdirAll(audit.ResponseDirectory, responseDirPermissions); err != nil {
			log.Error().
				Err(err).
				Str("path", audit.ResponseDirectory).
				Msg("Failed to create response directory")
			os.Exit(1)
		}
	}
}

// isTerminal returns true if the given file is a terminal.
func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd())
}

// ConsoleWriter returns a writer for zerolog that has NoColor:isTerminal(f).
func ConsoleWriter(f *os.File) io.Writer {
	noColor := !isTerminal(f)

	w := zerolog.ConsoleWriter{Out: f, NoColor: noColor, TimeFormat: time.DateTime}

	if !noColor {
		w.FormatPrepare = func(m map[string]any) error {
			// pretty print request logs
			if sys, ok := m["sys"]; ok && sys == "http" {
				m["message"] = fmt.Sprintf("[%s] %s %-5s %s", m["destination"], m["status_code"], m["method"], m["url"])
				delete(m, "sys")
				delete(m, "method")
				delete(m, "status_code")
				delete(m, "url")
				delete(m, "destination")
				delete(m, "request_id")
			}

			return nil
		}
	}

	return w
}
