package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"regexp"
	"strconv"

	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/core/authenticated"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// validation errors.
var (
	errUnixSocketWithHostPort        = errors.New("unix socket configured - cannot specify Host and Port simultaneously")
	errUnixSocketInvalidPermissions  = errors.New("invalid Basic.UnixSocketPermissions value")
	errUnixSocketUserDoesNotExist    = errors.New("user does not exist")
	errUnixSocketGroupDoesNotExist   = errors.New("group does not exist")
	errNoTokenSupplied               = errors.New("no token supplied. Please supply at least one token")
	errInvalidTokenLoadBalancing     = errors.New("invalid TokenLoadBalancing value")
	errEmptyStateFilepath            = errors.New("filepath for StateFilepath cannot be empty when limiter is enabled")
	errInvalidLimiterDetectionMethod = errors.New("invalid Limiter.DetectionMethod")
	errPasetoSecretRequired          = errors.New("basic.secret is required")
	errPasetoSecretInvalid           = errors.New("basic.secret is not a valid paseto key")
	errTurnstileSitekeyRequired      = errors.New("Limiter.TurnstileSiteKey is required")
	errTurnstileSecretKeyRequired    = errors.New("Limiter.TurnstileSecretKey is required")
	errInvalidIPv4Prefix             = errors.New("IPv4 prefix must be between 0 and 32")
	errInvalidIPv6Prefix             = errors.New("IPv6 prefix must be between 0 and 128")
)

var (
	fileModeOctalRegexp  = regexp.MustCompile(`^0?[0-7]{3}$`)
	fileModeStringRegexp = regexp.MustCompile(`^(?:[r-][w-][x-]){3}$`)
	digitsRegexp         = regexp.MustCompile(`^[0-9]+$`)
)

