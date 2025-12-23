// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package limiter

import (
	"net"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// IPv4 and IPv6 address lengths as measured in bits.
const (
	ipv4BitLength = 32
	ipv6BitLength = 128
)

// getClientIP extracts the client's IP address from an HTTP request with proxy awareness.
//
// Proxy headers (X-Forwarded-For, X-Real-IP) are only trusted when the connection
// comes from trusted sources (private/loopback networks).
func getClientIP(r *http.Request) string {
	// Extract IP from RemoteAddr by removing the port component.
	remoteIP := r.RemoteAddr
	if ip, _, err := net.SplitHostPort(remoteIP); err == nil {
		remoteIP = ip
	}

	// Only trust proxy headers if request comes from a trusted network.
	fromTrustedSource := false
	if ip := net.ParseIP(remoteIP); ip != nil {
		fromTrustedSource = ip.IsPrivate() || ip.IsLoopback()
	}

	if fromTrustedSource {
		// X-Real-IP takes precedence as it's typically the originating client IP
		// when set by a trusted proxy.
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}

		// If X-Real-IP isn't available, use the last IP in X-Forwarded-For.
		// This represents the client's IP in a chain of proxies.
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			parts := strings.Split(xff, ",")

			return strings.TrimSpace(parts[len(parts)-1])
		}
	} else {
		log.Warn().
			Str("remote_ip", remoteIP).
			Msg("Request from untrusted source, ignoring proxy headers")
	}

	// Fallback to the direct connection IP when proxy headers aren't available
	// or the source isn't trusted.
	if remoteIP != "" {
		return remoteIP
	}

	log.Error().
		Msg("Could not determine client IP")

	return ""
}

// ipMatchesList checks if an IP is within any of the provided CIDRs or matches them exactly.
func ipMatchesList(rawIP net.IP, cidrs []string) bool {
	ipStr := rawIP.String()

	for _, cidr := range cidrs {
		// Check for an exact match first.
		if ipStr == cidr {
			return true
		}

		// Try parsing as CIDR and check if IP is within subnet.
		_, subnet, err := net.ParseCIDR(cidr)
		if err == nil && subnet.Contains(rawIP) {
			return true
		}
	}

	// No match found in any CIDR or exact comparison.
	return false
}

func getNetwork(rawIP net.IP, ipv4Prefix, ipv6Prefix int) *net.IPNet {
	// Create mask based on IP version and configured prefix.
	var mask net.IPMask
	if rawIP.To4() != nil {
		mask = net.CIDRMask(ipv4Prefix, ipv4BitLength) // IPv4.
	} else {
		mask = net.CIDRMask(ipv6Prefix, ipv6BitLength) // IPv6.
	}

	// Create network with the IP and determined mask.
	return &net.IPNet{
		IP:   rawIP.Mask(mask),
		Mask: mask,
	}
}
