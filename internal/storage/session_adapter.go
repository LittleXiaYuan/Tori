package storage

import (
	"context"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/session"
)

// SessionRepoAdapter adapts storage.ConvRepo to implement session.Repo interface.
type SessionRepoAdapter struct {
	repo *ConvRepo
}

// NewSessionRepoAdapter creates a session repo adapter.
func NewSessionRepoAdapter(db *DB) *SessionRepoAdapter {
	return &SessionRepoAdapter{repo: NewConvRepo(db)}
}

func (a *SessionRepoAdapter) GetOrCreate(ctx context.Context, sessionID, tenantID string) error {
	_, err := a.repo.GetOrCreate(ctx, sessionID, tenantID)
	return err
}

func (a *SessionRepoAdapter) Append(ctx context.Context, sessionID, role, content string) error {
	return a.repo.Append(ctx, sessionID, role, content)
}

func (a *SessionRepoAdapter) GetMessages(ctx context.Context, sessionID string, limit int) ([]llm.Message, error) {
	msgs, err := a.repo.GetMessages(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]llm.Message, len(msgs))
	for i, m := range msgs {
		out[i] = llm.Message{Role: m.Role, Content: m.Content}
	}
	return out, nil
}

func (a *SessionRepoAdapter) Delete(ctx context.Context, sessionID string) error {
	return a.repo.Delete(ctx, sessionID)
}

func (a *SessionRepoAdapter) ListByTenant(ctx context.Context, tenantID string) ([]session.Session, error) {
	convs, err := a.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]session.Session, len(convs))
	for i, c := range convs {
		out[i] = session.Session{
			ID:        c.ID,
			TenantID:  c.TenantID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		}
	}
	return out, nil
}
