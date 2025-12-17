// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

func ArtworkPage(w http.ResponseWriter, r *http.Request) error {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		w.Header().Add("Server-Timing", fmt.Sprintf("total;dur=%.0f;desc=\"Total Time\"", float64(duration.Milliseconds())))
	}()

	id := utils.GetPathVar(r, "id")
	if _, err := strconv.Atoi(id); err != nil {
		return fmt.Errorf("invalid ID: %s", id)
	}

	// For Fast-Requests, route to fast path render
	//
	// All Fast-Requests are htmx-available since the header is added by it
	if r.Header.Get("Fast-Request") == "true" {
		return ArtworkPageFast(w, r)
	}

	illust, err := core.GetArtwork(w, r, id)
	if err != nil {
		return err
	}

	// Preload assets and write HTTP 103 since we're waiting on an API response
	preloadArtworkAssets(w, r, *illust)

	if config.Global.Response.EarlyHintsResponses {
		w.WriteHeader(http.StatusEarlyHints)
	}

	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	return views.Artwork(*illust, illust.Title).Render(r.Context(), w)
}

// ArtworkPageFast handles a fast path artwork page render.
func ArtworkPageFast(w http.ResponseWriter, r *http.Request) error {
	var (
		illust core.Illust
		err    error
	)

	illust.ID = r.Header.Get("Artwork-ID")

	illust.Title, err = url.QueryUnescape(r.Header.Get("Artwork-Title"))
	if err != nil {
		return fmt.Errorf("failed to decode title: %w", err)
	}

	illust.UserID = r.Header.Get("Artwork-User-ID")

	illust.UserName, err = url.QueryUnescape(r.Header.Get("Artwork-Username"))
	if err != nil {
		return fmt.Errorf("failed to decode Artwork-Username header: %w", err)
	}

	illust.Pages, err = strconv.Atoi(r.Header.Get("Artwork-Pages"))
	if err != nil {
		return fmt.Errorf("failed to parse Artwork-Pages header: %w", err)
	}

	illust.Width, err = strconv.Atoi(r.Header.Get("Artwork-Width"))
	if err != nil {
		return fmt.Errorf("failed to parse Artwork-Width header: %w", err)
	}

	illust.Height, err = strconv.Atoi(r.Header.Get("Artwork-Height"))
	if err != nil {
		return fmt.Errorf("failed to parse Artwork-Height header: %w", err)
	}

	illustType, err := strconv.Atoi(r.Header.Get("Artwork-IllustType"))
	if err != nil {
		return fmt.Errorf("failed to parse Artwork-IllustType header: %w", err)
	}

	illust.IllustType = core.IllustType(illustType)

	webpURL := r.Header.Get("Artwork-Master-Webp-1200-Url")

	// Initialize images slice and populate thumbnails
	illust.Images = make([]core.Thumbnails, illust.Pages)
	for pageNum := range illust.Pages {
		// For first page, use the provided URL directly
		pageWebpURL := webpURL

		// For subsequent pages, modify the URLs to reflect the correct page number
		if pageNum > 0 {
			// Replace _p0_ with _p{pageNum}_ in the URLs
			pageWebpURL = strings.Replace(webpURL, "_p0_", fmt.Sprintf("_p%d_", pageNum), 1)
		}

		// Generate thumbnails for this page
		thumbnails, err := core.PopulateThumbnailsFor(pageWebpURL)
		if err != nil {
			return fmt.Errorf("failed to generate thumbnails for image on page %d: %w", pageNum, err)
		}

		// Assign the generated thumbnails
		illust.Images[pageNum] = thumbnails

		// Set additional fields for this page
		illust.Images[pageNum].MasterWebp_1200 = pageWebpURL
		illust.Images[pageNum].Original = "" // originalURL not reliable, leave empty

		// Set dimensions and illustration type
		if pageNum == 0 {
			// Use actual dimensions for the first page
			illust.Images[pageNum].Width = illust.Width
			illust.Images[pageNum].Height = illust.Height
		} else {
			// Set dummy dimensions for subsequent pages
			// This reduces layout shift if the user opens the lightbox
			// before the htmx request to ArtworkPartial completes
			illust.Images[pageNum].Width = 1000
			illust.Images[pageNum].Height = 1000
		}

		// Same IllustType for all pages
		illust.Images[pageNum].IllustType = illust.IllustType
	}

	// We skip validating the image URLs provided in request headers since:
	// 1. These URLs are only used for Link preload headers
	// 2. The actual URLs used by the frontend come from GetArtworkByIDFast
	//
	// The priority here is sending HTTP 200 with Link headers as quickly as possible.
	// Adding URL validation would create an extra network request in the critical path,
	// slowing down the initial page render.

	// Preload assets, but don't write HTTP 103 since we're not waiting on an API response
	preloadArtworkAssets(w, r, illust)

	// Set cache control headers
	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	return views.Artwork(illust, illust.Title).Render(r.Context(), w)
}
