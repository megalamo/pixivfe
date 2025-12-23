// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
Package requestcontext provides per-request state management for HTTP handlers.

This package is separate because Go disallows a cyclic import graph.
*/
package request_context

import (
	"context"
	"net/http"

	"golang.org/x/text/language"

	"codeberg.org/pixivfe/pixivfe/v3/core/idgen"
	"codeberg.org/pixivfe/pixivfe/v3/i18n"
	"codeberg.org/pixivfe/pixivfe/v3/server/template/commondata"
)

// RequestContext carries request-scoped data through the middleware chain.
//
// This data survives the entire lifetime of a single HTTP request and is safe
// for concurrent access from multiple goroutines handling the same request.
type RequestContext struct {
	// RequestID is an identifier for tracing requests.
	RequestID string

	// Holds any critical error encountered during request processing.
	//
	// Automatically populated by middleware.CatchError when handlers return errors,
	// which interrupts normal response handling and renders an error page instead.
	RequestError error

	// HTTP status code to be sent in the response. Defaults to 200 OK.
	StatusCode int

	CommonData commondata.PageCommonData

	T language.Tag
}

// requestContextKeyType defines a unique type for a RequestContext key.
type requestContextKeyType struct{}

// requestContextKey is a unique key used to access RequestContext
// values from a context.Context.
var requestContextKey = requestContextKeyType{}

// WithRequestContext initializes a new request context and attaches it to
// the parent context.
//
// This is called once per request, first in the middleware chain (see main.go).
func WithRequestContext(ctx context.Context, r *http.Request, generateToken commondata.LinkTokenGenerator) context.Context {
	ctx = i18n.WithRequest(ctx, r)

	rc := RequestContext{
		RequestID:  idgen.Make(),
		StatusCode: http.StatusOK,
		T:          i18n.TagFrom(ctx),
	}
	commondata.PopulatePageCommonData(r, &rc.CommonData, generateToken)

	return context.WithValue(ctx, requestContextKey, &rc)
}

// FromContext extracts the RequestContext from a context, always returning
// a valid pointer.
//
// If no context is found, returns a zero-value instance.
func FromContext(ctx context.Context) *RequestContext {
	if v := ctx.Value(requestContextKey); v != nil {
		if rc, ok := v.(*RequestContext); ok {
			return rc
		}
	}

	// Always return a zero-value instance with an empty PageCommonData.
	return &RequestContext{CommonData: commondata.PageCommonData{}}
}

// FromRequest is a convenience wrapper for extracting RequestContext
// directly from HTTP requests.
//
// Prefer this in handlers that have access to the *http.Request object.
func FromRequest(r *http.Request) *RequestContext {
	return FromContext(r.Context())
}
