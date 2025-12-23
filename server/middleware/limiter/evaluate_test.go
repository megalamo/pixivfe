package limiter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/pixivfe/pixivfe/v3/config"
	"codeberg.org/pixivfe/pixivfe/v3/server/middleware"
)

func TestLimiter(t *testing.T) {
	setupLimiterTest(t)

	// Disable header checks to avoid unrelated header-based blocks in these tests.
	config.Global.Limiter.CheckHeaders = false

	tests := []struct {
		name            string
		path            string
		ip              string
		passList        []string
		blockList       []string
		detectionMethod config.LimiterDetectionMethod
		expectedStatus  int
		shouldCallNext  bool
	}{
		{
			name:            "Static path should bypass all checks",
			path:            "/css/tailwind-style.css",
			ip:              "1.1.1.1",
			detectionMethod: config.None,
			expectedStatus:  http.StatusOK,
			shouldCallNext:  true,
		},
		{
			name:     "Passed IP should bypass checks",
			path:     "/artworks/20",
			ip:       "1.1.1.1",
			passList: []string{"1.1.1.1/32"},
			// Use LinkToken here to ensure pass-list truly bypasses the challenge logic.
			detectionMethod: config.LinkToken,
			expectedStatus:  http.StatusOK,
			shouldCallNext:  true,
		},
		{
			name:            "Blocked IP should be rejected",
			path:            "/artworks/20",
			ip:              "1.1.1.1",
			blockList:       []string{"1.1.1.1/32"},
			detectionMethod: config.None,
			expectedStatus:  http.StatusForbidden,
			shouldCallNext:  false,
		},
		{
			name:            "Rate limit excluded path should bypass rate limiting",
			path:            "/about",
			ip:              "1.1.1.1",
			detectionMethod: config.None,
			expectedStatus:  http.StatusOK,
			shouldCallNext:  true,
		},
		{
			name:            "Redirect to challenge page when ping cookie missing",
			path:            "/artworks/20",
			ip:              "1.2.3.4",
			detectionMethod: config.LinkToken,
			expectedStatus:  http.StatusFound,
			shouldCallNext:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Configure pass/block lists and detection method for this test case.
			setTestPassIPs(tt.passList)
			setTestBlockIPs(tt.blockList)

			config.Global.Limiter.DetectionMethod = tt.detectionMethod

			// Setup mock next handler to verify it's called.
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			})

			// Setup test request.
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			req.RemoteAddr = tt.ip + ":12345" // Set RemoteAddr directly to test IP with port

			// Setup response recorder.
			rr := httptest.NewRecorder()

			// Execute middleware.
			handler := middleware.Wrap(Evaluate, next)
			handler.ServeHTTP(rr, req)

			// Verify response status code.
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			// Verify if next handler was called as expected.
			if nextCalled != tt.shouldCallNext {
				t.Errorf("next handler called = %v, want %v", nextCalled, tt.shouldCallNext)
			}
		})
	}
}

// TestIsAtomXMLPath verifies if isAtomXMLPath correctly identifies atom.xml routes.
func TestIsAtomXMLPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path         string
		expectResult bool
	}{
		{"/users/123/atom.xml", true},
		{"/atom.xml", true},
		{"/some/path/atom.xml", true},
		{"/users/123/atom.xml?category=manga", true},
		{"/some/other/path", false},
		{"/users/123", false},
		{"/atomxml", false},
		{"/atom", false},
		{"", false},
	}

	for _, tst := range tests {
		got := isAtomXMLPath(tst.path)
		if got != tst.expectResult {
			t.Errorf("Path %s: expected isAtomXMLPath=%v, got %v", tst.path, tst.expectResult, got)
		}
	}
}
