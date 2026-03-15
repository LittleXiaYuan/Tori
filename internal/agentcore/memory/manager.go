package memory

import (
	"context"
	"sort"
)

// Layer weights for cross-layer score normalization.
const (
	shortWeight = 0.6 // recency matters most
	midWeight   = 0.8 // facts are reliable
	longWeight  = 1.0 // knowledge base is highest quality
)

// Manager provides unified access to all three memory layers.
type Manager struct {
	Short     *ShortTerm
	Mid       *MidTerm
	Long      *LongTerm
	persister *Persister
}

// NewManager creates a memory manager with three layers.
func NewManager(short *ShortTerm, mid *MidTerm, long *LongTerm) *Manager {
	return &Manager{Short: short, Mid: mid, Long: long}
}

// SetPersister attaches a file persister.
func (m *Manager) SetPersister(p *Persister) {
	m.persister = p
}

// SetEmbedFunc injects the embedding function into the Long-term layer.
func (m *Manager) SetEmbedFunc(fn EmbedFunc) {
	m.Long.SetEmbedFunc(fn)
}

// AddMid stores an item in mid-term memory and marks dirty for persistence.
func (m *Manager) AddMid(ctx context.Context, tenantID string, item Item) error {
	err := m.Mid.Put(ctx, tenantID, item)
	if err == nil && m.persister != nil {
		m.persister.MarkDirty()
	}
	return err
}

// AddLong stores an item in long-term memory and marks dirty for persistence.
func (m *Manager) AddLong(ctx context.Context, tenantID string, item Item) error {
	err := m.Long.Put(ctx, tenantID, item)
	if err == nil && m.persister != nil {
		m.persister.MarkDirty()
	}
	return err
}

// StopPersister flushes and stops the persister.
func (m *Manager) StopPersister() {
	if m.persister != nil {
		m.persister.Stop()
	}
}

// SearchAll searches across all memory layers and returns results ranked by
// weighted score. Each layer's scores are multiplied by its weight to produce
// a unified ranking across short/mid/long.
func (m *Manager) SearchAll(ctx context.Context, tenantID, query string, limit int) ([]Item, error) {
	if limit <= 0 {
		limit = 10
	}
	// Fetch from each layer (request more than limit to allow cross-layer ranking)
	perLayer := limit * 2
	if perLayer < 5 {
		perLayer = 5
	}

	var all []Item

	if results, err := m.Short.Search(ctx, tenantID, query, perLayer); err == nil {
		for i := range results {
			results[i].Source = "short:" + results[i].Source
			results[i].Score *= shortWeight
		}
		all = append(all, results...)
	}
	if results, err := m.Mid.Search(ctx, tenantID, query, perLayer); err == nil {
		for i := range results {
			results[i].Source = "mid:" + results[i].Source
			results[i].Score *= midWeight
		}
		all = append(all, results...)
	}
	if results, err := m.Long.Search(ctx, tenantID, query, perLayer); err == nil {
		for i := range results {
			results[i].Source = "long:" + results[i].Source
			results[i].Score *= longWeight
		}
		all = append(all, results...)
	}

	// Sort by weighted score descending
	sort.Slice(all, func(i, j int) bool {
		return all[i].Score > all[j].Score
	})

	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// Stats returns memory counts per layer.
func (m *Manager) Stats(tenantID string) map[string]int {
	return map[string]int{
		"short": m.Short.Count(tenantID),
		"mid":   m.Mid.Count(tenantID),
		"long":  m.Long.Count(tenantID),
	}
}
