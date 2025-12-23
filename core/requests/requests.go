// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package requests

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/audit"
	"codeberg.org/pixivfe/pixivfe/v3/core/idgen"
	"codeberg.org/pixivfe/pixivfe/v3/core/tokenmanager"
	"codeberg.org/pixivfe/pixivfe/v3/server/request_context"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

const (
	// RandomToken uses a randomly generated token.
	RandomToken = "RandomToken"

	// NoToken doesn't set PHPSESSID at all.
	NoToken = "NoToken"
)

var (
	errInvalidJSON              = errors.New("response contained invalid JSON")
	errAPIResponseError         = errors.New("API response indicated error")
	errMissingRequiredPHPSESSID = errors.New("PHPSESSID cookie is required for POST requests")
	errUnsupportedPayloadType   = errors.New("unsupported payload type")
	errProxyBaseURLInvalid      = errors.New("proxy baseURL should end in /")
)

// APIError represents an error returned from the pixiv API or internal request handling.
type APIError struct {
	// StatusCode is the HTTP status code from the response.
	// Always >= 400 for API errors.
	StatusCode int

	// Message contains the error message from the API response.
	// Empty for internal request errors, populated for API errors.
	Message string

	// Err is the underlying error cause.
	// Set to errAPIResponseError for API errors, or the original error for internal failures.
	Err error
}

// Error returns a formatted error message including the status code and API message if available.
func (e *APIError) Error() string {
	var b strings.Builder

	b.WriteString(e.Err.Error())

	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}

	b.WriteString(fmt.Sprintf(" (status code: %d)", e.StatusCode))

	return b.String()
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *APIError) Unwrap() error {
	return e.Err
}

// GetJSONBody makes a GET request and extracts the JSON payload from the response.
//
// For standard API responses, it returns the content of the `body` field. For non-standard
// JSON responses (like from `/rpc/cps.php`) that do not contain an `error` or `body`
// field, it gracefully returns the entire response as the payload.
//
// Returns an error if:
//   - The request fails
//   - The response contains invalid JSON
//   - The "error" field is a boolean true
func GetJSONBody(
	ctx context.Context,
	url string,
	cookies map[string]string,
	incomingHeaders http.Header,
) ([]byte, error) {
	opts := RequestOptions{
		Method:          http.MethodGet,
		URL:             url,
		Cookies:         cookies,
		IncomingHeaders: incomingHeaders,
	}

	respBody, err := do(ctx, opts)
	if err != nil {
		return nil, err
	}

	return processJSONResponse(respBody)
}

// Get performs a GET request and wraps the returned response.Body in a bytes.Reader.
func Get(
	ctx context.Context,
	url string,
	cookies map[string]string,
	incomingHeaders http.Header,
) (io.Reader, error) {
	response, err := do(ctx, RequestOptions{
		Method:          http.MethodGet,
		URL:             url,
		Cookies:         cookies,
		IncomingHeaders: incomingHeaders,
	})
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(response), nil
}

// PostJSONBody performs a POST request and extracts the JSON payload from the response.
//
// For standard API responses, it returns the content of the `body` field. It handles API-level
// errors where the HTTP status is 200 OK but the JSON payload contains `{"error": true}`.
// For streaming, use Perform() instead.
//
// Returns an error if:
//   - The request fails (e.g., network error, non-2xx status code).
//   - The response contains invalid JSON.
//   - The "error" field in the JSON response is `true`.
func PostJSONBody(
	ctx context.Context,
	url string,
	payload any,
	cookies map[string]string,
	csrf string,
	contentType string,
	incomingHeaders http.Header,
) ([]byte, error) {
	// External packages must provide their own PHPSESSID cookie.
	if cookies == nil || cookies["PHPSESSID"] == "" {
		return nil, errMissingRequiredPHPSESSID
	}

	respBody, err := do(ctx, RequestOptions{
		Method:          http.MethodPost,
		URL:             url,
		Payload:         payload,
		Cookies:         cookies,
		CSRF:            csrf,
		ContentType:     contentType,
		IncomingHeaders: incomingHeaders,
	})
	if err != nil {
		return nil, err
	}

	return processJSONResponse(respBody)
}

