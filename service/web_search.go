package service

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
)

// WebSearchResult is one normalized search hit, shaped to map cleanly onto an
// Anthropic web_search_result block (type/url/title/page_age + snippet for citations).
type WebSearchResult struct {
	Title   string
	URL     string
	Snippet string
	PageAge string
}

// braveWebResponse is the subset of the Brave Web Search API response we consume.
// https://api-dashboard.search.brave.com/app/documentation/web-search/responses
type braveWebResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			PageAge     string `json:"page_age"`
			Age         string `json:"age"`
		} `json:"results"`
	} `json:"web"`
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// stripHTML removes the <strong>/<b> highlight markup Brave embeds in descriptions.
func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return strings.TrimSpace(s)
}

// BraveWebSearch executes a single web search via the Brave Search API and returns
// normalized results. count is clamped to Brave's [1,20] window.
func BraveWebSearch(query string, count int) ([]WebSearchResult, error) {
	key := setting.BraveSearchAPIKey
	if key == "" {
		return nil, fmt.Errorf("web search is enabled but no Brave Search API key is configured")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty search query")
	}
	if count <= 0 {
		count = setting.WebSearchResultsPerQuery
	}
	if count <= 0 {
		count = 5
	}
	if count > 20 {
		count = 20
	}

	endpoint := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), count)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	// NB: do not set Accept-Encoding manually — Go's transport only auto-decompresses
	// gzip when it adds the header itself; setting it here yields raw gzip bytes.
	req.Header.Set("X-Subscription-Token", key)

	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave search request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("brave search returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed braveWebResponse
	if err := common.DecodeJson(resp.Body, &parsed); err != nil {
		return nil, fmt.Errorf("brave search decode failed: %w", err)
	}

	results := make([]WebSearchResult, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		age := r.PageAge
		if age == "" {
			age = r.Age
		}
		results = append(results, WebSearchResult{
			Title:   stripHTML(r.Title),
			URL:     r.URL,
			Snippet: stripHTML(r.Description),
			PageAge: age,
		})
	}
	return results, nil
}
