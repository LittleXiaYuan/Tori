// Package sqlite implements the Ledger Backend interface using SQLite.
// This is the default, zero-configuration storage backend.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/ledgercore"

	_ "modernc.org/sqlite"
)

// Backend implements ledger.Backend using SQLite.
type Backend struct {
	db         *sql.DB
	mu         sync.RWMutex     // protects seq counters
	seqs       map[string]int64 // task_id -> latest seq (cache)
	seqsLoaded map[string]bool  // tracks which task seqs were loaded from DB
}

// New creates a new SQLite backend. The dbPath is the path to the database file.
// Use ":memory:" for an in-memory database (useful for testing).
func New(dbPath string) (*Backend, error) {
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}

	// Production hardening PRAGMAs
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",         // enforce declared REFERENCES constraints
		"PRAGMA cache_size = -8000",        // 8MB page cache
		"PRAGMA mmap_size = 134217728",     // 128MB memory-mapped I/O
		"PRAGMA temp_store = MEMORY",       // keep temp tables in memory
		"PRAGMA wal_autocheckpoint = 1000", // checkpoint every 1000 pages (~4MB)
	} {
		if _, err := db.Exec(pragma); err != nil {
			// Non-fatal: some PRAGMAs may not be supported
			_ = err
		}
	}

	return &Backend{db: db, seqs: make(map[string]int64), seqsLoaded: make(map[string]bool)}, nil
}

// Close checkpoints WAL and closes the database.
func (b *Backend) Close() error {
	// Final WAL checkpoint before closing to minimize data on disk
	b.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return b.db.Close()
}

// Checkpoint forces a WAL checkpoint (PASSIVE mode by default).
// Call periodically to prevent WAL file from growing unbounded.
func (b *Backend) Checkpoint(ctx context.Context) (walPages, checkpointed int, err error) {
	row := b.db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)")
	var busy int
	if err := row.Scan(&busy, &walPages, &checkpointed); err != nil {
		return 0, 0, fmt.Errorf("sqlite: checkpoint: %w", err)
	}
	return walPages, checkpointed, nil
}

// HealthCheck runs SQLite integrity check on the database.
func (b *Backend) HealthCheck(ctx context.Context) error {
	row := b.db.QueryRowContext(ctx, "PRAGMA integrity_check(1)")
	var result string
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("sqlite: health check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("sqlite: integrity check failed: %s", result)
	}
	var foreignKeys int
	if err := b.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return fmt.Errorf("sqlite: foreign key check: %w", err)
	}
	if foreignKeys != 1 {
		return fmt.Errorf("sqlite: foreign key enforcement disabled")
	}
	var version int
	if err := b.db.QueryRowContext(ctx,
		`SELECT version FROM schema_migrations WHERE id = 'ledger_sqlite'`,
	).Scan(&version); err != nil {
		return fmt.Errorf("sqlite: schema version check: %w", err)
	}
	if version != currentSchemaVersion {
		return fmt.Errorf("sqlite: schema version %d, want %d", version, currentSchemaVersion)
	}
	return nil
}

// Stats returns database statistics for monitoring.
type Stats struct {
	DBSizeBytes   int64            `json:"db_size_bytes"`
	WALSizeBytes  int64            `json:"wal_size_bytes"`
	PageSize      int              `json:"page_size"`
	PageCount     int64            `json:"page_count"`
	FreelistCount int64            `json:"freelist_count"`
	TableCounts   map[string]int64 `json:"table_counts"`
}

func (b *Backend) Stats(ctx context.Context) (*Stats, error) {
	s := &Stats{TableCounts: make(map[string]int64)}

	b.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&s.PageSize)
	b.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&s.PageCount)
	b.db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&s.FreelistCount)
	s.DBSizeBytes = int64(s.PageSize) * s.PageCount

	for _, table := range []string{"tasks", "events", "checkpoints", "memories", "artifacts", "kv_store", "graph_nodes", "graph_edges", "task_deps"} {
		var count int64
		if err := b.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err == nil {
			s.TableCounts[table] = count
		}
	}
	return s, nil
}

// ── Migration ──

const currentSchemaVersion = 2

func (b *Backend) Migrate(ctx context.Context) error {
	if _, err := b.db.ExecContext(ctx, schemaMigrationTable); err != nil {
		return fmt.Errorf("%w: %v", ledger.ErrMigrationFailed, err)
	}
	for _, stmt := range schema {
		if _, err := b.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%w: %v", ledger.ErrMigrationFailed, err)
		}
	}
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO schema_migrations (id, version, applied_at)
		 VALUES ('ledger_sqlite', ?, ?)
		 ON CONFLICT(id) DO UPDATE SET version = excluded.version, applied_at = excluded.applied_at`,
		currentSchemaVersion, formatTime(time.Now()),
	); err != nil {
		return fmt.Errorf("%w: %v", ledger.ErrMigrationFailed, err)
	}
	return nil
}

var schemaMigrationTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
	id         TEXT PRIMARY KEY,
	version    INTEGER NOT NULL,
	applied_at TEXT NOT NULL
)`

