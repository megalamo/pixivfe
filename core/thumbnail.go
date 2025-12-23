// Rewriting image URLs from pixiv.net with better ones

package core

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// filenameSuffixRegexp matches any suffix  that starts with an underscore
// and is followed by non-slash characters before the file extension.
var filenameSuffixRegexp = regexp.MustCompile(`_[^/_]+\.(jpg|png|jpeg)$`)

// Thumbnails holds different size variants of an artwork image.
//
// See specs/pixiv/thumbnails.md for more details.
type Thumbnails struct {
	Width           int        // Width of the Original image
	Height          int        // Height of the Original image=
	MasterWebp_1200 string     // Original aspect ratio, width limited to 1200px, WebP format
	Original        string     // Full-size original artwork, JPG or PNG format, returned by the API
	OriginalJPG     string     // Full-size original artwork, JPG format
	OriginalPNG     string     // Full-size original artwork, PNG format
	Webp_1200       string     // 1200x1200 thumbnail, quality 90, WebP format
	Video           string     // Video URL for ugoira
	Download        string     // Download URL for the original image
	IllustType      IllustType // Artwork type
}

func (work *ArtworkItem) PopulateThumbnails() error {
	thumbnails, err := PopulateThumbnailsFor(work.Thumbnail)
	if err != nil {
		return err
	}

	work.Thumbnails = thumbnails

	return nil
}

func GetOriginalAvatarURL(avatarURL string) string {
	if avatarURL == "" {
		return ""
	}

	// Find the last underscore and number segment before file extension
	lastDotIndex := strings.LastIndex(avatarURL, ".")
	if lastDotIndex == -1 {
		return avatarURL
	}

	lastUnderscoreIndex := strings.LastIndex(avatarURL[:lastDotIndex], "_")
	if lastUnderscoreIndex == -1 {
		return avatarURL
	}

	// Check if characters between underscore and dot are numeric
	numericPart := avatarURL[lastUnderscoreIndex+1 : lastDotIndex]
	if _, err := strconv.Atoi(numericPart); err != nil {
		return avatarURL
	}

	// Remove the _* segment
	return avatarURL[:lastUnderscoreIndex] + avatarURL[lastDotIndex:]
}

// PopulateThumbnailsFor is a helper function that populates all thumbnail sizes, including Original.
func PopulateThumbnailsFor(thumbnailURL string) (Thumbnails, error) {
	var thumbnails Thumbnails

	// auditor.SugaredLogger.Debugf("PopulateThumbnails called with Thumbnail URL: %s", thumbnailURL)

	// Parse the original Thumbnail URL to ensure it's valid
	parsedURL, err := url.Parse(thumbnailURL)
	if err != nil {
		return thumbnails, fmt.Errorf("invalid Thumbnail URL '%s': %w", thumbnailURL, err)
	}

	// Verify that the Thumbnail URL contains the expected pattern
	if !sizeQualityRe.MatchString(parsedURL.Path) {
		log.Warn().
			Str("url", thumbnailURL).
			Msg("Thumbnail URL does not match expected pattern. Using original URL for all thumbnail sizes.")

		thumbnails.OriginalJPG = thumbnailURL
		thumbnails.OriginalPNG = thumbnailURL

		return thumbnails, nil
	}

	// Define the desired sizes for the thumbnails along with corresponding fields
	thumbSizes := []struct {
		name  string
		size  string
		field *string
	}{
		{"Webp_1200", "1200x1200_80_webp", &thumbnails.Webp_1200},
	}

	// Generate regular thumbnails
	for _, thumb := range thumbSizes {
		finalURL, err := generateThumbnailURL(thumbnailURL, sizeQualityRe, thumb.size)
		if err != nil {
			return thumbnails, fmt.Errorf("error generating thumbnail URL for size %s: %w", thumb.size, err)
		}

		*thumb.field = finalURL
	}

	// Generate MasterWebp_1200 URL
	thumbnails.MasterWebp_1200 = generateMasterWebpURL(thumbnailURL, "")

	// Generate illustration and manga original URLs
	originalJPGURL, err := generateOriginalURL(thumbnailURL, "jpg")
	if err != nil {
		return thumbnails, fmt.Errorf("error generating original JPG URL: %w", err)
	}

	thumbnails.OriginalJPG = originalJPGURL

	originalPNGURL, err := generateOriginalURL(thumbnailURL, "png")
	if err != nil {
		return thumbnails, fmt.Errorf("error generating original PNG URL: %w", err)
	}

	thumbnails.OriginalPNG = originalPNGURL

	return thumbnails, nil
}

