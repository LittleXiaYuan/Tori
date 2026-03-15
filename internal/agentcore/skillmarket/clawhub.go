package skillmarket

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RemoteSkill represents a skill fetched from ClawHub.
type RemoteSkill struct {
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	License     string            `json:"license,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Downloads   int64             `json:"downloads"`
	Rating      float64           `json:"rating"`
	Content     string            `json:"content"`           // SKILL.md body
	Permissions []string          `json:"permissions"`       // declared permissions
	Requires    RemoteRequires    `json:"requires,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RemoteRequires lists external dependencies.
type RemoteRequires struct {
	Bins []string `json:"bins,omitempty"` // required binaries (e.g. ["python3", "ffmpeg"])
	Env  []string `json:"env,omitempty"`  // required env vars (e.g. ["OPENAI_API_KEY"])
}

// ClawHubProvider is an HTTP client for the ClawHub skill marketplace API.
type ClawHubProvider struct {
	baseURL    string
	httpClient *http.Client
	cacheDir   string

	mu          sync.Mutex
	lastRequest time.Time
	rateLimit   time.Duration // minimum interval between requests
}

// NewClawHubProvider creates a provider with rate limiting and local caching.
func NewClawHubProvider(baseURL, cacheDir string) *ClawHubProvider {
	if baseURL == "" {
		baseURL = "https://clawhub.ai"
	}
	return &ClawHubProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		cacheDir:  cacheDir,
	rateLimit:  2 * time.Second, // conservative rate limit to avoid 429
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// clawHubSearchResult matches the ClawHub /api/v1/search response item.
type clawHubSearchResult struct {
	Slug        string  `json:"slug"`
	DisplayName string  `json:"displayName"`
	Summary     string  `json:"summary"`
	Version     string  `json:"version"`
	Score       float64 `json:"score"`
	UpdatedAt   int64   `json:"updatedAt"`
}

// clawHubSkillListItem matches the ClawHub /api/v1/skills list response item.
type clawHubSkillListItem struct {
	Slug          string `json:"slug"`
	DisplayName   string `json:"displayName"`
	Summary       string `json:"summary"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	LatestVersion *struct {
		Version   string `json:"version"`
		Changelog string `json:"changelog"`
	} `json:"latestVersion"`
}

// Search queries ClawHub for skills matching the query string.
func (p *ClawHubProvider) Search(query string, limit int) ([]RemoteSkill, error) {
	if limit <= 0 {
		limit = 20
	}
	apiURL := fmt.Sprintf("%s/api/v1/search?q=%s&limit=%d", p.baseURL, url.QueryEscape(query), limit)
	slog.Info("clawhub search", "url", apiURL, "query", query)
	body, err := p.doGet(apiURL)
	if err != nil {
		slog.Error("clawhub search failed", "err", err, "query", query)
		return nil, fmt.Errorf("clawhub search: %w", err)
	}
	slog.Info("clawhub search response", "body_len", len(body), "query", query)
	var resp struct {
		Results []clawHubSearchResult `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("clawhub search decode: %w", err)
	}
	// Convert to our RemoteSkill format (limit results)
	var skills []RemoteSkill
	for i, r := range resp.Results {
		if i >= limit {
			break
		}
		skills = append(skills, RemoteSkill{
			Slug:        r.Slug,
			Name:        r.DisplayName,
			Version:     r.Version,
			Description: r.Summary,
			UpdatedAt:   time.UnixMilli(r.UpdatedAt),
		})
	}
	return skills, nil
}

// Fetch retrieves a single skill's full details by slug.
func (p *ClawHubProvider) Fetch(slug string) (*RemoteSkill, error) {
	// Check cache first
	if cached, err := p.loadCache(slug); err == nil {
		return cached, nil
	}

	url := fmt.Sprintf("%s/api/v1/skills/%s", p.baseURL, slug)
	body, err := p.doGet(url)
	if err != nil {
		return nil, fmt.Errorf("clawhub fetch %q: %w", slug, err)
	}
	var resp struct {
		Skill *struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"displayName"`
			Summary     string `json:"summary"`
			CreatedAt   int64  `json:"createdAt"`
			UpdatedAt   int64  `json:"updatedAt"`
		} `json:"skill"`
		LatestVersion *struct {
			Version   string `json:"version"`
			Changelog string `json:"changelog"`
		} `json:"latestVersion"`
		Owner *struct {
			Handle      string `json:"handle"`
			DisplayName string `json:"displayName"`
		} `json:"owner"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("clawhub fetch decode: %w", err)
	}
	if resp.Skill == nil {
		return nil, fmt.Errorf("skill %q not found", slug)
	}
	skill := &RemoteSkill{
		Slug:        resp.Skill.Slug,
		Name:        resp.Skill.DisplayName,
		Description: resp.Skill.Summary,
		CreatedAt:   time.UnixMilli(resp.Skill.CreatedAt),
		UpdatedAt:   time.UnixMilli(resp.Skill.UpdatedAt),
	}
	if resp.LatestVersion != nil {
		skill.Version = resp.LatestVersion.Version
	}
	if resp.Owner != nil {
		skill.Author = resp.Owner.Handle
		if resp.Owner.DisplayName != "" {
			skill.Author = resp.Owner.DisplayName
		}
	}
	// Write to cache
	p.writeCache(slug, body)
	return skill, nil
}

// Download retrieves the raw skill package content (SKILL.md + assets).
func (p *ClawHubProvider) Download(slug, version string) ([]byte, error) {
	params := fmt.Sprintf("slug=%s", slug)
	if version != "" {
		params += "&version=" + version
	}
	url := fmt.Sprintf("%s/api/v1/download?%s", p.baseURL, params)
	data, err := p.doGet(url)
	if err != nil {
		return nil, fmt.Errorf("clawhub download %q@%s: %w", slug, version, err)
	}
	return data, nil
}

// ListResult holds a page of skills plus a cursor for the next page.
type ListResult struct {
	Skills     []RemoteSkill `json:"skills"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

// Trending returns recently updated skills from ClawHub (sorted by updatedAt desc).
// ClawHub doesn't have a dedicated trending endpoint, so we use the skills list.
func (p *ClawHubProvider) Trending(limit int) ([]RemoteSkill, error) {
	result, err := p.TrendingPaged(limit, "")
	if err != nil {
		return nil, err
	}
	return result.Skills, nil
}

// TrendingPaged returns a page of skills with cursor support.
func (p *ClawHubProvider) TrendingPaged(limit int, cursor string) (*ListResult, error) {
	if limit <= 0 {
		limit = 20
	}
	url := fmt.Sprintf("%s/api/v1/skills?limit=%d&sort=downloads", p.baseURL, limit)
	if cursor != "" {
		url += "&cursor=" + cursor
	}
	body, err := p.doGet(url)
	if err != nil {
		return nil, fmt.Errorf("clawhub trending: %w", err)
	}
	var resp struct {
		Items      []clawHubSkillListItem `json:"items"`
		NextCursor *string                `json:"nextCursor"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("clawhub trending decode: %w", err)
	}
	result := &ListResult{}
	if resp.NextCursor != nil {
		result.NextCursor = *resp.NextCursor
	}
	for _, item := range resp.Items {
		s := RemoteSkill{
			Slug:        item.Slug,
			Name:        item.DisplayName,
			Description: item.Summary,
			CreatedAt:   time.UnixMilli(item.CreatedAt),
			UpdatedAt:   time.UnixMilli(item.UpdatedAt),
		}
		if item.LatestVersion != nil {
			s.Version = item.LatestVersion.Version
		}
		result.Skills = append(result.Skills, s)
	}
	return result, nil
}

