// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

const (
	resultsLimit    = 100
	fakeBookmarkTag = "虚偽users入りタグ"
)

// descendingSuffixes defines popularity thresholds in descending order.
var descendingSuffixes = []string{"100000", "50000", "30000", "10000", "5000", "1000", "500", "100", "50"}

// SearchResponse represents the JSON structure returned by the Pixiv API for the search endpoint.
type SearchResponse struct {
	IllustManga struct {
		Data []ArtworkItem `json:"data"`
	} `json:"illustManga"`
}

// isValidResult checks if an artwork meets the search criteria.
func isValidResult(item ArtworkItem) bool {
	// Check for the fake bookmark count tag
	for _, tag := range item.Tags {
		trimmedTag := strings.TrimSpace(tag)
		if strings.EqualFold(trimmedTag, fakeBookmarkTag) {
			return false
		}
	}

	// Invalidate if AI-generated since most fake results are
	if item.AIType == AIGenerated {
		return false
	}

	return true
}

// processSearchResults filters and adds valid results to the artworks slice.
func processSearchResults(items []ArtworkItem, artworks *[]ArtworkItem) bool {
	for _, item := range items {
		if isValidResult(item) {
			*artworks = append(*artworks, item)
			if len(*artworks) >= resultsLimit {
				return true
			}
		}
	}

	return false
}

// fetchResultsForSuffix retrieves all results for a specific popularity threshold.
func fetchResultsForSuffix(ctx context.Context, r *http.Request, settings WorkSearchSettings, suffix string, suffixArtworks *[]ArtworkItem) error {
	page := 1

	for len(*suffixArtworks) < resultsLimit {
		// A local copy of settings to avoid modifying the original.
		currentSettings := settings

		currentSettings.Name = fmt.Sprintf("%s%susers入り", settings.Name, suffix) // Adjust settings.Name to include the suffix.
		currentSettings.Page = strconv.Itoa(page)                                // Set page number.

		url, err := GetArtworkSearchURL(currentSettings)
		if err != nil {
			return err
		}

		body, err := requests.GetJSONBody(ctx, url, map[string]string{"PHPSESSID": untrusted.GetUserToken(r)}, r.Header)
		if err != nil {
			return fmt.Errorf("failed to fetch search results for suffix %s: %w", suffix, err)
		}

		// Unmarshal the response.
		var resp SearchResponse

		err = json.Unmarshal(RewriteEscapedImageURLs(r, body), &resp)
		if err != nil {
			return fmt.Errorf("failed to unmarshal search results: %w", err)
		}

		if len(resp.IllustManga.Data) == 0 {
			log.Debug().
				Str("suffix", suffix).
				Int("page", page).
				Msg("No more results found")

			break
		}

		reachedLimit := processSearchResults(resp.IllustManga.Data, suffixArtworks)
		if reachedLimit {
			return nil
		}

		page++
		log.Debug().
			Str("suffix", suffix).
			Int("page", page).
			Msg("Moving to next page")
	}

	return nil
}

