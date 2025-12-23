package middleware

import (
	"net/http"

	"github.com/mitchellh/go-server-timing"
)

func WithServerTiming(w http.ResponseWriter, r *http.Request, next http.Handler) {
	servertiming.Middleware(next, nil).ServeHTTP(w, r)
}