// Do sends an HTTP request and returns the raw *http.Response and the response body as a byte slice.
//
// This function handles the full lifecycle of an HTTP request, including caching, authentication,
// and logging. It returns the raw *http.Response and the response body as a slice of bytes.
//
// The `Body` field of the returned `*http.Response` is a `NopCloser` over these same bytes
// for convenience, but callers should prefer using the byte slice directly.
//
// This function does not check for non-OK status codes, leaving that task to the caller.
func Do(ctx context.Context, opts RequestOptions) (*http.Response, []byte, error) {
	tokenManager := tokenmanager.DefaultTokenManager
	userToken := opts.Cookies["PHPSESSID"]

	// For GET requests, determine cache policy and check for a cached response.
	var cachePolicy cachePolicy
	if opts.Method == http.MethodGet {
		cachePolicy = determineCachePolicy(opts.URL, userToken, opts.IncomingHeaders)
		if cachePolicy.cachedItem != nil {
			// A valid cached item was found. Construct a response and return it with the body bytes.
			item := cachePolicy.cachedItem

			return &http.Response{
				StatusCode: item.StatusCode,
				Header:     item.Header.Clone(),
				Body:       io.NopCloser(bytes.NewReader(item.Body)),
			}, item.Body, nil
		}
	}

	token, err := retrieveToken(tokenManager, userToken)
	if err != nil {
		return nil, nil, err
	}

	// Create a request object.
	req, err := newRequest(ctx, opts, token)
	if err != nil {
		return nil, nil, err
	}

	// Perform the request.
	resp, bodyBytes, err := sendRequest(ctx, req)
	if err != nil {
		// If making the request itself failed, don't mark the token as timed out.
		// Return nil for the body bytes.
		return nil, nil, err
	}

	// Handle token status based on the response
	if resp.StatusCode == http.StatusOK {
		tokenManager.MarkTokenStatus(token, tokenmanager.Good)
	} else {
		// Mark the token as timed out if the request succeeded but returned a non-OK response
		tokenManager.MarkTokenStatus(token, tokenmanager.TimedOut)
	}

	// Cache the response if it's a successful GET request and the policy allows it.
	// The cachePolicy was determined before the request was made.
	if opts.Method == http.MethodGet && resp.StatusCode == http.StatusOK && cachePolicy.shouldUseCache {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(cachedItem{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       bodyBytes,
			ExpiresAt:  time.Now().Add(config.Global.Cache.TTL),
			URL:        opts.URL,
		}); err != nil {
			// Log the error but don't fail the request.
			log.Ctx(ctx).Warn().Err(err).Msg("Failed to serialize item for cache")
		} else {
			cache.Add(
				generateCacheKey(opts.URL, userToken),
				buf.Bytes(),
			)
		}
	}

	return resp, bodyBytes, nil
}

// ProxyHandler proxies a request to the specified base URL.
//
// NOTE: We intentionally don't copy headers from the response.
func ProxyHandler(w http.ResponseWriter, r *http.Request, baseURL string) error {
	base, err := url.Parse(baseURL)
	if err != nil {
		// This should not happen if baseURL is configured correctly.
		// We treat it as an internal server error.
		return fmt.Errorf("proxy baseURL is not valid URL: %w", err)
	}

	if !strings.HasSuffix(base.Path, "/") {
		return fmt.Errorf("%w: %s", errProxyBaseURLInvalid, baseURL)
	}

	// Create the target URL by resolving the request's path and query against the base URL.
	// r.URL on a server request has the path and query for the incoming request.
	targetURL := base.ResolveReference(r.URL).String()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		if isContextCanceled(err) {
			return nil
		}

		return fmt.Errorf("failed to create request for %s: %w", targetURL, err)
	}

	req.Header.Add("User-Agent", config.GetRandomUserAgent())
	req.Header.Add("Referer", "https://www.pixiv.net/")

	//nolint:bodyclose // sendRequest closes the original body and returns a NopCloser.
	resp, bodyBytes, err := sendRequest(r.Context(), req)
	if err != nil {
		if isContextCanceled(err) {
			return nil
		}

		// makeRequest will have closed the body on error
		return fmt.Errorf("failed to proxy request to %s: %w", targetURL, err)
	}
	// The body from makeRequest is already closed, we just use the bytes.

	w.WriteHeader(resp.StatusCode)

	if _, err := w.Write(bodyBytes); err != nil {
		return fmt.Errorf("failed to write response body: %w", err)
	}

	return nil
}

// do performs a request using the given options, receives the already-read response body,
// and handles standard API error responses.
// It returns the raw body on success.
func do(ctx context.Context, opts RequestOptions) ([]byte, error) {
	resp, body, err := Do(ctx, opts)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		// Attempt to extract an error message from the JSON body.
		message := gjson.GetBytes(body, "message").String()

		// Fall back to the HTTP status text if no JSON message is found.
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}

		// As a final fallback for unknown status codes, use a generic error message.
		if message == "" {
			message = "An unknown API error occurred"
		}

		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    message,
			Err:        errAPIResponseError,
		}
	}

	return body, nil
}

// processJSONResponse parses a raw JSON response body from the Pixiv API.
//
// For standard API responses, it returns the content of the `body` field. For non-standard
// JSON responses (like from `/rpc/cps.php`) that do not contain an `error` or `body`
// field, it gracefully returns the entire response as the payload.
//
// It returns an error if the JSON is invalid or if the payload contains `"error": true`.
func processJSONResponse(respBody []byte) ([]byte, error) {
	if !gjson.Valid(string(respBody)) {
		return nil, fmt.Errorf("%w: %s", errInvalidJSON, string(respBody))
	}

	result := gjson.ParseBytes(respBody)

	if result.Get("error").Bool() {
		message := result.Get("message").String()
		if message == "" {
			message = "API response contained an error with no message"
		}

		return nil, fmt.Errorf("%w: %s", errAPIResponseError, message)
	}

	body := result.Get("body")

	if !body.Exists() {
		// If the "body" field does not exist and there was no "error",
		// assume the entire response is the payload. This handles endpoints
		// like /rpc/cps.php that have a different structure.
		return respBody, nil
	}

	return []byte(body.Raw), nil
}

