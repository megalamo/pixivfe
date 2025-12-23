// PixivFE/core/requests/lrucache/lrucache_test.go
// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package lrucache

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewLRUCache checks the creation of a new LRUCache with both valid and invalid sizes.
func TestNewLRUCache(t *testing.T) {
	t.Parallel()

	t.Run("ValidSize_NoCompression", func(t *testing.T) {
		t.Parallel()

		// Create an LRU cache of size 3, which is valid.
		cache, err := NewLRUCache(3, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cache == nil {
			t.Fatal("expected cache to be initialized")
		}

		// Immediately after creation, the cache should be empty.
		if cache.Len() != 0 {
			t.Errorf("expected cache length to be 0, got %d", cache.Len())
		}
	})

	t.Run("ValidSize_WithCompression", func(t *testing.T) {
		t.Parallel()

		// Create an LRU cache of size 3 with compression enabled.
		cache, err := NewLRUCache(3, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cache == nil {
			t.Fatal("expected cache to be initialized")
		}

		// Immediately after creation, the cache should be empty.
		if cache.Len() != 0 {
			t.Errorf("expected cache length to be 0, got %d", cache.Len())
		}
	})

	t.Run("InvalidSize", func(t *testing.T) {
		t.Parallel()

		// Create an LRU cache of size 0, which should fail.
		cache, err := NewLRUCache(0, false)
		if err == nil {
			t.Fatal("expected error when creating cache of size 0, got nil")
		}

		if cache != nil {
			t.Error("expected no cache to be returned on error")
		}
	})
}

// TestLRUCache_AddAndGet verifies that adding a key to the cache and retrieving it works correctly,
// and that eviction occurs once the capacity is reached.
func TestLRUCache_AddAndGet(t *testing.T) {
	t.Parallel()

	// Create a cache with capacity 2.
	cache, err := NewLRUCache(2, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add first key; eviction should not occur yet.
	evicted := cache.Add("foo", "bar")
	if evicted {
		t.Error("eviction should not occur when the cache is not full")
	}

	// Retrieve the newly added key.
	value, ok := cache.Get("foo")
	if !ok {
		t.Error("expected to retrieve value for key 'foo'")
	}

	if value != "bar" {
		t.Errorf("expected 'bar', got %v", value)
	}

	// Add second key.
	cache.Add("hello", "world")

	// Ensure the cache length is now 2.
	if cache.Len() != 2 {
		t.Errorf("expected cache length 2, got %d", cache.Len())
	}

	// Adding a third key should cause eviction of the least recently used item.
	evicted = cache.Add("key3", "value3")
	if !evicted {
		t.Error("expected eviction when adding third key to size 2 cache")
	}

	// "foo" should be evicted because it was the oldest after the second key was used.
	_, ok = cache.Get("foo")
	if ok {
		t.Error("expected 'foo' to be evicted, but it still exists")
	}
}

// TestLRUCache_AddExistingKey ensures that adding a key that already exists
// updates the value and does not evict any item.
func TestLRUCache_AddExistingKey(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(2, false)

	cache.Add("k1", "v1")
	cache.Add("k2", "v2")

	// Re-add k1 with new value; expect no eviction since the key already exists.
	evicted := cache.Add("k1", "v1-updated")
	if evicted {
		t.Error("re-adding an existing key should not evict anything")
	}

	// Verify the value was updated in the cache.
	val, ok := cache.Get("k1")
	if !ok {
		t.Error("expected to find updated key 'k1'")
	}

	if val != "v1-updated" {
		t.Errorf("expected 'v1-updated', got %v", val)
	}

	// Cache size should remain at 2 (no evictions).
	if cache.Len() != 2 {
		t.Errorf("expected cache length 2, got %d", cache.Len())
	}
}

// TestLRUCache_Peek checks that Peek returns the value without updating the item’s priority.
func TestLRUCache_Peek(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(2, false)

	cache.Add("foo", "bar")
	cache.Add("baz", "qux")

	// Peek at "foo"; this should not move it to the front of the usage list.
	val, ok := cache.Peek("foo")
	if !ok {
		t.Error("expected to peek value for 'foo'")
	}

	if val != "bar" {
		t.Errorf("expected 'bar', got %v", val)
	}

	// Now add a third key to force eviction of the least recently used item.
	cache.Add("third", "value3")

	// If the peek didn't promote "foo", it remains the oldest and should be evicted.
	_, ok = cache.Get("foo")
	if ok {
		t.Error("expected 'foo' to be evicted after adding 'third'")
	}

	_, ok = cache.Get("baz")
	if !ok {
		t.Error("expected 'baz' to remain in the cache")
	}
}

// TestLRUCache_Remove confirms that removing a key explicitly works.
func TestLRUCache_Remove(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(2, false)

	cache.Add("foo", "bar")
	cache.Add("key", "value")

	// Remove the key "foo" and verify it no longer exists.
	removed := cache.Remove("foo")
	if !removed {
		t.Error("expected to remove existing key 'foo'")
	}

	val, ok := cache.Get("foo")
	if ok || val != nil {
		t.Error("expected 'foo' to be removed from cache")
	}

	// Attempt to remove a key that doesn't exist.
	removed = cache.Remove("not-present")
	if removed {
		t.Error("expected false when removing a non-existent key, but got true")
	}
}

// TestLRUCache_Keys checks that Keys returns the slice of keys from oldest to newest.
func TestLRUCache_Keys(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(3, false)
	cache.Add("first", 1)
	cache.Add("second", 2)
	cache.Add("third", 3)

	// Keys() returns oldest-to-newest, i.e., from the back of the list to the front.
	keys := cache.Keys()
	expected := []string{"first", "second", "third"}

	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}

	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("Keys() mismatch: expected %v at idx %d, got %v", expected[i], i, k)
		}
	}

	// Access "first", which should move it to the newest position.
	cache.Get("first")

	keys = cache.Keys()
	// Now the oldest is "second", next is "third", and the newest is "first".
	expected = []string{"second", "third", "first"}

	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("After usage, Keys mismatch: expected %v at idx %d, got %v", expected[i], i, k)
		}
	}
}

