// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"

	"github.com/rs/zerolog/log"

	"github.com/megalamo/pixivfe/assets/views"
	"github.com/megalamo/pixivfe/config"
	"github.com/megalamo/pixivfe/core/audit"
	"github.com/megalamo/pixivfe/server/request_context"
	"github.com/megalamo/pixivfe/server/routes"
)

// CatchError wraps HTTP handlers that return an error, providing centralized error handling,
// response buffering, and request logging.
//
// It operates as follows:
//  1. It times the request for logging purposes.
//  2. It wraps the execution of the given handler, which has the signature
//     `func(w http.ResponseWriter, r *http.Request) error`. The handler's
//     output is buffered using an httptest.ResponseRecorder.
//  3. Any error returned by the handler is stored in the request context.
//
// After the handler runs, it decides on the final response:
//   - If the handler returns a `routes.ErrUnauthorized`, the middleware renders
//     a 401 Unauthorized page prompting the user to log in.
//   - If the handler returns any other error without writing an HTTP error status
//     code (i.e., status < 400), it's treated as an unhandled internal error.
//     The buffered response is discarded, and a generic 500 Internal Server Error
//     page is rendered.
//   - If the handler wrote a 404 Not Found status, the buffered response is
//     also discarded and replaced with the themed generic error page.
//   - In all other cases (e.g., a successful response), the buffered response
//     is written to the client.
//
// Finally, it logs the completed request details (status, duration, error, etc.)
// via the audit package.
func CatchError(handler func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := request_context.FromRequest(r)

		span := audit.Span{
			Destination: audit.ToUser,
			RequestID:   ctx.RequestID,
			Method:      r.Method,
			URL:         r.URL.String(),
		}

		_ = span.Begin(r.Context())
		defer span.End()

		recorder := httptest.NewRecorder()

		// Execute the handler, capturing its output and any returned error.
		err := handler(recorder, r)

		ctx.RequestError = err

		var unauthErr *routes.UnauthorizedError
		switch {
		case errors.As(ctx.RequestError, &unauthErr):
			// A handler signaled that the user must be authenticated.
			// Discard the recorder's content and render the unauthorized page.
			ctx.StatusCode = http.StatusUnauthorized

			w.Header().Add("HX-Push-Url", "/unauthorized")
			w.WriteHeader(ctx.StatusCode)

			pageData := views.UnauthorizedData{
				Title:            "Unauthorized",
				NoAuthReturnPath: unauthErr.NoAuthReturnPath,
				LoginReturnPath:  unauthErr.LoginReturnPath,
			}

			// Render the page. If rendering fails, log it. The original error will still be logged by the audit span.
			if renderErr := views.Unauthorized(pageData).Render(r.Context(), w); renderErr != nil {
				log.Err(renderErr).
					Str("original_error", ctx.RequestError.Error()).
					Msg("Failed to render the unauthorized page after an authorization error")
			}

		case (ctx.RequestError != nil && recorder.Code < http.StatusBadRequest) || (recorder.Code == http.StatusNotFound):
			// An unhandled error or a 404 occurred. Discard the recorder's contents
			// and render our generic error page.
			if recorder.Code == http.StatusNotFound {
				ctx.StatusCode = http.StatusNotFound
			} else {
				// For any other error caught by this logic, it's an internal server error.
				ctx.StatusCode = http.StatusInternalServerError
			}

			w.WriteHeader(ctx.StatusCode)
			routes.ErrorPage(w, r) // ErrorPage uses ctx.RequestError and ctx.StatusCode

		default:
			// This is a successful response or a handled error. We trust the recorder's output.
			if recorder.Code == 0 {
				recorder.Code = http.StatusOK
			}

			ctx.StatusCode = recorder.Code // Ensure ctx.StatusCode reflects the actual code for logging.
			maps.Copy(w.Header(), recorder.Header())
			w.WriteHeader(recorder.Code)

			if _, err := recorder.Body.WriteTo(w); err != nil {
				log.Err(err).Msg("Failed to write response body")
			}
		}

		span.StatusCode = ctx.StatusCode
		span.Error = ctx.RequestError

		// Log the application response if not excluded.
		if !config.Global.ShouldSkipServerLogging(r.URL.Path) {
			span.Log()
		}
	}
}
