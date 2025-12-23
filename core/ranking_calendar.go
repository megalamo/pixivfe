// Copyright 2023 - 2025, VnPower and the pixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	"codeberg.org/pixivfe/pixivfe/v3/core/requests"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

const (
	// RankingCalendarDefaultMode is the default mode for the ranking calendar.
	RankingCalendarDefaultMode = "daily"

	// earliestDate is the earliest date available in the ranking calendar.
	earliestDate = "2007-09-01"
)

var (
	// artworkIDRegex extracts the artwork ID from an image URL.
	artworkIDRegex = regexp.MustCompile(`/(\d+)_p0_(custom|square)1200\.jpg`)

	errInvalidYear  = errors.New("invalid year")
	errInvalidMonth = errors.New("invalid month")
)

// RankingCalendarData holds all the necessary data for rendering the ranking calendar page.
type RankingCalendarData struct {
	Title        string
	Calendar     []dayCalendar
	Mode         string
	Year         string
	MonthBefore  dateWrap
	MonthAfter   dateWrap
	ThisMonth    dateWrap
	EarliestDate string
}

// dayCalendar represents the data for a single day in the ranking calendar.
type dayCalendar struct {
	DayNumber   int        // The day of the month
	Date        string     // pixiv-compatible string that represents the date (format: YYYY-MM-DD)
	ImageURL    string     // Proxy URL to the image (optional, can be empty when no image is available)
	ArtworkLink string     // The link to the artwork page for this day
	Thumbnails  Thumbnails // Image links derived from ImageURL
}

// dateWrap is a struct that encapsulates date-related information for easier handling in templates.
type dateWrap struct {
	Link         string // URL-friendly date string
	Year         string
	Month        string
	MonthPadded  string // Two-digit representation of the month
	MonthLiteral string // Full name of the month
}

// GetRankingCalendar retrieves and processes the ranking calendar data from pixiv.
//
// It returns a slice of DayCalendar structs and any error encountered.
//
// @iacore: so the funny thing about pixiv is that they will return this month's data for a request of a future date. is it a bug or a feature?
func GetRankingCalendar(r *http.Request, mode, year, month string) (RankingCalendarData, error) {
	var data RankingCalendarData

	yearInt, err := strconv.Atoi(year)
	if err != nil {
		return data, fmt.Errorf("%w: %s", errInvalidYear, year)
	}

	monthInt, err := strconv.Atoi(month)
	if err != nil {
		return data, fmt.Errorf("%w: %s", errInvalidMonth, month)
	}

	resp, err := requests.Get(
		r.Context(),
		GetRankingCalendarURL(mode, yearInt, monthInt),
		map[string]string{"PHPSESSID": untrusted.GetUserToken(r)},
		r.Header)
	if err != nil {
		return data, err
	}

	doc, err := html.Parse(resp)
	if err != nil {
		return data, err
	}

	calendar := []dayCalendar{}
	dayCount := 0

	// Get the first day of the month
	yearInt, _ = strconv.Atoi(year)
	monthInt, _ = strconv.Atoi(month)

	firstDayOfMonth := time.Date(yearInt, time.Month(monthInt), 1, 0, 0, 0, 0, time.UTC)

	calendar, dayCount = addEmptyDaysBefore(calendar, firstDayOfMonth, dayCount) // Add empty days before the first day of the month

	numDays := daysIn(time.Month(monthInt), yearInt) // Get the number of days in the month

	calendar, dayCount = addDaysOfMonth(calendar,
		extractImageLinks(r, doc), dayCount, numDays, year, month) // Add days of the month
	calendar, dayCount = addEmptyDaysAfter(calendar, dayCount) // Add empty days after the last day to complete the week

	_ = dayCount

	// Populate Thumbnails field
	for i := range calendar {
		imageURL := calendar[i].ImageURL
		if imageURL == "" {
			// Skip if there is no image URL
			continue
		}

		// NOTE: We intentionally return with potentially missing thumbnails.
		thumbnails, _ := PopulateThumbnailsFor(imageURL)

		calendar[i].Thumbnails = thumbnails
	}

	// Calculate dates for navigation
	yearInt, _ = strconv.Atoi(year)
	monthInt, _ = strconv.Atoi(month)

	realDate := time.Date(yearInt, time.Month(monthInt), 1, 0, 0, 0, 0, time.UTC)
	monthBefore := realDate.AddDate(0, -1, 0)
	monthAfter := realDate.AddDate(0, 1, 0)

	data.Title = "Ranking calendar"
	data.Calendar = calendar
	data.Mode = mode
	data.Year = year
	data.MonthBefore = parseDate(monthBefore)
	data.MonthAfter = parseDate(monthAfter)
	data.ThisMonth = parseDate(realDate)
	data.EarliestDate = earliestDate

	return data, nil
}

