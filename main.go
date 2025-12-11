// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
PixivFE is an open-source alternative front-end for pixiv.
*/
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/audit"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/i18n"
	"codeberg.org/pixivfe/pixivfe/v3/server/assets"
	"codeberg.org/pixivfe/pixivfe/v3/server/middleware/limiter"
	"codeberg.org/pixivfe/pixivfe/v3/server/router"
	"codeberg.org/pixivfe/pixivfe/v3/server/template"
)

const (
	// Values for http.Server timeouts.
	// ref: gosec: G112
	readHeaderTimeout time.Duration = 15 * time.Second
	readTimeout       time.Duration = 15 * time.Second
	writeTimeout      time.Duration = 10 * time.Second
	idleTimeout       time.Duration = 30 * time.Second

	serverShutdownDeadline time.Duration = 5 * time.Second
)

var (
	errChmodSocket = errors.New("failed to change unix socket permissions")
	errChownSocket = errors.New("failed to change unix socket ownership")
)

// embeddedContent holds our static web server content.
//

//go:embed assets/css assets/fonts assets/icons assets/img assets/js assets/manifest.json assets/robots.txt
//go:embed i18n/tags/data/tag_translations.yaml
//go:embed all:po
var embeddedContent embed.FS

// init assigns the embedded filesystem to the exported assets.FS variable.
//
//nolint:gochecknoinits // this is a good use of init()
func init() {
	assets.FS = embeddedContent
}

// main is the entry point of the application.
func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("Application failed")
	}
}

// run orchestrates the application startup and graceful shutdown.
//
//nolint:funlen
func run() error {
	audit.SetDefaultLogger()

	if err := config.Global.LoadConfig(); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := i18n.Setup(); err != nil {
		return fmt.Errorf("failed to initialize i18n engine: %w", err)
	}

	log.Info().Msg("Initialized i18n engine")

	if err := template.LoadIcons("assets/icons"); err != nil {
		return fmt.Errorf("failed to load icons: %w", err)
	}

	// Initialize API response cache
	requests.Setup()

	router := router.NewRouter()
	router.DefineRoutes()
	router.RegisterMiddleware()

	// Create http.Server instance
	server := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// Channel to listen for server errors
	serverErrors := make(chan error, 1)

	// Start main server in a goroutine
	go func() {
		listener, err := chooseListener()
		if err != nil {
			serverErrors <- fmt.Errorf("failed to create listener: %w", err)

			return
		}

		serverErrors <- server.Serve(listener)
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until a shutdown signal or a server error is received
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
	case s := <-quit:
		log.Info().Str("signal", s.String()).Msg("Shutdown signal received")
		log.Info().Msg("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), serverShutdownDeadline)

		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("server forced to shutdown: %w", err)
		}
	}

	limiter.Fini()

	log.Info().Msg("Server exited gracefully")

	return nil
}

func chooseListener() (net.Listener, error) {
	// Check if we should use a Unix domain socket
	if config.Global.Basic.UnixSocket != "" {
		unixAddr := config.Global.Basic.UnixSocket

		unixListener, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", unixAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to start Unix socket listener on %v: %w", unixAddr, err)
		}

		if err = setupSocket(); err != nil {
			_ = unixListener.Close()

			return nil, err
		}

		// Assign the listener and log where we are listening
		log.Info().
			Str("address", unixAddr).
			Msg("Listening on Unix domain socket")

		return unixListener, nil
	}

	// Otherwise, fall back to TCP listener
	addr := net.JoinHostPort(config.Global.Basic.Host, config.Global.Basic.Port)

	tcpListener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to start TCP listener on %v: %w", addr, err)
	}

	addr = tcpListener.Addr().String()

	// Extract the port for logging
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		_ = tcpListener.Close()

		return nil, fmt.Errorf("failed to parse listener address %q: %w", addr, err)
	}

	// Log the address and convenient URL for local development
	log.Info().
		Str("address", addr).
		Str("port", port).
		Str("url", fmt.Sprintf("http://pixivfe.localhost:%v/", port)).
		Msg("Listening on address")

	return tcpListener, nil
}

func setupSocket() error {
	cfg := config.Global.Basic

	if cfg.UnixSocket == "" {
		return nil
	}

	uid, gid := -1, -1

	var err error

	if cfg.UnixSocketUser != "" {
		uid, err = parseUserOrGroupID(cfg.UnixSocketUser, "user")
		if err != nil {
			return err
		}
	}

	if cfg.UnixSocketGroup != "" {
		gid, err = parseUserOrGroupID(cfg.UnixSocketGroup, "group")
		if err != nil {
			return err
		}
	}

	if uid != -1 || gid != -1 {
		if err := os.Chown(cfg.UnixSocket, uid, gid); err != nil {
			return fmt.Errorf("%w: %w", errChownSocket, err)
		}
	}

	if err := os.Chmod(cfg.UnixSocket, cfg.UnixSocketPermissions); err != nil {
		return fmt.Errorf("%w: %w", errChmodSocket, err)
	}

	return nil
}

// parseUserOrGroupID attempts to parse a user or group identifier.
//
// It first tries to convert the value to an integer. If that fails, it
// performs a system lookup for the given kind ("user" or "group").
func parseUserOrGroupID(value, kind string) (int, error) {
	// Try to parse as a numeric ID first.
	if id, err := strconv.Atoi(value); err == nil {
		return id, nil
	}

	// If parsing fails, assume it's a name and look it up.
	var idStr string

	if kind == "user" {
		u, err := user.Lookup(value)
		if err != nil {
			return -1, fmt.Errorf("failed to lookup user '%s': %w", value, err)
		}

		idStr = u.Uid
	} else { // kind == "group"
		g, err := user.LookupGroup(value)
		if err != nil {
			return -1, fmt.Errorf("failed to lookup group '%s': %w", value, err)
		}

		idStr = g.Gid
	}

	// Parse the ID from the looked-up struct.
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return -1, fmt.Errorf("failed to parse %s ID from looked-up value '%s': %w", kind, value, err)
	}

	return id, nil
}
