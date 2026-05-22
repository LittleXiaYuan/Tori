package ledger

import (
	"context"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// DependencyManager handles inter-task dependency relationships.
type DependencyManager struct {
	backend Backend
}

// Create registers a dependency: toTaskID depends on fromTaskID.
func (dm *DependencyManager) Create(ctx context.Context, fromTaskID, toTaskID string, kind DepKind) (*TaskDependency, error) {
	d := &TaskDependency{
		ID:         ulid.New(),
		FromTaskID: fromTaskID,
		ToTaskID:   toTaskID,
		Kind:       kind,
		Satisfied:  false,
		CreatedAt:  time.Now(),
	}
	if err := dm.backend.CreateDependency(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// CreateData registers a data dependency with an artifact reference.
func (dm *DependencyManager) CreateData(ctx context.Context, fromTaskID, toTaskID, artifactRef string) (*TaskDependency, error) {
	d := &TaskDependency{
		ID:          ulid.New(),
		FromTaskID:  fromTaskID,
		ToTaskID:    toTaskID,
		Kind:        DepData,
		ArtifactRef: &artifactRef,
		Satisfied:   false,
		CreatedAt:   time.Now(),
	}
	if err := dm.backend.CreateDependency(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// Satisfy marks a dependency as satisfied.
func (dm *DependencyManager) Satisfy(ctx context.Context, depID string) error {
	return dm.backend.SatisfyDependency(ctx, depID)
}

// ListFor returns all dependencies involving a task (as source or target).
func (dm *DependencyManager) ListFor(ctx context.Context, taskID string) ([]*TaskDependency, error) {
	return dm.backend.ListDependencies(ctx, taskID)
}

// Blockers returns unsatisfied blocking dependencies for a task.
func (dm *DependencyManager) Blockers(ctx context.Context, taskID string) ([]*TaskDependency, error) {
	all, err := dm.backend.ListDependencies(ctx, taskID)
	if err != nil {
		return nil, err
	}
	var blockers []*TaskDependency
	for _, d := range all {
		if d.ToTaskID == taskID && !d.Satisfied && d.Kind == DepBlocking {
			blockers = append(blockers, d)
		}
	}
	return blockers, nil
}

// IsBlocked returns true if a task has unsatisfied blocking dependencies.
func (dm *DependencyManager) IsBlocked(ctx context.Context, taskID string) (bool, error) {
	blockers, err := dm.Blockers(ctx, taskID)
	if err != nil {
		return false, err
	}
	return len(blockers) > 0, nil
}
