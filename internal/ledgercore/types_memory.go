package ledger

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────
// MemoryEntry
// ──────────────────────────────────────────────

// MemoryKind classifies the type of memory entry.
type MemoryKind string

const (
	MemoryFact        MemoryKind = "fact"
	MemoryRule        MemoryKind = "rule"
	MemoryExperience  MemoryKind = "experience"
	MemoryArtifactRef MemoryKind = "artifact_ref"
	MemorySummary     MemoryKind = "summary"
	MemoryPreference  MemoryKind = "preference"
)

// MemoryEntry is a structured memory record with classification and provenance.
type MemoryEntry struct {
	ID          string     `json:"id"           db:"id"`
	TenantID    string     `json:"tenant_id"    db:"tenant_id"`
	TaskID      *string    `json:"task_id"      db:"task_id"` // nil = global memory
	Kind        MemoryKind `json:"kind"         db:"kind"`
	Key         string     `json:"key"          db:"key"`
	Content     string     `json:"content"      db:"content"`
	Source      string     `json:"source"       db:"source"` // "extraction" | "user" | "tool"
	Confidence  float64    `json:"confidence"   db:"confidence"`
	AccessCount int        `json:"access_count" db:"access_count"`
	LastAccess  *time.Time `json:"last_access"  db:"last_access"`
	ExpiresAt   *time.Time `json:"expires_at"   db:"expires_at"`
	Embedding   []float32  `json:"embedding,omitempty" db:"-"` // semantic vector
	Metadata    JSON       `json:"metadata"     db:"metadata"`
	CreatedAt   time.Time  `json:"created_at"   db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"   db:"updated_at"`
}

// MemoryQuery specifies criteria for searching memories.
type MemoryQuery struct {
	TenantID      string       `json:"tenant_id"`
	Query         string       `json:"query,omitempty"`
	Kinds         []MemoryKind `json:"kinds,omitempty"`
	TaskID        *string      `json:"task_id,omitempty"`
	Key           string       `json:"key,omitempty"`            // exact key match
	Source        string       `json:"source,omitempty"`         // exact source filter
	MinConfidence float64      `json:"min_confidence,omitempty"` // only return memories above this threshold
	Limit         int          `json:"limit,omitempty"`
	Offset        int          `json:"offset,omitempty"`         // pagination offset for batch processing
}

// VectorQuery performs approximate nearest neighbor search on memory embeddings.
type VectorQuery struct {
	TenantID  string       `json:"tenant_id"`
	Embedding []float32    `json:"embedding"`
	Kinds     []MemoryKind `json:"kinds,omitempty"`
	Limit     int          `json:"limit,omitempty"`
	MinScore  float64      `json:"min_score,omitempty"`
}

// EmbedFunc generates a vector embedding for the given text.
// Injected by the caller to decouple from specific providers.
type EmbedFunc func(ctx context.Context, text string) ([]float32, error)

// ──────────────────────────────────────────────
// Recall
// ──────────────────────────────────────────────

// RecallQuery specifies a task-aware memory recall request.
type RecallQuery struct {
	TenantID    string            `json:"tenant_id"`
	TaskID      string            `json:"task_id"`
	Query       string            `json:"query"`
	TaskGoal    string            `json:"task_goal"`
	TaskType    TaskType          `json:"task_type"`
	MemoryKinds []MemoryKind      `json:"memory_kinds,omitempty"`
	Limit       int               `json:"limit"`
	MinScore    float64           `json:"min_score"`
	Recency     *time.Duration    `json:"recency,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// RecallResult holds the results of a task-aware recall query.
type RecallResult struct {
	Entries     []ScoredEntry `json:"entries"`
	Artifacts   []Artifact    `json:"artifacts,omitempty"`
	TotalFound  int           `json:"total_found"`
	QueryTimeMs int64         `json:"query_time_ms"`
}

// ScoredEntry pairs a memory entry with its relevance score.
type ScoredEntry struct {
	Entry  MemoryEntry `json:"entry"`
	Score  float64     `json:"score"`
	Reason string      `json:"reason"`
}
