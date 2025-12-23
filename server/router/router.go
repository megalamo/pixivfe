// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package router

import (
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/server/middleware"
)

// Router wraps http.ServeMux and provides middleware chaining functionality.
type Router struct {
	*http.ServeMux

	middlewares []middleware.Middleware
}

// NewRouter creates a new Router instance.
func NewRouter() *Router {
	return &Router{
		ServeMux: http.NewServeMux(),
	}
}

// Use adds a middleware to the router's chain.
func (router *Router) Use(middleware middleware.Middleware) {
	router.middlewares = append(router.middlewares, middleware)
}

// runs router.middlewares[i] and every thereafter
func (router *Router) serve(i int, w http.ResponseWriter, r *http.Request) {
	if i < len(router.middlewares) {
		router.middlewares[i](w, r, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			router.serve(i+1, w, r)
		}))
	} else {
		router.ServeMux.ServeHTTP(w, r)
	}
}

// runs all middleware
func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.serve(0, w, r)
}
