// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
Package lrucache provides a thread-safe, fixed-capacity least-recently-used (LRU) cache.
Keys are strings. The cache evicts the least recently used entry when it reaches capacity.
When created with compression enabled via [NewLRUCache], string and []byte values may be
stored in compressed form and are transparently decompressed by [LRUCache.Get] and [LRUCache.Peek].
*/
package lrucache

import (
	"container/list"
	"errors"
	"sync"

	"github.com/klauspost/compress/zstd"
)

var ErrInvalidSize = errors.New("must provide a positive size")

// valueType describes what kind of value we store for transparent compression/decompression.
type valueType int

const (
	vtUnknown valueType = iota
	vtBytes
	vtString
)

// LRUCache is a fixed-capacity, least-recently-used cache that is safe for concurrent use.
// Instances must be constructed with [NewLRUCache]; the zero value is not ready for use.
// Keys are strings. When created with compression enabled, string and []byte values may be
// stored in compressed form and are transparently decompressed by [LRUCache.Get] and [LRUCache.Peek].
type LRUCache struct {
	size            int                      // Maximum capacity of the cache (number of entries)
	evictList       *list.List               // A doubly-linked list to manage the eviction order
	items           map[string]*list.Element // Maps string keys to their corresponding linked-list elements
	lock            sync.RWMutex             // For thread-safe operations
	compressEnabled bool                     // Whether transparent compression is enabled
	zstdEnc         *zstd.Encoder            // Reusable zstd encoder for block operations
	zstdDec         *zstd.Decoder            // Reusable zstd decoder for block operations
}

// cacheEntry holds the key/value pair stored in each linked-list element.
type cacheEntry struct {
	key        string
	value      any
	compressed bool
	vtype      valueType
}

// NewLRUCache creates a new cache with the specified maximum size.
//
// If compress is true, string and []byte values are stored in a compressed form
// when this reduces space and are transparently decompressed by [LRUCache.Get]
// and [LRUCache.Peek]. Values of other types are stored uncompressed.
//
// It returns an error if size is not a positive integer.
func NewLRUCache(size int, compress bool) (*LRUCache, error) {
	if size <= 0 {
		return nil, ErrInvalidSize
	}

	c := &LRUCache{
		size:            size,
		evictList:       list.New(),
		items:           make(map[string]*list.Element),
		compressEnabled: compress,
	}

	if compress {
		// Create reusable encoder/decoder for block (stateless) operations.
		// A nil writer/reader lets us use EncodeAll/DecodeAll without streams.
		enc, err := zstd.NewWriter(nil)
		if err != nil {
			return nil, err
		}

		dec, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))
		if err != nil {
			return nil, err
		}

		c.zstdEnc = enc
		c.zstdDec = dec
	}

	return c, nil
}

// Add adds or updates the value for key.
//
// If the key exists, it becomes the most recently used.
// If the cache is at capacity, the least recently used item is evicted.
// Add reports whether an eviction occurred.
func (c *LRUCache) Add(key string, value any) bool {
	// Prepare (and possibly compress) the value before acquiring the lock.
	storedVal, compressed, vtype := c.prepareValue(value)

	c.lock.Lock()
	defer c.lock.Unlock()

	// If the item already exists, move it to the front as "most recently used" and update its value.
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)

		if cacheEnt, ok := ent.Value.(*cacheEntry); ok {
			cacheEnt.value = storedVal
			cacheEnt.compressed = compressed
			cacheEnt.vtype = vtype
		}

		return false
	}

	// Otherwise, create a new entry and place it at the front.
	c.items[key] = c.evictList.PushFront(&cacheEntry{
		key:        key,
		value:      storedVal,
		compressed: compressed,
		vtype:      vtype,
	})

	// If we've exceeded our capacity, remove the oldest item from the back of the list.
	evicted := c.evictList.Len() > c.size
	if evicted {
		c.removeOldest()
	}

	return evicted
}

// Get retrieves the value for key and marks it as most recently used.
//
// The second result reports whether the key was found.
// When compression was enabled at construction, values stored as strings or byte slices
// are transparently decompressed. When returning a []byte value, Get returns a copy
// to prevent callers from mutating the cached data.
func (c *LRUCache) Get(key string) (any, bool) {
	// Lock for write since we will move the element to the front.
	c.lock.Lock()

	ent, ok := c.items[key]
	if !ok {
		c.lock.Unlock()
		return nil, false
	}

	c.evictList.MoveToFront(ent)

	cacheEnt, ok := ent.Value.(*cacheEntry)
	if !ok {
		c.lock.Unlock()
		return nil, false
	}

	// Copy fields needed for decompression and release the lock early.
	stored := cacheEnt.value
	compressed := cacheEnt.compressed
	vtype := cacheEnt.vtype

	c.lock.Unlock()

	return c.decompressValue(stored, compressed, vtype)
}

