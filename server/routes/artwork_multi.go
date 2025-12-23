// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/core"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

func ArtworkMultiPage(w http.ResponseWriter, r *http.Request) error {
	ids := strings.Split(utils.GetPathVar(r, "ids"), ",")
	artworks := make([]core.Illust, len(ids))
	wg := sync.WaitGroup{}

	// // gofiber/fasthttp's API is trash
	// // i can't replace r.Context() with this
	// // so i guess we will have to wait for network traffic to finish on error
	// ctx, cancel := context.WithCancel(r.Context())
	// defer cancel()
	// r.SetUserContext(ctx)
	var errGlobal error = nil

	for i, id := range ids {
		if _, err := strconv.Atoi(id); err != nil {
			errGlobal = fmt.Errorf("Invalid ID: %s", id)

			break
		}

		wg.Add(1)

		go func(i int, id string) {
			defer wg.Done()

			illust, err := core.GetArtwork(w, r, id)
			if err != nil {
				artworks[i] = core.Illust{
					Title: err.Error(), // this might be flaky
				}

				return
			}

			artworks[i] = *illust
		}(i, id)
	}

	// if err_global != nil {
	// 	cancel()
	// }

	wg.Wait()

	if errGlobal != nil {
		return errGlobal
	}

	for _, illust := range artworks {
		for _, img := range illust.Images {
			PreloadImage(w, img.MasterWebp_1200)
		}
	}

	if untrusted.GetUserToken(r) != "" {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d",
			int(config.Global.HTTPCache.MaxAge.Seconds()),
			int(config.Global.HTTPCache.StaleWhileRevalidate.Seconds())))
	}

	// return template.RenderHTML(w, r, Data_artworkMulti{
	// 	Artworks: artworks,
	// 	Title:    fmt.Sprintf("(%d images)", len(artworks)),
	// })
	return nil
}
