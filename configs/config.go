// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/rs/zerolog/log"

	_ "codeberg.org/pixivfe/pixivfe/v3/core/audit" // setup better logging format
	"codeberg.org/pixivfe/pixivfe/v3/core/idgen"
	"codeberg.org/pixivfe/pixivfe/v3/core/tokenmanager"
)

// Global exposes the server configuration.
var Global ServerConfig

// Possible values for limiterDetectionMethod.
const (
	None      LimiterDetectionMethod = ""
	LinkToken LimiterDetectionMethod = "linktoken"
	Turnstile LimiterDetectionMethod = "turnstile"
)

// LimiterDetectionMethod is a method used by package limiter to detect unwanted automated requests.
type LimiterDetectionMethod string

// ServerConfig holds the application configuration.
type ServerConfig struct {
	Build buildInfo `yaml:"-"`

	Basic struct {
		Host                     string      `env:"PIXIVFE_HOST,overwrite" yaml:"host"`
		Port                     string      `env:"PIXIVFE_PORT,overwrite" yaml:"port"`
		UnixSocket               string      `env:"PIXIVFE_UNIXSOCKET" yaml:"unixSocket"`
		RawUnixSocketPermissions string      `env:"PIXIVFE_UNIXSOCKET_PERMISSIONS" yaml:"unixSocketPermissions"`
		UnixSocketPermissions    os.FileMode `yaml:"-"`
		UnixSocketUser           string      `env:"PIXIVFE_UNIXSOCKET_USER" yaml:"unixSocketUser"`
		UnixSocketGroup          string      `env:"PIXIVFE_UNIXSOCKET_GROUP" yaml:"unixSocketGroup"`
		Token                    []string    `env:"PIXIVFE_TOKEN" yaml:"token"`
		// raw bytes of v4.public secret key
		PasetoSecret string `env:"PIXIVFE_SECRET" yaml:"secret"`
	} `yaml:"basic"`

	ContentProxies struct {
		RawImage  string  `env:"PIXIVFE_IMAGEPROXY,overwrite" yaml:"imageProxy"`
		Image     url.URL `yaml:"-"` // For i.pximg.net
		RawStatic string  `env:"PIXIVFE_STATICPROXY,overwrite" yaml:"staticProxy"`
		Static    url.URL `yaml:"-"` // For s.pximg.net
		RawUgoira string  `env:"PIXIVFE_UGOIRAPROXY,overwrite" yaml:"ugoiraProxy"`
		Ugoira    url.URL `yaml:"-"` // For ugoira.com
	} `yaml:"contentProxies"`

	TokenManager struct {
		LoadBalancing  string        `env:"PIXIVFE_TOKEN_LOAD_BALANCING,overwrite" yaml:"tokenLoadBalancing"`
		MaxRetries     int           `env:"PIXIVFE_TOKEN_MAX_RETRIES,overwrite" yaml:"tokenMaxRetries"`
		BaseTimeout    time.Duration `env:"PIXIVFE_TOKEN_BASE_TIMEOUT,overwrite" yaml:"tokenBaseTimeout"`
		MaxBackoffTime time.Duration `env:"PIXIVFE_TOKEN_MAX_BACKOFF_TIME,overwrite" yaml:"tokenMaxBackoffTime"`
	} `yaml:"tokenManager"`

	Cache struct {
		Enabled bool          `env:"PIXIVFE_CACHE,overwrite" yaml:"enabled"`
		Size    int           `env:"PIXIVFE_CACHE_SIZE,overwrite" yaml:"cacheSize"`
		TTL     time.Duration `env:"PIXIVFE_CACHE_TTL,overwrite" yaml:"cacheTTL"`
	} `yaml:"cache"`

	HTTPCache struct {
		MaxAge               time.Duration `env:"PIXIVFE_CACHE_CONTROL_MAX_AGE,overwrite" yaml:"cacheControlMaxAge"`
		StaleWhileRevalidate time.Duration `env:"PIXIVFE_CACHE_CONTROL_STALE_WHILE_REVALIDATE,overwrite" yaml:"cacheControlStaleWhileRevalidate"`
	} `yaml:"httpCache"`

	Request struct {
		AcceptLanguage string `env:"PIXIVFE_ACCEPTLANGUAGE,overwrite" yaml:"acceptLanguage"`
	} `yaml:"request"`

	Response struct {
		EarlyHintsResponses bool `env:"PIXIVFE_EARLY_HINTS_RESPONSES,overwrite" yaml:"earlyHintsResponses"`
	} `yaml:"response"`

	Feature struct {
		PopularSearch      bool `env:"PIXIVFE_POPULAR_SEARCH,overwrite" yaml:"popularSearch"`
		FastTagSuggestions bool `env:"PIXIVFE_FAST_TAG_SUGGESTIONS,overwrite" yaml:"fastTagSuggestions"`
		OpenAllButton      bool `env:"PIXIVFE_OPEN_ALL_BUTTON,overwrite" yaml:"openAllButton"`
	} `yaml:"feature"`

	Instance struct {
		StartingTime      string `yaml:"-"`
		FileServerCacheID string `yaml:"-"`
		RepoURL           string `env:"PIXIVFE_REPO_URL,overwrite" yaml:"repoUrl"`
	} `yaml:"instance"`

	Development struct {
		InDevelopment        bool   `env:"PIXIVFE_DEV" yaml:"inDevelopment"`
		SaveResponses        bool   `env:"PIXIVFE_SAVE_RESPONSES,overwrite" yaml:"saveResponses"`
		ResponseSaveLocation string `env:"PIXIVFE_RESPONSE_SAVE_LOCATION,overwrite" yaml:"responseSaveLocation"`
	} `yaml:"development"`

	Log struct {
		Level   string   `env:"PIXIVFE_LOG_LEVEL,overwrite" yaml:"logLevel"`
		Outputs []string `env:"PIXIVFE_LOG_OUTPUTS,overwrite" yaml:"logOutputs"`
		Format  string   `env:"PIXIVFE_LOG_FORMAT,overwrite" yaml:"logFormat"`
	} `yaml:"log"`

	Limiter struct {
		Enabled            bool                   `env:"PIXIVFE_LIMITER,overwrite" yaml:"enabled"`
		StateFilepath      string                 `env:"PIXIVFE_LIMITER_STATE_FILEPATH,overwrite" yaml:"stateFilepath"`
		PassIPs            []string               `env:"PIXIVFE_LIMITER_PASS_IPS,overwrite" yaml:"passList"`
		BlockIPs           []string               `env:"PIXIVFE_LIMITER_BLOCK_IPS,overwrite" yaml:"blockList"`
		FilterLocal        bool                   `env:"PIXIVFE_LIMITER_FILTER_LOCAL,overwrite" yaml:"filterLocal"`
		IPv4Prefix         int                    `env:"PIXIVFE_LIMITER_IPV4_PREFIX,overwrite" yaml:"ipv4Prefix"`
		IPv6Prefix         int                    `env:"PIXIVFE_LIMITER_IPV6_PREFIX,overwrite" yaml:"ipv6Prefix"`
		CheckHeaders       bool                   `env:"PIXIVFE_LIMITER_CHECK_HEADERS,overwrite" yaml:"checkHeaders"`
		DetectionMethod    LimiterDetectionMethod `env:"PIXIVFE_LIMITER_DETECTION_METHOD,overwrite" yaml:"detectionMethod"`
		TurnstileSitekey   string                 `env:"PIXIVFE_LIMITER_TURNSTILE_SITEKEY" yaml:"turnstileSitekey"`
		TurnstileSecretKey string                 `env:"PIXIVFE_LIMITER_TURNSTILE_SECRET_KEY" yaml:"turnstileSecretKey"`
	} `yaml:"limiter"`

	Internationalization struct {
		// Strict mode for missing keys.
		//
		// When enabled, missing keys are logged (deduplicated per locale+key) and
		// visibly wrapped using markers.
		StrictMissingKeys bool `env:"PIXIVFE_STRICT_MISSING_KEYS" yaml:"strictMissingKeys"`
	}
}

