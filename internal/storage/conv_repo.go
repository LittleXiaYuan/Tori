package storage

import (
	"context"
	"database/sql"
	"time"
)

// ConvSession represents a conversation in the database.
type ConvSession struct {
	ID        string
	TenantID  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ConvMessage represents a message in a conversation.
type ConvMessage struct {
	ID             int
	ConversationID string
	Role           string
	Content        string
	CreatedAt      time.Time
}

// ConvRepo provides PostgreSQL-backed conversation operations.
type ConvRepo struct {
	db *DB
}

// NewConvRepo creates a conversation repository.
func NewConvRepo(db *DB) *ConvRepo {
	return &ConvRepo{db: db}
}

func (r *ConvRepo) GetOrCreate(ctx context.Context, sessionID, tenantID string) (*ConvSession, error) {
	var s ConvSession
	err := r.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, created_at, updated_at FROM conversations WHERE id=$1`, sessionID).
		Scan(&s.ID, &s.TenantID, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		now := time.Now()
		_, err = r.db.ExecContext(ctx,
			`INSERT INTO conversations (id, tenant_id, created_at, updated_at) VALUES ($1, $2, $3, $4)`,
			sessionID, tenantID, now, now)
		if err != nil {
			return nil, err
		}
		return &ConvSession{ID: sessionID, TenantID: tenantID, CreatedAt: now, UpdatedAt: now}, nil
	}
	return &s, err
}

func (r *ConvRepo) Append(ctx context.Context, sessionID, role, content string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO conversation_messages (conversation_id, role, content) VALUES ($1, $2, $3)`,
		sessionID, role, content)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE conversations SET updated_at=NOW() WHERE id=$1`, sessionID)
	return err
}

func (r *ConvRepo) GetMessages(ctx context.Context, sessionID string, limit int) ([]ConvMessage, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, conversation_id, role, content, created_at FROM conversation_messages
		 WHERE conversation_id=$1 ORDER BY id DESC LIMIT $2`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []ConvMessage
	for rows.Next() {
		var m ConvMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	// Reverse to chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, rows.Err()
}

func (r *ConvRepo) Delete(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM conversations WHERE id=$1`, sessionID)
	return err
}

func (r *ConvRepo) ListByTenant(ctx context.Context, tenantID string) ([]ConvSession, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, tenant_id, created_at, updated_at FROM conversations
		 WHERE tenant_id=$1 ORDER BY updated_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConvSession
	for rows.Next() {
		var s ConvSession
		if err := rows.Scan(&s.ID, &s.TenantID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
