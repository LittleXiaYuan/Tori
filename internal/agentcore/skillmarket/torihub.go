package skillmarket

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HubProvider is the common interface for remote skill hub providers (ClawHub, ToriHub, etc.).
type HubProvider interface {
	// Name returns the display name of this hub (e.g. "clawhub", "torihub").
	Name() string
	// Search queries the hub for skills matching the query.
	Search(query string, limit int) ([]RemoteSkill, error)
	// Fetch retrieves a single skill by slug.
	Fetch(slug string) (*RemoteSkill, error)
	// Trending returns currently popular skills.
	Trending(limit int) ([]RemoteSkill, error)
}

// Ensure both providers implement HubProvider.
var _ HubProvider = (*ClawHubProvider)(nil)
var _ HubProvider = (*ToriHubProvider)(nil)

// Name returns "clawhub".
func (p *ClawHubProvider) Name() string { return "clawhub" }

// ToriHubProvider is an HTTP client for the ToriHub skill marketplace API.
// ToriHub is the Yunque project's own skill hub — API-compatible with ClawHub.
type ToriHubProvider struct {
	baseURL    string
	httpClient *http.Client

	mu          sync.Mutex
	lastRequest time.Time
	rateLimit   time.Duration
}

// NewToriHubProvider creates a ToriHub provider.
func NewToriHubProvider(baseURL string) *ToriHubProvider {
	if baseURL == "" {
		baseURL = "https://torihub.yunque.dev/api/v1"
	}
	return &ToriHubProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		rateLimit: 500 * time.Millisecond,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns "torihub".
func (p *ToriHubProvider) Name() string { return "torihub" }

// Search queries ToriHub for skills matching the query.
func (p *ToriHubProvider) Search(query string, limit int) ([]RemoteSkill, error) {
	if limit <= 0 {
		limit = 20
	}
	url := fmt.Sprintf("%s/skills/search?q=%s&limit=%d", p.baseURL, query, limit)
	body, err := p.doGet(url)
	if err != nil {
		return nil, fmt.Errorf("torihub search: %w", err)
	}
	var resp struct {
		Skills []RemoteSkill `json:"skills"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("torihub search decode: %w", err)
	}
	return resp.Skills, nil
}

// Fetch retrieves a single skill's full details by slug.
func (p *ToriHubProvider) Fetch(slug string) (*RemoteSkill, error) {
	url := fmt.Sprintf("%s/skills/%s", p.baseURL, slug)
	body, err := p.doGet(url)
	if err != nil {
		return nil, fmt.Errorf("torihub fetch %q: %w", slug, err)
	}
	var skill RemoteSkill
	if err := json.Unmarshal(body, &skill); err != nil {
		return nil, fmt.Errorf("torihub fetch decode: %w", err)
	}
	return &skill, nil
}

// Trending returns currently popular skills from ToriHub.
func (p *ToriHubProvider) Trending(limit int) ([]RemoteSkill, error) {
	if limit <= 0 {
		limit = 20
	}
	url := fmt.Sprintf("%s/skills/trending?limit=%d", p.baseURL, limit)
	body, err := p.doGet(url)
	if err != nil {
		return nil, fmt.Errorf("torihub trending: %w", err)
	}
	var resp struct {
		Skills []RemoteSkill `json:"skills"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("torihub trending decode: %w", err)
	}
	return resp.Skills, nil
}

func (p *ToriHubProvider) doGet(url string) ([]byte, error) {
	p.throttle()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YunqueAgent/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found (404)")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited by ToriHub (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (p *ToriHubProvider) throttle() {
	p.mu.Lock()
	defer p.mu.Unlock()
	elapsed := time.Since(p.lastRequest)
	if elapsed < p.rateLimit {
		time.Sleep(p.rateLimit - elapsed)
	}
	p.lastRequest = time.Now()
}

// AdaptToriHub converts a ToriHub RemoteSkill into our internal AdaptedSkill.
func AdaptToriHub(remote RemoteSkill) (*AdaptedSkill, error) {
	adapted, err := AdaptClawHub(remote) // same format
	if err != nil {
		return nil, err
	}
	adapted.Source = SourceToriHub
	return adapted, nil
}

func init() {
	// Log that ToriHub provider is available.
	slog.Debug("torihub provider registered")
}