// LoadConfig loads the configuration from various sources.
func (cfg *ServerConfig) LoadConfig() error {
	parsedConfigFlagValue := parseCommandLineArgs()

	// Check if the -config flag was explicitly set by the user.
	configFlagUserSet := false

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			configFlagUserSet = true
		}
	})

	var configFilePath string

	// Determine the config file path with the correct precedence:
	// 1. Command-line flag (-config)
	// 2. Environment variable (PIXIVFE_CONFIGFILE)
	// 3. Default path with fallback check
	if configFlagUserSet {
		// Command-line flag has the highest precedence.
		configFilePath = parsedConfigFlagValue
	} else if envVar := os.Getenv("PIXIVFE_CONFIGFILE"); envVar != "" {
		// Environment variable is next.
		configFilePath = envVar
	} else {
		// If neither flag nor env var was provided, use the default value
		// from the flag ("./config.yaml").
		configFilePath = parsedConfigFlagValue
		// Then, perform a fallback check for "./config.yml".
		if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
			ymlPath := "./config.yml"
			if _, statErr := os.Stat(ymlPath); statErr == nil {
				configFilePath = ymlPath
			}
		}
	}

	cfg.SetDefaults()

	cfg.Build.load()

	cfg.Instance.FileServerCacheID = idgen.Make()
	cfg.Instance.StartingTime = time.Now().UTC().Format("2006-01-02 15:04")

	if err := cfg.readYAML(configFilePath); err != nil {
		return fmt.Errorf("error loading YAML config: %w", err)
	}

	// @iacore: maybe this should be moved to the start of main()? there must be an existing library for this
	if err := useDotEnv(); err != nil {
		return fmt.Errorf("error using .env file: %w", err)
	}

	if err := readEnv(cfg); err != nil {
		return fmt.Errorf("error loading environment variables: %w", err)
	}

	if err := cfg.validateAndSet(); err != nil {
		return fmt.Errorf("configuration invalid: %w", err)
	}

	cfg.setupAudit()

	tokenmanager.DefaultTokenManager = tokenmanager.NewTokenManager(
		cfg.Basic.Token,
		cfg.TokenManager.MaxRetries,
		cfg.TokenManager.BaseTimeout,
		cfg.TokenManager.MaxBackoffTime,
		cfg.TokenManager.LoadBalancing,
	)

	cfg.print()

	// Heuristically check for containerized environment and warn if host is not a wildcard address.
	if isContainerized() && cfg.Basic.Host != "0.0.0.0" && cfg.Basic.Host != "::" {
		log.Warn().
			Str("host", cfg.Basic.Host).
			Msg("Running in a containerized environment but host is not a wildcard address (e.g., '0.0.0.0' or '::'). This may prevent the service from being accessible outside the container.")
	}

	return nil
}