// extractImageLinks extracts image links from the parsed HTML document.
func extractImageLinks(r *http.Request, doc *html.Node) []string {
	// Find all "img" elements in the document.
	imgSelection := goquery.NewDocumentFromNode(doc).Find("img")

	// NOTE: The actual number of links added might be less if some imgs don't have "data-src".
	links := make([]string, 0, imgSelection.Length())

	// Iterate over each found "img" element.
	// EachIter() provides an iterator where 'sel' is a *goquery.Selection for each individual img.
	for _, sel := range imgSelection.EachIter() {
		var src string

		// Find the "data-src" attribute in the current img node.
		for _, attr := range sel.Nodes[0].Attr { // Each 'sel' from EachIter() contains one node.
			if attr.Key == "data-src" {
				src = attr.Val

				break // Found "data-src", no need to check other attributes.
			}
		}

		if src == "" {
			continue // Skip this img element if it doesn't have a "data-src" attribute.
		}

		// Process the found URL.
		links = append(links, RewriteImageURLs(r, src))
	}

	return links
}

// addEmptyDaysBefore adds empty days to the calendar before the first day of the month.
func addEmptyDaysBefore(calendar []dayCalendar, firstDay time.Time, dayCount int) ([]dayCalendar, int) {
	emptyDays := int(firstDay.Weekday())
	for range emptyDays {
		calendar = append(calendar, dayCalendar{DayNumber: 0})
		dayCount++
	}

	return calendar, dayCount
}

// addDaysOfMonth adds the actual days of the month to the calendar.
func addDaysOfMonth(
	calendar []dayCalendar,
	links []string,
	dayCount, numDays int,
	year, month string,
) ([]dayCalendar, int) {
	for i := range numDays {
		var imageURL string
		if i < len(links) {
			imageURL = links[i]
		}

		var artworkLink string
		if artworkID := extractArtworkID(imageURL); artworkID != "" {
			artworkLink = "/artworks/" + artworkID
		}

		day := dayCalendar{
			DayNumber:   i + 1,
			ImageURL:    imageURL,
			ArtworkLink: artworkLink,
			Date:        fmt.Sprintf("%s-%s-%02d", year, month, i+1),
		}

		calendar = append(calendar, day)
		dayCount++
	}

	return calendar, dayCount
}

// addEmptyDaysAfter adds empty days to the calendar after the last day of the month to complete the week.
func addEmptyDaysAfter(calendar []dayCalendar, dayCount int) ([]dayCalendar, int) {
	for dayCount%7 != 0 {
		calendar = append(calendar, dayCalendar{DayNumber: 0})
		dayCount++
	}

	return calendar, dayCount
}

// daysIn returns the number of days in a given month and year.
func daysIn(month time.Month, year int) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// extractArtworkID extracts the artwork ID from the image URL.
func extractArtworkID(imageURL string) string {
	matches := artworkIDRegex.FindStringSubmatch(imageURL)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// parseDate converts a time.Time value into a dateWrap struct.
//
// Used to prepare date information for display and navigation.
func parseDate(t time.Time) dateWrap {
	var date dateWrap

	year := t.Year()
	month := t.Month()
	monthPadded := fmt.Sprintf("%02d", month)

	date.Link = fmt.Sprintf("%d-%s-01", year, monthPadded)
	date.Year = strconv.Itoa(year)
	date.Month = strconv.Itoa(int(month))
	date.MonthPadded = monthPadded
	date.MonthLiteral = month.String()

	return date
}
