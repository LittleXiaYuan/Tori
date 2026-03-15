package skillmarket

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Category classifies skills by domain.
type Category string

const (
	CatGeneral    Category = "general"
	CatEducation  Category = "education"
	CatCoding     Category = "coding"
	CatData       Category = "data"
	CatMedia      Category = "media"
	CatSearch     Category = "search"
	CatLanguage   Category = "language"
	CatProductivity Category = "productivity"
	CatCustom     Category = "custom"
)

// SkillMeta is enriched metadata for a skill in the marketplace.
type SkillMeta struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`     // semver e.g. "1.2.0"
	Description string    `json:"description"`
	Author      string    `json:"author"`
	Category    Category  `json:"category"`
	Tags        []string  `json:"tags"`
	License     string    `json:"license,omitempty"`
	Homepage    string    `json:"homepage,omitempty"`
	Deprecated  bool      `json:"deprecated,omitempty"`
	Installs    int64     `json:"installs"`    // download/install count
	Rating      float64   `json:"rating"`      // 0-5
	RatingCount int       `json:"rating_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MinVersion  string    `json:"min_version,omitempty"` // min agent version required
	Dependencies []string `json:"dependencies,omitempty"` // other skill names required
}

// Market is a skill marketplace with versioning, metadata, and discovery.
type Market struct {
	mu      sync.RWMutex
	skills  map[string]*SkillMeta // keyed by name
	dataDir string
}

// NewMarket creates a new skill marketplace.
func NewMarket(dataDir string) *Market {
	return &Market{
		skills:  make(map[string]*SkillMeta),
		dataDir: dataDir,
	}
}

// Publish registers or updates a skill in the marketplace.
func (m *Market) Publish(meta SkillMeta) error {
	if meta.Name == "" {
		return fmt.Errorf("skill name required")
	}
	if meta.Version == "" {
		return fmt.Errorf("skill version required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.skills[meta.Name]
	if exists {
		// Update: preserve install count and ratings
		meta.Installs = existing.Installs
		meta.Rating = existing.Rating
		meta.RatingCount = existing.RatingCount
		meta.CreatedAt = existing.CreatedAt
	} else {
		meta.CreatedAt = time.Now()
	}
	meta.UpdatedAt = time.Now()

	m.skills[meta.Name] = &meta
	return nil
}

// Get returns a skill's metadata.
func (m *Market) Get(name string) (*SkillMeta, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.skills[name]
	if !ok {
		return nil, false
	}
	copy := *s
	return &copy, true
}

// RecordInstall increments the install count.
func (m *Market) RecordInstall(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.skills[name]; ok {
		s.Installs++
	}
}

// Rate adds a rating to a skill.
func (m *Market) Rate(name string, score float64) error {
	if score < 0 || score > 5 {
		return fmt.Errorf("rating must be 0-5")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.skills[name]
	if !ok {
		return fmt.Errorf("skill not found: %s", name)
	}
	// Incremental average
	total := s.Rating * float64(s.RatingCount)
	s.RatingCount++
	s.Rating = (total + score) / float64(s.RatingCount)
	return nil
}

// Search finds skills matching a query string in name, description, or tags.
func (m *Market) Search(query string) []SkillMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q := strings.ToLower(query)
	var results []SkillMeta
	for _, s := range m.skills {
		if s.Deprecated {
			continue
		}
		if strings.Contains(strings.ToLower(s.Name), q) ||
			strings.Contains(strings.ToLower(s.Description), q) ||
			containsTag(s.Tags, q) {
			results = append(results, *s)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Installs > results[j].Installs
	})
	return results
}

// FindByCategory returns skills in a given category.
func (m *Market) FindByCategory(cat Category) []SkillMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []SkillMeta
	for _, s := range m.skills {
		if s.Category == cat && !s.Deprecated {
			results = append(results, *s)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Rating > results[j].Rating
	})
	return results
}

// FindByTag returns skills that have a specific tag.
func (m *Market) FindByTag(tag string) []SkillMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tag = strings.ToLower(tag)
	var results []SkillMeta
	for _, s := range m.skills {
		if s.Deprecated {
			continue
		}
		if containsTag(s.Tags, tag) {
			results = append(results, *s)
		}
	}
	return results
}

// TopRated returns the top N highest rated skills.
func (m *Market) TopRated(n int) []SkillMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []SkillMeta
	for _, s := range m.skills {
		if !s.Deprecated && s.RatingCount > 0 {
			all = append(all, *s)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Rating > all[j].Rating
	})
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// MostPopular returns the top N most installed skills.
func (m *Market) MostPopular(n int) []SkillMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []SkillMeta
	for _, s := range m.skills {
		if !s.Deprecated {
			all = append(all, *s)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Installs > all[j].Installs
	})
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// Deprecate marks a skill as deprecated.
func (m *Market) Deprecate(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.skills[name]
	if !ok {
		return false
	}
	s.Deprecated = true
	return true
}

// Remove deletes a skill from the marketplace.
func (m *Market) Remove(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.skills[name]; !ok {
		return false
	}
	delete(m.skills, name)
	return true
}

// All returns all non-deprecated skills.
func (m *Market) All() []SkillMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []SkillMeta
	for _, s := range m.skills {
		if !s.Deprecated {
			out = append(out, *s)
		}
	}
	return out
}

// Count returns total skill count.
func (m *Market) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.skills)
}

// Stats returns marketplace statistics.
func (m *Market) Stats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cats := make(map[Category]int)
	var totalInstalls int64
	deprecated := 0
	for _, s := range m.skills {
		cats[s.Category]++
		totalInstalls += s.Installs
		if s.Deprecated {
			deprecated++
		}
	}
	return map[string]any{
		"total":          len(m.skills),
		"deprecated":     deprecated,
		"total_installs": totalInstalls,
		"categories":     cats,
	}
}

// SaveTo persists marketplace data to a JSON file.
func (m *Market) SaveTo(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []SkillMeta
	for _, s := range m.skills {
		all = append(all, *s)
	}

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadFrom loads marketplace data from a JSON file.
func (m *Market) LoadFrom(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var skills []SkillMeta
	if err := json.Unmarshal(data, &skills); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range skills {
		m.skills[skills[i].Name] = &skills[i]
	}
	return nil
}

func containsTag(tags []string, query string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}
	return false
}
