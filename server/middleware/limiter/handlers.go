// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package limiter

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/server/routes"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// ChallengePage serves a dedicated challenge page (200 OK) for browsers.
// If a valid ping cookie already exists, it redirects back to the requested resource.
func ChallengePage(w http.ResponseWriter, r *http.Request) error {
	client, err := newClientInfo(r)
	if err != nil {
		return fmt.Errorf("failed to initialize client for challenge page: %w", err)
	}

	returnPath := utils.SanitizeReturnPath(utils.GetFormValue(r, returnPathFormat, "/"))

	// If the client already has a valid cookie, bounce them back immediately.
	if client.validatePingCookie(r) {
		log.Debug().
			Str("ip", client.ip.String()).
			Str("client_fingerprint", client.fingerprint).
			Str(returnPathFormat, returnPath).
			Msg("Client already verified, redirecting from challenge page")

		w.Header().Set("Cache-Control", "no-store")
		setVaryHeaders(w)
		http.Redirect(w, r, returnPath, http.StatusFound)

		return nil
	}

	// Serve the appropriate challenge page content with 200 OK
	w.Header().Set("Cache-Control", "no-store")
	setVaryHeaders(w)

	detectionMethod := config.Global.Limiter.DetectionMethod

	switch detectionMethod {
	case config.Turnstile:
		log.Info().
			Str("ip", client.ip.String()).
			Str("client_fingerprint", client.fingerprint).
			Str(returnPathFormat, returnPath).
			Str("method", string(detectionMethod)).
			Msg("Serving Turnstile challenge page")

		views.Challenge(views.ChallengePageData{
			Method:     config.Turnstile,
			Sitekey:    config.Global.Limiter.TurnstileSitekey,
			ReturnPath: returnPath,
		}).Render(r.Context(), w)

	case config.LinkToken:
		log.Info().
			Str("ip", client.ip.String()).
			Str("client_fingerprint", client.fingerprint).
			Str(returnPathFormat, returnPath).
			Str("method", string(detectionMethod)).
			Msg("Serving link token challenge page")

		token, err := GetOrCreateLinkToken(r)
		if err != nil {
			log.Err(err).
				Str("ip", client.ip.String()).
				Str("client_fingerprint", client.fingerprint).
				Msg("Failed to generate link token for challenge page")

			routes.BlockPage(w, routes.BlockData{Reason: "Failed to generate link token"}, http.StatusInternalServerError)

			return nil
		}

		views.Challenge(views.ChallengePageData{
			Method:     config.LinkToken,
			LinkToken:  token,
			ReturnPath: returnPath,
		}).Render(r.Context(), w)

	default:
		log.Warn().
			Str("ip", client.ip.String()).
			Str("client_fingerprint", client.fingerprint).
			Str(returnPathFormat, returnPath).
			Str("method", string(detectionMethod)).
			Msg("Unsupported detection method for challenge page")

		routes.BlockPage(w, routes.BlockData{Reason: "Unsupported detection method"}, http.StatusInternalServerError)
	}

	return nil
}

