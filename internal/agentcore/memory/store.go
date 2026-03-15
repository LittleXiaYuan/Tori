package memory

import (
	"context"
	"time"
)

// Item is a single memory entry.
type Item struct {
	ID         string    `json:"id"`
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	Source     string    `json:"source"`
	Category   string    `json:"category,omitempty"` // "fact", "preference", "knowledge", "experience"
	Score      float64   `json:"score,omitempty"`
	Embedding  []float32 `json:"embedding,omitempty"` // vector for semantic search
	AccessCnt  int       `json:"access_cnt,omitempty"`
	LastAccess time.Time `json:"last_access,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

// Store is the unified memory interface across all three layers.
type Store interface {
	// Put stores a memory item.
	Put(ctx context.Context, tenantID string, item Item) error
	// Get retrieves a memory item by key.
	Get(ctx context.Context, tenantID, key string) (*Item, error)
	// Search finds memories matching a query (semantic or keyword).
	Search(ctx context.Context, tenantID, query string, limit int) ([]Item, error)
	// Delete removes a memory item.
	Delete(ctx context.Context, tenantID, key string) error
	// List returns all memories for a tenant (with optional prefix filter).
	List(ctx context.Context, tenantID, prefix string, limit int) ([]Item, error)
}
