// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"bytes"
	"encoding/json"
)

// OptionalStrMap is a generic type that handles JSON responses that may return either
// a map object with string keysor an empty array.
//
// This type is necessary because the pixiv API can return [] instead of {}
// when no data is available.
//
// Example JSON responses that this type can handle:
//   - Valid map:   {"key": {"value": "data"}}
//   - Empty data:  []
type OptionalStrMap[V any] map[string]V

// UnmarshalJSON implements custom JSON unmarshaling for FlexibleMap.
//
// If an empty array is received, the map will be set to nil.
// Otherwise, the data will be unmarshaled as a standard map.
func (m *OptionalStrMap[V]) UnmarshalJSON(data []byte) error {
	// Handle empty array case
	if bytes.Equal(data, []byte("[]")) {
		*m = nil

		return nil
	}

	// Handle normal map case
	var nm map[string]V
	if err := json.Unmarshal(data, &nm); err != nil {
		return err
	}

	*m = OptionalStrMap[V](nm)

	return nil
}

// OptionalIntMap is a generic type that handles JSON responses that may return either
// a map object with integer keys or an empty array.
//
// This type is necessary because the pixiv APIs can return [] instead of {}
// when no data is available.
//
// Example JSON responses that this type can handle:
//   - Valid map:   {"123": {"value": "data"}}
//   - Empty data:  []
type OptionalIntMap[V any] map[int]V

// UnmarshalJSON implements custom JSON unmarshaling for OptionalIntMap.
//
// If an empty array is received, the map will be set to nil.
// Otherwise, the data will be unmarshaled as a standard map.
func (m *OptionalIntMap[V]) UnmarshalJSON(data []byte) error {
	// Handle empty array case
	if bytes.Equal(data, []byte("[]")) {
		*m = nil

		return nil
	}

	// Handle normal map case
	var nm map[int]V
	if err := json.Unmarshal(data, &nm); err != nil {
		return err
	}

	*m = OptionalIntMap[V](nm)

	return nil
}

// ExtractIDs is a method of OptionalIntMap to extract IDs and count from the map.
func (m OptionalIntMap[T]) ExtractIDs() ([]int, int) {
	ids := make([]int, 0, len(m))

	for k := range m {
		ids = append(ids, k)
	}

	return ids, len(m)
}