var schema = []string{
	`CREATE TABLE IF NOT EXISTS tasks (
		id             TEXT PRIMARY KEY,
		type           TEXT NOT NULL DEFAULT 'goal',
		goal           TEXT NOT NULL,
		status         TEXT NOT NULL DEFAULT 'created',
		tenant_id      TEXT NOT NULL,
		agent_id       TEXT NOT NULL DEFAULT 'default',
		user_id        TEXT NOT NULL DEFAULT '',
		parent_task_id TEXT,
		input          TEXT DEFAULT '{}',
		output         TEXT DEFAULT '{}',
		error          TEXT,
		retry_count    INTEGER NOT NULL DEFAULT 0,
		max_retries    INTEGER NOT NULL DEFAULT 2,
		checkpoint_ref TEXT,
		priority       INTEGER NOT NULL DEFAULT 0,
		metadata       TEXT DEFAULT '{}',
		version        INTEGER NOT NULL DEFAULT 0,
		created_at     TEXT NOT NULL,
		updated_at     TEXT NOT NULL,
		started_at     TEXT,
		finished_at    TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_tenant_status ON tasks(tenant_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_task_id)`,

	`CREATE TABLE IF NOT EXISTS events (
		id          TEXT PRIMARY KEY,
		task_id     TEXT NOT NULL REFERENCES tasks(id),
		kind        TEXT NOT NULL,
		seq         INTEGER NOT NULL,
		actor       TEXT NOT NULL,
		payload     TEXT DEFAULT '{}',
		parent_id   TEXT,
		duration_ms INTEGER,
		created_at  TEXT NOT NULL,
		UNIQUE(task_id, seq)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_events_task_seq ON events(task_id, seq)`,
	`CREATE INDEX IF NOT EXISTS idx_events_kind ON events(kind)`,

	`CREATE TABLE IF NOT EXISTS artifacts (
		id          TEXT PRIMARY KEY,
		task_id     TEXT NOT NULL REFERENCES tasks(id),
		name        TEXT NOT NULL,
		kind        TEXT NOT NULL,
		mime_type   TEXT NOT NULL DEFAULT 'application/octet-stream',
		size_bytes  INTEGER NOT NULL DEFAULT 0,
		storage_ref TEXT NOT NULL,
		checksum    TEXT NOT NULL,
		metadata    TEXT DEFAULT '{}',
		created_at  TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_artifacts_task ON artifacts(task_id)`,

	`CREATE TABLE IF NOT EXISTS memories (
		id           TEXT PRIMARY KEY,
		tenant_id    TEXT NOT NULL,
		task_id      TEXT,
		kind         TEXT NOT NULL,
		key          TEXT NOT NULL,
		content      TEXT NOT NULL,
		source       TEXT NOT NULL DEFAULT 'extraction',
		confidence   REAL NOT NULL DEFAULT 0.5,
		access_count INTEGER NOT NULL DEFAULT 0,
		last_access  TEXT,
		expires_at   TEXT,
		metadata     TEXT DEFAULT '{}',
		created_at   TEXT NOT NULL,
		updated_at   TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_memories_tenant_kind ON memories(tenant_id, kind)`,
	`CREATE INDEX IF NOT EXISTS idx_memories_key ON memories(tenant_id, key)`,
	`CREATE INDEX IF NOT EXISTS idx_memories_task ON memories(task_id)`,

	`CREATE TABLE IF NOT EXISTS checkpoints (
		id          TEXT PRIMARY KEY,
		task_id     TEXT NOT NULL REFERENCES tasks(id),
		event_seq   INTEGER NOT NULL,
		step_index  INTEGER NOT NULL,
		task_state  TEXT NOT NULL,
		working_mem TEXT DEFAULT '{}',
		size_bytes  INTEGER NOT NULL DEFAULT 0,
		reason      TEXT NOT NULL,
		created_at  TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_checkpoints_task ON checkpoints(task_id, created_at DESC)`,

	`CREATE TABLE IF NOT EXISTS task_dependencies (
		id           TEXT PRIMARY KEY,
		from_task_id TEXT NOT NULL REFERENCES tasks(id),
		to_task_id   TEXT NOT NULL REFERENCES tasks(id),
		kind         TEXT NOT NULL DEFAULT 'blocking',
		artifact_ref TEXT,
		satisfied    INTEGER NOT NULL DEFAULT 0,
		created_at   TEXT NOT NULL,
		UNIQUE(from_task_id, to_task_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_deps_to ON task_dependencies(to_task_id)`,

	// ── Embeddings (vector store) ──
	`CREATE TABLE IF NOT EXISTS embeddings (
		memory_id TEXT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
		vector    BLOB NOT NULL,
		dims      INTEGER NOT NULL
	)`,

	// ── Context Graph ──
	`CREATE TABLE IF NOT EXISTS graph_nodes (
		id        TEXT PRIMARY KEY,
		kind      TEXT NOT NULL,
		label     TEXT NOT NULL,
		ref_id    TEXT NOT NULL,
		tenant_id TEXT NOT NULL,
		metadata  TEXT DEFAULT '{}'
	)`,
	`DROP INDEX IF EXISTS idx_graph_nodes_ref`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_nodes_tenant_ref ON graph_nodes(tenant_id, kind, ref_id)`,
	`CREATE INDEX IF NOT EXISTS idx_graph_nodes_tenant ON graph_nodes(tenant_id, kind)`,

	`CREATE TABLE IF NOT EXISTS graph_edges (
		id        TEXT PRIMARY KEY,
		from_id   TEXT NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
		to_id     TEXT NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
		kind      TEXT NOT NULL,
		weight    REAL NOT NULL DEFAULT 1.0,
		metadata  TEXT DEFAULT '{}'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_graph_edges_from ON graph_edges(from_id)`,
	`CREATE INDEX IF NOT EXISTS idx_graph_edges_to ON graph_edges(to_id)`,

	// ── KV Store ── (replaces scattered JSON files)
	`CREATE TABLE IF NOT EXISTS kv_store (
		namespace  TEXT NOT NULL,
		key        TEXT NOT NULL,
		value      BLOB NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (namespace, key)
	)`,
}

// ── Time helpers ──

const timeFormat = time.RFC3339Nano

func formatTime(t time.Time) string { return t.Format(timeFormat) }
func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(timeFormat)
	return &s
}
func parseTime(s string) time.Time {
	t, _ := time.Parse(timeFormat, s)
	return t
}
func parseTimePtr(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t := parseTime(*s)
	return &t
}

func jsonBytes(v interface{}) []byte {
	b, _ := json.Marshal(v)
	if b == nil {
		return []byte("{}")
	}
	return b
}

// ── Task ──

func (b *Backend) CreateTask(ctx context.Context, t *ledger.Task) error {
	_, err := b.db.ExecContext(ctx,
		`INSERT INTO tasks (id, type, goal, status, tenant_id, agent_id, user_id,
			parent_task_id, input, output, error, retry_count, max_retries,
			checkpoint_ref, priority, metadata, version, created_at, updated_at,
			started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Type, t.Goal, t.Status, t.TenantID, t.AgentID, t.UserID,
		t.ParentTaskID, string(t.Input), string(t.Output), t.Error,
		t.RetryCount, t.MaxRetries, t.CheckpointRef, t.Priority,
		string(t.Metadata), t.Version,
		formatTime(t.CreatedAt), formatTime(t.UpdatedAt),
		formatTimePtr(t.StartedAt), formatTimePtr(t.FinishedAt),
	)
	return err
}

func (b *Backend) CreateTaskWithEvent(ctx context.Context, t *ledger.Task, e *ledger.Event) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := b.createTaskTx(ctx, tx, t); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := b.appendEventTx(ctx, tx, e); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (b *Backend) createTaskTx(ctx context.Context, tx *sql.Tx, t *ledger.Task) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tasks (id, type, goal, status, tenant_id, agent_id, user_id,
			parent_task_id, input, output, error, retry_count, max_retries,
			checkpoint_ref, priority, metadata, version, created_at, updated_at,
			started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Type, t.Goal, t.Status, t.TenantID, t.AgentID, t.UserID,
		t.ParentTaskID, string(t.Input), string(t.Output), t.Error,
		t.RetryCount, t.MaxRetries, t.CheckpointRef, t.Priority,
		string(t.Metadata), t.Version,
		formatTime(t.CreatedAt), formatTime(t.UpdatedAt),
		formatTimePtr(t.StartedAt), formatTimePtr(t.FinishedAt),
	)
	return err
}

