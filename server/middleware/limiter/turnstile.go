// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package limiter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/audit"
	"codeberg.org/pixivfe/pixivfe/v3/server/request_context"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

const (
	returnPathFormat   = "return_path"
	siteverifyEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
)

var (
	errFailedToSendVerificationRequest    = errors.New("failed to send turnstile verification request")
	errFailedToReadResponseBody           = errors.New("failed to read turnstile verification response body")
	errFailedToParseJSONResponse          = errors.New("failed to parse turnstile verification JSON response")
	errFailedToMarshalTurnstileRequest    = errors.New("failed to marshal turnstile request payload")
	errFailedToCreateTurnstileHTTPRequest = errors.New("failed to create turnstile HTTP request")
)

// NOTE: ephemeral_id is unimplemented as it is Enterprise only.
type siteverifyResponse struct {
	Success     bool     `json:"success"`      // Whether the operation was successful or not
	ChallengeTS string   `json:"challenge_ts"` // ISO timestamp for the time the challenge was solved
	Hostname    string   `json:"hostname"`     // Hostname for which the challenge was served
	ErrorCodes  []string `json:"error-codes"`  // List of errors that occurred
	Action      string   `json:"action"`       // Customer widget identifier passed to the widget on the client side
	CData       string   `json:"cdata"`        // Customer data passed to the widget on the client side
}

// NOTE: idempotency_key is unimplemented as we don't retry failed requests.
type siteverifyRequest struct {
	Secret   string `json:"secret"`   // Widget's secret key
	Response string `json:"response"` // Response provided by the Turnstile client-side render
	RemoteIP string `json:"remoteip"` // Visitor's IP address
}

func verifyTurnstileToken(r *http.Request, token, remoteIP string) (_ bool, err error) {
	jsonData, err := json.Marshal(siteverifyRequest{
		Secret:   config.Global.Limiter.TurnstileSecretKey,
		Response: token,
		RemoteIP: remoteIP,
	})
	if err != nil {
		return false, fmt.Errorf("%w: %w", errFailedToMarshalTurnstileRequest, err)
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, siteverifyEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Errorf("%w: %w", errFailedToCreateTurnstileHTTPRequest, err)
	}

	req.Header.Set("Content-Type", "application/json")

	span := audit.Span{
		Destination: audit.ToTurnstile,
		RequestID:   request_context.FromContext(r.Context()).RequestID,
		Method:      req.Method,
		URL:         siteverifyEndpoint,
	}

	defer func() { span.Error = err }()

	_ = span.Begin(r.Context())
	defer span.End()

	resp, err := utils.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("%w: %w", errFailedToSendVerificationRequest, err)
	}
	defer resp.Body.Close()

	span.StatusCode = resp.StatusCode

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("%w: %w", errFailedToReadResponseBody, err)
	}

	span.Body = body

	var result siteverifyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("%w: %w", errFailedToParseJSONResponse, err)
	}

	span.End()
	span.Log()

	if !result.Success {
		log.Warn().
			Str("hostname", result.Hostname).
			Str("action", result.Action).
			Str("cdata", result.CData).
			Strs("error-codes", result.ErrorCodes).
			Msg("Turnstile verification failed per Cloudflare API")
	}

	return result.Success, nil
}

// setErrorRetargetHeaders ensures that when the verification form targets <body> (challenge page mode),
// error responses are retargeted to the smaller #challenge-area and use an outerHTML swap,
// preventing full-body replacement on errors.
func setErrorRetargetHeaders(w http.ResponseWriter) {
	w.Header().Set("HX-Retarget", "#challenge-area")
	w.Header().Set("HX-Reswap", "outerHTML show:none")
}

// setVaryHeaders ensures intermediaries vary on Cookie & Accept for challenge/redirect responses.
func setVaryHeaders(w http.ResponseWriter) {
	// Use Add to avoid overwriting any existing Vary directives.
	w.Header().Add("Vary", "Cookie")
	w.Header().Add("Vary", "Accept")
}
