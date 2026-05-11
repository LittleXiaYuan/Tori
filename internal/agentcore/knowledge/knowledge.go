package knowledge

import (
	"context"
	"log/slog"
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
	Path       string     `json:"path,omitempty"`
	Trigger    string     `json:"trigger,omitempty"` // when to retrieve this knowledge
	ChunkSize  int        `json:"chunk_size"`
	ChunkCount int        `json:"chunk_count"`
	AddedAt    time.Time  `json:"added_at"`
}

// ──────────────────────────────────────────────
// Store — manages knowledge chunks
// ──────────────────────────────────────────────

// kvStore abstracts Ledger KV to avoid import cycles.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// Store holds ingested knowledge with search capability.
type Store struct {
	mu        sync.RWMutex
	sources   map[string]*Source
	chunks    []Chunk
	chunkSize int            // default chars per chunk
	semantic  *SemanticIndex // optional vector search index
	reranker  Reranker       // optional reranker for second-stage ranking
	kvs       kvStore        // optional Ledger KV persistence
	dirty     int

	bm25Cache   *BM25Index // cached BM25 index, rebuilt on chunk changes
	bm25Version int        // increments when chunks change
	bm25Built   int        // version at which bm25Cache was built

	semCache *SemanticCache // optional semantic query cache

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

// SetKVStore enables Ledger KV-backed persistence for knowledge chunks.
func (s *Store) SetKVStore(kvs kvStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvs = kvs

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type kvData struct {
		Sources map[string]*Source `json:"sources"`
		Chunks  []Chunk            `json:"chunks"`
	}
	var data kvData
	found, err := kvs.Get(ctx, "knowledge_data", &data)
	if err != nil {
		slog.Warn("knowledge: KV load failed", "err", err)
		return
	}
	if found && len(data.Chunks) > 0 {
		for id, src := range data.Sources {
			if _, exists := s.sources[id]; !exists {
				s.sources[id] = src
			}
		}
		existing := make(map[string]bool, len(s.chunks))
		for _, c := range s.chunks {
			existing[c.ID] = true
		}
		added := 0
		for _, c := range data.Chunks {
			if !existing[c.ID] {
				s.chunks = append(s.chunks, c)
				added++
			}
		}
		slog.Info("knowledge: loaded from Ledger KV", "chunks", added, "total", len(s.chunks))
	}
}

// FlushToKV persists current state to Ledger KV. Called during shutdown.
func (s *Store) FlushToKV() {
	s.mu.RLock()
	kvs := s.kvs
	sources := make(map[string]*Source, len(s.sources))
	for k, v := range s.sources {
		cp := *v
		sources[k] = &cp
	}
	chunks := make([]Chunk, len(s.chunks))
	copy(chunks, s.chunks)
	s.mu.RUnlock()

	if kvs == nil {
		return
	}
	type kvData struct {
		Sources map[string]*Source `json:"sources"`
		Chunks  []Chunk            `json:"chunks"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := kvs.Put(ctx, "knowledge_data", kvData{Sources: sources, Chunks: chunks}); err != nil {
		slog.Error("knowledge: flush to KV failed", "err", err)
	}
}

func (s *Store) persistKV() {
	s.dirty++
	if s.kvs == nil || s.dirty < 10 {
		return
	}
	s.dirty = 0
	go s.FlushToKV()
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
	s.bm25Version++
	if s.semCache != nil {
		s.semCache.Invalidate()
	}
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
	s.bm25Version++
	if s.semantic != nil && s.semantic.ready {
		s.semantic.mu.Lock()
		s.semantic.ready = false
		s.semantic.mu.Unlock()
		slog.Debug("knowledge: semantic index invalidated after new chunks added")
	}
	if s.semCache != nil {
		s.semCache.Invalidate()
	}
	slog.Debug("knowledge: ingested", "source", src.Name, "chunks", chunkCount)
	s.persistKV()
}
