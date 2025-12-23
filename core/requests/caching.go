// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package requests

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/requests/lrucache"
)

var (
	cache *lrucache.LRUCache

	// excludedCachePaths lists API endpoints for which responses are never cached.
	excludedCachePaths = []string{
		"/ajax/discovery/artworks",
		"/ajax/discovery/novels",
		"/ajax/discovery/users",
		"/ajax/illust/new",
	}
)

// cachedItem represents a cached HTTP response's components along with its expiration time and original URL.
type cachedItem struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	ExpiresAt  time.Time
	URL        string
}

// cachePolicy defines the caching behavior for a request.
type cachePolicy struct {
	// Whether to attempt fetching from the cache
	// and store any OK response that we receieve.
	shouldUseCache bool

	// The cached item if available and valid.
	cachedItem *cachedItem
}

// Setup initializes the API response cache based on parameters in GlobalConfig.
//
// It sets up an LRU cache with a specified size and logs the cache parameters.
// If caching is disabled in the configuration, it skips initialization.
func Setup() {
	if !config.Global.Cache.Enabled {
		log.Info().
			Msg("Cache is disabled, skipping cache initialization")

		return
	}

	var err error

	cache, err = lrucache.NewLRUCache(config.Global.Cache.Size, false)
	if err != nil {
		panic(fmt.Sprintf("failed to create cache: %v", err))
	}

	log.Info().
		Int("size", config.Global.Cache.Size).
		Msg("Initialized API response cache")
}

// The `generateCacheKey` function securely binds cached responses to both the request URL and the full authenticated
// session token by combining them into a hashed identifier.
//
// Using only the user ID portion of the token for cache keys would expose two risks:
//  1. Validating a token's authenticity would require an API call, undermining cache efficiency;
//  2. Attackers could forge tokens containing valid user IDs (e.g., `123456_invalidSessionSecret`)
//     to access cached private data for arbitrary users.
//
// By hashing the *entire* userToken alongside the URL, we ensure responses remain strictly scoped
// to the exact authentication session that originally requested them.
func generateCacheKey(url, userToken string) string {
	hasher := fnv.New32()

	_, _ = hasher.Write([]byte(url + ":" + userToken))

	return strconv.FormatUint(uint64(hasher.Sum32()), 16)
}

// determineCachePolicy determines the caching policy for a given request.
//
// It returns a CachePolicy struct indicating whether a valid cached response is available,
// or whether a new response should be stored in the cache.
func determineCachePolicy(rawURL, userToken string, headers http.Header) cachePolicy {
	if !config.Global.Cache.Enabled {
		return cachePolicy{}
	}

	if cache == nil {
		return cachePolicy{}
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return cachePolicy{} // Invalid URL, don't cache.
	}

	// Never cache responses for excluded paths.
	cleanPath := path.Clean(parsedURL.Path)
	for _, exclPath := range excludedCachePaths {
		if strings.HasPrefix(cleanPath, exclPath) {
			return cachePolicy{}
		}
	}

	// Honor "no-cache" directive from the downstream client: skip both read and write.
	lowerCacheControl := strings.ToLower(headers.Get("Cache-Control"))
	if strings.Contains(lowerCacheControl, "no-cache") {
		return cachePolicy{}
	}

	cacheKey := generateCacheKey(rawURL, userToken)

	// Try to serve a valid cached response immediately.
	if cached, found := cache.Get(cacheKey); found {
		cachedBytes, ok := cached.([]byte)
		if !ok {
			cache.Remove(cacheKey)
		} else {
			var item cachedItem
			if err := gob.NewDecoder(bytes.NewReader(cachedBytes)).Decode(&item); err != nil {
				log.Warn().Err(err).Str("key", cacheKey).Msg("Failed to decode cached item; removing")
				cache.Remove(cacheKey)
			} else if time.Now().Before(item.ExpiresAt) {
				// Fresh item found.
				return cachePolicy{
					shouldUseCache: true, // We are using the cache.
					cachedItem:     &item,
				}
			} else {
				// Item has expired.
				cache.Remove(cacheKey)
			}
		}
	}

	// No valid cached item was found. Decide whether to store the next response.
	return cachePolicy{
		shouldUseCache: !strings.Contains(lowerCacheControl, "no-store"),
	}
}

// InvalidateURLs removes all cached items where the cached URL starts with any of the provided URL prefixes.
//
// Takes a slice of URL prefixes to invalidate and returns the number of cache entries removed and their full URLs.
// Safe to call even if caching is disabled. If urls slice is nil or empty, returns 0 and an empty string slice.
//
// cache.Contains isn't used as we don't actually know which cache key to look for due to how generateCacheKey() works
//
// Scoping invalidation to a specific user's context is possible (e.g. per user ID), but offers little benefit
// for the additional complexity it introduces: how many users with different auth states are realistically
// hitting similar endpoints for it to matter?
func InvalidateURLs(urlPrefixes []string) (int, []string) {
	var invalidatedURLs []string

	if !config.Global.Cache.Enabled || cache == nil || len(urlPrefixes) == 0 {
		return 0, invalidatedURLs
	}

	keys := cache.Keys()
	invalidatedCount := 0

	for _, key := range keys {
		item, ok := cache.Peek(key)
		if !ok {
			continue
		}

		cachedBytes, ok := item.([]byte)
		if !ok {
			continue
		}

		var ci cachedItem

		// No need to log an error here; we're just peeking for invalidation.
		// If it's corrupt, it will be evicted eventually or removed on a failed [cache.Get].
		if err := gob.NewDecoder(bytes.NewReader(cachedBytes)).Decode(&ci); err != nil {
			continue
		}

		for _, prefix := range urlPrefixes {
			if strings.HasPrefix(ci.URL, prefix) {
				cache.Remove(key)

				invalidatedCount++

				invalidatedURLs = append(invalidatedURLs, ci.URL)

				break
			}
		}
	}

	log.Info().
		Int("count", invalidatedCount).
		Strs("urls", invalidatedURLs).
		Msg("Invalidated URLs")

	return invalidatedCount, invalidatedURLs
}
