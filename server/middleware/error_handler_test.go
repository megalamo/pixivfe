// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/pixivfe/pixivfe/v3/server/request_context"
	"github.com/stretchr/testify/assert"
)

// createTestRequest creates a test HTTP request with request context.
func createTestRequest(t *testing.T) *http.Request {
	t.Helper()
	req := httptest.NewRequest("GET", "/test", nil)

	// Add request context
	ctx := request_context.WithRequestContext(req.Context(), req, func(*http.Request) (string, error) { return "test-token", nil })
	req = req.WithContext(ctx)

	return req
}

// TestCatchError_Success tests CatchError when handler succeeds.
func TestCatchError_Success(t *testing.T) {
	handler := CatchError(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
		return nil
	})
	req := createTestRequest(t)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if body := rr.Body.String(); body != `{"status": "success"}` {
		t.Errorf("Expected body %q, got %q", `{"status": "success"}`, body)
	}
	if ctx := request_context.FromRequest(req); ctx.RequestError != nil {
		t.Errorf("Expected no error in context, got %v", ctx.RequestError)
	}
}

// TestCatchError_HandlerError tests CatchError when handler returns an error.
func TestCatchError_HandlerError(t *testing.T) {
	testError := errors.New("test handler error")
	handler := CatchError(func(w http.ResponseWriter, r *http.Request) error {
		return testError
	})
	req := createTestRequest(t)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, rr.Result().StatusCode, 500, "expect 500 status code")

	ctx := request_context.FromRequest(req)
	if ctx.RequestError == nil || ctx.RequestError.Error() != testError.Error() {
		t.Errorf("Expected error %q in context, got %v", testError, ctx.RequestError)
	}
}
