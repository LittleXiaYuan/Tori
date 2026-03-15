package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStructuredStoreAddAndGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStructuredStore(dir)

	rec, err := store.Add("tenant1", "preference", "User prefers Go language")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if rec.ID == "" || rec.Hash == "" {
		t.Fatal("expected non-empty ID and Hash")
	}
	if rec.Content != "User prefers Go language" {
		t.Fatalf("expected content, got %q", rec.Content)
	}
}

func TestStructuredStoreDedup(t *testing.T) {
	dir := t.TempDir()
	store := NewStructuredStore(dir)

	r1, _ := store.Add("t1", "fact", "sky is blue")
	r2, _ := store.Add("t1", "fact", "sky is blue")

	if r1.Hash != r2.Hash {
		t.Fatal("duplicate should return same hash")
	}

	// Should only have 1 record
	records := store.GetDay("t1", r1.CreatedAt[:10])
	if len(records) != 1 {
		t.Fatalf("expected 1 record after dedup, got %d", len(records))
	}
}

func TestStructuredStoreSearch(t *testing.T) {
	dir := t.TempDir()
	store := NewStructuredStore(dir)

	store.Add("t1", "lang", "User prefers Go")
	store.Add("t1", "food", "User likes sushi")
	store.Add("t1", "lang", "User also knows Python")

	results := store.Search("t1", "user", 10)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	results = store.Search("t1", "go", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'go', got %d", len(results))
	}

	results = store.Search("t1", "python", 1)
	if len(results) != 1 {
		t.Fatalf("expected 1 limited result, got %d", len(results))
	}
}

func TestStructuredStoreListDays(t *testing.T) {
	dir := t.TempDir()
	store := NewStructuredStore(dir)

	store.Add("t1", "", "memory1")

	days := store.ListDays("t1")
	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}
}

func TestStructuredStoreMarkdownFormat(t *testing.T) {
	dir := t.TempDir()
	store := NewStructuredStore(dir)

	store.Add("t1", "test", "hello world")

	// Read raw file
	entries, _ := os.ReadDir(filepath.Join(dir, "t1"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	data, _ := os.ReadFile(filepath.Join(dir, "t1", entries[0].Name()))
	content := string(data)

	if !containsSubstr(content, "# Memory") {
		t.Fatal("missing markdown header")
	}
	if !containsSubstr(content, entryStartPrefix) {
		t.Fatal("missing entry start marker")
	}
	if !containsSubstr(content, entryEndMarker) {
		t.Fatal("missing entry end marker")
	}
	if !containsSubstr(content, "hello world") {
		t.Fatal("missing content")
	}
}

func TestStructuredStoreEmptyContent(t *testing.T) {
	dir := t.TempDir()
	store := NewStructuredStore(dir)

	_, err := store.Add("t1", "", "")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestParseDayMarkdown(t *testing.T) {
	md := `# Memory 2026-03-09

<!-- MEMENTRY {"id":"mem_1","hash":"abc123","topic":"test","created_at":"2026-03-09T10:00:00Z"} -->
This is memory content
<!-- /MEMENTRY -->

<!-- MEMENTRY {"id":"mem_2","hash":"def456","created_at":"2026-03-09T11:00:00Z"} -->
Second memory
<!-- /MEMENTRY -->
`
	records := parseDayMarkdown(md)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Content != "This is memory content" {
		t.Fatalf("unexpected content: %q", records[0].Content)
	}
	if records[1].Content != "Second memory" {
		t.Fatalf("unexpected content: %q", records[1].Content)
	}
}

func TestGenerateHash(t *testing.T) {
	h1 := generateHash("topic", "content")
	h2 := generateHash("topic", "content")
	h3 := generateHash("topic", "different")

	if h1 != h2 {
		t.Fatal("same input should produce same hash")
	}
	if h1 == h3 {
		t.Fatal("different input should produce different hash")
	}
	if len(h1) != 32 {
		t.Fatalf("expected 32 char hash, got %d", len(h1))
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
