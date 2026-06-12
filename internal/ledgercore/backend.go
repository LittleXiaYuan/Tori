package ledger

import (
	"context"
	"time"
)

// Backend is the storage abstraction for Ledger.
// Implementations must be safe for concurrent use.
type Backend interface {
	// ── Task ──

	CreateTask(ctx context.Context, t *Task) error
	GetTask(ctx context.Context, id string) (*Task, error)
	UpdateTask(ctx context.Context, t *Task) error
	ListTasks(ctx context.Context, f TaskFilter) ([]*Task, error)
	// DeleteTask permanently removes a task together with its events,
	// checkpoints, artifacts, and dependency links.
	DeleteTask(ctx context.Context, id string) error

	// ── Event (append-only) ──

	// AppendEvent appends an event and returns the assigned sequence number.
	AppendEvent(ctx context.Context, e *Event) error
	// ListEvents returns events for a task, ordered by seq, after afterSeq.
	ListEvents(ctx context.Context, taskID string, afterSeq int64, limit int) ([]*Event, error)
	// CountEvents returns the number of events for a task.
	CountEvents(ctx context.Context, taskID string) (int64, error)
	// LatestEventSeq returns the highest sequence number for a task's events.
	// Returns 0 if no events exist.
	LatestEventSeq(ctx context.Context, taskID string) (int64, error)
	// QueryEvents performs a flexible event query with multiple filter dimensions.
	QueryEvents(ctx context.Context, q EventQuery) ([]*Event, error)

	// ── Checkpoint ──

	SaveCheckpoint(ctx context.Context, cp *Checkpoint) error
	LatestCheckpoint(ctx context.Context, taskID string) (*Checkpoint, error)
	ListCheckpoints(ctx context.Context, taskID string, limit int) ([]*Checkpoint, error)
	DeleteCheckpointsBefore(ctx context.Context, taskID string, beforeSeq int64) error

	// ── Memory ──

	PutMemory(ctx context.Context, m *MemoryEntry) error
	GetMemory(ctx context.Context, id string) (*MemoryEntry, error)
	DeleteMemory(ctx context.Context, id string) error
	SearchMemories(ctx context.Context, q MemoryQuery) ([]*MemoryEntry, error)

	// ── Artifact ──

	SaveArtifact(ctx context.Context, a *Artifact) error
	GetArtifact(ctx context.Context, id string) (*Artifact, error)
	ListArtifacts(ctx context.Context, taskID string) ([]*Artifact, error)

	// ── Task Dependencies ──

	CreateDependency(ctx context.Context, d *TaskDependency) error
	ListDependencies(ctx context.Context, taskID string) ([]*TaskDependency, error)
	SatisfyDependency(ctx context.Context, id string) error

	// ── Vector Search ──

	// PutEmbedding stores the embedding vector for a memory entry.
	PutEmbedding(ctx context.Context, memoryID string, embedding []float32) error
	// SearchByVector performs ANN search returning scored memory IDs.
	SearchByVector(ctx context.Context, q VectorQuery) ([]ScoredEntry, error)

	// ── Context Graph ──

	PutNode(ctx context.Context, n *GraphNode) error
	PutEdge(ctx context.Context, e *GraphEdge) error
	GetNode(ctx context.Context, nodeID string) (*GraphNode, error)
	GetNeighbors(ctx context.Context, nodeID string, maxDepth int, limit int) ([]*GraphNode, []*GraphEdge, error)
	ListNodes(ctx context.Context) ([]GraphNode, error)
	ListEdges(ctx context.Context) ([]GraphEdge, error)
	Neighbors(ctx context.Context, nodeID string) ([]*GraphEdge, error)
	DeleteNode(ctx context.Context, nodeID string) error
	FindNodeByRef(ctx context.Context, tenantID string, kind GraphNodeKind, refID string) (*GraphNode, error)

	// ── KV Store ──
	// General-purpose key-value storage for configuration, state, and operational
	// data that was previously scattered across JSON files. Each entry is
	// namespaced to avoid key collisions across subsystems.

	KVPut(ctx context.Context, entry *KVEntry) error
	KVGet(ctx context.Context, namespace, key string) (*KVEntry, error)
	KVDelete(ctx context.Context, namespace, key string) error
	KVList(ctx context.Context, namespace string) ([]*KVEntry, error)

	// ── Lifecycle ──

	// Migrate creates or updates the database schema.
	Migrate(ctx context.Context) error
	// Close releases all resources held by the backend.
	Close() error
}

// KVEntry is a namespaced key-value record for configuration and operational data.
type KVEntry struct {
	Namespace string    `json:"namespace"` // e.g. "trust", "emotion", "session", "config"
	Key       string    `json:"key"`
	Value     []byte    `json:"value"` // JSON-encoded payload
	UpdatedAt time.Time `json:"updated_at"`
}