var (
	staticSkippedPathPrefixes = []string{"/img/", "/css/", "/js/"}
	devSkippedPathPrefixes    = []string{"/proxy/s.pximg.net/", "/proxy/i.pximg.net/"}
)

// ShouldSkipServerLogging determines if a request should bypass the logging middleware.
func (cfg *ServerConfig) ShouldSkipServerLogging(path string) bool {
	for _, prefix := range staticSkippedPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	if cfg.Development.InDevelopment {
		for _, prefix := range devSkippedPathPrefixes {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}

	return false
}

// isContainerized checks for common indicators of a containerized environment.
//
// This is a heuristic and may not be 100% accurate.
func isContainerized() bool {
	// Check for a Kubernetes-injected environment variable.
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// Check for existence of container-specific files.
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if _, err := os.Stat("/.containerenv"); err == nil {
		return true
	}

	// Check the cgroup of the current process.
	// #nosec G304 -- We are checking for the existence and content of a well-known system file for heuristics.
	cgroup, err := os.ReadFile("/proc/self/cgroup")
	if err == nil {
		content := string(cgroup)

		// Check for keywords common in container cgroup paths.
		return strings.Contains(content, "docker") ||
			strings.Contains(content, "kubepods") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "lxc") ||
			strings.Contains(content, "crio") ||
			// systemd-nspawn containers
			strings.Contains(content, ".machine")
	}

	return false
}

// GetDurationEncoderOption returns a YAML encoder option that marshals
// time.Duration into a human-readable string format (e.g., "30m", "1h").
func GetDurationEncoderOption() yaml.EncodeOption {
	return yaml.CustomMarshaler[time.Duration](
		func(d time.Duration) ([]byte, error) {
			return yaml.Marshal(d.String())
		},
	)
}
