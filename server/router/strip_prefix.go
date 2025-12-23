package router

import (
	"net/http"
	"net/url"
	"strings"
)

type FallibleHandler = func(w http.ResponseWriter, r *http.Request) error

func StripPrefix(prefix string, h FallibleHandler) FallibleHandler {
	if prefix == "" {
		return h
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		p := strings.TrimPrefix(r.URL.Path, prefix)

		rp := strings.TrimPrefix(r.URL.RawPath, prefix)
		if len(p) < len(r.URL.Path) && (r.URL.RawPath == "" || len(rp) < len(r.URL.RawPath)) {
			r2 := new(http.Request)

			*r2 = *r
			r2.URL = new(url.URL)
			*r2.URL = *r.URL
			r2.URL.Path = p
			r2.URL.RawPath = rp

			return h(w, r2)
		} else {
			http.NotFound(w, r)

			return nil
		}
	}
}
