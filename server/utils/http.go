// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package utils

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
)

const (
	// clientSessionCacheSize defines the size of the TLS session cache.
	clientSessionCacheSize = 20

	// maxIdleConnsPerHost defines maximum idle connections to keep per host.
	maxIdleConnsPerHost = 20

	// bufferSize defines the read and write buffer size in bytes (32KB).
	bufferSize = 32 * 1024
)

// HTTPClient is a pre-configured http.Client.
var HTTPClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			ClientSessionCache: tls.NewLRUClientSessionCache(clientSessionCacheSize),
			MinVersion:         tls.VersionTLS12,
		},
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: maxIdleConnsPerHost,
		WriteBufferSize:     bufferSize,
		ReadBufferSize:      bufferSize,
	},
}

// IsConnectionSecure returns whether a connection is secure.
//
// Target environments are (containerized and bare metal):
//   - Internet -> reverse proxy (e.g. cloudflare) -> reverse proxy -> application
//   - Internet -> reverse proxy -> application
//   - LAN -> reverse proxy -> application
//   - LAN -> application
//   - localhost -> application
//
// This function will incorrectly return false if the last reverse proxy
// in the chain has a public IP address, but this is expected to be a small minority
// of deployments.
func IsConnectionSecure(r *http.Request) bool {
	// Always secure if directly using TLS
	if r.TLS != nil {
		return true
	}

	// Parse IP from RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false // Can't determine if it's secure
	}

	parsedIP := net.ParseIP(host)
	if parsedIP == nil {
		return false // Invalid IP
	}

	// Only trust X-Forwarded-Proto from private IPs
	if parsedIP.IsPrivate() && r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}

	return false
}

// RedirectToWhenceYouCame redirects the user back to the referring page if it's from the same origin.
//
// This helps prevent open redirects by checking the referrer against the current origin.
// If the referrer is not from the same origin, it responds with a 200 OK status.
//
// returnPath  Return to this URL. If empty, return to the referrer.
//
// Deprecated: Using this function breaks compatibility with instances that set `Referrer-Policy: no-referrer`.
func RedirectToWhenceYouCame(w http.ResponseWriter, r *http.Request, returnPath string) {
	if returnPath == "" {
		referrer := r.Referer()
		if strings.HasPrefix(referrer, GetOriginFromRequest(r)) {
			returnPath = referrer
		}
	}

	if returnPath == "" {
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, returnPath, http.StatusSeeOther)
	}
}
