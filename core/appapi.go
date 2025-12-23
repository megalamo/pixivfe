// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

// Based on https://gist.github.com/ZipFile/c9ebedb224406f4f11845ab700124362
// Don't panic on any errors in production btw.

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

const (
	appAPIUserAgent    = "PixivAndroidApp/5.0.234 (Android 11; Pixel 5)"
	appAPIRedirectURI  = "https://app-api.pixiv.net/web/v1/users/auth/pixiv/callback"
	appAPILoginURL     = "https://app-api.pixiv.net/web/v1/login"
	appAPIAuthTokenURL = "https://oauth.secure.pixiv.net/auth/token" //#nosec:G101 - false positive
	appAPIClientID     = "MOBrBDS8blbauoSck0ZfDbtuzpyT"
	appAPIClientSecret = "lsACyCD94FhDUtGTXi3QzcFE2uU1hqtDaKeqrdwj" //#nosec:G101 - false positive

	// codeVerifierLength is the PKCE code verifier length (32 bytes = 256 bits of entropy).
	codeVerifierLength = 32
)

type appAPICredentials struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func AppAPIRefresh(r *http.Request, refreshToken string) appAPICredentials {
	var credentials appAPICredentials

	body := fmt.Appendf(nil, `client_id=%s&client_secret=%s&grant_type=refresh_token&include_policy=true&refresh_token=%s`, appAPIClientID, appAPIClientSecret, refreshToken)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, appAPIAuthTokenURL, bytes.NewBuffer(body))
	if err != nil {
		panic(err)
	}

	req.Header.Set("User-Agent", appAPIUserAgent)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := utils.HTTPClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(data, &credentials)
	if err != nil {
		panic(err)
	}

	return credentials
}

func AppAPILogin(r *http.Request) appAPICredentials {
	var (
		credentials appAPICredentials
		s           string
	)

	codeVerifier, codeChallenge := oauthPKCE()

	// Let users open this URL and log in. Then enter the code.
	// TODO: Change this.
	_, _ = io.WriteString(os.Stdout, fmt.Sprintf("%s?code_challenge=%s&code_challenge_method=S256&client=pixiv-android\n", appAPILoginURL, codeChallenge))
	_, _ = fmt.Scanln(&s)

	body := fmt.Appendf(nil, `client_id=%s&client_secret=%s&code=%s&code_verifier=%s&grant_type=authorization_code&include_policy=true&redirect_uri=%s`, appAPIClientID, appAPIClientSecret, s, codeVerifier, appAPIRedirectURI)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, appAPIAuthTokenURL, bytes.NewBuffer(body))
	if err != nil {
		panic(err)
	}

	req.Header.Set("User-Agent", appAPIUserAgent)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := utils.HTTPClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(data, &credentials)
	if err != nil {
		panic(err)
	}

	return credentials
}

// GenerateRandomBytes returns securely generated random bytes.
//
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRandomStringURLSafe returns a URL-safe, base64 encoded
// securely generated random string.
//
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func generateRandomStringURLSafe(n int) (string, error) {
	b, err := generateRandomBytes(n)

	return base64.URLEncoding.EncodeToString(b), err
}

func s256(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data)

	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func oauthPKCE() (string, string) {
	codeVerifier, err := generateRandomStringURLSafe(codeVerifierLength)
	if err != nil {
		panic(err)
	}

	codeVerifier = strings.Trim(codeVerifier, "=")

	codeChallenge := strings.Trim(s256([]byte(codeVerifier)), "=")

	return codeVerifier, codeChallenge
}
