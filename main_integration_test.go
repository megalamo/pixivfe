// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build integration

/*
To run these tests, specify `-tags=integration` when running `go test`.
*/
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	// Server configuration constants.
	host      = "0.0.0.0:8282"
	authority = "http://0.0.0.0:8282"

	// Polling constants.
	retryCount  = 10
	dialTimeout = 250 * time.Millisecond
)

var test_token = os.Getenv("PIXIVFE_TEST_TOKEN")

// httpTestCase defines a test case.
type httpTestCase struct {
	URL                string
	Method             string
	ExpectedStatusCode int
	Cookies            []*http.Cookie

	// POST requests specific fields
	FormData map[string]string
}

// setDefault sets the default values for the test case.
func (c *httpTestCase) setDefault() {
	if c.ExpectedStatusCode == 0 {
		c.ExpectedStatusCode = 200
	}
}

// TestMain is used for global setup and teardown.
//
// It starts the server and waits for it to be available before running tests.
func TestMain(m *testing.M) {
	go func() {
		if err := run(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for the server.
	if !waitForServerReady() {
		log.Fatalf("Server did not start in time")
	}

	os.Exit(m.Run())
}

// waitForServerReady polls the server until it's available or the retries are exhausted.
func waitForServerReady() bool {
	for range retryCount {
		conn, err := net.DialTimeout("tcp", host, dialTimeout)
		if err == nil {
			_ = conn.Close()

			return true // Server is up.
		}

		time.Sleep(dialTimeout)
	}

	return false
}

// TestBasicAllRoutes tests all basic routes of the server.
func TestBasicAllRoutes(t *testing.T) {
	t.Parallel()

	testCases := []httpTestCase{
		// Thorough route tests
		// Newest routes
		{
			URL:    "/newest",
			Method: http.MethodGet,
		},

		// Discovery routes
		{
			URL:    "/discovery",
			Method: http.MethodGet,
		},
		{
			URL:    "/discovery?mode=r18",
			Method: http.MethodGet,
		},
		{
			URL:    "/discovery/novel",
			Method: http.MethodGet,
		},
		{
			URL:    "/discovery/novel?mode=r18",
			Method: http.MethodGet,
		},
		{
			URL:    "/discovery/users",
			Method: http.MethodGet,
		},

		// Ranking pages
		{
			URL:    "/ranking",
			Method: http.MethodGet,
		},
		{
			URL:    "/ranking?content=all&date=2023-02-12&page=1&mode=male",
			Method: http.MethodGet,
		},
		{
			URL:    "/ranking?content=manga&page=2&mode=weekly_r18",
			Method: http.MethodGet,
		},
		{
			URL:    "/ranking?content=ugoira&mode=daily_r18",
			Method: http.MethodGet,
		},
		{
			URL:    "/rankingCalendar?mode=daily_r18&date=2018-08-01",
			Method: http.MethodGet,
		},

		// Artwork page
		{
			URL:    "/artworks/121247335",
			Method: http.MethodGet,
		},
		// Artwork page (R-18)
		{
			URL:    "/artworks/120131626",
			Method: http.MethodGet,
		},
		// User page
		{
			URL:    "/users/810305",
			Method: http.MethodGet,
		},
		{
			// Atom feed
			URL:    "/users/810305/atom.xml",
			Method: http.MethodGet,
		},
		{
			// Atom feed with category
			URL:    "/users/810305/atom.xml?category=manga",
			Method: http.MethodGet,
		},
		{
			URL:    "/users/810305?category=novels",
			Method: http.MethodGet,
		},
		{
			URL:    "/users/810305?category=bookmarks",
			Method: http.MethodGet,
		},
		{
			URL:    "/users/2226515", // User doesn't have artworks
			Method: http.MethodGet,
		},
		// pixivision routes
		{
			// Index
			URL:    "/pixivision",
			Method: http.MethodGet,
		},
		{
			// Article
			URL:    "/pixivision/a/10128",
			Method: http.MethodGet,
		},
		{
			// Tag
			URL:    "/pixivision/t/27",
			Method: http.MethodGet,
		},
		{
			// Category
			URL:    "/pixivision/c/manga",
			Method: http.MethodGet,
		},

		// Search route
		{
			// Simple search
			URL:    "/search?name=original",
			Method: http.MethodGet,
		},
		{
			// Search with multiple parameters
			URL:    "/search?category=manga&mode=r18&name=original&order=date&smode=s_tag&ecd=&scd=&wlt=&wgt=&hlt=&hgt=&ratio=&tool=",
			Method: http.MethodGet,
		},
		{
			// Search with encoded characters
			URL:    "/search?name=Fate%2FGrandOrder",
			Method: http.MethodGet,
		},
		{
			// Search with CJK characters
			URL:    "/search?name=この素晴らしい世界に祝福を",
			Method: http.MethodGet,
		},

		// Settings
		{
			URL:    "/settings",
			Method: http.MethodGet,
		},
		{
			URL:    "/self/login",
			Method: http.MethodGet,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s %s", tc.Method, tc.URL), func(t *testing.T) {
			t.Parallel()
			tc.setDefault()

			resp := makeRequest(t, buildRequest(t, authority+tc.URL, tc.Method, nil))
			defer resp.Body.Close()

			if resp.StatusCode != tc.ExpectedStatusCode {
				t.Errorf("expected status %d, got %d", tc.ExpectedStatusCode, resp.StatusCode)
			}
		})
	}
}

func TestAuthenticatedRoutes(t *testing.T) {
	if test_token == "" {
		t.Errorf("Test token was not set, not testing authenticated routes.")
	}

	loginReq := httpTestCase{
		URL:                "/settings/token",
		Method:             http.MethodPost,
		ExpectedStatusCode: 200,
		FormData: map[string]string{
			"token":      test_token,
			"returnPath": "",
		},
	}

	resp := makeRequest(t, buildRequestWithFormData(t, authority+loginReq.URL, loginReq.Method, loginReq.FormData, nil))
	defer resp.Body.Close()

	if resp.StatusCode != loginReq.ExpectedStatusCode {
		t.Errorf("expected status %d, got %d", loginReq.ExpectedStatusCode, resp.StatusCode)
	}

	defaultAuthCookies := []*http.Cookie{
		{
			Name:     "PHPSESSID",
			Value:    test_token,
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		},
	}

	testCases := []httpTestCase{
		{
			URL:     "/self",
			Method:  http.MethodGet,
			Cookies: defaultAuthCookies,
		},
		{
			URL:     "/self/bookmarks",
			Method:  http.MethodGet,
			Cookies: defaultAuthCookies,
		},
		{
			URL:     "/self/followingUsers",
			Method:  http.MethodGet,
			Cookies: defaultAuthCookies,
		},
		{
			URL:     "/self/followingWorks",
			Method:  http.MethodGet,
			Cookies: defaultAuthCookies,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s %s", tc.Method, tc.URL), func(t *testing.T) {
			t.Parallel()
			tc.setDefault()

			resp := makeRequest(t, buildRequest(t, authority+tc.URL, tc.Method, defaultAuthCookies))
			defer resp.Body.Close()

			if resp.StatusCode != tc.ExpectedStatusCode {
				t.Errorf("expected status %d, got %d", tc.ExpectedStatusCode, resp.StatusCode)
			}
		})
	}

	logoutReq := httpTestCase{
		URL:                "/settings/logout",
		Method:             http.MethodPost,
		ExpectedStatusCode: 200,
	}

	resp = makeRequest(t, buildRequestWithFormData(t, authority+logoutReq.URL, logoutReq.Method, nil, logoutReq.Cookies))
	defer resp.Body.Close()

	if resp.StatusCode != logoutReq.ExpectedStatusCode {
		t.Errorf("expected status %d, got %d", logoutReq.ExpectedStatusCode, resp.StatusCode)
	}
}

func buildRequest(t *testing.T, link, method string, cookies []*http.Cookie) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.TODO(), method, link, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	for _, v := range cookies {
		req.AddCookie(v)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:122.0) Gecko/20100101 Firefox/122.0")

	return req
}

func buildRequestWithFormData(t *testing.T, link, method string, formData map[string]string, cookies []*http.Cookie) *http.Request {
	t.Helper()

	form := url.Values{}

	for k, v := range formData {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(context.TODO(), method, link, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	for _, v := range cookies {
		req.AddCookie(v)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:122.0) Gecko/20100101 Firefox/122.0")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return req
}

func makeRequest(t *testing.T, req *http.Request) *http.Response {
	t.Helper()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}

	return resp
}
