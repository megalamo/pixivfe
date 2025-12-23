package commondata

import (
	"net/http"
	"net/url"

	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// PageCommonData holds common variables accessible in templates and handlers.
//
// It is automatically populated for each request and attached to the
// requestcontext.RequestContext.
//
// Usage:
//
//	// In an HTTP handler:
//	rc := requestcontext.FromRequest(r)
//	cd := rc.CommonData
//	// Now you can access fields like cd.BaseURL, cd.LoggedIn, etc.
type PageCommonData struct {
	// BaseURL is the origin URL (scheme + host) of the current request.
	BaseURL string

	// CurrentPath is the URL path from request (e.g., "/users/123").
	CurrentPath string

	// CurrentPathWithParams is the full request URI including query parameters.
	CurrentPathWithParams string

	// HtmxCurrentPath is the path parsed from HX-Current-URL header for htmx requests.
	HtmxCurrentPath string

	// FullURL is the complete URL (scheme + host + path) of the request, not including query parameters.
	FullURL string

	// LoggedIn is true if user has a valid pixiv session token cookie.
	LoggedIn bool

	// Queries is the URL query parameters (first value only for each key).
	Queries map[string]string

	// CookieList is all PixivFE cookies as key-value map.
	CookieList map[cookie.CookieName]string

	// CookieListOrdered is the same cookies in defined order for consistent display.
	CookieListOrdered []struct {
		K cookie.CookieName
		V string
	}

	// IsHtmxRequest is true if request has an HX-Request header set to "true".
	IsHtmxRequest bool

	// IsFastRequest is true if request has a Fast-Request header set to "true".
	IsFastRequest bool

	// HX-Trigger header value, if present
	HXTrigger string

	// LinkToken is the generated CSS link token for bot detection (if limiter enabled).
	LinkToken string
}

// LinkTokenGenerator is the function signature of limiter.GetOrCreateLinkToken.
type LinkTokenGenerator func(*http.Request) (string, error)

// PopulatePageCommonData fills the PageCommonData struct from the request.
func PopulatePageCommonData(r *http.Request, data *PageCommonData, generateLinkToken LinkTokenGenerator) {
	data.BaseURL = utils.GetOriginFromRequest(r)
	data.CurrentPath = r.URL.Path
	data.CurrentPathWithParams = r.URL.RequestURI()
	data.FullURL = r.URL.Scheme + "://" + r.Host + r.URL.Path

	// Parse HX-Current-URL to get proper current path for async requests.
	if htmxCurrentURL := r.Header.Get("HX-Current-URL"); htmxCurrentURL != "" {
		if parsedURL, err := url.Parse(htmxCurrentURL); err == nil {
			data.HtmxCurrentPath = parsedURL.Path
		}
	}

	data.LoggedIn = untrusted.GetUserToken(r) != ""

	data.Queries = make(map[string]string)

	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			data.Queries[k] = v[0]
		}
	}

	data.CookieList = make(map[cookie.CookieName]string, len(cookie.AllCookieNames))
	data.CookieListOrdered = make([]struct {
		K cookie.CookieName
		V string
	}, 0, len(cookie.AllCookieNames))

	for _, name := range cookie.AllCookieNames {
		val := untrusted.GetCookie(r, name)

		data.CookieList[name] = val
		data.CookieListOrdered = append(data.CookieListOrdered, struct {
			K cookie.CookieName
			V string
		}{K: name, V: val})
	}

	data.IsHtmxRequest = r.Header.Get("HX-Request") == "true"
	data.IsFastRequest = r.Header.Get("Fast-Request") == "true"
	data.HXTrigger = r.Header.Get("HX-Trigger")

	if config.Global.Limiter.Enabled && generateLinkToken != nil {
		var err error

		data.LinkToken, err = generateLinkToken(r)
		if err != nil {
			log.Err(err).
				Msg("Failed to generate link token")

			data.LinkToken = ""
		}
	}
}
