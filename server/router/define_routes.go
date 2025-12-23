package router

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"runtime/trace"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/server/assets"
	"codeberg.org/pixivfe/pixivfe/v3/server/middleware"
	"codeberg.org/pixivfe/pixivfe/v3/server/middleware/limiter"
	"codeberg.org/pixivfe/pixivfe/v3/server/routes"
)

// DefineRoutes sets up all the routes for the application using our custom Router.
//
// It returns a *Router without middleware.
func (router *Router) DefineRoutes() {
	fileServerHandler := fileServer()

	// Serve specific files from the root of the 'assets' subdirectory.
	router.Handle("GET /manifest.json", fileServerHandler)
	router.Handle("GET /robots.txt", fileServerHandler)

	// Serve files from subdirectories within 'assets'.
	// Patterns ending in "/" are prefix matches.
	router.Handle("GET /img/", fileServerHandler)
	router.Handle("GET /css/", fileServerHandler)
	router.Handle("GET /js/", fileServerHandler)
	router.Handle("GET /fonts/", fileServerHandler)

	// Proxy routes
	router.Handle("GET /proxy/i.pximg.net/", middleware.CatchError(StripPrefix("/proxy/i.pximg.net/", routes.IPximgProxy)))
	router.Handle("GET /proxy/booth.pximg.net/", middleware.CatchError(StripPrefix("/proxy/booth.pximg.net/", routes.BoothPximgProxy)))
	router.Handle("GET /proxy/embed.pixiv.net/", middleware.CatchError(StripPrefix("/proxy/embed.pixiv.net/", routes.EmbedPixivProxy)))
	router.Handle("GET /proxy/s.pximg.net/", middleware.CatchError(StripPrefix("/proxy/s.pximg.net/", routes.SPximgProxy)))
	router.Handle("GET /proxy/source.pixiv.net/", middleware.CatchError(StripPrefix("/proxy/source.pixiv.net/", routes.SourcePixivProxy)))
	// We include a /ugoira/ segment for compatibility with external ugoira proxies that
	// can only reverse proxy the t-hk.ugoira.com domain directly (e.g. caddy)
	router.Handle("GET /proxy/ugoira.com/ugoira/", middleware.CatchError(StripPrefix("/proxy/ugoira.com/ugoira/", routes.UgoiraProxy)))

	// About routes
	router.HandleFunc("GET /about", middleware.CatchError(routes.AboutPage))

	// Newest routes
	router.HandleFunc("GET /newest", middleware.CatchError(routes.NewestPage))

	// Discovery routes
	router.HandleFunc("GET /discovery", middleware.CatchError(routes.DiscoveryPage))
	router.HandleFunc("POST /discovery", middleware.CatchError(routes.DiscoveryPageRefresh))
	router.HandleFunc("GET /discovery/novel", middleware.CatchError(routes.NovelDiscoveryPage))
	router.HandleFunc("POST /discovery/novel", middleware.CatchError(routes.NovelDiscoveryPageRefresh))
	router.HandleFunc("GET /discovery/users", middleware.CatchError(routes.UserDiscoveryPage))
	router.HandleFunc("POST /discovery/users", middleware.CatchError(routes.UserDiscoveryPageRefresh))

	// Ranking routes
	router.HandleFunc("GET /ranking", middleware.CatchError(routes.RankingPage))
	router.HandleFunc("GET /rankingCalendar", middleware.CatchError(routes.RankingCalendarPage))

	// User routes
	router.HandleFunc("GET /users/{id}", middleware.CatchError(routes.UserPage))
	router.HandleFunc("GET /users/{id}/atom.xml", middleware.CatchError(routes.UserAtomFeed))
	router.HandleFunc("/member.php", redirectWithQueryParam("/users/", "id"))

	// Artwork routes
	router.HandleFunc("GET /artworks/{id}", middleware.CatchError(routes.ArtworkPage))
	router.HandleFunc("/member_illust.php", redirectWithQueryParam("/artworks/", "illust_id"))

	// Manga series routes
	router.HandleFunc("GET /users/{user_id}/series/{series_id}", middleware.CatchError(routes.MangaSeriesPage))

	// Novel routes
	router.HandleFunc("GET /novel/{id}", middleware.CatchError(routes.NovelPage))
	router.HandleFunc("GET /novel/series/{id}", middleware.CatchError(routes.NovelSeriesPage))
	router.HandleFunc("GET /novel/show.php", redirectWithQueryParam("/novel/", "id"))

	// Pixivision routes
	router.HandleFunc("GET /pixivision", middleware.CatchError(routes.PixivisionHomePage))
	router.HandleFunc("GET /pixivision/a/{id}", middleware.CatchError(routes.PixivisionArticlePage))
	router.HandleFunc("GET /pixivision/c/{id}", middleware.CatchError(routes.PixivisionCategoryPage))
	router.HandleFunc("GET /pixivision/t/{id}", middleware.CatchError(routes.PixivisionTagPage))

	// Settings routes
	router.HandleFunc("GET /settings", middleware.CatchError(routes.SettingsPage))
	router.HandleFunc("POST /settings/{action}", middleware.CatchError(routes.SettingsPOST))

	// User action routes
	router.HandleFunc("GET /self", middleware.CatchError(routes.SelfUserPage))
	router.HandleFunc("GET /self/followingUsers", middleware.CatchError(routes.SelfFollowingUsersPage))
	router.HandleFunc("GET /self/followingWorks", middleware.CatchError(routes.SelfFollowingWorksPage))
	router.HandleFunc("GET /self/bookmarks", middleware.CatchError(routes.SelfBookmarksPage))
	router.HandleFunc("GET /self/login", middleware.CatchError(routes.LoginPage))

	router.HandleFunc("POST /self/addBookmark/{artwork_id}", middleware.CatchError(routes.AddBookmarkRoute))
	router.HandleFunc("POST /self/deleteBookmark/{bookmark_id}", middleware.CatchError(routes.DeleteBookmarkRoute))
	router.HandleFunc("POST /self/like/{artwork_id}", middleware.CatchError(routes.LikeRoute))
	router.HandleFunc("POST /api/follow", middleware.CatchError(routes.FollowRoute))

	// oEmbed routes
	router.HandleFunc("GET /oembed", middleware.CatchError(routes.Oembed))

	// Search routes
	router.HandleFunc("GET /search", middleware.CatchError(routes.SearchPage))

	// REST API routes (for htmx)
	router.HandleFunc("GET /api/artwork", middleware.CatchError(routes.ArtworkPartial))
	router.HandleFunc("GET /api/recent", middleware.CatchError(routes.RecentPartial))
	router.HandleFunc("GET /api/related", middleware.CatchError(routes.RelatedPartial))
	router.HandleFunc("GET /api/comments", middleware.CatchError(routes.CommentsPartial))
	router.HandleFunc("GET /api/discovery", middleware.CatchError(routes.DiscoveryPartial))
	router.HandleFunc("GET /api/tag-completions", middleware.CatchError(routes.TagCompletionsPartial))
	router.HandleFunc("GET /api/street", middleware.CatchError(routes.StreetPartial))

	// Challenge page route
	if config.Global.Limiter.Enabled {
		router.HandleFunc("GET /limiter/challenge", middleware.CatchError(limiter.ChallengePage))

		// Turnstile routes
		if config.Global.Limiter.DetectionMethod == config.Turnstile {
			router.HandleFunc("POST /limiter/turnstile/verify", middleware.CatchError(limiter.TurnstileVerify))
		}

		// Link token routes
		if config.Global.Limiter.DetectionMethod == config.LinkToken {
			router.HandleFunc("GET /limiter/{token}", middleware.CatchError(limiter.LinkTokenChallenge))
		}
	}

	if config.Global.Development.InDevelopment {
		router.HandleFunc("GET /dev/components", middleware.CatchError(routes.ComponentsPage))
	}

	// Index page routes
	// /{$} matches only the root path
	router.HandleFunc("GET /{$}", middleware.CatchError(routes.IndexPage))
	router.HandleFunc("GET /street", middleware.CatchError(routes.StreetPage))

	if config.Global.Development.InDevelopment {
		registerDebugRoutes(router)
	}
}

