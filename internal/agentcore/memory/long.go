package memory

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EmbedFunc generates embeddings for texts. Injected from llm.Client.
type EmbedFunc func(ctx context.Context, texts []string) ([][]float32, error)

// LongTerm is a knowledge base with vector search (embeddings) + BM25 fallback.
// When an EmbedFunc is set, items are embedded on Put and searched by cosine similarity.
// Without embeddings, falls back to BM25 with proper corpus-wide IDF.
type LongTerm struct {
	mu      sync.RWMutex
	items   map[string][]Item // tenantID -> items
	embedFn EmbedFunc
	// BM25 stats: separate mutex to avoid write-under-RLock race when
	// Search() lazily rebuilds stats under items RLock.
	statsMu    sync.Mutex
	avgDocLen  map[string]float64        // tenantID -> average doc length
	docFreq    map[string]map[string]int // tenantID -> term -> doc count
	statsDirty map[string]bool
}

// NewLongTerm creates a long-term knowledge store.
func NewLongTerm() *LongTerm {
	return &LongTerm{
		items:      make(map[string][]Item),
		avgDocLen:  make(map[string]float64),
		docFreq:    make(map[string]map[string]int),
		statsDirty: make(map[string]bool),
	}
}

// SetEmbedFunc injects the embedding function (typically from llm.Client.Embed).
func (l *LongTerm) SetEmbedFunc(fn EmbedFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.embedFn = fn
}

func (l *LongTerm) Put(ctx context.Context, tenantID string, item Item) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	// Generate embedding BEFORE acquiring the lock to avoid unlock/relock races.
	if len(item.Embedding) == 0 && item.Value != "" {
		l.mu.RLock()
		fn := l.embedFn
		l.mu.RUnlock()
		if fn != nil {
			embeddings, err := fn(ctx, []string{item.Value})
			if err == nil && len(embeddings) > 0 {
				item.Embedding = embeddings[0]
			}
		}
	}

	l.mu.Lock()
	// Content dedup: repeated /v1/memory/add of the same fact, re-extracted
	// facts, and duplicate Ledger entries (which are re-Put on load) would
	// otherwise pile up identical items that waste recall slots and tokens.
	// If an identical fact already exists for this tenant, refresh its access
	// instead of appending a duplicate.
	if norm := strings.TrimSpace(item.Value); norm != "" {
		for i := range l.items[tenantID] {
			if strings.TrimSpace(l.items[tenantID][i].Value) == norm {
				l.items[tenantID][i].AccessCnt++
				l.items[tenantID][i].LastAccess = time.Now()
				l.mu.Unlock()
				return nil
			}
		}
	}
	l.items[tenantID] = append(l.items[tenantID], item)
	l.mu.Unlock()
	l.statsMu.Lock()
	l.statsDirty[tenantID] = true
	l.statsMu.Unlock()
	return nil
}