func (b *Backend) GetTask(ctx context.Context, id string) (*ledger.Task, error) {
	row := b.db.QueryRowContext(ctx,
		`SELECT id, type, goal, status, tenant_id, agent_id, user_id,
			parent_task_id, input, output, error, retry_count, max_retries,
			checkpoint_ref, priority, metadata, version, created_at, updated_at,
			started_at, finished_at
		FROM tasks WHERE id = ?`, id)
	return scanTask(row)
}

func (b *Backend) UpdateTask(ctx context.Context, t *ledger.Task) error {
	res, err := b.db.ExecContext(ctx,
		`UPDATE tasks SET type=?, goal=?, status=?, tenant_id=?, agent_id=?, user_id=?,
			parent_task_id=?, input=?, output=?, error=?, retry_count=?, max_retries=?,
			checkpoint_ref=?, priority=?, metadata=?, version=version+1,
			updated_at=?, started_at=?, finished_at=?
		WHERE id=? AND version=?`,
		t.Type, t.Goal, t.Status, t.TenantID, t.AgentID, t.UserID,
		t.ParentTaskID, string(t.Input), string(t.Output), t.Error,
		t.RetryCount, t.MaxRetries, t.CheckpointRef, t.Priority,
		string(t.Metadata), formatTime(t.UpdatedAt),
		formatTimePtr(t.StartedAt), formatTimePtr(t.FinishedAt),
		t.ID, t.Version,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ledger.ErrVersionConflict
	}
	t.Version++
	return nil
}

func (b *Backend) UpdateTaskWithEvent(ctx context.Context, t *ledger.Task, e *ledger.Event) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := b.updateTaskTx(ctx, tx, t); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := b.appendEventTx(ctx, tx, e); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (b *Backend) updateTaskTx(ctx context.Context, tx *sql.Tx, t *ledger.Task) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE tasks SET type=?, goal=?, status=?, tenant_id=?, agent_id=?, user_id=?,
			parent_task_id=?, input=?, output=?, error=?, retry_count=?, max_retries=?,
			checkpoint_ref=?, priority=?, metadata=?, version=version+1,
			updated_at=?, started_at=?, finished_at=?
		WHERE id=? AND version=?`,
		t.Type, t.Goal, t.Status, t.TenantID, t.AgentID, t.UserID,
		t.ParentTaskID, string(t.Input), string(t.Output), t.Error,
		t.RetryCount, t.MaxRetries, t.CheckpointRef, t.Priority,
		string(t.Metadata), formatTime(t.UpdatedAt),
		formatTimePtr(t.StartedAt), formatTimePtr(t.FinishedAt),
		t.ID, t.Version,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ledger.ErrVersionConflict
	}
	t.Version++
	return nil
}

func (b *Backend) ListTasks(ctx context.Context, f ledger.TaskFilter) ([]*ledger.Task, error) {
	var clauses []string
	var args []interface{}

	if f.TenantID != "" {
		clauses = append(clauses, "tenant_id = ?")
		args = append(args, f.TenantID)
	}
	if len(f.Status) > 0 {
		ph := make([]string, len(f.Status))
		for i, s := range f.Status {
			ph[i] = "?"
			args = append(args, string(s))
		}
		clauses = append(clauses, "status IN ("+strings.Join(ph, ",")+")")
	}
	if f.Type != nil {
		clauses = append(clauses, "type = ?")
		args = append(args, string(*f.Type))
	}
	if f.ParentTaskID != nil {
		clauses = append(clauses, "parent_task_id = ?")
		args = append(args, *f.ParentTaskID)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, type, goal, status, tenant_id, agent_id, user_id,
		parent_task_id, input, output, error, retry_count, max_retries,
		checkpoint_ref, priority, metadata, version, created_at, updated_at,
		started_at, finished_at
		FROM tasks` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*ledger.Task
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanTask(row *sql.Row) (*ledger.Task, error) {
	t := &ledger.Task{}
	var input, output, metadata string
	var createdAt, updatedAt string
	var startedAt, finishedAt *string

	err := row.Scan(
		&t.ID, &t.Type, &t.Goal, &t.Status, &t.TenantID, &t.AgentID, &t.UserID,
		&t.ParentTaskID, &input, &output, &t.Error, &t.RetryCount, &t.MaxRetries,
		&t.CheckpointRef, &t.Priority, &metadata, &t.Version,
		&createdAt, &updatedAt, &startedAt, &finishedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ledger.ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}

	t.Input = ledger.JSON(input)
	t.Output = ledger.JSON(output)
	t.Metadata = ledger.JSON(metadata)
	t.CreatedAt = parseTime(createdAt)
	t.UpdatedAt = parseTime(updatedAt)
	t.StartedAt = parseTimePtr(startedAt)
	t.FinishedAt = parseTimePtr(finishedAt)
	return t, nil
}

