package router

import (
	"github.com/megalamo/pixivfe/config"
	"github.com/megalamo/pixivfe/server/middleware"
	"github.com/megalamo/pixivfe/server/middleware/limiter"
	"github.com/megalamo/pixivfe/server/middleware/set_request_context"
)

func (router *Router) RegisterMiddleware() {
	// the first middleware is the most outer / first executed one
	router.Use(middleware.WithServerTiming)
	router.Use(middleware.NormalizeURL)                // handle trailing slashes and /en/ prefix removal
	router.Use(set_request_context.WithRequestContext) // needed for everything else
	router.Use(middleware.SetResponseHeaders)          // all pages need this

	if config.Global.Limiter.Enabled {
		limiter.Init()

		router.Use(limiter.Evaluate)
	}
}