// TurnstileVerify handles the verification of the Turnstile token submitted by the client.
func TurnstileVerify(w http.ResponseWriter, r *http.Request) error {
	currentSiteKey := config.Global.Limiter.TurnstileSitekey

	client, err := newClientInfo(r)
	if err != nil {
		log.Err(err).Msg("Failed to initialize client for turnstile verification")
		setErrorRetargetHeaders(w)
		w.WriteHeader(http.StatusInternalServerError)

		return views.ChallengeArea(views.ChallengeAreaProps{
			Method:  config.Turnstile,
			Sitekey: currentSiteKey,
			Messages: []string{
				"Verification failed",
				"Your request could not be verified. Please refresh and try again.",
			},
			IsError:    true,
			ReturnPath: utils.SanitizeReturnPath(utils.GetFormValue(r, returnPathFormat, "/")),
		}).Render(r.Context(), w)
	}

	returnPath := utils.SanitizeReturnPath(utils.GetFormValue(r, returnPathFormat, "/"))

	if err := r.ParseForm(); err != nil {
		log.Warn().
			Err(err).
			Str("ip", client.ip.String()).
			Str("client_fingerprint", client.fingerprint).
			Msg("Failed to parse form")

		setErrorRetargetHeaders(w)
		w.WriteHeader(http.StatusBadRequest)

		return views.ChallengeArea(views.ChallengeAreaProps{
			Method:  config.Turnstile,
			Sitekey: currentSiteKey,
			Messages: []string{
				"Verification failed",
				"Your request could not be verified. Please refresh and try again.",
			},
			IsError:    true,
			ReturnPath: returnPath,
		}).Render(r.Context(), w)
	}

	turnstileToken := r.FormValue("cf-turnstile-response")
	if turnstileToken == "" {
		log.Warn().
			Str("ip", client.ip.String()).
			Str("client_fingerprint", client.fingerprint).
			Msg("Missing turnstile token in form")

		setErrorRetargetHeaders(w)
		w.WriteHeader(http.StatusBadRequest)

		return views.ChallengeArea(views.ChallengeAreaProps{
			Method:  config.Turnstile,
			Sitekey: currentSiteKey,
			Messages: []string{
				"Verification failed",
				"Your request could not be verified. Please refresh and try again.",
			},
			IsError:    true,
			ReturnPath: returnPath,
		}).Render(r.Context(), w)
	}

	clientIPStr := client.ip.String()

	verified, err := verifyTurnstileToken(r, turnstileToken, clientIPStr)
	if err != nil {
		log.Err(err).
			Str("ip", clientIPStr).
			Str("client_fingerprint", client.fingerprint).
			Msg("Failed to verify turnstile token with Cloudflare")

		setErrorRetargetHeaders(w)

		if errors.Is(err, errFailedToSendVerificationRequest) {
			w.WriteHeader(http.StatusServiceUnavailable)

			return views.ChallengeArea(views.ChallengeAreaProps{
				Method:  config.Turnstile,
				Sitekey: currentSiteKey,
				Messages: []string{
					"Verification error",
					"Could not reach the verification service. Please refresh and try again shortly.",
				},
				IsError:    true,
				ReturnPath: returnPath,
			}).Render(r.Context(), w)
		}

		w.WriteHeader(http.StatusInternalServerError)

		return views.ChallengeArea(views.ChallengeAreaProps{
			Method:  config.Turnstile,
			Sitekey: currentSiteKey,
			Messages: []string{
				"Verification failed",
				"Your request could not be verified. Please refresh and try again.",
			},
			IsError:    true,
			ReturnPath: returnPath,
		}).Render(r.Context(), w)
	}

	if !verified {
		log.Warn().
			Str("ip", clientIPStr).
			Str("client_fingerprint", client.fingerprint).
			Msg("Invalid turnstile token reported by Cloudflare")

		setErrorRetargetHeaders(w)
		w.WriteHeader(http.StatusBadRequest)

		return views.ChallengeArea(views.ChallengeAreaProps{
			Method:  config.Turnstile,
			Sitekey: currentSiteKey,
			Messages: []string{
				"Verification failed",
				"Your request could not be verified. Please refresh and try again.",
			},
			IsError:    true,
			ReturnPath: returnPath,
		}).Render(r.Context(), w)
	}

	http.SetCookie(w, createAccessTokenCookie(r))

	log.Info().
		Str("ip", clientIPStr).
		Str("client_fingerprint", client.fingerprint).
		Msg("Turnstile verification successful, ping cookie set")

	// After successful verification, redirect back to the original resource
	// so the browser reloads it with the new cookie.
	//
	// NOTE: The verification form uses hx-target="body" and hx-swap="innerHTML".
	// This ensures that the follow-up GET triggered by an HTTP 303 See Other will have its
	// response swapped into the entire <body>, and hx-push-url="true" updates the address bar.
	w.Header().Set("Cache-Control", "no-store")
	setVaryHeaders(w)
	http.Redirect(w, r, returnPath, http.StatusSeeOther)

	return nil
}

// LinkTokenChallenge handles the route that a client needs to request in order to pass a link token challenge.
//
// This is analogous to SearXNG's "/client{token}.css" endpoint in botdetection/link_token.py.
func LinkTokenChallenge(w http.ResponseWriter, r *http.Request) error {
	client, err := newClientInfo(r)
	if err != nil {
		return err
	}

	// If the client already has a valid ping cookie (but still received a challenge page for whatever reason),
	// just return HTTP 204.
	if client.validatePingCookie(r) {
		w.WriteHeader(http.StatusNoContent)

		return nil
	}

	// Extract link token from URL path (e.g., "/limiter/{token}" -> "{token}").
	tokenParam := r.PathValue("token")

	// Consume the link token for the client fingerprint.
	if !globalTokenStorage.consumeLinkToken(tokenParam, client.fingerprint) {
		log.Warn().
			Str("token", tokenParam).
			Msg("Invalid or expired token")
		w.WriteHeader(http.StatusNotFound)

		return fmt.Errorf("%w: %q", errInvalidOrExpiredToken, tokenParam)
	}

	// Valid token => create and set a signed cookie.
	http.SetCookie(w, createAccessTokenCookie(r))

	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)

	return nil
}
