package tenant

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"
)

// Tenant represents an isolated workspace (organization, project, user).
type Tenant struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	APIKey    string            `json:"api_key"`
	Config    map[string]string `json:"config"`
	CreatedAt time.Time         `json:"created_at"`
}

// MaskedKey returns the API key with middle portion hidden.
func (t *Tenant) MaskedKey() string {
	k := t.APIKey
	if len(k) <= 10 {
		return k[:3] + "***"
	}
	return k[:6] + "..." + k[len(k)-4:]
}

// Repo is the optional persistence backend for tenants.
type Repo interface {
	Create(ctx context.Context, id, name, apiKey, config string, createdAt time.Time) error
	GetByID(ctx context.Context, id string) (*Tenant, error)
	GetByAPIKey(ctx context.Context, key string) (*Tenant, error)
	List(ctx context.Context) ([]*Tenant, error)
	Delete(ctx context.Context, id string) error
}

// Manager handles tenant registration, lookup, and API key validation.
// Uses in-memory cache for fast lookups and optional DB repo for persistence.
type Manager struct {
	mu      sync.RWMutex
	tenants map[string]*Tenant // id -> tenant
	keys    map[string]string  // apiKey -> tenantID
	repo    Repo               // optional DB backend
}

// NewManager creates a tenant manager.
func NewManager() *Manager {
	return &Manager{
		tenants: make(map[string]*Tenant),
		keys:    make(map[string]string),
	}
}

// SetRepo attaches a persistence backend and loads existing tenants into cache.
func (m *Manager) SetRepo(repo Repo) {
	m.repo = repo
	// Load existing tenants from DB into cache
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tenants, err := repo.List(ctx)
	if err != nil {
		slog.Error("failed to load tenants from DB", "err", err)
		return
	}
	m.mu.Lock()
	for _, t := range tenants {
		m.tenants[t.ID] = t
		m.keys[t.APIKey] = t.ID
	}
	m.mu.Unlock()
	slog.Info("loaded tenants from DB", "count", len(tenants))
}

// Register creates a new tenant and returns it with an API key.
func (m *Manager) Register(name string) *Tenant {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	apiKey := "ya_" + generateKey()

	t := &Tenant{
		ID:        id,
		Name:      name,
		APIKey:    apiKey,
		Config:    make(map[string]string),
		CreatedAt: time.Now(),
	}
	m.tenants[id] = t
	m.keys[apiKey] = id

	// Persist to DB if available
	if m.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.repo.Create(ctx, t.ID, t.Name, t.APIKey, "{}", t.CreatedAt); err != nil {
			slog.Error("failed to persist tenant", "id", id, "err", err)
		}
	}
	return t
}

// RegisterWithID creates a tenant with a fixed ID and API key (for persistence across restarts).
func (m *Manager) RegisterWithID(id, name, apiKey string) *Tenant {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := &Tenant{
		ID:        id,
		Name:      name,
		APIKey:    apiKey,
		Config:    make(map[string]string),
		CreatedAt: time.Now(),
	}
	m.tenants[id] = t
	m.keys[apiKey] = id

	if m.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.repo.Create(ctx, t.ID, t.Name, t.APIKey, "{}", t.CreatedAt); err != nil {
			slog.Error("failed to persist tenant", "id", id, "err", err)
		}
	}
	return t
}

// ByAPIKey returns the tenant for the given API key, or nil. The comparison
// scans every registered key so that response timing does not leak which keys
// are currently registered; an attacker who knows the length distribution of
// our keys would otherwise be able to probe existence by wall-clock timing
// against the map lookup below.
func (m *Manager) ByAPIKey(key string) *Tenant {
	if key == "" {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var match string
	for k, tid := range m.keys {
		// subtle.ConstantTimeCompare returns 0/1 even for length mismatches
		// (it enforces equal length first). We pull the tid into a local var
		// so that the branch-free pattern below does not early-return.
		if subtle.ConstantTimeCompare([]byte(k), []byte(key)) == 1 {
			match = tid
		}
	}
	if match == "" {
		return nil
	}
	return m.tenants[match]
}

// ByID returns the tenant by ID.
func (m *Manager) ByID(id string) *Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tenants[id]
}

// List returns all tenants.
func (m *Manager) List() []*Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		out = append(out, t)
	}
	return out
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateKey() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}
