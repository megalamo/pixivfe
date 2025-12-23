// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package template

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"math"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"codeberg.org/pixivfe/pixivfe/v3/server/assets"
)

// RelativeTimeData holds the numeric value and description for relative time.
type RelativeTimeData struct {
	Value       string
	Description string
	Time        string
}

// iconCache holds all of our SVGs keyed by filename (without the “.svg” suffix).
var iconCache = make(map[string]string)

// LoadIcons scans the given directory for “.svg” files, reads each file into
// memory, wraps its bytes in HTML, and stores it in iconCache.
//
// If any operation fails, LoadIcons returns an error for the caller to handle.
func LoadIcons(dir string) error {
	// ReadDir returns a list of directory entries (files + subdirectories).
	entries, err := fs.ReadDir(assets.FS, dir)
	if err != nil {
		return fmt.Errorf("reading icons directory %q: %w", dir, err)
	}

	// Pre-allocate map capacity to avoid repeated growth.
	iconCache = make(map[string]string, len(entries))

	for _, entry := range entries {
		// Skip subdirectories — we only want files.
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Fast path: only care about .svg files.
		if !strings.HasSuffix(name, ".svg") {
			continue
		}

		// Use path.Join, not filepath.Join. The embedded filesystem requires
		// forward slashes ('/') as path separators on all operating systems.
		fullPath := path.Join(dir, name)

		content, err := fs.ReadFile(assets.FS, fullPath)
		if err != nil {
			return fmt.Errorf("reading icon %q: %w", fullPath, err)
		}

		// Strip off the “.svg” extension to form the map key.
		key := strings.TrimSuffix(name, ".svg")

		// Wrap the raw bytes as HTML and store in the cache.
		iconCache[key] = string(content)
	}

	return nil
}

// NaturalTime formats a time.Time value as a natural language string.
//
// TODO: tailor the format per locale.
func NaturalTime(date time.Time) string {
	return date.Format("Monday, 2 January 2006, at 3:04 PM")
}

// RelativeTime returns a RelativeTimeData struct with a relative description based on the given date.
//
// The "Yesterday" case is special and is triggered when the date matches exactly the previous
// calendar day (i.e., same year, month, and previous day), regardless of the exact number of
// hours that have elapsed.
//
// TODO: tailor the format per locale.
func RelativeTime(date time.Time) RelativeTimeData {
	now := time.Now()
	duration := now.Sub(date)

	// local helper function to choose the correct singular/plural unit.
	pluralize := func(value int, singular, plural string) string {
		if value == 1 {
			return singular
		}

		return plural
	}

	// For future dates, simply show the day and full date/time formatting.
	if duration < 0 {
		return RelativeTimeData{
			Value:       date.Format("2"),
			Description: date.Format("January 2006 3:04 PM"),
		}
	}

	// Less than one minute ago.
	if duration < time.Minute {
		return RelativeTimeData{
			Value: "Just now",
		}
	}

	// Less than one hour: display minutes.
	if duration < time.Hour {
		minutes := int(duration.Minutes())

		return RelativeTimeData{
			Value:       fmt.Sprintf("%d %s", minutes, pluralize(minutes, "minute", "minutes")),
			Description: "ago",
		}
	}

	// Less than one day: display hours.
	const hoursInDay = 24
	if duration < hoursInDay*time.Hour {
		hours := int(duration.Hours())

		return RelativeTimeData{
			Value:       fmt.Sprintf("%d %s", hours, pluralize(hours, "hour", "hours")),
			Description: "ago",
		}
	}

	// Check if the date corresponds to 'yesterday'
	yesterday := now.AddDate(0, 0, -1)
	if date.Year() == yesterday.Year() && date.Month() == yesterday.Month() && date.Day() == yesterday.Day() {
		return RelativeTimeData{
			Value:       "Yesterday",
			Description: "at",
			Time:        date.Format("3:04 PM"),
		}
	}

	// Less than one week: display days.
	const daysInWeek = 7
	if duration < daysInWeek*hoursInDay*time.Hour {
		days := int(duration.Hours() / hoursInDay)

		return RelativeTimeData{
			Value:       fmt.Sprintf("%d %s", days, pluralize(days, "day", "days")),
			Description: "ago",
		}
	}

	// Less than one month (using a 31-day threshold): display weeks.
	const daysInMonth = 31
	if duration < daysInMonth*hoursInDay*time.Hour {
		weeks := int(duration.Hours() / (hoursInDay * daysInWeek))

		return RelativeTimeData{
			Value:       fmt.Sprintf("%d %s", weeks, pluralize(weeks, "week", "weeks")),
			Description: "ago",
		}
	}

	// Calculate total month difference (ignoring day differences for simplicity).
	yearDiff := now.Year() - date.Year()
	monthDiff := int(now.Month()) - int(date.Month())

	const monthsInYear = 12

	months := yearDiff*monthsInYear + monthDiff

	// Less than one year: display months.
	if months < monthsInYear {
		return RelativeTimeData{
			Value:       fmt.Sprintf("%d %s", months, pluralize(months, "month", "months")),
			Description: "ago",
		}
	}

	// Otherwise, show years.
	years := months / monthsInYear

	return RelativeTimeData{
		Value:       fmt.Sprintf("%d %s", years, pluralize(years, "year", "years")),
		Description: "ago",
	}
}

