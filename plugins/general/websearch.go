package general

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"yunque-agent/pkg/skills"
)

// SearchResult is a single web search result.
type SearchResult = searchResult

// SearchFunc is a function that performs web search, used to inject external providers.
type SearchFunc func(ctx context.Context, query string, limit int) ([]searchResult, error)

// WebSearchSkill searches the web with automatic fallback:
// 1. External search function (Brave/Tavily/SearXNG) if configured
// 2. Bing HTML scraping (accessible in China)
// 3. DuckDuckGo HTML scraping (fallback)
type WebSearchSkill struct {
	client     *http.Client
	externalFn SearchFunc
}

func NewWebSearchSkill() *WebSearchSkill {
	return &WebSearchSkill{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// SetExternalSearch injects an external search function as the highest priority.
func (s *WebSearchSkill) SetExternalSearch(fn SearchFunc) {
	s.externalFn = fn
}

func (s *WebSearchSkill) Name() string        { return "web_search" }
func (s *WebSearchSkill) Description() string { return "搜索互联网获取最新信息" }
func (s *WebSearchSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":       map[string]any{"type": "string", "description": "搜索关键词"},
			"max_results": map[string]any{"type": "integer", "description": "最大结果数(默认5)"},
		},
		"required": []string{"query"},
	}
}

type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func (s *WebSearchSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	maxResults := 5
	if mr, ok := args["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
		if maxResults > 10 {
			maxResults = 10
		}
	}

	// Fallback chain: Registry → Bing → DuckDuckGo
	var results []searchResult
	var err error

	// 1. Try external search function (Brave/Tavily/SearXNG)
	if s.externalFn != nil {
		results, err = s.externalFn(ctx, query, maxResults)
		if err == nil && len(results) > 0 {
			goto done
		}
	}

	// 2. Try Bing (accessible in China)
	results, err = s.searchBing(ctx, query, maxResults)
	if err == nil && len(results) > 0 {
		goto done
	}

	// 3. Fallback to DuckDuckGo
	results, err = s.searchDDG(ctx, query, maxResults)
	if err != nil {
		return "", fmt.Errorf("all search engines failed: %w", err)
	}

done:

	out, _ := json.Marshal(map[string]any{
		"query":   query,
		"results": results,
		"count":   len(results),
	})
	return string(out), nil
}

// searchDDG scrapes DuckDuckGo HTML search results.
func (s *WebSearchSkill) searchDDG(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	u := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}
	html := string(body)

	return parseDDGResults(html, maxResults), nil
}

var (
	reResultBlock = regexp.MustCompile(`(?s)<a[^>]*class="result__a"[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	reSnippet     = regexp.MustCompile(`(?s)<a[^>]*class="result__snippet"[^>]*>(.*?)</a>`)
	reTag         = regexp.MustCompile(`<[^>]*>`)
)

func parseDDGResults(html string, max int) []searchResult {
	links := reResultBlock.FindAllStringSubmatch(html, max*2)
	snippets := reSnippet.FindAllStringSubmatch(html, max*2)

	var results []searchResult
	for i, m := range links {
		if len(results) >= max {
			break
		}
		rawURL := m[1]
		title := cleanHTML(m[2])
		if title == "" {
			continue
		}

		// DDG wraps URLs in a redirect; extract actual URL
		actualURL := extractDDGURL(rawURL)

		snippet := ""
		if i < len(snippets) && len(snippets[i]) > 1 {
			snippet = cleanHTML(snippets[i][1])
		}

		results = append(results, searchResult{
			Title:   title,
			URL:     actualURL,
			Snippet: snippet,
		})
	}
	return results
}

func extractDDGURL(raw string) string {
	if strings.Contains(raw, "uddg=") {
		if u, err := url.Parse(raw); err == nil {
			if actual := u.Query().Get("uddg"); actual != "" {
				return actual
			}
		}
	}
	return raw
}

func cleanHTML(s string) string {
	s = reTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#x27;", "'")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return strings.TrimSpace(s)
}

// searchBing scrapes Bing search results (accessible in China).
func (s *WebSearchSkill) searchBing(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	u := "https://www.bing.com/search?q=" + url.QueryEscape(query) + "&count=" + fmt.Sprintf("%d", maxResults)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}
	return parseBingResults(string(body), maxResults), nil
}

var (
	reBingResult = regexp.MustCompile(`(?s)<li class="b_algo"[^>]*>(.*?)</li>`)
	reBingTitle  = regexp.MustCompile(`(?s)<h2><a[^>]*href="([^"]+)"[^>]*>(.*?)</a></h2>`)
	reBingSnip   = regexp.MustCompile(`(?s)<p[^>]*>(.*?)</p>`)
)

func parseBingResults(html string, max int) []searchResult {
	blocks := reBingResult.FindAllStringSubmatch(html, max*2)
	var results []searchResult
	for _, block := range blocks {
		if len(results) >= max {
			break
		}
		inner := block[1]
		titleMatch := reBingTitle.FindStringSubmatch(inner)
		if titleMatch == nil {
			continue
		}
		resultURL := titleMatch[1]
		title := cleanHTML(titleMatch[2])
		snippet := ""
		snipMatch := reBingSnip.FindStringSubmatch(inner)
		if snipMatch != nil {
			snippet = cleanHTML(snipMatch[1])
		}
		results = append(results, searchResult{
			Title:   title,
			URL:     resultURL,
			Snippet: snippet,
		})
	}
	return results
}
