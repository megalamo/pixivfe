// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"encoding/json"
	"net/http"
)

type BlockData struct {
	Reason string `json:"reason"`
}

// BlockPage writes a block page as a JSON response.
//
// It sets the appropriate headers, writes the given HTTP status code,
// and then writes the JSON body.
func BlockPage(w http.ResponseWriter, data BlockData, statusCode int) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(data)
}