func (l *LongTerm) Get(_ context.Context, tenantID, key string) (*Item, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, item := range l.items[tenantID] {
		if item.Key == key || item.ID == key {
			l.items[tenantID][i].AccessCnt++
			l.items[tenantID][i].LastAccess = time.Now()
			cp := l.items[tenantID][i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (l *LongTerm) Search(ctx context.Context, tenantID, query string, limit int) ([]Item, error) {
	if limit <= 0 {
		limit = 10
	}

	// Check if embeddings are available and get embedFn reference under lock.
	l.mu.RLock()
	items := l.items[tenantID]
	if len(items) == 0 {
		l.mu.RUnlock()
		return nil, nil
	}
	hasEmbeddings := false
	for _, item := range items {
		if len(item.Embedding) > 0 {
			hasEmbeddings = true
			break
		}
	}
	fn := l.embedFn
	l.mu.RUnlock()

	// Call embedding API outside the lock to avoid data races.
	if hasEmbeddings && fn != nil {
		queryEmb, err := fn(ctx, []string{query})
		if err == nil && len(queryEmb) > 0 && len(queryEmb[0]) > 0 {
			l.mu.RLock()
			defer l.mu.RUnlock()
			return l.vectorSearch(tenantID, queryEmb[0], limit), nil
		}
	}

	// Fallback: BM25 with real IDF
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.bm25Search(tenantID, query, limit), nil
}

// vectorSearch finds items by cosine similarity to the query embedding.
func (l *LongTerm) vectorSearch(tenantID string, queryEmb []float32, limit int) []Item {
	type scored struct {
		item  Item
		score float64
	}
	var results []scored

	for _, item := range l.items[tenantID] {
		if len(item.Embedding) == 0 {
			continue
		}
		sim := cosineSimilarity(queryEmb, item.Embedding)
		if sim > 0.1 { // minimum similarity threshold
			item.Score = sim
			results = append(results, scored{item: item, score: sim})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	var out []Item
	for i, r := range results {
		if i >= limit {
			break
		}
		out = append(out, r.item)
	}
	return out
}

// bm25Search uses BM25 with corpus-wide IDF for keyword-based retrieval.
func (l *LongTerm) bm25Search(tenantID, query string, limit int) []Item {
	items := l.items[tenantID]
	N := float64(len(items))
	if N == 0 {
		return nil
	}

	l.rebuildStats(tenantID)

	queryTerms := tokenizeForIDF(query)
	if len(queryTerms) == 0 {
		return nil
	}

	// Snapshot stats under statsMu to avoid racing with concurrent rebuilds
	l.statsMu.Lock()
	avgDL := l.avgDocLen[tenantID]
	df := l.docFreq[tenantID]
	l.statsMu.Unlock()
	if avgDL == 0 {
		avgDL = 50
	}

	k1 := 1.5
	b := 0.75

	type scored struct {
		item  Item
		score float64
	}
	var results []scored

	for _, item := range items {
		docTerms := tokenizeForIDF(item.Value)
		docLen := float64(len(docTerms))
		if docLen == 0 {
			continue
		}

		// Build TF map
		tf := make(map[string]int)
		for _, t := range docTerms {
			tf[t]++
		}

		score := 0.0
		for _, term := range queryTerms {
			termTF := float64(tf[term])
			if termTF == 0 {
				continue
			}
			// Real IDF: log((N - df + 0.5) / (df + 0.5) + 1)
			termDF := 1.0
			if df != nil {
				if d, ok := df[term]; ok {
					termDF = float64(d)
				}
			}
			idf := math.Log((N-termDF+0.5)/(termDF+0.5) + 1.0)
			// BM25 TF saturation
			tfNorm := (termTF * (k1 + 1)) / (termTF + k1*(1-b+b*docLen/avgDL))
			score += idf * tfNorm
		}
		if score > 0 {
			item.Score = score
			results = append(results, scored{item: item, score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	var out []Item
	for i, r := range results {
		if i >= limit {
			break
		}
		out = append(out, r.item)
	}
	return out
}

func (l *LongTerm) rebuildStats(tenantID string) {
	l.statsMu.Lock()
	defer l.statsMu.Unlock()

	if !l.statsDirty[tenantID] && l.docFreq[tenantID] != nil {
		return
	}

	items := l.items[tenantID]
	totalLen := 0.0
	df := make(map[string]int)

	for _, item := range items {
		terms := tokenizeForIDF(item.Value)
		totalLen += float64(len(terms))
		seen := make(map[string]bool)
		for _, t := range terms {
			if !seen[t] {
				df[t]++
				seen[t] = true
			}
		}
	}

	if len(items) > 0 {
		l.avgDocLen[tenantID] = totalLen / float64(len(items))
	}
	l.docFreq[tenantID] = df
	l.statsDirty[tenantID] = false
}

func (l *LongTerm) Delete(_ context.Context, tenantID, key string) error {
	l.mu.Lock()
	items := l.items[tenantID]
	found := false
	for i, item := range items {
		if item.Key == key || item.ID == key {
			l.items[tenantID] = append(items[:i], items[i+1:]...)
			found = true
			break
		}
	}
	l.mu.Unlock()
	if found {
		l.statsMu.Lock()
		l.statsDirty[tenantID] = true
		l.statsMu.Unlock()
	}
	return nil
}

func (l *LongTerm) List(_ context.Context, tenantID, prefix string, limit int) ([]Item, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var results []Item
	for _, item := range l.items[tenantID] {
		if prefix == "" || strings.HasPrefix(item.Key, prefix) {
			results = append(results, item)
		}
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results, nil
}

// Count returns the total number of items for a tenant.
func (l *LongTerm) Count(tenantID string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.items[tenantID])
}

// ExportAll returns all items grouped by tenant ID.
// Used by external persistence backends (e.g. LedgerPersister).
func (l *LongTerm) ExportAll() map[string][]Item {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make(map[string][]Item)
	for tid, items := range l.items {
		out[tid] = append(out[tid], items...)
	}
	return out
}

// --- Vector math ---

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
