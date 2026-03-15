package knowledge

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Source types
// ──────────────────────────────────────────────

type SourceType string

const (
	SourceText SourceType = "text"
	SourceFile SourceType = "file" // .txt, .md
	SourceCSV  SourceType = "csv"
	SourceJSON SourceType = "json"
	SourceURL  SourceType = "url"
	SourcePDF  SourceType = "pdf" // plain text extraction
	SourceRepo SourceType = "repo"
)

// ──────────────────────────────────────────────
// Chunk — a knowledge fragment
// ──────────────────────────────────────────────

// Chunk is a piece of ingested knowledge.
type Chunk struct {
	ID       string            `json:"id"`
	SourceID string            `json:"source_id"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Index    int               `json:"index"` // chunk index within source
}

// Source represents a knowledge source.
type Source struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       SourceType `json:"type"`
	Path       string     `json:"path,omitempty"` // file path or URL
	ChunkSize  int        `json:"chunk_size"`
	ChunkCount int        `json:"chunk_count"`
	AddedAt    time.Time  `json:"added_at"`
}

// ──────────────────────────────────────────────
// Store — manages knowledge chunks
// ──────────────────────────────────────────────

// Store holds ingested knowledge with search capability.
type Store struct {
	mu        sync.RWMutex
	sources   map[string]*Source
	chunks    []Chunk
	chunkSize int            // default chars per chunk
	semantic  *SemanticIndex // optional vector search index
	reranker  Reranker       // optional reranker for second-stage ranking

	// Metrics callbacks (optional, set via SetMetricsHooks)
	onSearch func(searchType string, duration time.Duration, results int)
	onRerank func(provider string, duration time.Duration, err error)
}

type PreparedChunk struct {
	Content  string
	Metadata map[string]string
}

// NewStore creates a knowledge store.
func NewStore(chunkSize int) *Store {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	return &Store{
		sources:   make(map[string]*Source),
		chunkSize: chunkSize,
	}
}

// SetMetricsHooks sets optional callbacks for recording search and rerank metrics.
func (s *Store) SetMetricsHooks(
	onSearch func(searchType string, duration time.Duration, results int),
	onRerank func(provider string, duration time.Duration, err error),
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSearch = onSearch
	s.onRerank = onRerank
}

// ──────────────────────────────────────────────
// Ingest methods
// ──────────────────────────────────────────────

// IngestText ingests raw text content.
func (s *Store) IngestText(name, content string) (*Source, error) {
	if content == "" {
		return nil, fmt.Errorf("knowledge: empty content")
	}
	src := s.newSource(name, SourceText)
	chunks := splitText(content, s.chunkSize)
	s.addChunks(src, chunks, nil)
	return src, nil
}

// IngestURL ingests text content fetched from a URL.
func (s *Store) IngestURL(name, sourceURL, content string) (*Source, error) {
	if content == "" {
		return nil, fmt.Errorf("knowledge: empty content")
	}
	if name == "" {
		name = sourceURL
	}
	src := s.newSource(name, SourceURL)
	src.Path = sourceURL
	chunks := splitText(content, s.chunkSize)
	s.addChunks(src, chunks, map[string]string{"url": sourceURL})
	return src, nil
}

// IngestDirectory ingests a local repository or code directory as a single source.
func (s *Store) IngestDirectory(root string, maxFiles int) (*Source, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("knowledge: stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("knowledge: path is not a directory")
	}
	if maxFiles <= 0 {
		maxFiles = 200
	}
	if maxFiles > 1000 {
		maxFiles = 1000
	}

	name := filepath.Base(root)
	src := s.newSource(name, SourceRepo)
	src.Path = root

	prepared := make([]PreparedChunk, 0, maxFiles)
	count := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipRepoDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if count >= maxFiles {
			return io.EOF
		}
		if shouldSkipRepoFile(path, d.Name()) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil || len(data) == 0 || len(data) > 512<<10 {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = filepath.Base(path)
		}
		language := detectRepoLanguage(path)
		chunks := splitRepoContent(filepath.ToSlash(rel), language, string(data), s.chunkSize)
		for _, chunk := range chunks {
			prepared = append(prepared, PreparedChunk{
				Content: chunk,
				Metadata: map[string]string{
					"file": filepath.ToSlash(rel),
					"lang": language,
					"root": filepath.ToSlash(filepath.Clean(root)),
				},
			})
		}
		count++
		return nil
	})
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("knowledge: walk directory: %w", err)
	}
	if len(prepared) == 0 {
		return nil, fmt.Errorf("knowledge: no supported files found")
	}
	s.addPreparedChunks(src, prepared)
	return src, nil
}

// IngestFile ingests a text file (.txt, .md).
func (s *Store) IngestFile(path string) (*Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: read file: %w", err)
	}
	name := filepath.Base(path)
	src := s.newSource(name, SourceFile)
	src.Path = path
	chunks := splitText(string(data), s.chunkSize)
	s.addChunks(src, chunks, map[string]string{"file": path})
	return src, nil
}

// IngestCSV ingests a CSV file, treating each row as a chunk.
func (s *Store) IngestCSV(path string) (*Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: open csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("knowledge: csv headers: %w", err)
	}

	name := filepath.Base(path)
	src := s.newSource(name, SourceCSV)
	src.Path = path

	var chunks []string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		// Format as key: value pairs
		var parts []string
		for i, h := range headers {
			if i < len(record) {
				parts = append(parts, h+": "+record[i])
			}
		}
		chunks = append(chunks, strings.Join(parts, " | "))
	}

	s.addChunks(src, chunks, map[string]string{"file": path, "format": "csv"})
	return src, nil
}

// IngestJSON ingests a JSON file (array of objects or single object).
func (s *Store) IngestJSON(path string) (*Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: read json: %w", err)
	}

	name := filepath.Base(path)
	src := s.newSource(name, SourceJSON)
	src.Path = path

	var chunks []string

	// Try array first
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		for _, obj := range arr {
			b, _ := json.Marshal(obj)
			chunks = append(chunks, string(b))
		}
	} else {
		// Try single object
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err == nil {
			for k, v := range obj {
				b, _ := json.Marshal(v)
				chunks = append(chunks, fmt.Sprintf("%s: %s", k, string(b)))
			}
		} else {
			// Fallback: treat as text
			chunks = splitText(string(data), s.chunkSize)
		}
	}

	s.addChunks(src, chunks, map[string]string{"file": path, "format": "json"})
	return src, nil
}

// IngestPDF ingests a PDF file (extracts text lines from binary).
// This is a simplified extraction that finds readable text runs.
func (s *Store) IngestPDF(path string) (*Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: read pdf: %w", err)
	}

	name := filepath.Base(path)
	src := s.newSource(name, SourcePDF)
	src.Path = path

	// Simple text extraction: find text between BT/ET markers or readable runs
	text := extractReadableText(data)
	if text == "" {
		return nil, fmt.Errorf("knowledge: no text extracted from PDF")
	}

	chunks := splitText(text, s.chunkSize)
	s.addChunks(src, chunks, map[string]string{"file": path, "format": "pdf"})
	return src, nil
}

// ──────────────────────────────────────────────
// Search
// ──────────────────────────────────────────────

// Search returns chunks matching a query (substring match).
func (s *Store) Search(query string, limit int) []Chunk {
	return s.SearchFiltered(query, limit, "", "")
}

// SearchFiltered returns chunks matching a query with optional file/language filters.
func (s *Store) SearchFiltered(query string, limit int, fileFilter, langFilter string) []Chunk {
	start := time.Now()
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 10
	}
	query = strings.ToLower(query)
	fileFilter = strings.ToLower(strings.TrimSpace(fileFilter))
	langFilter = strings.ToLower(strings.TrimSpace(langFilter))
	var results []Chunk
	for _, c := range s.chunks {
		if fileFilter != "" {
			if c.Metadata == nil || !strings.Contains(strings.ToLower(c.Metadata["file"]), fileFilter) {
				continue
			}
		}
		if langFilter != "" {
			if c.Metadata == nil || !strings.EqualFold(c.Metadata["lang"], langFilter) {
				continue
			}
		}
		if strings.Contains(strings.ToLower(c.Content), query) {
			results = append(results, c)
			if len(results) >= limit {
				break
			}
		}
	}
	if s.onSearch != nil {
		s.onSearch("substring", time.Since(start), len(results))
	}
	return results
}

// GetBySource returns all chunks from a source.
func (s *Store) GetBySource(sourceID string) []Chunk {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Chunk
	for _, c := range s.chunks {
		if c.SourceID == sourceID {
			out = append(out, c)
		}
	}
	return out
}

// Sources returns all registered sources.
func (s *Store) Sources() []*Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Source, 0, len(s.sources))
	for _, src := range s.sources {
		cp := *src
		out = append(out, &cp)
	}
	return out
}

// HasCodeSources returns whether any repo-type sources exist in the store.
func (s *Store) HasCodeSources() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, src := range s.sources {
		if src.Type == SourceRepo {
			return true
		}
	}
	return false
}

// RemoveSource deletes a source and its chunks.
func (s *Store) RemoveSource(sourceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sources[sourceID]; !ok {
		return false
	}
	delete(s.sources, sourceID)
	var kept []Chunk
	for _, c := range s.chunks {
		if c.SourceID != sourceID {
			kept = append(kept, c)
		}
	}
	s.chunks = kept
	return true
}

// Stats returns store statistics.
func (s *Store) Stats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return StoreStats{
		Sources:   len(s.sources),
		Chunks:    len(s.chunks),
		ChunkSize: s.chunkSize,
	}
}

// StoreStats holds store metrics.
type StoreStats struct {
	Sources   int `json:"sources"`
	Chunks    int `json:"chunks"`
	ChunkSize int `json:"chunk_size"`
}

// ──────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────

func (s *Store) newSource(name string, st SourceType) *Source {
	src := &Source{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      st,
		ChunkSize: s.chunkSize,
		AddedAt:   time.Now(),
	}
	s.mu.Lock()
	s.sources[src.ID] = src
	s.mu.Unlock()
	return src
}

func (s *Store) addChunks(src *Source, texts []string, meta map[string]string) {
	prepared := make([]PreparedChunk, 0, len(texts))
	for _, text := range texts {
		prepared = append(prepared, PreparedChunk{Content: text, Metadata: meta})
	}
	s.addPreparedChunks(src, prepared)
}

func (s *Store) addPreparedChunks(src *Source, prepared []PreparedChunk) {
	s.mu.Lock()
	defer s.mu.Unlock()
	chunkCount := 0
	for i, item := range prepared {
		text := item.Content
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		chunk := Chunk{
			ID:       uuid.New().String(),
			SourceID: src.ID,
			Content:  text,
			Index:    i,
			Metadata: item.Metadata,
		}
		s.chunks = append(s.chunks, chunk)
		chunkCount++
	}
	src.ChunkCount = chunkCount
	slog.Debug("knowledge: ingested", "source", src.Name, "chunks", chunkCount)
}

// splitText splits text into chunks of approximately maxChars.
func splitText(text string, maxChars int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}

	var chunks []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	var current strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if current.Len()+len(line)+1 > maxChars && current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// extractReadableText finds printable ASCII runs in binary data.
func extractReadableText(data []byte) string {
	var sb strings.Builder
	var run strings.Builder
	for _, b := range data {
		if b >= 32 && b < 127 || b == '\n' || b == '\r' || b == '\t' {
			run.WriteByte(b)
		} else {
			if run.Len() > 20 { // only keep runs > 20 chars
				sb.WriteString(run.String())
				sb.WriteByte('\n')
			}
			run.Reset()
		}
	}
	if run.Len() > 20 {
		sb.WriteString(run.String())
	}
	return sb.String()
}

func shouldSkipRepoDir(name string) bool {
	switch name {
	case ".git", ".svn", ".hg", "node_modules", "vendor", "dist", "build", ".next", "coverage", "tmp", "Temp":
		return true
	default:
		return false
	}
}

func shouldSkipRepoFile(path, name string) bool {
	if strings.HasPrefix(name, ".") && name != ".env.example" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".rs", ".md", ".json", ".yaml", ".yml", ".sql", ".sh", ".txt":
		return false
	default:
		return true
	}
}

func detectRepoLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".js":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".md":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".sql":
		return "sql"
	case ".sh":
		return "shell"
	default:
		return "text"
	}
}

func splitRepoContent(relPath, language, content string, maxChars int) []string {
	header := fmt.Sprintf("FILE: %s\nLANG: %s\n\n", relPath, language)
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	if language == "markdown" || language == "text" || language == "json" || language == "yaml" {
		parts := splitText(trimmed, maxChars-len(header))
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			out = append(out, header+part)
		}
		return out
	}

	lines := strings.Split(trimmed, "\n")
	var chunks []string
	var current strings.Builder
	current.WriteString(header)
	for _, line := range lines {
		if current.Len()+len(line)+1 > maxChars && current.Len() > len(header) {
			chunks = append(chunks, current.String())
			current.Reset()
			current.WriteString(header)
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > len(header) {
		chunks = append(chunks, current.String())
	}
	return chunks
}