// Serve static files from embedded assets.
func fileServer() http.HandlerFunc {
	staticContentFS, err := fs.Sub(assets.FS, "assets")
	if err != nil {
		panic(fmt.Errorf("failed to create sub-filesystem for embedded 'assets' directory: %w", err))
	}

	fileServer := http.FileServer(http.FS(staticContentFS))
	fileServerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=3600")
		// Using a strong ETag for static files embedded via go:embed
		// ref: https://www.rfc-editor.org/rfc/rfc9110#weak.and.strong.validators
		//
		// Since go:embed requires rebuilding when files change, we use a per-instance
		// cache ID to ensure browsers fetch fresh content after any deployment.
		w.Header().Set("ETag", config.Global.Instance.FileServerCacheID)
		fileServer.ServeHTTP(w, r)
	})

	return fileServerHandler
}

var flightRecorder = trace.NewFlightRecorder(trace.FlightRecorderConfig{MinAge: time.Minute})

func registerDebugRoutes(router *Router) {
	err := flightRecorder.Start()
	if err != nil {
		panic(err)
	}

	router.HandleFunc("GET /debug/pprof/", pprof.Index)
	router.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	router.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	router.HandleFunc("GET /debug/flight", func(w http.ResponseWriter, r *http.Request) {
		_, _ = flightRecorder.WriteTo(w)
	})
}