// newRequest constructs an *http.Request from RequestOptions.
func newRequest(ctx context.Context, opts RequestOptions, token *tokenmanager.Token) (*http.Request, error) {
	var (
		reqBody           io.Reader
		contentTypeHeader string
	)

	if opts.Method == http.MethodPost {
		switch v := opts.Payload.(type) {
		case string:
			reqBody = bytes.NewBufferString(v)
			contentTypeHeader = opts.ContentType
		case map[string]string:
			body, formContentType, err := createMultipartFormData(v)
			if err != nil {
				return nil, err
			}

			reqBody = body
			contentTypeHeader = formContentType
		default:
			return nil, errUnsupportedPayloadType
		}
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.URL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("User-Agent", config.GetRandomUserAgent())
	req.Header.Add("Accept-Language", config.Global.Request.AcceptLanguage)

	// Consolidate and set cookies, with managed token values taking precedence.
	finalCookies := make(map[string]string)
	maps.Copy(finalCookies, opts.Cookies)

	// Override with token-specific cookies.
	for name, value := range map[string]string{
		"yuid_b":    token.YUIDB,
		"p_ab_d_id": token.PAbDID,
		"p_ab_id":   token.PAbID,
		"p_ab_id_2": token.PAbID2,
	} {
		if value != "" {
			finalCookies[name] = value
		}
	}

	// The managed PHPSESSID token takes ultimate precedence.
	if token.Value != NoToken {
		finalCookies["PHPSESSID"] = token.Value
	} else {
		// If NoToken is specified, ensure no PHPSESSID is sent.
		delete(finalCookies, "PHPSESSID")
	}

	for name, value := range finalCookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	if opts.Method == http.MethodPost {
		req.Header.Add("X-Csrf-Token", opts.CSRF)

		if contentTypeHeader != "" {
			req.Header.Set("Content-Type", contentTypeHeader)
		}
	}

	// The /rpc/cps.php endpoint requires a Referer header
	req.Header.Set("Referer", "https://www.pixiv.net/")

	return req, nil
}

// sendRequest executes the HTTP request, reads the body for auditing, and returns the response
// with a new, readable body stream, along with the raw body bytes.
func sendRequest(
	ctx context.Context,
	req *http.Request,
) (_ *http.Response, _ []byte, err error) {
	span := audit.Span{
		Destination: audit.ToPixiv,
		RequestID:   request_context.FromContext(ctx).RequestID + "-" + idgen.Make(),
		Method:      req.Method,
		URL:         req.URL.String(),
	}

	defer func() { span.Error = err }()

	_ = span.Begin(ctx)
	defer span.End() // in case of error

	resp, err := utils.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	span.StatusCode = resp.StatusCode

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	span.Body = body

	span.End()
	span.Log()

	// Replace the consumed body with a new reader so the caller can still read it.
	resp.Body = io.NopCloser(bytes.NewReader(body))

	return resp, body, nil
}

// retrieveToken obtains a valid token for the request.
func retrieveToken(tokenManager *tokenmanager.TokenManager, userToken string) (*tokenmanager.Token, error) {
	// If a specific token (e.g. from user cookies) is provided, use it.
	if userToken != "" && userToken != RandomToken {
		return &tokenmanager.Token{Value: userToken}, nil
	}

	if userToken == NoToken {
		return tokenmanager.CreateRandomToken(), nil
	}

	// Otherwise, get a token from the pool.
	token := tokenManager.GetToken()
	if token == nil {
		tokenManager.ResetAllTokens()

		return nil, fmt.Errorf(
			`All tokens (%d) are timed out, resetting all tokens to their initial good state.
Consider providing additional tokens in PIXIVFE_TOKEN or reviewing token management configuration.
Please refer the following documentation for additional information:
- https://pixivfe-docs.pages.dev/hosting/api-authentication/`,
			len(config.Global.Basic.Token),
		)
	}

	return token, nil
}

// createMultipartFormData constructs multipart form data from a map of fields.
//
// It is used to prepare data for POST requests that require multipart encoding.
func createMultipartFormData(fields map[string]string) (*bytes.Buffer, string, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			_ = writer.Close()

			return nil, "", fmt.Errorf("failed to write multipart form field %q: %w", k, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return body, writer.FormDataContentType(), nil
}

// isContextCanceled returns true if the error is due to context cancellation or deadline exceeded.
// In these cases, we should simply stop processing and return, as the client has disconnected.
func isContextCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