// FormatDuration returns a human-readable string representation of a time.Duration.
//
// It formats the duration in the most appropriate unit (minutes, hours, days, etc.)
// and returns a string like "2 hours", "3 days", etc.
func FormatDuration(duration time.Duration) string {
	if duration <= 0 {
		return ""
	}

	// local helper function to choose the correct singular/plural unit.
	pluralize := func(value int, singular, plural string) string {
		if value == 1 {
			return singular
		}

		return plural
	}

	// Less than one minute
	if duration < time.Minute {
		seconds := int(duration.Seconds())
		if seconds == 0 {
			return "moments"
		}

		return fmt.Sprintf("%d %s", seconds, pluralize(seconds, "second", "seconds"))
	}

	// Less than one hour: display minutes
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d %s", minutes, pluralize(minutes, "minute", "minutes"))
	}

	// Less than one day: display hours
	const hoursInDay = 24
	if duration < hoursInDay*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d %s", hours, pluralize(hours, "hour", "hours"))
	}

	// Less than one week: display days
	const daysInWeek = 7
	if duration < daysInWeek*hoursInDay*time.Hour {
		days := int(duration.Hours() / hoursInDay)
		return fmt.Sprintf("%d %s", days, pluralize(days, "day", "days"))
	}

	// Less than one month (using a 30-day threshold): display weeks
	const daysInMonth = 30
	if duration < daysInMonth*hoursInDay*time.Hour {
		weeks := int(duration.Hours() / (hoursInDay * daysInWeek))
		return fmt.Sprintf("%d %s", weeks, pluralize(weeks, "week", "weeks"))
	}

	// Otherwise, display months (approximation)
	months := int(duration.Hours() / (hoursInDay * daysInMonth))

	return fmt.Sprintf("%d %s", months, pluralize(months, "month", "months"))
}

// AbbrevInt formats n using short suffixes.
// Input of n <= 0 returns "0".
func AbbrevInt(n int) string {
	if n <= 0 {
		return "0"
	}

	// formatScaled turns an integer tenths value
	// into a compact string, trimming any trailing .0.
	formatScaled := func(scaled int64) string {
		whole := scaled / 10

		frac := scaled % 10
		if frac == 0 {
			return strconv.FormatInt(whole, 10)
		}

		return strconv.FormatInt(whole, 10) + "." + strconv.FormatInt(frac, 10)
	}

	x := int64(n)

	const (
		thousand int64 = 1_000
		million  int64 = 1_000_000
		billion  int64 = 1_000_000_000
		trillion int64 = 1_000_000_000_000
	)

	units := []struct {
		threshold int64
		suffix    string
	}{
		{trillion, "T"},
		{billion, "B"}, // Less confusing than "G"
		{million, "M"},
		{thousand, "k"},
	}

	for i, u := range units {
		if x >= u.threshold {
			// scaled is the value in tenths of the unit, rounded to nearest
			// for example, 1.2k => scaled == 12
			scaled := (x*10 + u.threshold/2) / u.threshold

			// If rounding gives 1000.0 of this unit, promote to the next larger unit
			// so 999_950 -> 1.0m, 999_950_000 -> 1.0b, etc.
			if scaled >= 10_000 && i > 0 {
				prev := units[i-1]
				scaledPrev := (x*10 + prev.threshold/2) / prev.threshold

				return formatScaled(scaledPrev) + prev.suffix
			}

			return formatScaled(scaled) + u.suffix
		}
	}

	// Below 1000, return the plain number
	return strconv.FormatInt(x, 10)
}

