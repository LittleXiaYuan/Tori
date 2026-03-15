package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MemoryItem represents a memory record in the database.
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
	ExpiresAt  time.Time // only for short-term
}

// MemoryRepo provides PostgreSQL-backed memory operations.
type MemoryRepo struct {
	db *DB
}

// NewMemoryRepo creates a memory repository.
func NewMemoryRepo(db *DB) *MemoryRepo {
	return &MemoryRepo{db: db}
}

// --- Short-term ---

func (r *MemoryRepo) ShortPut(ctx context.Context, tenantID string, item MemoryItem) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO memory_short (id, tenant_id, key, value, source, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (tenant_id, key) DO UPDATE SET value=$4, source=$5, expires_at=$7`,
		item.ID, tenantID, item.Key, item.Value, item.Source, item.CreatedAt, item.ExpiresAt)
	return err
}

func (r *MemoryRepo) ShortGet(ctx context.Context, tenantID, key string) (*MemoryItem, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, key, value, source, created_at, expires_at FROM memory_short
		 WHERE tenant_id=$1 AND key=$2 AND expires_at > NOW()`, tenantID, key)
	var it MemoryItem
	it.TenantID = tenantID
	err := row.Scan(&it.ID, &it.Key, &it.Value, &it.Source, &it.CreatedAt, &it.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &it, err
}

func (r *MemoryRepo) ShortSearch(ctx context.Context, tenantID, query string, limit int) ([]MemoryItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, key, value, source, created_at FROM memory_short
		 WHERE tenant_id=$1 AND expires_at > NOW() AND value ILIKE '%' || $2 || '%'
		 LIMIT $3`, tenantID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows, tenantID)
}

func (r *MemoryRepo) ShortGC(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM memory_short WHERE expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *MemoryRepo) ShortCount(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memory_short WHERE tenant_id=$1 AND expires_at > NOW()`, tenantID).Scan(&count)
	return count, err
}

// --- Mid-term ---

func (r *MemoryRepo) MidPut(ctx context.Context, tenantID string, item MemoryItem) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO memory_mid (id, tenant_id, key, value, source, category, access_cnt, last_access, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (tenant_id, key) DO UPDATE SET value=$4, source=$5, category=COALESCE(NULLIF($6,''), memory_mid.category),
		   access_cnt=memory_mid.access_cnt+1, last_access=$8`,
		item.ID, tenantID, item.Key, item.Value, item.Source, item.Category,
		item.AccessCnt, time.Now(), item.CreatedAt)
	return err
}

func (r *MemoryRepo) MidGet(ctx context.Context, tenantID, key string) (*MemoryItem, error) {
	// Increment access count on read
	row := r.db.QueryRowContext(ctx,
		`UPDATE memory_mid SET access_cnt=access_cnt+1, last_access=NOW()
		 WHERE tenant_id=$1 AND key=$2
		 RETURNING id, key, value, source, category, access_cnt, last_access, created_at`, tenantID, key)
	var it MemoryItem
	it.TenantID = tenantID
	err := row.Scan(&it.ID, &it.Key, &it.Value, &it.Source, &it.Category, &it.AccessCnt, &it.LastAccess, &it.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &it, err
}

func (r *MemoryRepo) MidSearch(ctx context.Context, tenantID, query string, limit int) ([]MemoryItem, error) {
	// Use pg_trgm similarity for fuzzy search, fall back to ILIKE
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, key, value, source, category, access_cnt, last_access, created_at,
		        similarity(value, $2) AS sim
		 FROM memory_mid
		 WHERE tenant_id=$1 AND (value ILIKE '%' || $2 || '%' OR similarity(value, $2) > 0.1)
		 ORDER BY sim DESC, last_access DESC
		 LIMIT $3`, tenantID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItemsFull(rows, tenantID)
}

func (r *MemoryRepo) MidCount(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memory_mid WHERE tenant_id=$1`, tenantID).Scan(&count)
	return count, err
}

// --- Long-term (with vector) ---

func (r *MemoryRepo) LongPut(ctx context.Context, tenantID string, item MemoryItem, embedding []float32) error {
	if len(embedding) == 0 {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO memory_long (id, tenant_id, key, value, source, category, access_cnt, last_access, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 ON CONFLICT (tenant_id, key) DO UPDATE SET value=$4, source=$5, category=COALESCE(NULLIF($6,''), memory_long.category),
			   access_cnt=memory_long.access_cnt+1, last_access=$8`,
			item.ID, tenantID, item.Key, item.Value, item.Source, item.Category,
			item.AccessCnt, time.Now(), item.CreatedAt)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO memory_long (id, tenant_id, key, value, source, category, embedding, access_cnt, last_access, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::vector, $8, $9, $10)
		 ON CONFLICT (tenant_id, key) DO UPDATE SET value=$4, source=$5, category=COALESCE(NULLIF($6,''), memory_long.category),
		   embedding=$7::vector, access_cnt=memory_long.access_cnt+1, last_access=$9`,
		item.ID, tenantID, item.Key, item.Value, item.Source, item.Category,
		vectorToString(embedding), item.AccessCnt, time.Now(), item.CreatedAt)
	return err
}

func (r *MemoryRepo) LongSearchVector(ctx context.Context, tenantID string, embedding []float32, limit int) ([]MemoryItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, key, value, source, category, access_cnt, last_access, created_at,
		        1 - (embedding <=> $2::vector) AS sim
		 FROM memory_long
		 WHERE tenant_id=$1 AND embedding IS NOT NULL
		 ORDER BY embedding <=> $2::vector
		 LIMIT $3`, tenantID, vectorToString(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItemsFull(rows, tenantID)
}

func (r *MemoryRepo) LongSearchText(ctx context.Context, tenantID, query string, limit int) ([]MemoryItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, key, value, source, category, access_cnt, last_access, created_at,
		        similarity(value, $2) AS sim
		 FROM memory_long
		 WHERE tenant_id=$1 AND (value ILIKE '%' || $2 || '%' OR similarity(value, $2) > 0.1)
		 ORDER BY sim DESC
		 LIMIT $3`, tenantID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItemsFull(rows, tenantID)
}

func (r *MemoryRepo) LongCount(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memory_long WHERE tenant_id=$1`, tenantID).Scan(&count)
	return count, err
}

// --- helpers ---

func scanItems(rows *sql.Rows, tenantID string) ([]MemoryItem, error) {
	var items []MemoryItem
	for rows.Next() {
		var it MemoryItem
		it.TenantID = tenantID
		if err := rows.Scan(&it.ID, &it.Key, &it.Value, &it.Source, &it.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func scanItemsFull(rows *sql.Rows, tenantID string) ([]MemoryItem, error) {
	var items []MemoryItem
	for rows.Next() {
		var it MemoryItem
		var sim float64
		it.TenantID = tenantID
		if err := rows.Scan(&it.ID, &it.Key, &it.Value, &it.Source, &it.Category, &it.AccessCnt, &it.LastAccess, &it.CreatedAt, &sim); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func vectorToString(v []float32) string {
	s := "["
	for i, f := range v {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%f", f)
	}
	s += "]"
	return s
}
