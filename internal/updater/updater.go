package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/version"
)

// Config for the update checker.
type Config struct {
	Enabled       bool
	RepoOwner     string
	RepoName      string
	CheckInterval time.Duration
}

// DefaultConfig returns defaults pointing to the GitHub repo.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		RepoOwner:     "user",
		RepoName:      "yunque-agent",
		CheckInterval: 6 * time.Hour,
	}
}

// ReleaseInfo describes an available update.
type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

// Checker periodically checks GitHub Releases for a newer version.
type Checker struct {
	cfg    Config
	mu     sync.RWMutex
	latest *ReleaseInfo
	hasNew bool
}

// NewChecker creates an update checker.
func NewChecker(cfg Config) *Checker {
	return &Checker{cfg: cfg}
}

// Start begins periodic checks.
func (c *Checker) Start(ctx context.Context) {
	if !c.cfg.Enabled {
		slog.Info("auto-update: disabled")
		return
	}

	c.checkOnce()

	ticker := time.NewTicker(c.cfg.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkOnce()
		}
	}
}

// Latest returns the latest release info and whether it's newer than current.
func (c *Checker) Latest() (*ReleaseInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest, c.hasNew
}

func (c *Checker) checkOnce() {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
		c.cfg.RepoOwner, c.cfg.RepoName)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "YunqueAgent/"+version.Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Debug("auto-update: check failed", "err", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return
	}

	var rel ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return
	}

	c.mu.Lock()
	c.latest = &rel
	c.hasNew = isNewer(rel.TagName, version.Version)
	c.mu.Unlock()

	if c.hasNew {
		slog.Info("auto-update: new version available",
			"current", version.Version,
			"latest", rel.TagName,
			"url", rel.HTMLURL)
	}
}

// isNewer returns true if remoteTag is a newer semver than current.
func isNewer(remoteTag, current string) bool {
	remote := strings.TrimPrefix(remoteTag, "v")
	current = strings.TrimPrefix(current, "v")

	if strings.Contains(current, "-dev") || strings.Contains(current, "-dirty") {
		return false
	}

	rParts := strings.SplitN(remote, ".", 3)
	cParts := strings.SplitN(current, ".", 3)

	for i := 0; i < 3; i++ {
		var r, c string
		if i < len(rParts) {
			r = rParts[i]
		}
		if i < len(cParts) {
			c = cParts[i]
		}
		// Strip pre-release suffix for comparison
		r = strings.SplitN(r, "-", 2)[0]
		c = strings.SplitN(c, "-", 2)[0]
		if r > c {
			return true
		}
		if r < c {
			return false
		}
	}
	return false
}