// validateAndSet validates the server configuration and populates some fields.
func (cfg *ServerConfig) validateAndSet() error {
	// Handle listener configuration
	if cfg.Basic.UnixSocket != "" {
		if cfg.Basic.Host != "" || cfg.Basic.Port != "" {
			return errUnixSocketWithHostPort
		}

		// Handle unix socket permissions
		switch {
		case cfg.Basic.RawUnixSocketPermissions == "":
			cfg.Basic.UnixSocketPermissions = 0o666
		case fileModeOctalRegexp.MatchString(cfg.Basic.RawUnixSocketPermissions):
			rawModeUint64, _ := strconv.ParseUint(cfg.Basic.RawUnixSocketPermissions, 8, 32)

			cfg.Basic.UnixSocketPermissions = os.FileMode(rawModeUint64)
		case fileModeStringRegexp.MatchString(cfg.Basic.RawUnixSocketPermissions):
			mode := os.FileMode(0)

			for i, c := range cfg.Basic.RawUnixSocketPermissions {
				// If permission bit is set
				if c != '-' {
					// Set i-th bit from the end
					const bitsInByte = 8

					mode |= 1 << (bitsInByte - i)
				}
			}

			cfg.Basic.UnixSocketPermissions = mode
		default:
			return errUnixSocketInvalidPermissions
		}

		// Check if user is valid
		if cfg.Basic.UnixSocketUser != "" {
			if digitsRegexp.MatchString(cfg.Basic.UnixSocketUser) {
				if _, err := user.LookupId(cfg.Basic.UnixSocketUser); err != nil {
					return errUnixSocketUserDoesNotExist
				}
			} else {
				if _, err := user.Lookup(cfg.Basic.UnixSocketUser); err != nil {
					return errUnixSocketUserDoesNotExist
				}
			}
		}

		// Check if group is valid
		if cfg.Basic.UnixSocketGroup != "" {
			if digitsRegexp.MatchString(cfg.Basic.UnixSocketGroup) {
				if _, err := user.LookupGroupId(cfg.Basic.UnixSocketGroup); err != nil {
					return errUnixSocketGroupDoesNotExist
				}
			} else {
				if _, err := user.LookupGroup(cfg.Basic.UnixSocketGroup); err != nil {
					return errUnixSocketGroupDoesNotExist
				}
			}
		}
	} else {
		// Set TCP defaults
		if cfg.Basic.Host == "" {
			cfg.Basic.Host = "localhost"
			log.Info().
				Str("host", cfg.Basic.Host).
				Msg("Binding to default host")
		}

		if cfg.Basic.Port == "" {
			cfg.Basic.Port = "8282"
			log.Info().
				Str("port", cfg.Basic.Port).
				Msg("Using default port")
		}
	}

	// Check tokens
	if len(cfg.Basic.Token) == 0 {
		return errNoTokenSupplied
	}

	// Validate image proxy
	if err := validateProxy(&cfg.ContentProxies.RawImage, BuiltInImageProxyPath, "image"); err != nil {
		return err
	}

	if cfg.ContentProxies.RawImage == BuiltInImageProxyPath {
		cfg.ContentProxies.Image = url.URL{Path: BuiltInImageProxyPath}
	} else {
		parsedURL, _ := url.Parse(cfg.ContentProxies.RawImage)

		cfg.ContentProxies.Image = *parsedURL
	}

	// Validate static proxy
	if err := validateProxy(&cfg.ContentProxies.RawStatic, BuiltInStaticProxyPath, "static"); err != nil {
		return err
	}

	if cfg.ContentProxies.RawStatic == BuiltInStaticProxyPath {
		cfg.ContentProxies.Static = url.URL{Path: BuiltInStaticProxyPath}
	} else {
		parsedURL, _ := url.Parse(cfg.ContentProxies.RawStatic)

		cfg.ContentProxies.Static = *parsedURL
	}

	// Validate ugoira proxy
	if err := validateProxy(&cfg.ContentProxies.RawUgoira, BuiltInUgoiraProxyPath, "ugoira"); err != nil {
		return err
	}

	if cfg.ContentProxies.RawUgoira == BuiltInUgoiraProxyPath {
		cfg.ContentProxies.Ugoira = url.URL{Path: BuiltInUgoiraProxyPath}
	} else {
		parsedURL, _ := url.Parse(cfg.ContentProxies.RawUgoira)

		cfg.ContentProxies.Ugoira = *parsedURL
	}

	// Validate RepoURL
	repoURL, err := utils.ParseURL(cfg.Instance.RepoURL, "Repo")
	if err != nil {
		return fmt.Errorf("invalid repo URL: %w", err)
	}

	cfg.Instance.RepoURL = repoURL.String()

	// Validate TokenLoadBalancing
	switch cfg.TokenManager.LoadBalancing {
	case "round-robin", "random", "least-recently-used":
		// valid
	default:
		return errInvalidTokenLoadBalancing
	}

	// Skip validating Limiter configuration if it's not enabled
	if !cfg.Limiter.Enabled {
		return nil
	}

	// Check if the user explicitly set an empty filepath
	if cfg.Limiter.StateFilepath == "" {
		return errEmptyStateFilepath
	}

	// Validate DetectionMethod
	switch cfg.Limiter.DetectionMethod {
	case None, LinkToken, Turnstile:
		// valid
	default:
		return errInvalidLimiterDetectionMethod
	}

	// paseto secret key is required if a detection method that uses it is enabled
	if cfg.Limiter.DetectionMethod == LinkToken || cfg.Limiter.DetectionMethod == Turnstile {
		if cfg.Basic.PasetoSecret == "" {
			return errPasetoSecretRequired
		}

		err := PasetoValidator.LoadSecretKeyFromHex(cfg.Basic.PasetoSecret)
		if err != nil {
			key := authenticated.NewSecretKeyHex()
			log.Error().Err(err).Msgf(`Generated secret key (put this in config.yaml)\nbasic:\n  secret: "%s"`, key)

			return errPasetoSecretInvalid
		}

		// remove key. no longer needed.
		cfg.Basic.PasetoSecret = ""
	}

	// Turnstile specific configuration
	if cfg.Limiter.DetectionMethod == Turnstile {
		if cfg.Limiter.TurnstileSitekey == "" {
			return errTurnstileSitekeyRequired
		}

		if cfg.Limiter.TurnstileSecretKey == "" {
			return errTurnstileSecretKeyRequired
		}
	}

	if cfg.Limiter.IPv4Prefix < 0 || cfg.Limiter.IPv4Prefix > 32 {
		return errInvalidIPv4Prefix
	}

	if cfg.Limiter.IPv6Prefix < 0 || cfg.Limiter.IPv6Prefix > 128 {
		return errInvalidIPv6Prefix
	}

	return nil
}

// validateProxy validates ContentProxies.
func validateProxy(rawURL *string, defaultPath, proxyType string) error {
	if *rawURL == defaultPath {
		return nil
	}

	_, err := utils.ParseURL(*rawURL, proxyType+" proxy server")
	if err != nil {
		return fmt.Errorf("invalid %s proxy URL: %w", proxyType, err)
	}

	return nil
}