// Peek retrieves the value for key without modifying the LRU order.
//
// The second result reports whether the key was found.
// When compression was enabled at construction, values stored as strings or byte slices
// are transparently decompressed. When returning a []byte value, Peek returns a copy
// to prevent callers from mutating the cached data.
func (c *LRUCache) Peek(key string) (any, bool) {
	c.lock.RLock()

	ent, ok := c.items[key]
	if !ok {
		c.lock.RUnlock()
		return nil, false
	}

	cacheEnt, ok := ent.Value.(*cacheEntry)
	if !ok {
		c.lock.RUnlock()
		return nil, false
	}

	// Copy fields needed for decompression and release the lock early.
	stored := cacheEnt.value
	compressed := cacheEnt.compressed
	vtype := cacheEnt.vtype

	c.lock.RUnlock()

	return c.decompressValue(stored, compressed, vtype)
}

// Remove deletes the entry associated with key from the cache.
//
// Remove reports whether the key was present and removed.
func (c *LRUCache) Remove(key string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if ent, ok := c.items[key]; ok {
		c.removeElement(ent)

		return true
	}

	return false
}

// Keys returns a slice of all keys in the cache, from the oldest to the newest.
func (c *LRUCache) Keys() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()

	keys := make([]string, len(c.items))
	index := 0

	// The back of the list is the oldest entry.
	for ent := c.evictList.Back(); ent != nil; ent = ent.Prev() {
		if cacheEnt, ok := ent.Value.(*cacheEntry); ok {
			keys[index] = cacheEnt.key
			index++
		}
	}

	return keys
}

// Len returns the current number of items in the cache.
func (c *LRUCache) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.evictList.Len()
}

// removeOldest removes the oldest item from both the linked list and the map.
func (c *LRUCache) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
	}
}

// removeElement is a helper function that removes a specific list element
// from the eviction list and deletes it from the map.
//
// Used to remove a given LRU entry.
func (c *LRUCache) removeElement(e *list.Element) {
	c.evictList.Remove(e)

	if kv, ok := e.Value.(*cacheEntry); ok {
		delete(c.items, kv.key)
	}
}

// prepareValue evaluates whether we should compress the provided value.
// If compression is enabled and the value is a string or []byte, it will be compressed.
// Compression is only stored if it reduces size.
// To prevent callers from mutating the cache, uncompressed []byte values are copied before being stored.
//
// This function performs the heavy work of compression and is safe to call without holding the lock,
// as the underlying zstd Encoder supports concurrent EncodeAll calls.
func (c *LRUCache) prepareValue(value any) (stored any, compressed bool, vtype valueType) {
	switch v := value.(type) {
	case []byte:
		if c.compressEnabled {
			vtype = vtBytes
		}

		// Fast path for nil or empty slice, which are safe to store directly.
		if len(v) == 0 {
			return v, false, vtype
		}

		// Try to compress if enabled.
		if c.compressEnabled {
			compressedBytes := c.zstdEnc.EncodeAll(v, nil)
			if len(compressedBytes) < len(v) {
				return compressedBytes, true, vtype
			}
		}

		// If not compressed (either because it was disabled or ineffective), store a copy.
		copied := make([]byte, len(v))
		copy(copied, v)

		return copied, false, vtype

	case string:
		if c.compressEnabled {
			vtype = vtString
		}

		// Strings are immutable, so no copy is needed.
		if len(v) == 0 {
			return v, false, vtype
		}

		// Try to compress if enabled.
		if c.compressEnabled {
			orig := []byte(v)

			compressedBytes := c.zstdEnc.EncodeAll(orig, nil)
			if len(compressedBytes) < len(orig) {
				return compressedBytes, true, vtype
			}
		}

		return v, false, vtype

	default:
		// Unsupported types are stored as-is.
		return value, false, vtUnknown
	}
}

// decompressValue returns the actual value to callers, performing decompression if needed.
// To prevent callers from mutating the cache, a copy of uncompressed []byte values is returned.
// If decompression fails (which should be extremely rare), the value is considered unavailable.
//
// This function does not access or modify any cache state and should be called without holding the lock.
func (c *LRUCache) decompressValue(stored any, compressed bool, vtype valueType) (any, bool) {
	if !compressed {
		if b, ok := stored.([]byte); ok {
			// Return a copy to prevent mutation of the cached value.
			if b == nil {
				return nil, true
			}

			copied := make([]byte, len(b))
			copy(copied, b)

			return copied, true
		}
		// Strings are immutable, and other types are returned as-is.
		return stored, true
	}

	// Only strings/bytes can be compressed by our implementation.
	comp, ok := stored.([]byte)
	if !ok || c.zstdDec == nil {
		return nil, false
	}

	decoded, err := c.zstdDec.DecodeAll(comp, nil)
	if err != nil {
		return nil, false
	}

	switch vtype {
	case vtString:
		return string(decoded), true
	case vtBytes:
		return decoded, true
	default:
		// Should not happen, but fall back to bytes.
		return decoded, true
	}
}
