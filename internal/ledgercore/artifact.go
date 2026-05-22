package ledger

import (
	"context"
	"encoding/json"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// ArtifactManager handles task artifact metadata.
// Actual file storage is handled externally; Ledger tracks metadata only.
type ArtifactManager struct {
	backend Backend
	events  *EventStore
}

// Save records an artifact's metadata and emits a creation event.
func (am *ArtifactManager) Save(ctx context.Context, a *Artifact) error {
	now := time.Now()
	if a.ID == "" {
		a.ID = ulid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.MimeType == "" {
		a.MimeType = "application/octet-stream"
	}
	if a.Metadata == nil {
		a.Metadata = JSON("{}")
	}

	if err := am.backend.SaveArtifact(ctx, a); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]interface{}{
		"artifact_id": a.ID,
		"name":        a.Name,
		"kind":        a.Kind,
		"size":        a.SizeBytes,
	})
	if err != nil {
		return err
	}
	return am.events.Append(ctx, &Event{
		ID:        ulid.New(),
		TaskID:    a.TaskID,
		Kind:      EventArtifactCreated,
		Actor:     "runtime",
		Payload:   payload,
		CreatedAt: now,
	})
}

// Get retrieves an artifact by ID.
func (am *ArtifactManager) Get(ctx context.Context, id string) (*Artifact, error) {
	return am.backend.GetArtifact(ctx, id)
}

// List returns all artifacts for a task.
func (am *ArtifactManager) List(ctx context.Context, taskID string) ([]*Artifact, error) {
	return am.backend.ListArtifacts(ctx, taskID)
}
