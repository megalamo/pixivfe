package searxng

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"codeberg.org/pixivfe/pixivfe/v3/server/utils"
)

var errHTTPRequestFailed = errors.New("HTTP request failed")

// SearXNG client
// construct this struct manually.
type SearchClient struct {
	// SearXNG base URL (should have a trailing /)
	BaseURL   string
	UserAgent string
}

// see https://docs.searxng.org/dev/search_api.html#parameters
type SearchRequest struct {
	Q        string
	Language string
	Pageno   int
}

func (params SearchRequest) Format(baseURL string) (string, error) {
	url, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	query := url.Query()
	query.Set("q", params.Q)

	if params.Language != "" {
		query.Set("language", params.Language)
	}

	if params.Pageno != 0 {
		query.Set("Pageno", strconv.Itoa(params.Pageno))
	}

	query.Set("engines", "duckduckgo,bing")
	query.Set("format", "json")
	query.Set("safe_search", "0")

	url.RawQuery = query.Encode()

	return url.String(), nil
}

type SearchResponse struct {
	Query               string         `json:"query"`
	NumberOfResults     int            `json:"number_of_results"`
	Results             []SearchResult `json:"results"`
	Answers             []any          `json:"answers"`
	Corrections         []any          `json:"corrections"`
	Infoboxes           []any          `json:"infoboxes"`
	Suggestions         []any          `json:"suggestions"`
	UnresponsiveEngines []any          `json:"unresponsive_engines"`
}

type SearchResult struct {
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	Engine      string         `json:"engine"`
	ParsedURL   []string       `json:"parsed_url"`
	Template    string         `json:"template"`
	Engines     []string       `json:"engines"`
	Positions   []int          `json:"positions"`
	Score       float64        `json:"score"`
	Category    string         `json:"category"`
	PublishDate string         `json:"publishedDate,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func (s *SearchClient) Search(ctx context.Context, request SearchRequest) (*SearchResponse, error) {
	requestURL, err := request.Format(s.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build search URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	if s.UserAgent != "" {
		req.Header.Set("User-Agent", s.UserAgent)
	}

	resp, err := utils.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("%w with status code %d: %s", errHTTPRequestFailed, resp.StatusCode, string(body))
	}

	var searxngResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searxngResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return &searxngResp, nil
}