func scanTaskRow(rows *sql.Rows) (*ledger.Task, error) {
	t := &ledger.Task{}
	var input, output, metadata string
	var createdAt, updatedAt string
	var startedAt, finishedAt *string

	err := rows.Scan(
		&t.ID, &t.Type, &t.Goal, &t.Status, &t.TenantID, &t.AgentID, &t.UserID,
		&t.ParentTaskID, &input, &output, &t.Error, &t.RetryCount, &t.MaxRetries,
		&t.CheckpointRef, &t.Priority, &metadata, &t.Version,
		&createdAt, &updatedAt, &startedAt, &finishedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Input = ledger.JSON(input)
	t.Output = ledger.JSON(output)
	t.Metadata = ledger.JSON(metadata)
	t.CreatedAt = parseTime(createdAt)
	t.UpdatedAt = parseTime(updatedAt)
	t.StartedAt = parseTimePtr(startedAt)
	t.FinishedAt = parseTimePtr(finishedAt)
	return t, nil
}

// ── Event ──

func (b *Backend) AppendEvent(ctx context.Context, e *ledger.Event) error {
	return b.appendEventExec(ctx, b.db, e)
}

type eventExecer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (b *Backend) appendEventTx(ctx context.Context, tx *sql.Tx, e *ledger.Event) error {
	return b.appendEventExec(ctx, tx, e)
}

func (b *Backend) appendEventExec(ctx context.Context, execer eventExecer, e *ledger.Event) error {
	if e.Seq != 0 {
		return insertEvent(ctx, execer, e)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	prevSeq := b.seqs[e.TaskID]
	prevLoaded := b.seqsLoaded[e.TaskID]

	// On first access per task, load latest seq from DB to prevent conflicts after restart.
	if !prevLoaded {
		var maxSeq sql.NullInt64
		if err := execer.QueryRowContext(ctx, `SELECT MAX(seq) FROM events WHERE task_id = ?`, e.TaskID).Scan(&maxSeq); err == nil && maxSeq.Valid {
			b.seqs[e.TaskID] = maxSeq.Int64
			prevSeq = maxSeq.Int64
		}
		b.seqsLoaded[e.TaskID] = true
	}
	b.seqs[e.TaskID]++
	e.Seq = b.seqs[e.TaskID]

	if err := insertEvent(ctx, execer, e); err != nil {
		if prevLoaded {
			b.seqs[e.TaskID] = prevSeq
			b.seqsLoaded[e.TaskID] = true
		} else {
			delete(b.seqs, e.TaskID)
			delete(b.seqsLoaded, e.TaskID)
		}
		e.Seq = 0
		return err
	}
	return nil
}

func insertEvent(ctx context.Context, execer eventExecer, e *ledger.Event) error {
	_, err := execer.ExecContext(ctx,
		`INSERT INTO events (id, task_id, kind, seq, actor, payload, parent_id, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.TaskID, e.Kind, e.Seq, e.Actor,
		string(e.Payload), e.ParentID, e.DurationMs,
		formatTime(e.CreatedAt),
	)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return ledger.ErrEventSeqConflict
	}
	return err
}

func (b *Backend) ListEvents(ctx context.Context, taskID string, afterSeq int64, limit int) ([]*ledger.Event, error) {
	var rows *sql.Rows
	var err error
	if limit <= 0 {
		rows, err = b.db.QueryContext(ctx,
			`SELECT id, task_id, kind, seq, actor, payload, parent_id, duration_ms, created_at
			FROM events WHERE task_id = ? AND seq > ? ORDER BY seq`,
			taskID, afterSeq)
	} else {
		rows, err = b.db.QueryContext(ctx,
			`SELECT id, task_id, kind, seq, actor, payload, parent_id, duration_ms, created_at
			FROM events WHERE task_id = ? AND seq > ? ORDER BY seq LIMIT ?`,
			taskID, afterSeq, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEventRows(rows)
}

func (b *Backend) CountEvents(ctx context.Context, taskID string) (int64, error) {
	var count int64
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE task_id = ?`, taskID).Scan(&count)
	return count, err
}

func (b *Backend) LatestEventSeq(ctx context.Context, taskID string) (int64, error) {
	var seq sql.NullInt64
	err := b.db.QueryRowContext(ctx, `SELECT MAX(seq) FROM events WHERE task_id = ?`, taskID).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 0, nil
	}
	return seq.Int64, nil
}

func (b *Backend) QueryEvents(ctx context.Context, q ledger.EventQuery) ([]*ledger.Event, error) {
	var clauses []string
	var args []interface{}

	if q.TaskID != "" {
		clauses = append(clauses, "task_id = ?")
		args = append(args, q.TaskID)
	}
	if len(q.Kinds) > 0 {
		ph := make([]string, len(q.Kinds))
		for i, k := range q.Kinds {
			ph[i] = "?"
			args = append(args, string(k))
		}
		clauses = append(clauses, "kind IN ("+strings.Join(ph, ",")+")")
	}
	if len(q.Actors) > 0 {
		ph := make([]string, len(q.Actors))
		for i, a := range q.Actors {
			ph[i] = "?"
			args = append(args, a)
		}
		clauses = append(clauses, "actor IN ("+strings.Join(ph, ",")+")")
	}
	if q.After != nil {
		clauses = append(clauses, "created_at > ?")
		args = append(args, formatTime(*q.After))
	}
	if q.Before != nil {
		clauses = append(clauses, "created_at < ?")
		args = append(args, formatTime(*q.Before))
	}
	if q.AfterSeq > 0 {
		clauses = append(clauses, "seq > ?")
		args = append(args, q.AfterSeq)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	query := `SELECT id, task_id, kind, seq, actor, payload, parent_id, duration_ms, created_at
		FROM events` + where + ` ORDER BY created_at ASC, seq ASC`

	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}
	if q.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", q.Offset)
	}

	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEventRows(rows)
}

