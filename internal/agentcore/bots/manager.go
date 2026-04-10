package bots

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// kvStore abstracts Ledger KV to avoid import cycles with internal/ledger.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// Bot represents an independent agent instance with its own identity and config.
type Bot struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	PersonaDir  string         `json:"persona_dir,omitempty"`
	IsActive    bool           `json:"is_active"`
	Status      string         `json:"status"`
	Config      BotConfig      `json:"config"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ReasoningEffort constants.
const (
	ReasoningLow    = "low"
	ReasoningMedium = "medium"
	ReasoningHigh   = "high"
)

// BotConfig holds per-bot configuration.
type BotConfig struct {
	Model            string  `json:"model,omitempty"`
	MaxSteps         int     `json:"max_steps,omitempty"`
	Temperature      float64 `json:"temperature,omitempty"`
	MaxContextMsgs   int     `json:"max_context_msgs,omitempty"`
	Language         string  `json:"language,omitempty"`
	ReasoningEnabled bool    `json:"reasoning_enabled"`
	ReasoningEffort  string  `json:"reasoning_effort,omitempty"`
}

const (
	StatusReady    = "ready"
	StatusStopped  = "stopped"
	StatusCreating = "creating"
)

// Manager manages multiple bot instances.
type Manager struct {
	mu   sync.RWMutex
	bots map[string]*Bot
	kvs  kvStore
}

// NewManager creates a multi-bot manager.
func NewManager() *Manager {
	return &Manager{bots: make(map[string]*Bot)}
}

// SetKVStore enables Ledger KV-backed persistence for bots.
func (m *Manager) SetKVStore(kvs kvStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kvs = kvs

	if len(m.bots) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := kvs.Put(ctx, "bots", m.bots); err != nil {
			slog.Warn("bots: KV migration failed", "err", err)
		} else {
			slog.Info("bots: migrated to KV", "count", len(m.bots))
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var bots map[string]*Bot
	found, err := kvs.Get(ctx, "bots", &bots)
	if err != nil {
		slog.Warn("bots: KV load failed", "err", err)
		return
	}
	if found && len(bots) > 0 {
		m.bots = bots
		slog.Info("bots: loaded from KV", "count", len(bots))
	}
}

func (m *Manager) persistKV() {
	if m.kvs == nil {
		return
	}
	snap := make(map[string]*Bot, len(m.bots))
	for k, v := range m.bots {
		snap[k] = v
	}
	kvs := m.kvs
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := kvs.Put(ctx, "bots", snap); err != nil {
			slog.Warn("bots: KV save failed", "err", err)
		}
	}()
}

// Create adds a new bot instance.
func (m *Manager) Create(name, description string, cfg BotConfig) (*Bot, error) {
	if name == "" {
		return nil, fmt.Errorf("bot name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check name uniqueness
	for _, b := range m.bots {
		if b.Name == name {
			return nil, fmt.Errorf("bot name already exists: %s", name)
		}
	}

	bot := &Bot{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		IsActive:    true,
		Status:      StatusReady,
		Config:      applyDefaults(cfg),
		Metadata:    map[string]any{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.bots[bot.ID] = bot
	m.persistKV()
	return bot, nil
}

// Get returns a bot by ID.
func (m *Manager) Get(id string) (*Bot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.bots[id]
	if !ok {
		return nil, false
	}
	copy := *b
	return &copy, true
}

// GetByName returns a bot by name.
func (m *Manager) GetByName(name string) (*Bot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, b := range m.bots {
		if b.Name == name {
			copy := *b
			return &copy, true
		}
	}
	return nil, false
}

// List returns all bots.
func (m *Manager) List() []Bot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Bot, 0, len(m.bots))
	for _, b := range m.bots {
		result = append(result, *b)
	}
	return result
}

// Update modifies bot profile fields.
func (m *Manager) Update(id string, name, description *string, cfg *BotConfig) (*Bot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.bots[id]
	if !ok {
		return nil, fmt.Errorf("bot not found: %s", id)
	}
	if name != nil && *name != "" {
		// Check uniqueness
		for _, other := range m.bots {
			if other.ID != id && other.Name == *name {
				return nil, fmt.Errorf("bot name already exists: %s", *name)
			}
		}
		b.Name = *name
	}
	if description != nil {
		b.Description = *description
	}
	if cfg != nil {
		if cfg.Model != "" {
			b.Config.Model = cfg.Model
		}
		if cfg.MaxSteps > 0 {
			b.Config.MaxSteps = cfg.MaxSteps
		}
		if cfg.Temperature > 0 {
			b.Config.Temperature = cfg.Temperature
		}
		if cfg.MaxContextMsgs > 0 {
			b.Config.MaxContextMsgs = cfg.MaxContextMsgs
		}
		if cfg.Language != "" {
			b.Config.Language = cfg.Language
		}
		b.Config.ReasoningEnabled = cfg.ReasoningEnabled
		if cfg.ReasoningEffort != "" {
			b.Config.ReasoningEffort = normalizeEffort(cfg.ReasoningEffort)
		}
	}
	b.UpdatedAt = time.Now()
	m.persistKV()
	copy := *b
	return &copy, nil
}

// SetActive toggles a bot's active state.
func (m *Manager) SetActive(id string, active bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.bots[id]
	if !ok {
		return fmt.Errorf("bot not found: %s", id)
	}
	b.IsActive = active
	if active {
		b.Status = StatusReady
	} else {
		b.Status = StatusStopped
	}
	b.UpdatedAt = time.Now()
	m.persistKV()
	return nil
}

// Delete removes a bot.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.bots[id]; !ok {
		return fmt.Errorf("bot not found: %s", id)
	}
	delete(m.bots, id)
	m.persistKV()
	return nil
}

// Count returns the number of bots.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.bots)
}

// ActiveCount returns the number of active bots.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, b := range m.bots {
		if b.IsActive {
			count++
		}
	}
	return count
}

func applyDefaults(cfg BotConfig) BotConfig {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 8
	}
	if cfg.Temperature <= 0 {
		cfg.Temperature = 0.7
	}
	if cfg.MaxContextMsgs <= 0 {
		cfg.MaxContextMsgs = 20
	}
	if cfg.Language == "" {
		cfg.Language = "auto"
	}
	cfg.ReasoningEffort = normalizeEffort(cfg.ReasoningEffort)
	return cfg
}

func normalizeEffort(e string) string {
	switch e {
	case ReasoningLow, ReasoningMedium, ReasoningHigh:
		return e
	default:
		return ReasoningMedium
	}
}
