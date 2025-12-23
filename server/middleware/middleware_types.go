package middleware

import "net/http"

type Middleware func(w http.ResponseWriter, r *http.Request, next http.Handler)

func Wrap(m Middleware, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m(w, r, next)
	}
}