func scanEventRows(rows *sql.Rows) ([]*ledger.Event, error) {
	var events []*ledger.Event
	for rows.Next() {
		e := &ledger.Event{}
		var payload, createdAt string
		err := rows.Scan(&e.ID, &e.TaskID, &e.Kind, &e.Seq, &e.Actor,
			&payload, &e.ParentID, &e.DurationMs, &createdAt)
		if err != nil {
			return nil, err
		}
		e.Payload = ledger.JSON(payload)
		e.CreatedAt = parseTime(createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}

// ── Checkpoint ──

func (b *Backend) SaveCheckpoint(ctx context.Context, cp *ledger.Checkpoint) error {
	_, err := b.db.ExecContext(ctx,
		`INSERT INTO checkpoints (id, task_id, event_seq, step_index, task_state, working_mem, size_bytes, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cp.ID, cp.TaskID, cp.EventSeq, cp.StepIndex,
		string(cp.TaskState), string(cp.WorkingMem),
		cp.SizeBytes, cp.Reason, formatTime(cp.CreatedAt),
	)
	return err
}

func (b *Backend) LatestCheckpoint(ctx context.Context, taskID string) (*ledger.Checkpoint, error) {
	row := b.db.QueryRowContext(ctx,
		`SELECT id, task_id, event_seq, step_index, task_state, working_mem, size_bytes, reason, created_at
		FROM checkpoints WHERE task_id = ? ORDER BY created_at DESC LIMIT 1`, taskID)

	cp := &ledger.Checkpoint{}
	var taskState, workingMem, createdAt string
	err := row.Scan(&cp.ID, &cp.TaskID, &cp.EventSeq, &cp.StepIndex,
		&taskState, &workingMem, &cp.SizeBytes, &cp.Reason, &createdAt)
	if err == sql.ErrNoRows {
		return nil, ledger.ErrCheckpointNotFound
	}
	if err != nil {
		return nil, err
	}
	cp.TaskState = ledger.JSON(taskState)
	cp.WorkingMem = ledger.JSON(workingMem)
	cp.CreatedAt = parseTime(createdAt)
	return cp, nil
}

func (b *Backend) ListCheckpoints(ctx context.Context, taskID string, limit int) ([]*ledger.Checkpoint, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, task_id, event_seq, step_index, task_state, working_mem, size_bytes, reason, created_at
		FROM checkpoints WHERE task_id = ? ORDER BY created_at DESC LIMIT ?`, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cps []*ledger.Checkpoint
	for rows.Next() {
		cp := &ledger.Checkpoint{}
		var taskState, workingMem, createdAt string
		err := rows.Scan(&cp.ID, &cp.TaskID, &cp.EventSeq, &cp.StepIndex,
			&taskState, &workingMem, &cp.SizeBytes, &cp.Reason, &createdAt)
		if err != nil {
			return nil, err
		}
		cp.TaskState = ledger.JSON(taskState)
		cp.WorkingMem = ledger.JSON(workingMem)
		cp.CreatedAt = parseTime(createdAt)
		cps = append(cps, cp)
	}
	return cps, rows.Err()
}

func (b *Backend) DeleteCheckpointsBefore(ctx context.Context, taskID string, beforeSeq int64) error {
	_, err := b.db.ExecContext(ctx,
		`DELETE FROM checkpoints WHERE task_id = ? AND event_seq < ?`, taskID, beforeSeq)
	return err
}

// ── Memory ──

func (b *Backend) PutMemory(ctx context.Context, m *ledger.MemoryEntry) error {
	_, err := b.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO memories
			(id, tenant_id, task_id, kind, key, content, source, confidence,
			 access_count, last_access, expires_at, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.TenantID, m.TaskID, m.Kind, m.Key, m.Content, m.Source,
		m.Confidence, m.AccessCount,
		formatTimePtr(m.LastAccess), formatTimePtr(m.ExpiresAt),
		string(m.Metadata), formatTime(m.CreatedAt), formatTime(m.UpdatedAt),
	)
	return err
}

func (b *Backend) GetMemory(ctx context.Context, id string) (*ledger.MemoryEntry, error) {
	row := b.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, task_id, kind, key, content, source, confidence,
			access_count, last_access, expires_at, metadata, created_at, updated_at
		FROM memories WHERE id = ?`, id)
	return scanMemory(row)
}

func (b *Backend) DeleteMemory(ctx context.Context, id string) error {
	res, err := b.db.ExecContext(ctx, `DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ledger.ErrMemoryNotFound
	}
	return nil
}

func (b *Backend) SearchMemories(ctx context.Context, q ledger.MemoryQuery) ([]*ledger.MemoryEntry, error) {
	var clauses []string
	var args []interface{}

	clauses = append(clauses, "tenant_id = ?")
	args = append(args, q.TenantID)

	if len(q.Kinds) > 0 {
		ph := make([]string, len(q.Kinds))
		for i, k := range q.Kinds {
			ph[i] = "?"
			args = append(args, string(k))
		}
		clauses = append(clauses, "kind IN ("+strings.Join(ph, ",")+")")
	}
	if q.TaskID != nil {
		clauses = append(clauses, "task_id = ?")
		args = append(args, *q.TaskID)
	}
	if q.Query != "" {
		clauses = append(clauses, "(content LIKE ? OR key LIKE ?)")
		pattern := "%" + q.Query + "%"
		args = append(args, pattern, pattern)
	}
	if q.Key != "" {
		clauses = append(clauses, "key = ?")
		args = append(args, q.Key)
	}
	if q.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, q.Source)
	}
	if q.MinConfidence > 0 {
		clauses = append(clauses, "confidence >= ?")
		args = append(args, q.MinConfidence)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, tenant_id, task_id, kind, key, content, source, confidence,
		access_count, last_access, expires_at, metadata, created_at, updated_at
		FROM memories WHERE ` + strings.Join(clauses, " AND ") +
		` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*ledger.MemoryEntry
	for rows.Next() {
		m, err := scanMemoryRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, m)
	}
	return entries, rows.Err()
}

