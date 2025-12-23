// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLegacyRedirect(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	expectedStatusCode := http.StatusPermanentRedirect
	expectedLocation := "/artworks/12345678"

	redirectWithQueryParam("/artworks/", "illust_id").ServeHTTP(
		rr,
		httptest.NewRequest(http.MethodGet, "/member_illust.php?illust_id=12345678", nil))

	if rr.Code != expectedStatusCode {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, expectedStatusCode)
	}

	location := rr.Header().Get("Location")
	if location != expectedLocation {
		t.Errorf("handler returned wrong Location header: got %q want %q", location, expectedLocation)
	}
}
