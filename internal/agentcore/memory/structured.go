package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	memoryDateLayout    = "2006-01-02"
	entryStartPrefix    = "<!-- MEMENTRY "
	entryStartSuffix    = " -->"
	entryEndMarker      = "<!-- /MEMENTRY -->"
	memFileHeader       = "# Memory %s\n\n"
)

// MemoryRecord is a single structured memory entry.
type MemoryRecord struct {
	ID        string `json:"id"`
	Topic     string `json:"topic,omitempty"`
	Content   string `json:"content"`
	Hash      string `json:"hash"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// StructuredStore persists memory as Markdown day-files with hash-based dedup.
// Directory structure: <baseDir>/<tenantID>/YYYY-MM-DD.md
type StructuredStore struct {
	baseDir string
	mu      sync.Mutex
}

// NewStructuredStore creates a structured memory store.
func NewStructuredStore(baseDir string) *StructuredStore {
	os.MkdirAll(baseDir, 0o755)
	return &StructuredStore{baseDir: baseDir}
}

// Add writes a memory record to the day file, deduplicating by hash.
func (s *StructuredStore) Add(tenantID, topic, content string) (*MemoryRecord, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("content is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	date := now.Format(memoryDateLayout)
	hash := generateHash(topic, content)

	dir := filepath.Join(s.baseDir, tenantID)
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, date+".md")

	// Load existing records
	existing := s.loadDayFile(path)

	// Check for duplicate hash
	for _, r := range existing {
		if r.Hash == hash {
			return &r, nil // already exists
		}
	}

	record := MemoryRecord{
		ID:        fmt.Sprintf("mem_%d", now.UnixNano()),
		Topic:     strings.TrimSpace(topic),
		Content:   strings.TrimSpace(content),
		Hash:      hash,
		CreatedAt: now.Format(time.RFC3339),
		UpdatedAt: now.Format(time.RFC3339),
	}

	existing = append(existing, record)
	if err := s.writeDayFile(path, date, existing); err != nil {
		return nil, err
	}
	return &record, nil
}

// GetDay returns all memory records for a given date.
func (s *StructuredStore) GetDay(tenantID, date string) []MemoryRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.baseDir, tenantID, date+".md")
	return s.loadDayFile(path)
}

// Search returns records matching the query across all days for a tenant.
func (s *StructuredStore) Search(tenantID, query string, limit int) []MemoryRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(s.baseDir, tenantID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	query = strings.ToLower(strings.TrimSpace(query))
	var results []MemoryRecord

	// Search from newest to oldest
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		records := s.loadDayFile(path)
		for _, r := range records {
			if strings.Contains(strings.ToLower(r.Content), query) ||
				strings.Contains(strings.ToLower(r.Topic), query) {
				results = append(results, r)
				if limit > 0 && len(results) >= limit {
					return results
				}
			}
		}
	}
	return results
}

// ListDays returns available date strings for a tenant.
func (s *StructuredStore) ListDays(tenantID string) []string {
	dir := filepath.Join(s.baseDir, tenantID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var days []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			days = append(days, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(days)))
	return days
}

func (s *StructuredStore) loadDayFile(path string) []MemoryRecord {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseDayMarkdown(string(data))
}

func (s *StructuredStore) writeDayFile(path, date string, records []MemoryRecord) error {
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt < records[j].CreatedAt
	})

	var b strings.Builder
	fmt.Fprintf(&b, memFileHeader, date)

	for _, r := range records {
		meta := map[string]string{
			"id":   r.ID,
			"hash": r.Hash,
		}
		if r.Topic != "" {
			meta["topic"] = r.Topic
		}
		if r.CreatedAt != "" {
			meta["created_at"] = r.CreatedAt
		}
		if r.UpdatedAt != "" {
			meta["updated_at"] = r.UpdatedAt
		}
		rawMeta, _ := json.Marshal(meta)
		b.WriteString(entryStartPrefix)
		b.Write(rawMeta)
		b.WriteString(entryStartSuffix)
		b.WriteString("\n")
		b.WriteString(r.Content)
		b.WriteString("\n")
		b.WriteString(entryEndMarker)
		b.WriteString("\n\n")
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func parseDayMarkdown(content string) []MemoryRecord {
	lines := strings.Split(content, "\n")
	var records []MemoryRecord

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, entryStartPrefix) || !strings.HasSuffix(line, entryStartSuffix) {
			continue
		}
		metaJSON := strings.TrimSuffix(strings.TrimPrefix(line, entryStartPrefix), entryStartSuffix)
		var meta map[string]string
		if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
			continue
		}

		start := i + 1
		end := start
		for ; end < len(lines); end++ {
			if strings.TrimSpace(lines[end]) == entryEndMarker {
				break
			}
		}
		if end >= len(lines) {
			break
		}

		body := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
		records = append(records, MemoryRecord{
			ID:        meta["id"],
			Topic:     meta["topic"],
			Content:   body,
			Hash:      meta["hash"],
			CreatedAt: meta["created_at"],
			UpdatedAt: meta["updated_at"],
		})
		i = end
	}
	return records
}

func generateHash(topic, content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(topic) + "\n" + strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:16]) // first 16 bytes = 32 hex chars
}