func scanMemory(row *sql.Row) (*ledger.MemoryEntry, error) {
	m := &ledger.MemoryEntry{}
	var metadata, createdAt, updatedAt string
	var lastAccess, expiresAt *string

	err := row.Scan(&m.ID, &m.TenantID, &m.TaskID, &m.Kind, &m.Key, &m.Content,
		&m.Source, &m.Confidence, &m.AccessCount, &lastAccess, &expiresAt,
		&metadata, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, ledger.ErrMemoryNotFound
	}
	if err != nil {
		return nil, err
	}
	m.Metadata = ledger.JSON(metadata)
	m.CreatedAt = parseTime(createdAt)
	m.UpdatedAt = parseTime(updatedAt)
	m.LastAccess = parseTimePtr(lastAccess)
	m.ExpiresAt = parseTimePtr(expiresAt)
	return m, nil
}

func scanMemoryRow(rows *sql.Rows) (*ledger.MemoryEntry, error) {
	m := &ledger.MemoryEntry{}
	var metadata, createdAt, updatedAt string
	var lastAccess, expiresAt *string

	err := rows.Scan(&m.ID, &m.TenantID, &m.TaskID, &m.Kind, &m.Key, &m.Content,
		&m.Source, &m.Confidence, &m.AccessCount, &lastAccess, &expiresAt,
		&metadata, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	m.Metadata = ledger.JSON(metadata)
	m.CreatedAt = parseTime(createdAt)
	m.UpdatedAt = parseTime(updatedAt)
	m.LastAccess = parseTimePtr(lastAccess)
	m.ExpiresAt = parseTimePtr(expiresAt)
	return m, nil
}

// ── Artifact ──

func (b *Backend) SaveArtifact(ctx context.Context, a *ledger.Artifact) error {
	_, err := b.db.ExecContext(ctx,
		`INSERT INTO artifacts (id, task_id, name, kind, mime_type, size_bytes, storage_ref, checksum, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.TaskID, a.Name, a.Kind, a.MimeType, a.SizeBytes,
		a.StorageRef, a.Checksum, string(a.Metadata), formatTime(a.CreatedAt),
	)
	return err
}

func (b *Backend) GetArtifact(ctx context.Context, id string) (*ledger.Artifact, error) {
	row := b.db.QueryRowContext(ctx,
		`SELECT id, task_id, name, kind, mime_type, size_bytes, storage_ref, checksum, metadata, created_at
		FROM artifacts WHERE id = ?`, id)

	a := &ledger.Artifact{}
	var metadata, createdAt string
	err := row.Scan(&a.ID, &a.TaskID, &a.Name, &a.Kind, &a.MimeType, &a.SizeBytes,
		&a.StorageRef, &a.Checksum, &metadata, &createdAt)
	if err == sql.ErrNoRows {
		return nil, ledger.ErrArtifactNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Metadata = ledger.JSON(metadata)
	a.CreatedAt = parseTime(createdAt)
	return a, nil
}

func (b *Backend) ListArtifacts(ctx context.Context, taskID string) ([]*ledger.Artifact, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, task_id, name, kind, mime_type, size_bytes, storage_ref, checksum, metadata, created_at
		FROM artifacts WHERE task_id = ? ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var arts []*ledger.Artifact
	for rows.Next() {
		a := &ledger.Artifact{}
		var metadata, createdAt string
		err := rows.Scan(&a.ID, &a.TaskID, &a.Name, &a.Kind, &a.MimeType, &a.SizeBytes,
			&a.StorageRef, &a.Checksum, &metadata, &createdAt)
		if err != nil {
			return nil, err
		}
		a.Metadata = ledger.JSON(metadata)
		a.CreatedAt = parseTime(createdAt)
		arts = append(arts, a)
	}
	return arts, rows.Err()
}

// ── Task Dependencies ──

func (b *Backend) CreateDependency(ctx context.Context, d *ledger.TaskDependency) error {
	_, err := b.db.ExecContext(ctx,
		`INSERT INTO task_dependencies (id, from_task_id, to_task_id, kind, artifact_ref, satisfied, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.FromTaskID, d.ToTaskID, d.Kind, d.ArtifactRef,
		boolToInt(d.Satisfied), formatTime(d.CreatedAt),
	)
	return err
}

func (b *Backend) ListDependencies(ctx context.Context, taskID string) ([]*ledger.TaskDependency, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, from_task_id, to_task_id, kind, artifact_ref, satisfied, created_at
		FROM task_dependencies WHERE to_task_id = ? OR from_task_id = ?`, taskID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []*ledger.TaskDependency
	for rows.Next() {
		d := &ledger.TaskDependency{}
		var satisfied int
		var createdAt string
		err := rows.Scan(&d.ID, &d.FromTaskID, &d.ToTaskID, &d.Kind,
			&d.ArtifactRef, &satisfied, &createdAt)
		if err != nil {
			return nil, err
		}
		d.Satisfied = satisfied != 0
		d.CreatedAt = parseTime(createdAt)
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

func (b *Backend) SatisfyDependency(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx,
		`UPDATE task_dependencies SET satisfied = 1 WHERE id = ?`, id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ── Vector Embeddings ──

func (b *Backend) PutEmbedding(ctx context.Context, memoryID string, embedding []float32) error {
	blob := float32sToBytes(embedding)
	_, err := b.db.ExecContext(ctx,
		`INSERT INTO embeddings (memory_id, vector, dims) VALUES (?, ?, ?)
		 ON CONFLICT(memory_id) DO UPDATE SET vector = excluded.vector, dims = excluded.dims`,
		memoryID, blob, len(embedding))
	return err
}

func (b *Backend) SearchByVector(ctx context.Context, q ledger.VectorQuery) ([]ledger.ScoredEntry, error) {
	// Load all embeddings for the tenant (brute-force for SQLite; efficient up to ~50K entries)
	query := `SELECT e.memory_id, e.vector, e.dims,
		m.id, m.tenant_id, m.task_id, m.kind, m.key, m.content, m.source,
		m.confidence, m.access_count, m.last_access, m.expires_at, m.metadata,
		m.created_at, m.updated_at
		FROM embeddings e JOIN memories m ON e.memory_id = m.id
		WHERE m.tenant_id = ?`
	args := []any{q.TenantID}

	if len(q.Kinds) > 0 {
		placeholders := make([]string, len(q.Kinds))
		for i, k := range q.Kinds {
			placeholders[i] = "?"
			args = append(args, string(k))
		}
		query += " AND m.kind IN (" + strings.Join(placeholders, ",") + ")"
	}

	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	minScore := q.MinScore
	if minScore == 0 {
		minScore = 0.3
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}

	type candidate struct {
		entry ledger.MemoryEntry
		vec   []float32
	}
	var candidates []candidate

	for rows.Next() {
		var memID string
		var blob []byte
		var dims int
		var m ledger.MemoryEntry
		var lastAccess, expiresAt, metadata, createdAt, updatedAt *string
		var taskID *string

		if err := rows.Scan(&memID, &blob, &dims,
			&m.ID, &m.TenantID, &taskID, &m.Kind, &m.Key, &m.Content, &m.Source,
			&m.Confidence, &m.AccessCount, &lastAccess, &expiresAt, &metadata,
			&createdAt, &updatedAt); err != nil {
			continue
		}
		m.TaskID = taskID
		if lastAccess != nil {
			t, _ := time.Parse(timeFormat, *lastAccess)
			m.LastAccess = &t
		}
		if expiresAt != nil {
			t, _ := time.Parse(timeFormat, *expiresAt)
			m.ExpiresAt = &t
		}
		if metadata != nil {
			m.Metadata = ledger.JSON(*metadata)
		}
		if createdAt != nil {
			m.CreatedAt, _ = time.Parse(timeFormat, *createdAt)
		}
		if updatedAt != nil {
			m.UpdatedAt, _ = time.Parse(timeFormat, *updatedAt)
		}

		vec := bytesToFloat32s(blob, dims)
		candidates = append(candidates, candidate{entry: m, vec: vec})
	}

	// Compute cosine similarity
	var results []ledger.ScoredEntry
	for _, c := range candidates {
		sim := ledger.CosineSimilarity(q.Embedding, c.vec)
		if sim >= minScore {
			c.entry.Embedding = c.vec
			results = append(results, ledger.ScoredEntry{
				Entry:  c.entry,
				Score:  sim,
				Reason: "semantic",
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// ── Context Graph ──

func (b *Backend) PutNode(ctx context.Context, n *ledger.GraphNode) error {
	if n.Metadata == nil {
		n.Metadata = ledger.JSON("{}")
	}
	_, err := b.db.ExecContext(ctx,
		`INSERT INTO graph_nodes (id, kind, label, ref_id, tenant_id, metadata)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(tenant_id, kind, ref_id) DO UPDATE SET label = excluded.label, metadata = excluded.metadata`,
		n.ID, n.Kind, n.Label, n.RefID, n.TenantID, string(n.Metadata))
	return err
}

func (b *Backend) PutEdge(ctx context.Context, e *ledger.GraphEdge) error {
	if e.Metadata == nil {
		e.Metadata = ledger.JSON("{}")
	}
	_, err := b.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO graph_edges (id, from_id, to_id, kind, weight, metadata)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, e.FromID, e.ToID, e.Kind, e.Weight, string(e.Metadata))
	return err
}

func (b *Backend) GetNeighbors(ctx context.Context, nodeID string, maxDepth int, limit int) ([]*ledger.GraphNode, []*ledger.GraphEdge, error) {
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if limit <= 0 {
		limit = 50
	}

	visited := map[string]bool{nodeID: true}
	var allEdges []*ledger.GraphEdge
	frontier := []string{nodeID}

	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		var nextFrontier []string
		for _, nid := range frontier {
			rows, err := b.db.QueryContext(ctx,
				`SELECT id, from_id, to_id, kind, weight, metadata FROM graph_edges
				 WHERE from_id = ? OR to_id = ?`, nid, nid)
			if err != nil {
				continue
			}
			for rows.Next() {
				var e ledger.GraphEdge
				var meta string
				if err := rows.Scan(&e.ID, &e.FromID, &e.ToID, &e.Kind, &e.Weight, &meta); err != nil {
					continue
				}
				e.Metadata = ledger.JSON(meta)
				allEdges = append(allEdges, &e)

				neighbor := e.ToID
				if neighbor == nid {
					neighbor = e.FromID
				}
				if !visited[neighbor] {
					visited[neighbor] = true
					nextFrontier = append(nextFrontier, neighbor)
				}
			}
			rows.Close()
		}
		frontier = nextFrontier
	}

	// Load all visited nodes
	var allNodes []*ledger.GraphNode
	for nid := range visited {
		var n ledger.GraphNode
		var meta string
		err := b.db.QueryRowContext(ctx,
			`SELECT id, kind, label, ref_id, tenant_id, metadata FROM graph_nodes WHERE id = ?`, nid).
			Scan(&n.ID, &n.Kind, &n.Label, &n.RefID, &n.TenantID, &meta)
		if err == nil {
			n.Metadata = ledger.JSON(meta)
			allNodes = append(allNodes, &n)
		}
	}

	if len(allNodes) > limit {
		allNodes = allNodes[:limit]
	}
	return allNodes, allEdges, nil
}

func (b *Backend) DeleteNode(ctx context.Context, nodeID string) error {
	b.db.ExecContext(ctx, `DELETE FROM graph_edges WHERE from_id = ? OR to_id = ?`, nodeID, nodeID)
	_, err := b.db.ExecContext(ctx, `DELETE FROM graph_nodes WHERE id = ?`, nodeID)
	return err
}

func (b *Backend) GetNode(ctx context.Context, nodeID string) (*ledger.GraphNode, error) {
	var n ledger.GraphNode
	var meta string
	err := b.db.QueryRowContext(ctx,
		`SELECT id, kind, label, ref_id, tenant_id, metadata FROM graph_nodes WHERE id = ?`,
		nodeID).
		Scan(&n.ID, &n.Kind, &n.Label, &n.RefID, &n.TenantID, &meta)
	if err != nil {
		return nil, err
	}
	n.Metadata = ledger.JSON(meta)
	return &n, nil
}

func (b *Backend) ListNodes(ctx context.Context) ([]ledger.GraphNode, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT id, kind, label, ref_id, tenant_id, metadata FROM graph_nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []ledger.GraphNode
	for rows.Next() {
		var n ledger.GraphNode
		var meta string
		if err := rows.Scan(&n.ID, &n.Kind, &n.Label, &n.RefID, &n.TenantID, &meta); err != nil {
			continue
		}
		n.Metadata = ledger.JSON(meta)
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (b *Backend) ListEdges(ctx context.Context) ([]ledger.GraphEdge, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT id, from_id, to_id, kind, weight, metadata FROM graph_edges`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var edges []ledger.GraphEdge
	for rows.Next() {
		var e ledger.GraphEdge
		var meta string
		if err := rows.Scan(&e.ID, &e.FromID, &e.ToID, &e.Kind, &e.Weight, &meta); err != nil {
			continue
		}
		e.Metadata = ledger.JSON(meta)
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

func (b *Backend) Neighbors(ctx context.Context, nodeID string) ([]*ledger.GraphEdge, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, from_id, to_id, kind, weight, metadata FROM graph_edges WHERE from_id = ? OR to_id = ?`,
		nodeID, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var edges []*ledger.GraphEdge
	for rows.Next() {
		var e ledger.GraphEdge
		var meta string
		if err := rows.Scan(&e.ID, &e.FromID, &e.ToID, &e.Kind, &e.Weight, &meta); err != nil {
			continue
		}
		e.Metadata = ledger.JSON(meta)
		edges = append(edges, &e)
	}
	return edges, rows.Err()
}

func (b *Backend) FindNodeByRef(ctx context.Context, tenantID string, kind ledger.GraphNodeKind, refID string) (*ledger.GraphNode, error) {
	var n ledger.GraphNode
	var meta string
	var err error
	if tenantID == "" {
		err = b.db.QueryRowContext(ctx,
			`SELECT id, kind, label, ref_id, tenant_id, metadata FROM graph_nodes WHERE kind = ? AND ref_id = ? ORDER BY tenant_id LIMIT 1`,
			kind, refID).
			Scan(&n.ID, &n.Kind, &n.Label, &n.RefID, &n.TenantID, &meta)
	} else {
		err = b.db.QueryRowContext(ctx,
			`SELECT id, kind, label, ref_id, tenant_id, metadata FROM graph_nodes WHERE tenant_id = ? AND kind = ? AND ref_id = ?`,
			tenantID, kind, refID).
			Scan(&n.ID, &n.Kind, &n.Label, &n.RefID, &n.TenantID, &meta)
	}
	if err != nil {
		return nil, err
	}
	n.Metadata = ledger.JSON(meta)
	return &n, nil
}

// ── Float32 ???[]byte conversion ──

func float32sToBytes(fs []float32) []byte {
	buf := make([]byte, len(fs)*4)
	for i, f := range fs {
		bits := math.Float32bits(f)
		buf[i*4] = byte(bits)
		buf[i*4+1] = byte(bits >> 8)
		buf[i*4+2] = byte(bits >> 16)
		buf[i*4+3] = byte(bits >> 24)
	}
	return buf
}

func bytesToFloat32s(data []byte, dims int) []float32 {
	if len(data) < dims*4 {
		return nil
	}
	fs := make([]float32, dims)
	for i := range fs {
		bits := uint32(data[i*4]) |
			uint32(data[i*4+1])<<8 |
			uint32(data[i*4+2])<<16 |
			uint32(data[i*4+3])<<24
		fs[i] = math.Float32frombits(bits)
	}
	return fs
}

// Verify Backend interface compliance at compile time.
var _ ledger.Backend = (*Backend)(nil)

// ── Event deletion support for compaction ──

// DeleteEvent removes a single event by ID.
func (b *Backend) DeleteEvent(ctx context.Context, eventID string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, eventID)
	return err
}

// DeleteEvents removes events by IDs in a single batch (efficient for compaction).
func (b *Backend) DeleteEvents(ctx context.Context, eventIDs []string) error {
	if len(eventIDs) == 0 {
		return nil
	}
	// Process in batches of 500 (SQLite variable limit)
	const batchSize = 500
	for i := 0; i < len(eventIDs); i += batchSize {
		end := i + batchSize
		if end > len(eventIDs) {
			end = len(eventIDs)
		}
		batch := eventIDs[i:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for j, id := range batch {
			placeholders[j] = "?"
			args[j] = id
		}
		query := `DELETE FROM events WHERE id IN (` + strings.Join(placeholders, ",") + `)`
		if _, err := b.db.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}

// Verify deletion interface compliance at compile time.
var _ ledger.EventDeleter = (*Backend)(nil)
var _ ledger.EventBatchDeleter = (*Backend)(nil)

// ── KV Store ──

func (b *Backend) KVPut(ctx context.Context, entry *ledger.KVEntry) error {
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now()
	}
	_, err := b.db.ExecContext(ctx,
		`INSERT INTO kv_store (namespace, key, value, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(namespace, key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		entry.Namespace, entry.Key, entry.Value, entry.UpdatedAt.Format(timeFormat),
	)
	return err
}

func (b *Backend) KVGet(ctx context.Context, namespace, key string) (*ledger.KVEntry, error) {
	row := b.db.QueryRowContext(ctx,
		`SELECT namespace, key, value, updated_at FROM kv_store WHERE namespace=? AND key=?`,
		namespace, key,
	)
	var e ledger.KVEntry
	var updatedAt string
	if err := row.Scan(&e.Namespace, &e.Key, &e.Value, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ledger.ErrKVNotFound
		}
		return nil, err
	}
	e.UpdatedAt, _ = time.Parse(timeFormat, updatedAt)
	return &e, nil
}

func (b *Backend) KVDelete(ctx context.Context, namespace, key string) error {
	_, err := b.db.ExecContext(ctx,
		`DELETE FROM kv_store WHERE namespace=? AND key=?`,
		namespace, key,
	)
	return err
}

func (b *Backend) KVList(ctx context.Context, namespace string) ([]*ledger.KVEntry, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT namespace, key, value, updated_at FROM kv_store WHERE namespace=? ORDER BY key`,
		namespace,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*ledger.KVEntry
	for rows.Next() {
		var e ledger.KVEntry
		var updatedAt string
		if err := rows.Scan(&e.Namespace, &e.Key, &e.Value, &updatedAt); err != nil {
			return nil, err
		}
		e.UpdatedAt, _ = time.Parse(timeFormat, updatedAt)
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}
