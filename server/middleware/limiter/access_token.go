// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

/*
The PixivFE-Ping cookie is an identifier assigned to verified user agent + IP address.

When a browser successfully passes a [config.LimiterDetectionMethod],
this cookie is created and signed, which should then be attached to the client's future requests.

The cookie consists of a Unix timestamp and a client fingerprint.
*/
package limiter

import (
	"net/http"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/authenticated"
	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

const (
	expireAfterSeconds = 604800 // 7 days
	pasetoSubject      = "allowed access"
)

// local to ping cookie
var pasetoParser = paseto.MakeParser([]paseto.Rule{
	paseto.NotExpired(),
	paseto.Subject(pasetoSubject),
})

// createAccessTokenCookie generates a cookie that allows access to major server functionality
func createAccessTokenCookie(r *http.Request) *http.Cookie {
	client, err := newClientInfo(r)
	if err != nil {
		return nil
	}

	token := paseto.NewToken()
	token.SetExpiration(time.Now().Add(expireAfterSeconds * time.Second))
	token.SetSubject(pasetoSubject) // will be checked by the parser `pasetoParser`
	token.SetString("clientFingerprint", client.fingerprint)

	signedToken := token.V4Sign(config.PasetoValidator.SecretKey, []byte(authenticated.Implicit))

	return &http.Cookie{
		Name:     string(cookie.AccessCookie),
		Value:    signedToken,
		Path:     "/",
		MaxAge:   expireAfterSeconds,
		Secure:   utils.IsConnectionSecure(r),
		HttpOnly: true,
		SameSite: untrusted.CookieSameSite,
	}
}

// verifyAccessTokenCokie checks if a request is allowed access
func verifyAccessTokenCokie(cookie *http.Cookie, r *http.Request) bool {
	if cookie == nil {
		return false
	}

	token, err := pasetoParser.ParseV4Public(config.PasetoValidator.SecretKey.Public(), cookie.Value, []byte(authenticated.Implicit))
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Invalid token for ping cookie")

		return false
	}

	providedFingerprint, err := token.GetString("clientFingerprint")
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Invalid token for ping cookie")

		return false
	}

	// Create a client and compare the fingerprint generated with the one provided in the cookie
	client, err := newClientInfo(r)
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Cannot extract client info from request")

		return false
	}

	if providedFingerprint != client.fingerprint {
		log.Warn().
			Msg("Fingerprint mismatch for ping cookie")

		return false
	}

	return true
}
