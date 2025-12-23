// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import "time"

const (
	// Default cache TTL in minutes.
	defaultCacheTTLMinutes = 60
	// Default HTTP cache max age in seconds.
	defaultHTTPCacheMaxAgeSeconds = 30
	// Default HTTP cache stale while revalidate in seconds.
	defaultHTTPCacheStaleWhileRevalidateSeconds = 60

	// Default token manager base timeout in milliseconds.
	defaultTokenManagerBaseTimeoutMs = 1000
	// Default token manager max backoff time in milliseconds.
	defaultTokenManagerMaxBackoffTimeMs = 32000
)

// SetDefaults populates the configuration with default values.
func (cfg *ServerConfig) SetDefaults() {
	cfg.Basic.Host = "localhost"
	cfg.Basic.Port = "8282"

	cfg.ContentProxies.RawImage = BuiltInImageProxyPath
	cfg.ContentProxies.RawStatic = BuiltInStaticProxyPath
	cfg.ContentProxies.RawUgoira = BuiltInUgoiraProxyPath

	cfg.TokenManager.LoadBalancing = "round-robin"
	cfg.TokenManager.MaxRetries = 5
	cfg.TokenManager.BaseTimeout = defaultTokenManagerBaseTimeoutMs * time.Millisecond
	cfg.TokenManager.MaxBackoffTime = defaultTokenManagerMaxBackoffTimeMs * time.Millisecond

	cfg.Cache.Enabled = false
	cfg.Cache.Size = 100
	cfg.Cache.TTL = defaultCacheTTLMinutes * time.Minute

	cfg.HTTPCache.MaxAge = defaultHTTPCacheMaxAgeSeconds * time.Second
	cfg.HTTPCache.StaleWhileRevalidate = defaultHTTPCacheStaleWhileRevalidateSeconds * time.Second

	cfg.Request.AcceptLanguage = "en-US,en;q=0.5"

	cfg.Response.EarlyHintsResponses = false

	cfg.Feature.PopularSearch = false
	cfg.Feature.FastTagSuggestions = false
	cfg.Feature.OpenAllButton = false

	cfg.Instance.RepoURL = "https://codeberg.org/PixivFE/PixivFE"

	cfg.Development.SaveResponses = false
	cfg.Development.ResponseSaveLocation = "/tmp/pixivfe/responses"

	cfg.Log.Level = "info"
	cfg.Log.Outputs = []string{"/dev/stderr"}
	cfg.Log.Format = "console"

	cfg.Limiter.Enabled = false
	cfg.Limiter.StateFilepath = "./data/limiter_state.json"
	cfg.Limiter.FilterLocal = false
	cfg.Limiter.IPv4Prefix = 24
	cfg.Limiter.IPv6Prefix = 48
	cfg.Limiter.CheckHeaders = true
	cfg.Limiter.DetectionMethod = ""

	cfg.Internationalization.StrictMissingKeys = false
}
