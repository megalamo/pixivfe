// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

// maxPrefetchedImages defines the maximum number of prefetched images
// to avoid excessive HTTP response header sizes.
const maxPrefetchedImages = 10

// PreloadImage writes a Link header to preload an image with high priority.
func PreloadImage(w http.ResponseWriter, url string) {
	link := makePreloadImageLink(url)

	w.Header().Add("Link", link)
}

// makePreloadImageLink returns a Link header fragment to preload an image with high priority.
//
// We return a string instead of writing the header immediately so we can merge everything into one Link header later.
func makePreloadImageLink(url string) string {
	return fmt.Sprintf("<%s>; rel=\"preload\"; as=\"image\"; fetchpriority=\"high\"", url)
}

// makePrefetchImageLink returns a Link header fragment to prefetch an image with low priority.
func makePrefetchImageLink(url string) string {
	return fmt.Sprintf("<%s>; rel=\"prefetch\"; as=\"image\"; fetchpriority=\"low\"", url)
}

// preloadArtworkAssets collects all necessary Link header fragments and writes them
// as a single "Link" header value.
func preloadArtworkAssets(w http.ResponseWriter, r *http.Request, illust core.Illust) {
	// linkValues gathers all Link header values.
	var linkValues []string

	// Switch on IllustType to figure out which links to generate.
	switch illust.IllustType {
	case core.Illustration:
		// Preload the first image at high priority.
		linkValues = append(linkValues, makePreloadImageLink(illust.Images[0].MasterWebp_1200))

		// Prefetch remaining images at low priority, up to the MaxPrefetchedImages limit.
		for i := range min(len(illust.Images)-1, maxPrefetchedImages) {
			linkValues = append(linkValues, makePrefetchImageLink(illust.Images[i].MasterWebp_1200))
		}

		// Prefetch the original image at low priority.
		linkValues = append(linkValues, makePrefetchImageLink(illust.Images[0].Original))
	case core.Manga:
		// For manga, we want to preload both the possible .jpg and .png versions.
		originalURL := illust.Images[0].Original
		originalURLBeforeSwap := originalURL

		// Determine file extension and swap.
		originalExt := path.Ext(originalURL)
		switch originalExt {
		case ".jpg":
			originalURL = strings.TrimSuffix(originalURL, ".jpg") + ".png"
		case ".png":
			originalURL = strings.TrimSuffix(originalURL, ".png") + ".jpg"
		}

		// Preload both potential URLs at high priority.
		linkValues = append(linkValues, makePreloadImageLink(originalURLBeforeSwap))
		linkValues = append(linkValues, makePreloadImageLink(originalURL))
	case core.Ugoira: // For ugoira, preload the animated image URL.
		linkValues = append(linkValues,
			makePreloadImageLink(utils.GetProxyBase(untrusted.GetUgoiraProxy(r))+"/ugoira/"+illust.ID))
	default: // Invalid IllustType, don't preload/prefetch anything.
	}

	// Only write a single Link header, joined by commas (RFC 8288 friendly).
	if len(linkValues) > 0 {
		// We use Add to not interfere with any prior Link header writes.
		w.Header().Add("Link", strings.Join(linkValues, ", "))
	}
}
