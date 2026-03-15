package storage

import (
	"context"
	"time"

	"yunque-agent/internal/controlplane/tenant"
)

// TenantRepoAdapter adapts storage.TenantRepo to implement tenant.Repo interface.
type TenantRepoAdapter struct {
	repo *TenantRepo
}

// NewTenantRepoAdapter creates a tenant repo adapter.
func NewTenantRepoAdapter(db *DB) *TenantRepoAdapter {
	return &TenantRepoAdapter{repo: NewTenantRepo(db)}
}

func (a *TenantRepoAdapter) Create(ctx context.Context, id, name, apiKey, config string, createdAt time.Time) error {
	return a.repo.Create(ctx, TenantRow{
		ID:        id,
		Name:      name,
		APIKey:    apiKey,
		Config:    config,
		CreatedAt: createdAt,
	})
}

func (a *TenantRepoAdapter) GetByID(ctx context.Context, id string) (*tenant.Tenant, error) {
	row, err := a.repo.GetByID(ctx, id)
	if err != nil || row == nil {
		return nil, err
	}
	return rowToTenant(row), nil
}

func (a *TenantRepoAdapter) GetByAPIKey(ctx context.Context, key string) (*tenant.Tenant, error) {
	row, err := a.repo.GetByAPIKey(ctx, key)
	if err != nil || row == nil {
		return nil, err
	}
	return rowToTenant(row), nil
}

func (a *TenantRepoAdapter) List(ctx context.Context) ([]*tenant.Tenant, error) {
	rows, err := a.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*tenant.Tenant, len(rows))
	for i, row := range rows {
		out[i] = rowToTenant(&row)
	}
	return out, nil
}

func (a *TenantRepoAdapter) Delete(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}

func rowToTenant(row *TenantRow) *tenant.Tenant {
	return &tenant.Tenant{
		ID:        row.ID,
		Name:      row.Name,
		APIKey:    row.APIKey,
		Config:    make(map[string]string),
		CreatedAt: row.CreatedAt,
	}
}
