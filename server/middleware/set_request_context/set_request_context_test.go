// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package set_request_context

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/megalamo/pixivfe/server/middleware"
	"github.com/megalamo/pixivfe/server/request_context"
)

// TestWithRequestContext_AttachesContext tests that request context is properly attached.
func TestWithRequestContext_AttachesContext(t *testing.T) {
	t.Parallel()

	var (
		requestID  string
		statusCode int
	)

	handler := middleware.Wrap(WithRequestContext, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := request_context.FromRequest(r)

		requestID = ctx.RequestID
		statusCode = ctx.StatusCode

		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if requestID == "" {
		t.Error("Expected request ID to be set")
	}

	if statusCode != http.StatusOK {
		t.Errorf("Expected status code %d in context, got %d", http.StatusOK, statusCode)
	}
}

// TestWithRequestContext_GeneratesUniqueRequestIDs tests that each request gets a unique ID.
func TestWithRequestContext_GeneratesUniqueRequestIDs(t *testing.T) {
	t.Parallel()

	var requestIDs []string

	handler := middleware.Wrap(WithRequestContext, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestIDs = append(requestIDs, request_context.FromRequest(r).RequestID)

		w.WriteHeader(http.StatusOK)
	}))

	for range 3 {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))
	}

	if len(requestIDs) != 3 {
		t.Fatalf("Expected 3 request IDs, got %d", len(requestIDs))
	}

	seen := make(map[string]bool)
	for _, id := range requestIDs {
		if seen[id] {
			t.Errorf("Duplicate request ID found: %s", id)
		}

		seen[id] = true
	}
}

// TestWithRequestContext_CallsNextHandler verifies that the next handler is called.
func TestWithRequestContext_CallsNextHandler(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	handler := middleware.Wrap(WithRequestContext, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true

		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))

	if !handlerCalled {
		t.Error("Expected next handler to be called")
	}
}

// TestWithRequestContext_PreservesRequestData tests that original request data is preserved.
func TestWithRequestContext_PreservesRequestData(t *testing.T) {
	t.Parallel()

	var (
		receivedMethod string
		receivedURL    string
	)

	handler := middleware.Wrap(WithRequestContext, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedURL = r.URL.Path

		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/test", nil))

	if receivedMethod != "POST" {
		t.Errorf("Expected method 'POST', got '%s'", receivedMethod)
	}

	if receivedURL != "/api/test" {
		t.Errorf("Expected URL '/api/test', got '%s'", receivedURL)
	}
}

// TestWithRequestContext_NoErrors verifies no error is set initially.
func TestWithRequestContext_NoErrors(t *testing.T) {
	t.Parallel()

	var requestError error

	handler := middleware.Wrap(WithRequestContext, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestError = request_context.FromRequest(r).RequestError

		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))

	if requestError != nil {
		t.Errorf("Expected no error in request context, got %v", requestError)
	}
}
