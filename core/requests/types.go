// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package requests

import (
	"net/http"
)

// RequestOptions are parameters for handleRequest.
type RequestOptions struct {
	Method          string
	URL             string
	Cookies         map[string]string
	IncomingHeaders http.Header
	Payload         any
	CSRF            string
	ContentType     string
}
