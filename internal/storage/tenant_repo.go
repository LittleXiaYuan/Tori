package storage

import (
	"context"
	"database/sql"
	"time"
)

// TenantRow represents a tenant record in the database.
type TenantRow struct {
	ID        string
	Name      string
	APIKey    string
	Config    string // JSON
	CreatedAt time.Time
}

// TenantRepo provides PostgreSQL-backed tenant operations.
type TenantRepo struct {
	db *DB
}

// NewTenantRepo creates a tenant repository.
func NewTenantRepo(db *DB) *TenantRepo {
	return &TenantRepo{db: db}
}

func (r *TenantRepo) Create(ctx context.Context, t TenantRow) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tenants (id, name, api_key, config, created_at) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO NOTHING`,
		t.ID, t.Name, t.APIKey, t.Config, t.CreatedAt)
	return err
}

func (r *TenantRepo) GetByID(ctx context.Context, id string) (*TenantRow, error) {
	var t TenantRow
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, api_key, COALESCE(config, '{}'), created_at FROM tenants WHERE id=$1`, id).
		Scan(&t.ID, &t.Name, &t.APIKey, &t.Config, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &t, err
}

func (r *TenantRepo) GetByAPIKey(ctx context.Context, key string) (*TenantRow, error) {
	var t TenantRow
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, api_key, COALESCE(config, '{}'), created_at FROM tenants WHERE api_key=$1`, key).
		Scan(&t.ID, &t.Name, &t.APIKey, &t.Config, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &t, err
}

func (r *TenantRepo) List(ctx context.Context) ([]TenantRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, api_key, COALESCE(config, '{}'), created_at FROM tenants ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TenantRow
	for rows.Next() {
		var t TenantRow
		if err := rows.Scan(&t.ID, &t.Name, &t.APIKey, &t.Config, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TenantRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tenants WHERE id=$1`, id)
	return err
}