// PrettyNumber pretty prints an integer with commas as thousands separators.
//
// Negative numbers are handled by first setting aside the sign.
func PrettyNumber(n int) string {
	// Determine sign
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n // work with an absolute value
	}

	// Convert the integer to a string using strconv.Itoa.
	numStr := strconv.Itoa(n)

	// If there are no more than three digits, no comma is needed.
	const maxDigitsWithoutComma = 3
	if len(numStr) <= maxDigitsWithoutComma {
		return sign + numStr
	}

	// Build digit groups from right to left.
	var groups []string

	const digitsPerGroup = 3
	for len(numStr) > digitsPerGroup {
		// Extract the last three digits.
		groups = append([]string{numStr[len(numStr)-digitsPerGroup:]}, groups...)
		// Remove the processed group.
		numStr = numStr[:len(numStr)-digitsPerGroup]
	}
	// Prepend any remaining digits.
	if len(numStr) > 0 {
		groups = append([]string{numStr}, groups...)
	}

	// Join the groups with commas, and add a preceding negative sign if needed.
	return sign + strings.Join(groups, ",")
}

// OrdinalNumeral returns an integer as its ordinal form in a string.
//
//nolint:mnd
func OrdinalNumeral(num int) string {
	// Handle special cases for 11, 12, 13
	lastTwo := num % 100
	if lastTwo == 11 || lastTwo == 12 || lastTwo == 13 {
		return strconv.Itoa(num) + "th"
	}

	// Handle cases based on last digit
	lastDigit := num % 10
	switch lastDigit {
	case 1:
		return strconv.Itoa(num) + "st"
	case 2:
		return strconv.Itoa(num) + "nd"
	case 3:
		return strconv.Itoa(num) + "rd"
	default:
		return strconv.Itoa(num) + "th"
	}
}

// IsFirstPathPart checks if the first part of the current path matches the given path.
func IsFirstPathPart(currentPath, pathToCheck string) bool {
	// Trim any trailing slashes from both paths
	currentPath = strings.TrimRight(currentPath, "/")
	pathToCheck = strings.TrimRight(pathToCheck, "/")

	// Split the current path into parts
	const maxPathParts = 3

	parts := strings.SplitN(currentPath, "/", maxPathParts)

	const minPathParts = 2
	// Check if we have at least two parts (empty string and the first path part)
	if len(parts) < minPathParts {
		return false
	}

	// Compare the first path part with the pathToCheck
	return "/"+parts[1] == pathToCheck
}

// FormatWorkIDs formats an integer slice of work IDs
// into a comma-separated string.
//
// Used to correctly format work IDs for the
// recent works API call in artworkFast.jet.html.
func FormatWorkIDs(ids []int) string {
	if len(ids) == 0 {
		return ""
	}

	// Convert ints to strings
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = strconv.Itoa(id)
	}

	// Join with commas
	return strings.Join(strIDs, ",")
}

// RenderIcon returns an SVG (as HTML), optionally
// injecting a trusted CSS class into the <svg> tag.
//
// iconName is developer‑controlled, and iconCache only ever
// contains vetted SVG blobs; directly returning HTML
// is therefore safe.
//
// If iconName is not found, a simple text placeholder is returned.
//
// The optional classes parameter can be used to inject a single
// CSS class into the <svg> tag. Only the first string in classes
// is used.
func RenderIcon(iconName string, classes ...string) string {
	raw, ok := iconCache[iconName]
	if !ok {
		// iconName is guaranteed safe, so we can inline it directly
		return "[missing icon: " + iconName + "]"
	}

	svg := raw

	// if a class was provided, inject it into the <svg> tag
	if len(classes) > 0 && classes[0] != "" {
		// classes[0] is also guaranteed safe
		svg = strings.Replace(svg, "<svg", `<svg class="`+classes[0]+`"`, 1)
	}

	return svg
}

// GetSpecialEffects returns special effect URLs based on the effect name.
func GetSpecialEffects(s string) string {
	if s == "pixivSakuraEffect" {
		return "/proxy/source.pixiv.net/special/seasonal-effect-tag/pixiv-sakura-effect/effect.png"
	}

	return ""
}

// Floor returns the floor of a float64 as an int.
func Floor(i float64) int {
	return int(math.Floor(i))
}

// RenderToString converts a templ.Component to its string representation.
//
// Handling errors in templates is awkward, so if an error occurs during rendering,
// it is formatted into a string and returned.
func RenderToString(c templ.Component) string {
	var buffer bytes.Buffer

	err := c.Render(context.Background(), &buffer)
	if err != nil {
		return fmt.Errorf("templ: failed to render component: %w", err).Error()
	}

	return buffer.String()
}
