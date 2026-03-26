package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Result is a single search result.
type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Provider performs web searches.
type Provider interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}

// Registry manages multiple search providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	primary   string
}

// NewRegistry creates a search provider registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds a provider.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
	if r.primary == "" {
		r.primary = p.Name()
	}
}

// SetPrimary sets the default search provider.
func (r *Registry) SetPrimary(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.providers[name]; !ok {
		return false
	}
	r.primary = name
	return true
}

// Search uses the primary provider.
func (r *Registry) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	r.mu.RLock()
	p, ok := r.providers[r.primary]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no search provider configured")
	}
	return p.Search(ctx, query, limit)
}

// SearchWith uses a named provider.
func (r *Registry) SearchWith(ctx context.Context, name, query string, limit int) ([]Result, error) {
	r.mu.RLock()
	p, ok := r.providers[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("search provider not found: %s", name)
	}
	return p.Search(ctx, query, limit)
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	return names
}

// --- Brave Search ---

type BraveProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	logger  *slog.Logger
}

func NewBrave(apiKey string) *BraveProvider {
	return &BraveProvider{
		apiKey:  apiKey,
		baseURL: "https://api.search.brave.com/res/v1/web/search",
		client:  &http.Client{Timeout: 15 * time.Second},
		logger:  slog.Default().With(slog.String("search", "brave")),
	}
}

func (b *BraveProvider) Name() string { return "brave" }

func (b *BraveProvider) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 5
	}
	u, _ := url.Parse(b.baseURL)
	q := u.Query()
	q.Set("q", query)
	q.Set("count", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", b.apiKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("brave search error %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		results = append(results, Result{Title: r.Title, URL: r.URL, Snippet: r.Description})
	}
	return results, nil
}

// --- Tavily Search ---

type TavilyProvider struct {
	apiKey string
	client *http.Client
}

func NewTavily(apiKey string) *TavilyProvider {
	return &TavilyProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TavilyProvider) Name() string { return "tavily" }

func (t *TavilyProvider) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 5
	}
	payload, _ := json.Marshal(map[string]any{
		"api_key":     t.apiKey,
		"query":       query,
		"max_results": limit,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tavily error %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&parsed)
	results := make([]Result, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		results = append(results, Result{Title: r.Title, URL: r.URL, Snippet: r.Content})
	}
	return results, nil
}

// --- SearXNG (self-hosted) ---

type SearXNGProvider struct {
	baseURL string
	client  *http.Client
}

func NewSearXNG(baseURL string) *SearXNGProvider {
	return &SearXNGProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *SearXNGProvider) Name() string { return "searxng" }

func (s *SearXNGProvider) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	u, _ := url.Parse(s.baseURL + "/search")
	q := u.Query()
	q.Set("q", query)
	q.Set("format", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&parsed)

	if limit <= 0 || limit > len(parsed.Results) {
		limit = len(parsed.Results)
	}
	results := make([]Result, 0, limit)
	for i := 0; i < limit; i++ {
		r := parsed.Results[i]
		results = append(results, Result{Title: r.Title, URL: r.URL, Snippet: r.Content})
	}
	return results, nil
}

// FormatResults returns a text summary of search results for LLM consumption.
func FormatResults(results []Result) string {
	if len(results) == 0 {
		return "No search results found."
	}
	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet))
	}
	return sb.String()
}

// ──────────────────────────────────────────────
// GenericHTTP — plugin-registered search provider
// ──────────────────────────────────────────────

// GenericHTTPProvider is a search provider registered by plugins at runtime.
// It calls a remote HTTP endpoint that returns JSON results.
type GenericHTTPProvider struct {
	name       string
	baseURL    string
	apiKey     string
	searchPath string
	client     *http.Client
}

// NewGenericHTTP creates a generic HTTP search provider for plugin extensions.
func NewGenericHTTP(name, baseURL, apiKey, searchPath string) *GenericHTTPProvider {
	if searchPath == "" {
		searchPath = "/search"
	}
	return &GenericHTTPProvider{
		name:       name,
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		searchPath: searchPath,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *GenericHTTPProvider) Name() string { return p.name }

func (p *GenericHTTPProvider) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	reqURL := fmt.Sprintf("%s%s?q=%s&limit=%d",
		p.baseURL, p.searchPath, url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generic search %q: %w", p.name, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("generic search %q HTTP %d: %s", p.name, resp.StatusCode, string(body))
	}

	// Try common response formats
	var results []Result

	// Format 1: {"results": [...]}
	var wrapped struct {
		Results []Result `json:"results"`
	}
	if json.Unmarshal(body, &wrapped) == nil && len(wrapped.Results) > 0 {
		return wrapped.Results, nil
	}

	// Format 2: direct array [...]
	if json.Unmarshal(body, &results) == nil {
		return results, nil
	}

	return nil, fmt.Errorf("generic search %q: unknown response format", p.name)
}
