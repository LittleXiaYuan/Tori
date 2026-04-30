// Package storage is DEPRECATED. All runtime persistence is handled through
// internal/ledger (backed by its own SQLite database). This package is
// retained only as a reference for the original schema; it is not imported
// by any production code. Safe to remove in a future cleanup pass.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"yunque-agent/internal/agentcore/llm"

	_ "modernc.org/sqlite"
)

// MemoryItem represents a memory record.
type MemoryItem struct {
	ID         string
	TenantID   string
	Key        string
	Value      string
	Source     string
	Category   string
	AccessCnt  int
	LastAccess time.Time
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// ConvSession represents a conversation session.
type ConvSession struct {
	ID        string
	TenantID  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SQLite implements persistence for memories, sessions, bots, inbox, and models.
//
// Deprecated (informational): the agent currently persists this data through
// `internal/ledger` (which itself uses SQLite under the hood). This type is
// retained as a self-contained helper for future tooling but is not part of
// the live runtime path.
type SQLite struct {
	db *sql.DB
}

// New opens or creates a SQLite database and runs migrations.
func New(path string) (*SQLite, error) {
	if path == "" {
		path = "data/yunque.db"
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite single-writer
	s := &SQLite{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

func (s *SQLite) migrate() error {
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			source TEXT DEFAULT '',
			category TEXT DEFAULT '',
			score REAL DEFAULT 0,
			embedding BLOB,
			access_cnt INTEGER DEFAULT 0,
			last_access DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			UNIQUE(tenant_id, key)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_tenant ON memories(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_category ON memories(tenant_id, category)`,

		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_tenant ON sessions(tenant_id)`,

		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id)`,

		`CREATE TABLE IF NOT EXISTS bots (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			status TEXT DEFAULT 'idle',
			is_active INTEGER DEFAULT 1,
			config TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS inbox (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			content TEXT NOT NULL,
			action TEXT DEFAULT '',
			is_read INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_inbox_read ON inbox(is_read)`,

		`CREATE TABLE IF NOT EXISTS models (
			id TEXT PRIMARY KEY,
			model_id TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT DEFAULT 'chat',
			client_type TEXT DEFAULT 'openai',
			base_url TEXT DEFAULT '',
			input_modalities TEXT DEFAULT '[]',
			supports_reasoning INTEGER DEFAULT 0,
			dimensions INTEGER DEFAULT 0,
			is_primary INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range ddl {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec ddl: %w\nSQL: %s", err, stmt)
		}
	}
	slog.Info("sqlite migrations complete")
	return nil
}

// ── SQLite Memory operations ──

func (s *SQLite) MemoryPut(ctx context.Context, tenantID string, item MemoryItem) error {
	var embBytes []byte
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memories (id, tenant_id, key, value, source, category, score, embedding, access_cnt, last_access, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(tenant_id, key) DO UPDATE SET
			value=excluded.value, source=excluded.source, category=excluded.category,
			score=excluded.score, embedding=excluded.embedding, access_cnt=excluded.access_cnt,
			last_access=excluded.last_access`,
		item.ID, tenantID, item.Key, item.Value, item.Source, item.Category,
		0.0, embBytes, item.AccessCnt, item.LastAccess, item.CreatedAt, item.ExpiresAt,
	)
	return err
}

func (s *SQLite) MemoryGet(ctx context.Context, tenantID, key string) (*MemoryItem, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, key, value, source, category, access_cnt, created_at
		 FROM memories WHERE tenant_id = ? AND key = ?`, tenantID, key)

	var item MemoryItem
	item.TenantID = tenantID
	err := row.Scan(&item.ID, &item.Key, &item.Value, &item.Source, &item.Category,
		&item.AccessCnt, &item.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	s.db.ExecContext(ctx,
		`UPDATE memories SET access_cnt = access_cnt + 1, last_access = ? WHERE tenant_id = ? AND key = ?`,
		time.Now(), tenantID, key)

	return &item, nil
}

func (s *SQLite) MemorySearch(ctx context.Context, tenantID, query string, limit int) ([]MemoryItem, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, key, value, source, category, access_cnt, created_at
		 FROM memories WHERE tenant_id = ? AND (key LIKE ? OR value LIKE ?)
		 ORDER BY access_cnt DESC LIMIT ?`,
		tenantID, "%"+query+"%", "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSQLiteMemoryItems(rows, tenantID)
}

func (s *SQLite) MemoryDelete(ctx context.Context, tenantID, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM memories WHERE tenant_id = ? AND key = ?`, tenantID, key)
	return err
}

func (s *SQLite) MemoryList(ctx context.Context, tenantID, prefix string, limit int) ([]MemoryItem, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT id, key, value, source, category, access_cnt, created_at FROM memories WHERE tenant_id = ?`
	args := []interface{}{tenantID}
	if prefix != "" {
		q += ` AND key LIKE ?`
		args = append(args, prefix+"%")
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSQLiteMemoryItems(rows, tenantID)
}

func scanSQLiteMemoryItems(rows *sql.Rows, tenantID string) ([]MemoryItem, error) {
	var items []MemoryItem
	for rows.Next() {
		var it MemoryItem
		it.TenantID = tenantID
		if err := rows.Scan(&it.ID, &it.Key, &it.Value, &it.Source, &it.Category, &it.AccessCnt, &it.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ── Session Repo implementation ──

func (s *SQLite) GetOrCreate(ctx context.Context, sessionID, tenantID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, tenant_id) VALUES (?, ?) ON CONFLICT(id) DO NOTHING`,
		sessionID, tenantID)
	return err
}

func (s *SQLite) Append(ctx context.Context, sessionID, role, content string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO messages (session_id, role, content) VALUES (?, ?, ?)`,
		sessionID, role, content)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, sessionID)
	return err
}

func (s *SQLite) GetMessages(ctx context.Context, sessionID string, limit int) ([]llm.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT role, content FROM (
			SELECT role, content, id FROM messages WHERE session_id = ? ORDER BY id DESC LIMIT ?
		) sub ORDER BY id ASC`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []llm.Message
	for rows.Next() {
		var m llm.Message
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (s *SQLite) SessionDelete(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

func (s *SQLite) SessionListByTenant(ctx context.Context, tenantID string) ([]ConvSession, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, created_at, updated_at FROM sessions WHERE tenant_id = ? ORDER BY updated_at DESC`,
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConvSession
	for rows.Next() {
		var r ConvSession
		if err := rows.Scan(&r.ID, &r.TenantID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Bot persistence ──

type BotRow struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	IsActive    bool      `json:"is_active"`
	Config      string    `json:"config"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *SQLite) PutBot(ctx context.Context, bot BotRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bots (id, name, description, status, is_active, config, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, description=excluded.description, status=excluded.status,
			is_active=excluded.is_active, config=excluded.config, updated_at=excluded.updated_at`,
		bot.ID, bot.Name, bot.Description, bot.Status, bot.IsActive, bot.Config, bot.CreatedAt, bot.UpdatedAt,
	)
	return err
}

func (s *SQLite) GetBot(ctx context.Context, id string) (*BotRow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, status, is_active, config, created_at, updated_at FROM bots WHERE id = ?`, id)
	var b BotRow
	err := row.Scan(&b.ID, &b.Name, &b.Description, &b.Status, &b.IsActive, &b.Config, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &b, err
}

func (s *SQLite) ListBots(ctx context.Context) ([]BotRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, status, is_active, config, created_at, updated_at FROM bots ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BotRow
	for rows.Next() {
		var b BotRow
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.Status, &b.IsActive, &b.Config, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *SQLite) DeleteBot(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bots WHERE id = ?`, id)
	return err
}

// ── Inbox persistence ──

type InboxRow struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	Content   string    `json:"content"`
	Action    string    `json:"action"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *SQLite) PushInbox(ctx context.Context, item InboxRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO inbox (id, source, content, action, is_read, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		item.ID, item.Source, item.Content, item.Action, item.IsRead, item.CreatedAt,
	)
	return err
}

func (s *SQLite) ListInbox(ctx context.Context, limit int) ([]InboxRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, source, content, action, is_read, created_at FROM inbox ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InboxRow
	for rows.Next() {
		var r InboxRow
		if err := rows.Scan(&r.ID, &r.Source, &r.Content, &r.Action, &r.IsRead, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLite) MarkAllInboxRead(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE inbox SET is_read = 1 WHERE is_read = 0`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SQLite) InboxCounts(ctx context.Context) (total, unread int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*), SUM(CASE WHEN is_read = 0 THEN 1 ELSE 0 END) FROM inbox`).Scan(&total, &unread)
	return
}

// ── Model persistence ──

type ModelRow struct {
	ID                string    `json:"id"`
	ModelID           string    `json:"model_id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`
	ClientType        string    `json:"client_type"`
	BaseURL           string    `json:"base_url"`
	InputModalities   string    `json:"input_modalities"` // JSON array
	SupportsReasoning bool      `json:"supports_reasoning"`
	Dimensions        int       `json:"dimensions"`
	IsPrimary         bool      `json:"is_primary"`
	CreatedAt         time.Time `json:"created_at"`
}

func (s *SQLite) PutModel(ctx context.Context, m ModelRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO models (id, model_id, name, type, client_type, base_url, input_modalities, supports_reasoning, dimensions, is_primary, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			model_id=excluded.model_id, name=excluded.name, type=excluded.type,
			client_type=excluded.client_type, base_url=excluded.base_url,
			input_modalities=excluded.input_modalities, supports_reasoning=excluded.supports_reasoning,
			dimensions=excluded.dimensions, is_primary=excluded.is_primary`,
		m.ID, m.ModelID, m.Name, m.Type, m.ClientType, m.BaseURL,
		m.InputModalities, m.SupportsReasoning, m.Dimensions, m.IsPrimary, m.CreatedAt,
	)
	return err
}

func (s *SQLite) ListModels(ctx context.Context) ([]ModelRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, model_id, name, type, client_type, base_url, input_modalities, supports_reasoning, dimensions, is_primary, created_at
		 FROM models ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelRow
	for rows.Next() {
		var m ModelRow
		if err := rows.Scan(&m.ID, &m.ModelID, &m.Name, &m.Type, &m.ClientType, &m.BaseURL,
			&m.InputModalities, &m.SupportsReasoning, &m.Dimensions, &m.IsPrimary, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *SQLite) DeleteModel(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM models WHERE id = ?`, id)
	return err
}
