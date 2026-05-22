package ledger

import (
	"context"
	"fmt"
)

// Ledger is the main entry point for the state infrastructure.
// It provides access to all subsystems through its public fields.
type Ledger struct {
	// Tasks manages task lifecycle with event sourcing.
	Tasks *TaskManager

	// Events provides direct access to the event log.
	Events *EventStore

	// Checkpoints manages execution snapshots for crash recovery.
	Checkpoints *CheckpointManager

	// Resume handles task recovery from checkpoints.
	Resume *ResumeManager

	// Memory manages structured memory entries.
	Memory *MemoryStore

	// Recall provides task-aware memory retrieval (multi-stage).
	Recall *RecallEngine

	// Vector provides semantic embedding search over memories.
	Vector *VectorIndex

	// Graph provides entity relationship traversal.
	Graph *ContextGraph

	// Lifecycle manages memory consolidation, decay, and GC.
	Lifecycle *MemoryLifecycle

	// Artifacts manages task output metadata.
	Artifacts *ArtifactManager

	// Deps manages inter-task dependencies.
	Deps *DependencyManager

	// Bus provides real-time event streaming via pub/sub.
	Bus *EventBus

	// KV provides namespaced key-value storage for configuration and
	// operational data, replacing scattered JSON files with a single
	// SQLite-backed store.
	KV *KVStore

	backend Backend
}

// Open creates a new Ledger instance with the given storage backend.
// It runs migrations automatically upon creation.
//
// To use SQLite (zero-config default):
//
//	backend, _ := sqlite.New("./data/ledger/ledger.db")
//	ldg, _ := ledger.Open(backend)
//
// To use PostgreSQL (production):
//
//	backend, _ := postgres.New("postgres://user:pass@localhost/db")
//	ldg, _ := ledger.Open(backend)
func Open(b Backend) (*Ledger, error) {
	if b == nil {
		return nil, fmt.Errorf("ledger: backend must not be nil")
	}

	// Auto-migrate
	if err := b.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("ledger: migrate: %w", err)
	}

	bus := NewEventBus()
	es := &EventStore{backend: b, bus: bus}
	tm := &TaskManager{backend: b, events: es}
	cm := &CheckpointManager{backend: b, events: es}
	rm := &ResumeManager{backend: b, events: es, checkpoints: cm}
	ms := &MemoryStore{backend: b, events: es, pendingAccess: make(map[string]int)}
	vi := &VectorIndex{backend: b}
	re := &RecallEngine{backend: b, weights: DefaultWeights(), vector: vi}
	am := &ArtifactManager{backend: b, events: es}
	dm := &DependencyManager{backend: b}
	cg := &ContextGraph{backend: b}
	lc := NewMemoryLifecycle(b, vi)
	kv := &KVStore{backend: b}

	return &Ledger{
		Tasks:       tm,
		Events:      es,
		Checkpoints: cm,
		Resume:      rm,
		Memory:      ms,
		Recall:      re,
		Vector:      vi,
		Graph:       cg,
		Lifecycle:   lc,
		Artifacts:   am,
		Deps:        dm,
		Bus:         bus,
		KV:          kv,
		backend:     b,
	}, nil
}

// Close releases all resources held by the Ledger.
func (l *Ledger) Close() error {
	// Flush pending memory access counts before closing
	if l.Memory != nil {
		l.Memory.FlushAccessCounts(context.Background())
	}
	if l.Bus != nil {
		l.Bus.Close()
	}
	if l.backend != nil {
		return l.backend.Close()
	}
	return nil
}

// Backend returns the underlying storage backend for advanced use.
func (l *Ledger) Backend() Backend {
	return l.backend
}

// HealthChecker is an optional interface for backends that support health checks.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// Checkpointer is an optional interface for backends that support WAL checkpointing.
type Checkpointer interface {
	Checkpoint(ctx context.Context) (walPages, checkpointed int, err error)
}

// HealthCheck verifies database integrity. Returns nil if healthy.
func (l *Ledger) HealthCheck(ctx context.Context) error {
	if hc, ok := l.backend.(HealthChecker); ok {
		return hc.HealthCheck(ctx)
	}
	return nil
}

// Checkpoint triggers a WAL checkpoint if the backend supports it.
func (l *Ledger) Checkpoint(ctx context.Context) error {
	if cp, ok := l.backend.(Checkpointer); ok {
		_, _, err := cp.Checkpoint(ctx)
		return err
	}
	return nil
}