// generateThumbnailURL constructs a thumbnail URL for a given size.
func generateThumbnailURL(urlStr string, re *regexp.Regexp, size string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL '%s': %w", urlStr, err)
	}

	newPath := re.ReplaceAllString(parsedURL.Path, fmt.Sprintf("/c/%s/", size))

	// If the new path is the same as the original, it means either:
	// - The URL is already at the desired size.
	// - The regex did not match, which has been handled earlier in PopulateThumbnailsFor.
	// In both cases, returning the original URL is acceptable.
	if newPath == parsedURL.Path {
		return parsedURL.String(), nil
	}

	updatedURL := *parsedURL // Create a copy of the original URL

	updatedURL.Path = newPath

	return updatedURL.String(), nil
}

// generateOriginalURL constructs the original image URL.
func generateOriginalURL(urlStr, extension string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL '%s': %w", urlStr, err)
	}

	clonedURL := *parsedURL

	// Remove size/quality segment (e.g., /c/250x250_80_a2/... -> /...)
	clonedURL.Path = sizeQualityRe.ReplaceAllString(clonedURL.Path, "/")

	// Convert either '/custom-thumb/' or '/img-master/' to '/img-original/'
	//
	// A thumbnail URL can only have one or the other, not both
	if strings.Contains(clonedURL.Path, "/custom-thumb/") {
		clonedURL.Path = strings.Replace(clonedURL.Path, "/custom-thumb/", "/img-original/", 1)
	} else if strings.Contains(clonedURL.Path, "/img-master/") {
		clonedURL.Path = strings.Replace(clonedURL.Path, "/img-master/", "/img-original/", 1)
	}

	// Remove filename suffix and force specified extension
	clonedURL.Path = filenameSuffixRegexp.ReplaceAllString(clonedURL.Path, "."+extension)

	// Clean the path to resolve redundant elements (e.g., '//' -> '/')
	clonedURL.Path = path.Clean(clonedURL.Path)

	return clonedURL.String(), nil
}

var replaceOriginalWithMaster = strings.NewReplacer(
	"/img-original/", "/img-master/",
	"/custom-thumb/", "/img-master/",
	"/novel-cover-original/", "/novel-cover-master/",
)

// generateMasterWebpURL converts a pixiv image URL to its master WebP format.
//
// If proxyBase is provided, it replaces any existing proxy base; otherwise preserves it.
func generateMasterWebpURL(urlStr, proxyBase string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// If the URL is invalid, return it as is.
		return urlStr
	}

	// 1. Find the start of the actual pixiv image path to separate
	// it from any preceding proxy path segments.
	var (
		base, imagePath string
		fullPath        = parsedURL.Path
		startIndex      = -1
	)

	for _, marker := range pathMarkers {
		if idx := strings.Index(fullPath, marker); idx != -1 {
			// Find the earliest occurrence of any marker to correctly handle paths
			// that might contain multiple markers (e.g., a proxy path).
			if startIndex == -1 || idx < startIndex {
				startIndex = idx
			}
		}
	}

	if startIndex != -1 {
		base = fullPath[:startIndex]
		imagePath = fullPath[startIndex:]
	} else {
		// Fallback: If no known markers are found, check if the URL ends
		// with a recognizable pixiv image filename.
		lastSlash := strings.LastIndex(fullPath, "/")
		if lastSlash == -1 {
			// Not a recognizable pixiv image: No path structure.
			return urlStr
		}

		filename := fullPath[lastSlash+1:]
		if !baseFileRe.MatchString(filename) {
			// Not a recognizable pixiv image: Filename pattern does not match.
			return urlStr
		}

		// This looks like a pixiv image in an unknown path.
		// Process it by treating the path up to the filename as the base.
		base = fullPath[:lastSlash]
		imagePath = fullPath[lastSlash:]
	}

	// 2. Normalize the image path.
	// a. Remove any existing size/quality segment (e.g., /c/250x250_80_a2/).
	newPath := sizeQualityRe.ReplaceAllString(imagePath, "/")
	// b. Replace various path segments with the canonical master path.
	newPath = replaceOriginalWithMaster.Replace(newPath)
	// c. Handle the special case for /img/ which is not handled by the replacer.
	if strings.HasPrefix(newPath, "/img/") {
		newPath = strings.Replace(newPath, "/img/", "/img-master/img/", 1)
	}

	// 3. Normalize the filename to the master format.
	// The master format uses a specific suffix and a .jpg extension, even for WebP.
	newPath = baseFileRe.ReplaceAllString(newPath, "${1}_master1200.jpg")

	// 4. Prepend the WebP quality/size specifier to request the correct image type.
	newPath = "/c/1200x1200_80_webp" + newPath

	// 5. Clean the final path to resolve any double slashes.
	newPath = path.Clean(newPath)

	// 6. Re-assemble the final URL.
	if proxyBase != "" {
		// A new proxy base is provided, so return a relative path with this new base.
		return strings.TrimSuffix(proxyBase, "/") + newPath
	}

	// No new proxy base was provided. Reconstruct the URL preserving the original
	// scheme, host, and any detected base.
	parsedURL.Path = path.Clean(base + newPath)

	return parsedURL.String()
}
