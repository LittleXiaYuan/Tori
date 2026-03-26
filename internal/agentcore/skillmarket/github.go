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

// GitHubSkillProvider installs skills directly from GitHub repositories.
// It reads SKILL.md from a GitHub repo path, supporting two formats:
//
//   - "owner/repo"          → reads SKILL.md from repo root
//   - "owner/repo/path/dir" → reads SKILL.md from the specified subdirectory
//
// This enables users to publish skills on any public GitHub repo and
// install them with: POST /api/skillhub/install {"slug": "owner/repo"}
type GitHubSkillProvider struct {
	mu          sync.Mutex
	lastRequest time.Time
	rateLimit   time.Duration
	client      *http.Client
	token       string // optional GitHub personal access token for higher rate limits
}

// NewGitHubSkillProvider creates a provider for GitHub-hosted skills.
func NewGitHubSkillProvider(githubToken string) *GitHubSkillProvider {
	return &GitHubSkillProvider{
		rateLimit: 500 * time.Millisecond,
		token:     githubToken,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Name identifies this provider.
func (p *GitHubSkillProvider) Name() string { return "github" }

// IsGitHubSlug returns true if the slug looks like a GitHub repo reference (contains "/").
// This distinguishes GitHub slugs from ClawHub/ToriHub slugs.
func IsGitHubSlug(slug string) bool {
	return strings.Count(slug, "/") >= 1 && !strings.HasPrefix(slug, "http")
}

// Fetch retrieves a skill's metadata by reading its SKILL.md from GitHub.
// slug format: "owner/repo" or "owner/repo/path/to/skill"
func (p *GitHubSkillProvider) Fetch(slug string) (*RemoteSkill, error) {
	owner, repo, subPath := parseGitHubSlug(slug)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid GitHub skill slug %q: expected owner/repo or owner/repo/path", slug)
	}

	content, err := p.fetchRawFile(owner, repo, subPath, "SKILL.md")
	if err != nil {
		return nil, fmt.Errorf("github fetch SKILL.md for %q: %w", slug, err)
	}

	skill := &RemoteSkill{
		Slug:      slug,
		Name:      repo,
		Content:   content,
		Author:    owner,
		UpdatedAt: time.Now(),
	}

	// Try to extract name and description from SKILL.md frontmatter or first heading
	skill.Name, skill.Description = extractSkillMeta(content)
	if skill.Name == "" {
		skill.Name = repo
	}

	// Try to get repo metadata from GitHub API
	if meta, err := p.fetchRepoMeta(owner, repo); err == nil {
		if meta.Description != "" && skill.Description == "" {
			skill.Description = meta.Description
		}
		if meta.Stars > 0 {
			skill.Downloads = int64(meta.Stars)
		}
		if meta.DefaultBranch != "" {
			skill.Version = "main"
			if meta.DefaultBranch != "main" {
				skill.Version = meta.DefaultBranch
			}
		}
	}

	slog.Info("github skill fetched", "slug", slug, "name", skill.Name)
	return skill, nil
}

// Search is not supported by GitHub provider (no central index).
// Returns an empty list rather than an error so callers can gracefully skip.
func (p *GitHubSkillProvider) Search(query string, limit int) ([]RemoteSkill, error) {
	return nil, nil
}

// Trending is not supported by GitHub provider.
func (p *GitHubSkillProvider) Trending(limit int) ([]RemoteSkill, error) {
	return nil, nil
}

// DownloadContent returns the full SKILL.md content as bytes for installation.
func (p *GitHubSkillProvider) DownloadContent(slug string) ([]byte, error) {
	skill, err := p.Fetch(slug)
	if err != nil {
		return nil, err
	}
	return []byte(skill.Content), nil
}

// ── Internal helpers ──

// parseGitHubSlug splits "owner/repo" or "owner/repo/path/to/dir" into parts.
func parseGitHubSlug(slug string) (owner, repo, subPath string) {
	parts := strings.SplitN(slug, "/", 3)
	if len(parts) < 2 {
		return "", "", ""
	}
	owner = parts[0]
	repo = parts[1]
	if len(parts) == 3 {
		subPath = parts[2]
	}
	return
}

// fetchRawFile retrieves a file from a GitHub repo via raw.githubusercontent.com.
func (p *GitHubSkillProvider) fetchRawFile(owner, repo, subPath, filename string) (string, error) {
	var rawURL string
	if subPath == "" {
		rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/%s",
			owner, repo, filename)
	} else {
		rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/%s/%s",
			owner, repo, strings.Trim(subPath, "/"), filename)
	}

	body, err := p.doGet(rawURL)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type githubRepoMeta struct {
	Description   string `json:"description"`
	Stars         int    `json:"stargazers_count"`
	DefaultBranch string `json:"default_branch"`
}

func (p *GitHubSkillProvider) fetchRepoMeta(owner, repo string) (*githubRepoMeta, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	body, err := p.doGet(url)
	if err != nil {
		return nil, err
	}
	var meta githubRepoMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// extractSkillMeta extracts name and description from SKILL.md content.
// Looks for "# Name" heading and first paragraph.
func extractSkillMeta(content string) (name, description string) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			name = strings.TrimPrefix(line, "# ")
			// Description: next non-empty line that doesn't start with #
			for _, nextLine := range lines[i+1:] {
				nextLine = strings.TrimSpace(nextLine)
				if nextLine != "" && !strings.HasPrefix(nextLine, "#") && !strings.HasPrefix(nextLine, "<!--") {
					description = nextLine
					break
				}
			}
			return
		}
	}
	return
}

func (p *GitHubSkillProvider) doGet(rawURL string) ([]byte, error) {
	p.mu.Lock()
	elapsed := time.Since(p.lastRequest)
	if elapsed < p.rateLimit {
		time.Sleep(p.rateLimit - elapsed)
	}
	p.lastRequest = time.Now()
	p.mu.Unlock()

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YunqueAgent/1.0")
	req.Header.Set("Accept", "application/json")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found: %s", rawURL)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited by GitHub API (status %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API HTTP %d: %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB max for skill files
}
