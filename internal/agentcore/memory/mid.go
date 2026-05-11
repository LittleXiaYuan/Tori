package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MidTerm is a structured fact store with TF-IDF search.
// Stores extracted knowledge, user preferences, and observations.
// Deduplicates by content hash and supports category-based filtering.
type MidTerm struct {
	mu    sync.RWMutex
	items map[string]map[string]Item // tenantID -> key -> Item
	// IDF cache: separate mutex to avoid write-under-RLock race when
	// Search() lazily rebuilds the IDF index under items RLock.
	idfMu    sync.Mutex
	idfCache map[string]map[string]float64 // tenantID -> term -> idf
	idfDirty map[string]bool               // tenantID -> needs rebuild
}

// NewMidTerm creates a mid-term memory store.
func NewMidTerm() *MidTerm {
	return &MidTerm{
		items:    make(map[string]map[string]Item),
		idfCache: make(map[string]map[string]float64),
		idfDirty: make(map[string]bool),
	}
}

func (m *MidTerm) Put(_ context.Context, tenantID string, item Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.items[tenantID] == nil {
		m.items[tenantID] = make(map[string]Item)
	}
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Key == "" {
		item.Key = hashFact(item.Value)
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	// Dedup: check if same content hash already exists
	if existing, ok := m.items[tenantID][item.Key]; ok {
		// Update existing: merge source, bump timestamp
		existing.Value = item.Value
		if item.Source != "" {
			existing.Source = item.Source
		}
		if item.Category != "" {
			existing.Category = item.Category
		}
		existing.AccessCnt++
		existing.LastAccess = time.Now()
		m.items[tenantID][item.Key] = existing
	} else {
		// Also check for near-duplicate by scanning existing values
		dupKey := m.findSimilarFact(tenantID, item.Value, 0.80)
		if dupKey != "" {
			// Merge into existing similar fact
			existing := m.items[tenantID][dupKey]
			existing.AccessCnt++
			existing.LastAccess = time.Now()
			if len(item.Value) > len(existing.Value) {
				existing.Value = item.Value // keep longer/more detailed version
			}
			m.items[tenantID][dupKey] = existing
		} else {
			item.AccessCnt = 1
			item.LastAccess = time.Now()
			m.items[tenantID][item.Key] = item
		}
	}
	m.idfMu.Lock()
	m.idfDirty[tenantID] = true
	m.idfMu.Unlock()
	return nil
}

// findSimilarFact returns the key of an existing fact with Jaccard similarity >= threshold.
func (m *MidTerm) findSimilarFact(tenantID, value string, threshold float64) string {
	newTokens := tokenizeForIDF(value)
	if len(newTokens) == 0 {
		return ""
	}
	newSet := make(map[string]bool, len(newTokens))
	for _, t := range newTokens {
		newSet[t] = true
	}

	for key, existing := range m.items[tenantID] {
		existTokens := tokenizeForIDF(existing.Value)
		if len(existTokens) == 0 {
			continue
		}
		existSet := make(map[string]bool, len(existTokens))
		for _, t := range existTokens {
			existSet[t] = true
		}
		// Jaccard similarity
		intersection := 0
		for t := range newSet {
			if existSet[t] {
				intersection++
			}
		}
		union := len(newSet) + len(existSet) - intersection
		if union > 0 && float64(intersection)/float64(union) >= threshold {
			return key
		}
	}
	return ""
}

func (m *MidTerm) Get(_ context.Context, tenantID, key string) (*Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.items[tenantID] == nil {
		return nil, nil
	}
	item, ok := m.items[tenantID][key]
	if !ok {
		return nil, nil
	}
	// Track access
	item.AccessCnt++
	item.LastAccess = time.Now()
	m.items[tenantID][key] = item
	return &item, nil
}

func (m *MidTerm) Search(_ context.Context, tenantID, query string, limit int) ([]Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenant := m.items[tenantID]
	if tenant == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	// Rebuild IDF if dirty
	idf := m.getIDF(tenantID)

	queryTerms := tokenizeForIDF(query)
	if len(queryTerms) == 0 {
		// No query: return most recently accessed items
		var all []Item
		for _, item := range tenant {
			all = append(all, item)
		}
		sort.Slice(all, func(i, j int) bool {
			return all[i].LastAccess.After(all[j].LastAccess)
		})
		if len(all) > limit {
			all = all[:limit]
		}
		return all, nil
	}

	type scored struct {
		item  Item
		score float64
	}
	var results []scored

	for _, item := range tenant {
		score := tfidfScore(item.Value, queryTerms, idf)
		if score > 0 {
			// Boost by access frequency (log scale)
			if item.AccessCnt > 1 {
				score *= 1.0 + 0.1*math.Log(float64(item.AccessCnt))
			}
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
	return out, nil
}

func (m *MidTerm) Delete(_ context.Context, tenantID, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.items[tenantID] != nil {
		delete(m.items[tenantID], key)
		m.idfMu.Lock()
		m.idfDirty[tenantID] = true
		m.idfMu.Unlock()
	}
	return nil
}

func (m *MidTerm) List(_ context.Context, tenantID, prefix string, limit int) ([]Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenant := m.items[tenantID]
	if tenant == nil {
		return nil, nil
	}
	var results []Item
	for _, item := range tenant {
		if prefix == "" || strings.HasPrefix(item.Key, prefix) {
			results = append(results, item)
		}
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results, nil
}

// Count returns the number of items for a tenant.
func (m *MidTerm) Count(tenantID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items[tenantID])
}

// ExportAll returns all items grouped by tenant ID.
// Used by external persistence backends (e.g. LedgerPersister).
func (m *MidTerm) ExportAll() map[string][]Item {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]Item)
	for tid, items := range m.items {
		for _, item := range items {
			out[tid] = append(out[tid], item)
		}
	}
	return out
}

// --- TF-IDF engine ---

// getIDF returns (or rebuilds) the IDF map for a tenant.
// Uses a separate idfMu to safely rebuild the index while callers
// hold only items RLock.
func (m *MidTerm) getIDF(tenantID string) map[string]float64 {
	m.idfMu.Lock()
	defer m.idfMu.Unlock()

	if !m.idfDirty[tenantID] && m.idfCache[tenantID] != nil {
		return m.idfCache[tenantID]
	}

	tenant := m.items[tenantID]
	N := float64(len(tenant))
	if N == 0 {
		return nil
	}

	df := make(map[string]int)
	for _, item := range tenant {
		seen := make(map[string]bool)
		for _, term := range tokenizeForIDF(item.Value) {
			if !seen[term] {
				df[term]++
				seen[term] = true
			}
		}
	}

	idf := make(map[string]float64, len(df))
	for term, count := range df {
		idf[term] = math.Log(N/float64(count)) + 1.0
	}

	m.idfCache[tenantID] = idf
	m.idfDirty[tenantID] = false
	return idf
}

func tfidfScore(doc string, queryTerms []string, idf map[string]float64) float64 {
	docLower := strings.ToLower(doc)
	docTerms := tokenizeForIDF(docLower)
	if len(docTerms) == 0 {
		return 0
	}

	// Build term frequency map for doc
	tf := make(map[string]int)
	for _, t := range docTerms {
		tf[t]++
	}
	docLen := float64(len(docTerms))

	score := 0.0
	for _, term := range queryTerms {
		termLower := strings.ToLower(term)
		freq := tf[termLower]
		if freq == 0 {
			continue
		}
		// Normalized TF: freq / docLen
		ntf := float64(freq) / docLen
		termIDF := 1.0
		if idf != nil {
			if v, ok := idf[termLower]; ok {
				termIDF = v
			}
		}
		score += ntf * termIDF
	}
	return score
}

// tokenizeForIDF splits text into searchable tokens.
// Latin text is split by whitespace/punctuation. CJK is represented as bigrams + unigrams.
func tokenizeForIDF(s string) []string {
	s = strings.ToLower(s)
	var tokens []string

	// Latin/digit words only (CJK chars act as separators here)
	for _, w := range strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if len(w) > 0 {
			tokens = append(tokens, w)
		}
	}

	// CJK bigrams for better Chinese search
	runes := []rune(s)
	for i := 0; i < len(runes)-1; i++ {
		if runes[i] >= 0x4e00 && runes[i] <= 0x9fff {
			tokens = append(tokens, string(runes[i:i+2]))
		}
	}
	// Also add individual CJK chars as unigrams
	for _, r := range runes {
		if r >= 0x4e00 && r <= 0x9fff {
			tokens = append(tokens, string(r))
		}
	}
	return tokens
}

func hashFact(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}
