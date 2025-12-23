// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tidwall/gjson"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

const (
	// While we can technically fetch up to 100 artworks at a time,
	// such a large number can produce poor UX due to scroll fatigue.
	//
	// Plus it's divisible by the grid-cols values used
	// in the frontend (1, 2, 3, 4, and 5).
	discoveryArtworksLimit = 60
	discoveryNovelsLimit   = 100
	discoveryUsersLimit    = 12
)

// DiscoveryData defines the data used to render the Discovery page.
type DiscoveryData struct {
	Title    string
	Artworks []ArtworkItem
}

func GetDiscoveryArtworks(r *http.Request, mode string) ([]ArtworkItem, error) {
	var artworks []ArtworkItem

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetDiscoveryURL(mode, discoveryArtworksLimit),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	// We only want the "thumbnails.illust" field
	err = json.Unmarshal([]byte(gjson.GetBytes(RewriteEscapedImageURLs(r, resp), "thumbnails.illust").Raw), &artworks)
	if err != nil {
		return nil, err
	}

	// Populate thumbnails for each artwork
	for id, artwork := range artworks {
		if err := artwork.PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for artwork ID %d: %w", id, err)
		}

		artworks[id] = artwork
	}

	return artworks, nil
}

func GetDiscoveryNovels(r *http.Request, mode string) ([]*NovelBrief, error) {
	var novels []*NovelBrief

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetDiscoveryNovelURL(mode, discoveryNovelsLimit),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	// We only want the "thumbnails.novel" field
	err = json.Unmarshal([]byte(gjson.GetBytes(RewriteEscapedImageURLs(r, resp), "thumbnails.novel").Raw), &novels)
	if err != nil {
		return nil, err
	}

	// Convert RawTags to Tags for each novel
	for i := range novels {
		novels[i].Tags = novels[i].RawTags.ToTags()
	}

	return novels, nil
}

// GetDiscoveryUsers retrieves users on the discovery page along with their associated artworks and novels.
func GetDiscoveryUsers(r *http.Request) ([]*User, error) {
	var (
		artworks []ArtworkItem
		novels   []*NovelBrief
		users    []*User
	)

	resp, err := requests.GetJSONBody(
		r.Context(),
		GetDiscoveryUserURL(discoveryUsersLimit),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return nil, err
	}

	resp = RewriteEscapedImageURLs(r, resp)

	// Users
	if err = json.Unmarshal([]byte(gjson.GetBytes(resp, "users").Raw), &users); err != nil {
		return nil, fmt.Errorf("error unmarshalling users in GetDiscoveryUser: %w", err)
	}

	// Artworks
	if err = json.Unmarshal([]byte(gjson.GetBytes(resp, "thumbnails.illust").Raw), &artworks); err != nil {
		return nil, fmt.Errorf("error unmarshalling artworks in GetDiscoveryUser: %w", err)
	}

	// Populate thumbnails for each artwork
	for id, artwork := range artworks {
		if err := artwork.PopulateThumbnails(); err != nil {
			return nil, fmt.Errorf("failed to populate thumbnails for artwork ID %d: %w", id, err)
		}

		artworks[id] = artwork
	}

	// Novels
	if err = json.Unmarshal([]byte(gjson.GetBytes(resp, "thumbnails.novel").Raw), &novels); err != nil {
		return nil, fmt.Errorf("error unmarshalling novels in GetDiscoveryUser: %w", err)
	}

	// Associate artworks and novels with users
	associateContentWithUsers(users, artworks, novels)

	return users, nil
}