// searchPopular performs a search for popular artworks using a descending popularity threshold strategy.
//
// The search strategy is as follows:
//
// 1. Starts by checking the highest popularity threshold (100000 users).
//   - If results are found at the highest threshold, it stores these results.
//   - Continues checking lower thresholds in descending order (e.g., 50000, 30000, etc.),
//     adding results until the resultsLimit is reached.
//
// 2. If no results are found at the highest threshold:
//
//   - Temporarily stores results from the lowest threshold (50 users).
//
//   - Then, checks all intermediate thresholds (50000 down to 100 users).
//
//   - Uses the temporarily stored lowest threshold results only if the resultsLimit
//     cannot be reached with results from higher thresholds.
//
//     3. Assembles the final list of results in descending order of popularity thresholds,
//     stopping when the total number of results reaches resultsLimit.
func searchPopular(ctx context.Context, r *http.Request, settings WorkSearchSettings) (workResults, error) {
	log.Debug().
		Str("query", settings.Name).
		Str("mode", settings.Category).
		Msg("Starting popular search")

	artworksPerSuffix := make(map[string][]ArtworkItem)

	var totalArtworks int

	// Start with highest suffix
	highestSuffix := descendingSuffixes[0]
	log.Debug().
		Str("suffix", highestSuffix).
		Msg("Searching with highest suffix")

	var highestSuffixArtworks []ArtworkItem

	err := fetchResultsForSuffix(ctx, r, settings, highestSuffix, &highestSuffixArtworks)
	if err != nil {
		log.Err(err).
			Str("suffix", highestSuffix).
			Msg("Error fetching results for highest suffix")

		return workResults{}, err
	}

	if len(highestSuffixArtworks) > 0 {
		// Found results with highest suffix, continue with lower thresholds
		artworksPerSuffix[highestSuffix] = highestSuffixArtworks

		totalArtworks += len(highestSuffixArtworks)

		// Iterate over the remaining suffixes except the last one (lowest threshold)
		for i := 1; i < len(descendingSuffixes)-1; i++ {
			if totalArtworks >= resultsLimit {
				break
			}

			suffix := descendingSuffixes[i]

			var suffixArtworks []ArtworkItem

			err := fetchResultsForSuffix(ctx, r, settings, suffix, &suffixArtworks)
			if err != nil {
				log.Err(err).
					Str("suffix", suffix).
					Msg("Error fetching results for suffix")

				continue
			}

			artworksPerSuffix[suffix] = suffixArtworks

			totalArtworks += len(suffixArtworks)
		}
	} else {
		// No results with highest suffix, check all other thresholds before using lowest
		var lowestSuffixArtworks []ArtworkItem

		lowestSuffix := descendingSuffixes[len(descendingSuffixes)-1]

		// Store lowest threshold results temporarily
		err := fetchResultsForSuffix(ctx, r, settings, lowestSuffix, &lowestSuffixArtworks)
		if err != nil {
			return workResults{}, fmt.Errorf("failed to fetch results for lowest suffix %s: %w", lowestSuffix, err)
		}

		// If no results are found from the lowest threshold, exit early
		if len(lowestSuffixArtworks) == 0 {
			log.Debug().
				Str("suffix", lowestSuffix).
				Msg("No results found for the lowest suffix. Exiting early.")

			return workResults{
				Data:  []ArtworkItem{},
				Total: 0,
			}, nil
		}

		// Check intermediate thresholds (50000 down to 100)
		for i := 1; i < len(descendingSuffixes)-1; i++ {
			suffix := descendingSuffixes[i]

			var suffixArtworks []ArtworkItem

			err := fetchResultsForSuffix(ctx, r, settings, suffix, &suffixArtworks)
			if err != nil {
				log.Err(err).
					Str("suffix", suffix).
					Msg("Error fetching results for suffix")

				continue
			}

			artworksPerSuffix[suffix] = suffixArtworks

			totalArtworks += len(suffixArtworks)

			if totalArtworks >= resultsLimit {
				break
			}
		}

		// Only use lowest threshold results if we haven't reached resultsLimit
		if totalArtworks < resultsLimit && len(lowestSuffixArtworks) > 0 {
			// totalArtworks += len(lowestSuffixArtworks)
			artworksPerSuffix[lowestSuffix] = lowestSuffixArtworks
		}
	}

	// Assemble final results in descending order of popularity
	var artworks []ArtworkItem

	for _, suffix := range descendingSuffixes {
		arts := artworksPerSuffix[suffix]
		if len(artworks)+len(arts) >= resultsLimit {
			artworks = append(artworks, arts[:resultsLimit-len(artworks)]...)

			break
		}

		artworks = append(artworks, arts...)
	}

	total := len(artworks)
	log.Debug().
		Str("query", settings.Name).
		Str("mode", settings.Category).
		Int("resultsCount", total).
		Msg("Completed popular search")

	// // Sort by ID in descending order
	// sort.Slice(artworks, func(i, j int) bool {
	// 	return artworks[i].ID > artworks[j].ID
	// })

	return workResults{
		Data:  artworks,
		Total: total,
	}, nil
}
