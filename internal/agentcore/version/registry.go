// Package version provides plugin/tool version management with remote update checking.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Component represents a versioned component (plugin, tool, module).
type Component struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version,omitempty"`
	Source         string    `json:"source,omitempty"` // "builtin", "plugin", "mcp"
	UpdateURL      string    `json:"update_url,omitempty"`
	LastChecked    time.Time `json:"last_checked,omitempty"`
	UpdateAvail    bool      `json:"update_available"`
}

// Registry tracks component versions and checks for updates.
type Registry struct {
	mu         sync.RWMutex
	components map[string]*Component
	httpClient *http.Client
}

// NewRegistry creates a version registry.
func NewRegistry() *Registry {
	return &Registry{
		components: make(map[string]*Component),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Register adds or updates a component.
func (r *Registry) Register(c Component) {
	if c.ID == "" {
		c.ID = c.Name
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.components[c.ID] = &c
}

// Get returns a component by ID.
func (r *Registry) Get(id string) (*Component, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.components[id]
	if !ok {
		return nil, false
	}
	copy := *c
	return &copy, true
}

// List returns all registered components.
func (r *Registry) List() []Component {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Component, 0, len(r.components))
	for _, c := range r.components {
		result = append(result, *c)
	}
	return result
}

// Remove removes a component by ID.
func (r *Registry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.components[id]; !ok {
		return false
	}
	delete(r.components, id)
	return true
}

// Count returns total component count.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.components)
}

// UpdatesAvailable returns components with pending updates.
func (r *Registry) UpdatesAvailable() []Component {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Component
	for _, c := range r.components {
		if c.UpdateAvail {
			result = append(result, *c)
		}
	}
	return result
}

// CheckUpdate checks a single component for updates via its UpdateURL.
// The remote endpoint should return JSON: {"version": "x.y.z"}
func (r *Registry) CheckUpdate(ctx context.Context, id string) (*Component, error) {
	r.mu.RLock()
	c, ok := r.components[id]
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("component not found: %s", id)
	}
	url := c.UpdateURL
	current := c.CurrentVersion
	r.mu.RUnlock()

	if url == "" {
		return nil, fmt.Errorf("no update URL for %s", id)
	}

	latest, err := r.fetchRemoteVersion(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("check update %s: %w", id, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	comp, ok := r.components[id]
	if !ok {
		return nil, fmt.Errorf("component removed during check: %s", id)
	}
	comp.LatestVersion = latest
	comp.LastChecked = time.Now()
	comp.UpdateAvail = latest != "" && latest != current && CompareVersions(latest, current) > 0

	copy := *comp
	return &copy, nil
}

// CheckAllUpdates checks all components with UpdateURL for updates.
func (r *Registry) CheckAllUpdates(ctx context.Context) []Component {
	r.mu.RLock()
	ids := make([]string, 0, len(r.components))
	for id, c := range r.components {
		if c.UpdateURL != "" {
			ids = append(ids, id)
		}
	}
	r.mu.RUnlock()

	var results []Component
	for _, id := range ids {
		if c, err := r.CheckUpdate(ctx, id); err == nil && c.UpdateAvail {
			results = append(results, *c)
		}
	}
	return results
}

// SetVersion updates a component's current version (e.g., after upgrade).
func (r *Registry) SetVersion(id, version string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.components[id]
	if !ok {
		return false
	}
	c.CurrentVersion = version
	if c.LatestVersion == version {
		c.UpdateAvail = false
	}
	return true
}

func (r *Registry) fetchRemoteVersion(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.Version, nil
}

// CompareVersions compares two semver-like version strings.
// Returns: >0 if a > b, 0 if a == b, <0 if a < b.
func CompareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] - pb[i]
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	var parts [3]int
	idx := 0
	num := 0
	started := false
	for _, ch := range v {
		if ch >= '0' && ch <= '9' {
			num = num*10 + int(ch-'0')
			started = true
		} else if ch == '.' && started {
			if idx < 3 {
				parts[idx] = num
			}
			idx++
			num = 0
		}
	}
	if idx < 3 && started {
		parts[idx] = num
	}
	return parts
}