// TestLRUCache_Len verifies the length of the cache under various operations.
func TestLRUCache_Len(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(2, false)

	// Initially, the cache should be empty.
	if cache.Len() != 0 {
		t.Errorf("expected newly created cache size to be 0, got %d", cache.Len())
	}

	cache.Add("a", "b")

	// Now the cache should have exactly 1 item.
	if cache.Len() != 1 {
		t.Errorf("expected cache length to be 1, got %d", cache.Len())
	}

	cache.Add("c", "d")

	// Cache is now full (size 2).
	if cache.Len() != 2 {
		t.Errorf("expected cache length to be 2, got %d", cache.Len())
	}

	cache.Add("e", "f") // This will evict the oldest item.

	// The length should remain at 2 after the eviction.
	if cache.Len() != 2 {
		t.Errorf("expected cache length to remain 2 after eviction, got %d", cache.Len())
	}
}

// TestLRUCache_Compression_String ensures that with compression enabled, a compressible string
// is stored compressed and returned transparently as a string.
func TestLRUCache_Compression_String(t *testing.T) {
	t.Parallel()

	cache, err := NewLRUCache(2, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Highly compressible string.
	s := strings.Repeat("abc123", 64*1024/6) // ~64KiB compressible payload
	cache.Add("k", s)

	val, ok := cache.Get("k")
	if !ok {
		t.Fatal("expected to retrieve compressed string value")
	}

	strVal, ok := val.(string)
	if !ok {
		t.Fatalf("expected string type, got %T", val)
	}

	if strVal != s {
		t.Fatal("decompressed string does not match original")
	}

	// Inspect internal state to ensure value is stored compressed.
	cache.lock.RLock()

	ent := cache.items["k"]
	ce := ent.Value.(*cacheEntry)

	cache.lock.RUnlock()

	if !ce.compressed || ce.vtype != vtString {
		t.Fatalf("expected entry to be compressed string, got compressed=%v vtype=%v", ce.compressed, ce.vtype)
	}
}

// TestLRUCache_Compression_Bytes ensures that with compression enabled, a compressible byte slice
// is stored compressed and returned transparently as []byte.
func TestLRUCache_Compression_Bytes(t *testing.T) {
	t.Parallel()

	cache, err := NewLRUCache(2, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Highly compressible bytes (zeros).
	b := make([]byte, 128*1024) // 128KiB of zeros
	cache.Add("kb", b)

	val, ok := cache.Get("kb")
	if !ok {
		t.Fatal("expected to retrieve compressed byte slice")
	}

	buf, ok := val.([]byte)
	if !ok {
		t.Fatalf("expected []byte type, got %T", val)
	}

	if len(buf) != len(b) {
		t.Fatalf("expected same length after decompress, got %d want %d", len(buf), len(b))
	}

	for i := range buf {
		if buf[i] != 0 {
			t.Fatalf("decompressed bytes differ at index %d", i)
		}
	}

	// Inspect internal state to ensure value is stored compressed.
	cache.lock.RLock()

	ent := cache.items["kb"]
	ce := ent.Value.(*cacheEntry)

	cache.lock.RUnlock()

	if !ce.compressed || ce.vtype != vtBytes {
		t.Fatalf("expected entry to be compressed bytes, got compressed=%v vtype=%v", ce.compressed, ce.vtype)
	}
	// Ensure we actually saved space: stored compressed bytes should be smaller than original.
	if comp, ok := ce.value.([]byte); ok {
		if len(comp) >= len(b) {
			t.Fatalf("expected compressed size < original, got %d >= %d", len(comp), len(b))
		}
	} else {
		t.Fatalf("internal value is not []byte when compressed, got %T", ce.value)
	}
}

// TestLRUCache_Compression_Uncompressible ensures that data that doesn't compress well
// will be stored uncompressed even when compression is enabled.
func TestLRUCache_Compression_Uncompressible(t *testing.T) {
	t.Parallel()

	cache, err := NewLRUCache(2, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pseudorandom bytes are unlikely to compress.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 64*1024) // 64KiB

	_, _ = r.Read(b)

	cache.Add("rnd", b)

	cache.lock.RLock()

	ent := cache.items["rnd"]
	ce := ent.Value.(*cacheEntry)

	cache.lock.RUnlock()

	if ce.compressed {
		t.Fatalf("expected uncompressible data to be stored uncompressed")
	}

	// Roundtrip to ensure value is still returned correctly.
	val, ok := cache.Get("rnd")
	if !ok {
		t.Fatal("expected to retrieve value")
	}

	got, ok := val.([]byte)
	if !ok {
		t.Fatalf("expected []byte type, got %T", val)
	}

	if len(got) != len(b) {
		t.Fatalf("expected same length, got %d want %d", len(got), len(b))
	}

	for i := range got {
		if got[i] != b[i] {
			t.Fatalf("bytes differ at %d", i)
		}
	}
}

// TestLRUCache_Compression_Disabled confirms that with compression disabled, items are stored uncompressed.
func TestLRUCache_Compression_Disabled(t *testing.T) {
	t.Parallel()

	cache, err := NewLRUCache(2, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := strings.Repeat("aaa", 4096)
	cache.Add("k", s)

	cache.lock.RLock()

	ent := cache.items["k"]
	ce := ent.Value.(*cacheEntry)

	cache.lock.RUnlock()

	if ce.compressed {
		t.Fatalf("expected compressed=false when cache compression disabled")
	}

	if ce.vtype != vtUnknown {
		t.Fatalf("expected vtype=vtUnknown when compression disabled, got %v", ce.vtype)
	}

	val, ok := cache.Get("k")
	if !ok {
		t.Fatal("expected to retrieve value")
	}

	if val != s {
		t.Fatalf("expected roundtrip string to match")
	}
}

// TestLRUCache_Compression_Concurrency runs concurrent Add/Get to ensure thread safety
// with compression enabled.
func TestLRUCache_Compression_Concurrency(t *testing.T) {
	t.Parallel()

	cache, err := NewLRUCache(128, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup

	const n = 64

	// Prepare payloads
	payloadStr := strings.Repeat("xyz-", 8192)
	payloadBytes := []byte(strings.Repeat("1234567890", 8192))

	// Writers
	for i := 0; i < n; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			if i%2 == 0 {
				cache.Add("skey-"+strconv.Itoa(i), payloadStr)
			} else {
				cache.Add("bkey-"+strconv.Itoa(i), payloadBytes)
			}
		}(i)
	}

	// Readers
	for i := 0; i < n; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			key := ""
			if i%2 == 0 {
				key = "skey-" + strconv.Itoa(i)
				// It may not exist yet if scheduler delays writers; that's fine,
				// this test primarily checks for race/panics under compression.
				if v, ok := cache.Get(key); ok {
					_, _ = v.(string) // type assertion should work for strings
				}
			} else {
				key = "bkey-" + strconv.Itoa(i)
				if v, ok := cache.Get(key); ok {
					_, _ = v.([]byte) // type assertion should work for bytes
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestLRUCache_Peek_Miss(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(1, false)
	if v, ok := cache.Peek("nope"); ok || v != nil {
		t.Fatalf("expected Peek miss to return (nil,false), got (%v,%v)", v, ok)
	}
}

func TestLRUCache_RealizeValue_CompressedTypeMismatch(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(1, true)
	cache.Add("k", "val")

	// Force an inconsistent internal state: compressed=true but value is not []byte.
	cache.lock.Lock()

	ent := cache.items["k"]
	ce := ent.Value.(*cacheEntry)

	ce.compressed = true
	ce.value = "not-bytes"
	ce.vtype = vtString

	cache.lock.Unlock()

	// Get should fail to realize the value.
	if v, ok := cache.Get("k"); ok || v != nil {
		t.Fatalf("expected Get to fail for compressed non-bytes value, got (%v,%v)", v, ok)
	}

	// Peek should also fail to realize.
	if v, ok := cache.Peek("k"); ok || v != nil {
		t.Fatalf("expected Peek to fail for compressed non-bytes value, got (%v,%v)", v, ok)
	}
}

func TestLRUCache_RealizeValue_DecoderNil(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(1, true)
	cache.Add("k", []byte("data"))

	// Make decoder nil and mark entry as compressed bytes.
	cache.lock.Lock()

	ent := cache.items["k"]
	ce := ent.Value.(*cacheEntry)

	ce.compressed = true
	ce.value = []byte{0x01, 0x02, 0x03}
	cache.zstdDec = nil
	cache.lock.Unlock()

	if v, ok := cache.Get("k"); ok || v != nil {
		t.Fatalf("expected Get to fail when decoder is nil, got (%v,%v)", v, ok)
	}
}

func TestLRUCache_RealizeValue_DecodeError(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(1, true)
	cache.Add("k", []byte("x"))

	// Force invalid compressed bytes so decoder returns an error.
	cache.lock.Lock()

	ent := cache.items["k"]
	ce := ent.Value.(*cacheEntry)

	ce.compressed = true
	ce.value = []byte("not a zstd stream")
	ce.vtype = vtBytes

	cache.lock.Unlock()

	if v, ok := cache.Get("k"); ok || v != nil {
		t.Fatalf("expected Get to fail on decode error, got (%v,%v)", v, ok)
	}
}

func TestLRUCache_RealizeValue_DefaultVType(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(1, true)

	// Add compressible bytes.
	orig := make([]byte, 8192) // zeros compress well
	cache.Add("kb", orig)

	// Set vtype to unknown to exercise default branch after decode.
	cache.lock.Lock()

	ent := cache.items["kb"]

	ce := ent.Value.(*cacheEntry)
	if !ce.compressed {
		cache.lock.Unlock()
		t.Fatalf("expected entry to be compressed")
	}

	ce.vtype = vtUnknown

	cache.lock.Unlock()

	v, ok := cache.Get("kb")
	if !ok {
		t.Fatalf("expected Get to succeed")
	}

	b, ok := v.([]byte)
	if !ok {
		t.Fatalf("expected []byte return type, got %T", v)
	}

	if len(b) != len(orig) {
		t.Fatalf("length mismatch, got %d want %d", len(b), len(orig))
	}

	for i := range b {
		if b[i] != orig[i] {
			t.Fatalf("bytes differ at %d", i)
		}
	}
}

func TestLRUCache_PrepareValue_EmptyStringAndBytes(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(10, true)

	cache.Add("es", "")
	cache.Add("eb", []byte{})

	// Inspect internal states.
	cache.lock.RLock()

	ces := cache.items["es"].Value.(*cacheEntry)
	ceb := cache.items["eb"].Value.(*cacheEntry)
	cache.lock.RUnlock()

	if ces.compressed || ces.vtype != vtString {
		t.Fatalf("empty string: expected compressed=false vtype=vtString, got compressed=%v vtype=%v", ces.compressed, ces.vtype)
	}

	if ceb.compressed || ceb.vtype != vtBytes {
		t.Fatalf("empty bytes: expected compressed=false vtype=vtBytes, got compressed=%v vtype=%v", ceb.compressed, ceb.vtype)
	}

	// Roundtrip
	if v, ok := cache.Get("es"); !ok || v.(string) != "" {
		t.Fatalf("expected empty string roundtrip, got (%v,%v)", v, ok)
	}

	if v, ok := cache.Get("eb"); !ok {
		t.Fatalf("expected to retrieve empty []byte")
	} else if b, ok := v.([]byte); !ok || len(b) != 0 {
		t.Fatalf("expected empty []byte roundtrip, got %T len=%d", v, len(b))
	}
}

func TestLRUCache_Compression_StringUncompressible(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(2, true)

	s := "x" // very short; compressed form will be larger, so should store uncompressed
	cache.Add("sx", s)

	cache.lock.RLock()

	ce := cache.items["sx"].Value.(*cacheEntry)
	cache.lock.RUnlock()

	if ce.compressed || ce.vtype != vtString {
		t.Fatalf("expected uncompressed vtString for short string, got compressed=%v vtype=%v", ce.compressed, ce.vtype)
	}

	if v, ok := cache.Get("sx"); !ok || v.(string) != s {
		t.Fatalf("expected to retrieve original short string, got (%v,%v)", v, ok)
	}
}

func TestLRUCache_PrepareValue_UnsupportedType_WithCompression(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(2, true)
	cache.Add("num", 42)

	cache.lock.RLock()

	ce := cache.items["num"].Value.(*cacheEntry)
	cache.lock.RUnlock()

	if ce.compressed || ce.vtype != vtUnknown {
		t.Fatalf("expected unsupported type stored uncompressed with vtUnknown, got compressed=%v vtype=%v", ce.compressed, ce.vtype)
	}

	if v, ok := cache.Get("num"); !ok || v.(int) != 42 {
		t.Fatalf("expected to retrieve original int 42, got (%v,%v)", v, ok)
	}
}

func TestLRUCache_Keys_Empty(t *testing.T) {
	t.Parallel()

	cache, _ := NewLRUCache(1, false)
	if keys := cache.Keys(); len(keys) != 0 {
		t.Fatalf("expected empty Keys() on new cache, got %v", keys)
	}
}
