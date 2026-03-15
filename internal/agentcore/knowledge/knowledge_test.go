package knowledge

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tori-knowledge-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestIngestText(t *testing.T) {
	s := NewStore(500)
	src, err := s.IngestText("notes", "This is some knowledge about Go programming.")
	if err != nil {
		t.Fatal(err)
	}
	if src.Type != SourceText {
		t.Fatal("wrong type")
	}
	chunks := s.GetBySource(src.ID)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestIngestTextEmpty(t *testing.T) {
	s := NewStore(500)
	_, err := s.IngestText("empty", "")
	if err == nil {
		t.Fatal("expected error for empty")
	}
}

func TestIngestTextChunking(t *testing.T) {
	s := NewStore(50)
	long := ""
	for i := 0; i < 20; i++ {
		long += "This is line number " + string(rune('A'+i)) + " of text.\n"
	}
	src, _ := s.IngestText("long", long)
	chunks := s.GetBySource(src.ID)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
}

func TestIngestFile(t *testing.T) {
	dir := tmpDir(t)
	path := filepath.Join(dir, "doc.md")
	os.WriteFile(path, []byte("# Hello\nThis is a markdown document."), 0o644)

	s := NewStore(500)
	src, err := s.IngestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if src.Type != SourceFile {
		t.Fatal("wrong type")
	}
	if src.Path != path {
		t.Fatal("wrong path")
	}
}

func TestIngestURL(t *testing.T) {
	s := NewStore(500)
	src, err := s.IngestURL("DeepWiki VS Code", "https://deepwiki.com/microsoft/vscode", "VS Code Codebase Overview\nRepository Layout")
	if err != nil {
		t.Fatal(err)
	}
	if src.Type != SourceURL {
		t.Fatal("wrong type")
	}
	if src.Path != "https://deepwiki.com/microsoft/vscode" {
		t.Fatal("wrong path")
	}
	chunks := s.GetBySource(src.ID)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Metadata["url"] != "https://deepwiki.com/microsoft/vscode" {
		t.Fatal("missing url metadata")
	}
}

func TestIngestDirectory(t *testing.T) {
	dir := tmpDir(t)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Demo\n\nThis is a demo repo."), 0o644)

	s := NewStore(120)
	src, err := s.IngestDirectory(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if src.Type != SourceRepo {
		t.Fatal("wrong type")
	}
	if src.Path != dir {
		t.Fatal("wrong repo path")
	}
	chunks := s.GetBySource(src.ID)
	if len(chunks) == 0 {
		t.Fatal("expected repo chunks")
	}
	if chunks[0].Metadata["file"] == "" {
		t.Fatal("expected file metadata")
	}
	if chunks[0].Metadata["lang"] == "" {
		t.Fatal("expected language metadata")
	}
}

func TestIngestCSV(t *testing.T) {
	dir := tmpDir(t)
	path := filepath.Join(dir, "data.csv")
	f, _ := os.Create(path)
	w := csv.NewWriter(f)
	w.Write([]string{"name", "age", "city"})
	w.Write([]string{"Alice", "30", "Beijing"})
	w.Write([]string{"Bob", "25", "Shanghai"})
	w.Flush()
	f.Close()

	s := NewStore(500)
	src, err := s.IngestCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if src.Type != SourceCSV {
		t.Fatal("wrong type")
	}
	chunks := s.GetBySource(src.ID)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks (rows), got %d", len(chunks))
	}
	if !contains(chunks[0].Content, "Alice") {
		t.Fatal("missing Alice")
	}
}

func TestIngestJSON(t *testing.T) {
	dir := tmpDir(t)
	path := filepath.Join(dir, "data.json")
	arr := []map[string]interface{}{
		{"name": "Go", "year": 2009},
		{"name": "Rust", "year": 2010},
	}
	data, _ := json.Marshal(arr)
	os.WriteFile(path, data, 0o644)

	s := NewStore(500)
	src, err := s.IngestJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if src.Type != SourceJSON {
		t.Fatal("wrong type")
	}
	chunks := s.GetBySource(src.ID)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestIngestJSONObject(t *testing.T) {
	dir := tmpDir(t)
	path := filepath.Join(dir, "obj.json")
	obj := map[string]interface{}{"key1": "value1", "key2": "value2"}
	data, _ := json.Marshal(obj)
	os.WriteFile(path, data, 0o644)

	s := NewStore(500)
	src, _ := s.IngestJSON(path)
	chunks := s.GetBySource(src.ID)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestSearch(t *testing.T) {
	s := NewStore(500)
	s.IngestText("go", "Go is a statically typed language created at Google")
	s.IngestText("rust", "Rust focuses on memory safety and performance")

	results := s.Search("memory safety", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !contains(results[0].Content, "Rust") {
		t.Fatal("wrong result")
	}
}

func TestSearchLimit(t *testing.T) {
	s := NewStore(500)
	s.IngestText("a", "hello world one")
	s.IngestText("b", "hello world two")
	s.IngestText("c", "hello world three")

	results := s.Search("hello", 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 (limit), got %d", len(results))
	}
}

func TestSearchFiltered(t *testing.T) {
	s := NewStore(500)
	src := s.newSource("repo", SourceRepo)
	src.Path = "repo"
	s.addPreparedChunks(src, []PreparedChunk{
		{Content: "FILE: cmd/main.go\nLANG: go\n\npackage main\nfunc main() {}", Metadata: map[string]string{"file": "cmd/main.go", "lang": "go"}},
		{Content: "FILE: web/app.tsx\nLANG: tsx\n\nexport default function App() {}", Metadata: map[string]string{"file": "web/app.tsx", "lang": "tsx"}},
	})

	results := s.SearchFiltered("function", 10, "web/", "tsx")
	if len(results) != 1 {
		t.Fatalf("expected 1 filtered result, got %d", len(results))
	}
	if results[0].Metadata["file"] != "web/app.tsx" {
		t.Fatal("wrong filtered file")
	}
}

func TestHasCodeSources(t *testing.T) {
	s := NewStore(500)
	if s.HasCodeSources() {
		t.Fatal("empty store should have no code sources")
	}
	s.IngestText("docs", "some documentation")
	if s.HasCodeSources() {
		t.Fatal("text source should not count as code")
	}
	src := s.newSource("myrepo", SourceRepo)
	src.Path = "/tmp/repo"
	s.addPreparedChunks(src, []PreparedChunk{
		{Content: "package main", Metadata: map[string]string{"file": "main.go", "lang": "go"}},
	})
	if !s.HasCodeSources() {
		t.Fatal("should detect repo source as code")
	}
}

func TestSources(t *testing.T) {
	s := NewStore(500)
	s.IngestText("a", "x")
	s.IngestText("b", "y")
	if len(s.Sources()) != 2 {
		t.Fatal("expected 2 sources")
	}
}

func TestRemoveSource(t *testing.T) {
	s := NewStore(500)
	src, _ := s.IngestText("temp", "data")
	s.IngestText("keep", "data2")
	if !s.RemoveSource(src.ID) {
		t.Fatal("should remove")
	}
	if len(s.Sources()) != 1 {
		t.Fatal("expected 1 remaining")
	}
	if len(s.Search("data", 10)) != 1 {
		t.Fatal("should only find keep")
	}
}

func TestRemoveSourceNotFound(t *testing.T) {
	s := NewStore(500)
	if s.RemoveSource("nope") {
		t.Fatal("should not find")
	}
}

func TestStats(t *testing.T) {
	s := NewStore(500)
	s.IngestText("a", "content")
	stats := s.Stats()
	if stats.Sources != 1 || stats.Chunks != 1 {
		t.Fatal("wrong stats")
	}
}

func TestSplitText(t *testing.T) {
	chunks := splitText("short", 1000)
	if len(chunks) != 1 {
		t.Fatal("should be 1 chunk")
	}
}

func TestExtractReadableText(t *testing.T) {
	// Mix of binary and text
	data := []byte{0, 0, 0}
	data = append(data, []byte("This is readable text that is long enough to extract from binary data")...)
	data = append(data, []byte{0, 0, 0}...)

	text := extractReadableText(data)
	if !contains(text, "readable") {
		t.Fatal("should extract readable text")
	}
}

func TestDefaultChunkSize(t *testing.T) {
	s := NewStore(0)
	if s.chunkSize != 1000 {
		t.Fatal("should default to 1000")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