// ── HTTP + rate limiting ──

func (p *ClawHubProvider) doGet(url string) ([]byte, error) {
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
		// Retry once after a delay
		resp.Body.Close()
		retryAfter := 3 * time.Second
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if sec, err := strconv.Atoi(ra); err == nil {
				retryAfter = time.Duration(sec) * time.Second
			}
		}
		slog.Warn("clawhub 429, retrying", "retry_after", retryAfter)
		time.Sleep(retryAfter)
		resp2, err2 := p.httpClient.Do(req)
		if err2 != nil {
			return nil, fmt.Errorf("clawhub retry: %w", err2)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("clawhub retry HTTP %d", resp2.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp2.Body, 10<<20))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB max
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (p *ClawHubProvider) throttle() {
	p.mu.Lock()
	defer p.mu.Unlock()
	elapsed := time.Since(p.lastRequest)
	if elapsed < p.rateLimit {
		time.Sleep(p.rateLimit - elapsed)
	}
	p.lastRequest = time.Now()
}

// ── Local cache ──

func (p *ClawHubProvider) cachePathFor(slug string) string {
	return filepath.Join(p.cacheDir, slug+".json")
}

func (p *ClawHubProvider) loadCache(slug string) (*RemoteSkill, error) {
	if p.cacheDir == "" {
		return nil, fmt.Errorf("no cache dir")
	}
	data, err := os.ReadFile(p.cachePathFor(slug))
	if err != nil {
		return nil, err
	}
	// Cache entries expire after 1 hour
	info, err := os.Stat(p.cachePathFor(slug))
	if err != nil || time.Since(info.ModTime()) > 1*time.Hour {
		return nil, fmt.Errorf("cache expired")
	}
	var skill RemoteSkill
	if err := json.Unmarshal(data, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

func (p *ClawHubProvider) writeCache(slug string, data []byte) {
	if p.cacheDir == "" {
		return
	}
	if err := os.MkdirAll(p.cacheDir, 0755); err != nil {
		slog.Warn("clawhub cache: mkdir failed", "err", err)
		return
	}
	if err := os.WriteFile(p.cachePathFor(slug), data, 0644); err != nil {
		slog.Warn("clawhub cache: write failed", "err", err)
	}
}
