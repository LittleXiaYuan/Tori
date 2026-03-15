package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ──────────────────────────────────────────────
// ProviderType — what kind of provider
// ──────────────────────────────────────────────

// ProviderType identifies the functional type of a provider.
type ProviderType string

const (
	ProviderTypeChat      ProviderType = "chat"
	ProviderTypeEmbedding ProviderType = "embedding"
	ProviderTypeTTS       ProviderType = "tts"
	ProviderTypeSTT       ProviderType = "stt"
	ProviderTypeRerank    ProviderType = "rerank"
)

// ──────────────────────────────────────────────
// ProviderConfig — JSON/env configuration
// ──────────────────────────────────────────────

// ProviderConfig is the configuration for a single LLM provider instance.
type ProviderConfig struct {
	ID           string       `json:"id"`
	DisplayName  string       `json:"display_name,omitempty"`
	Type         ProviderType `json:"type"` // "chat", "embedding", etc.
	BaseURL      string       `json:"base_url"`
	APIKeys      []string     `json:"api_keys"` // supports key rotation
	Model        string       `json:"model"`
	Enabled      bool         `json:"enabled"`
	Priority     int          `json:"priority,omitempty"`     // lower = higher priority
	Capabilities []Capability `json:"capabilities,omitempty"` // chat, tools, vision, etc.
	Tier         string       `json:"tier,omitempty"`         // "fast", "smart", "expert"
}

// ──────────────────────────────────────────────
// ProviderInstance — runtime provider with client + rotation
// ──────────────────────────────────────────────

// ProviderInstance wraps a Client with key rotation and metadata.
type ProviderInstance struct {
	Config  ProviderConfig
	Client  *Client
	keys    []string
	keyIdx  atomic.Int64
	enabled bool
	mu      sync.RWMutex
}

func newProviderInstance(cfg ProviderConfig) *ProviderInstance {
	key := ""
	if len(cfg.APIKeys) > 0 {
		key = cfg.APIKeys[0]
	}
	p := &ProviderInstance{
		Config:  cfg,
		Client:  NewClient(cfg.BaseURL, key, cfg.Model),
		keys:    cfg.APIKeys,
		enabled: cfg.Enabled,
	}
	return p
}

// RotateKey switches to the next API key (round-robin).
func (p *ProviderInstance) RotateKey() string {
	if len(p.keys) <= 1 {
		return ""
	}
	next := p.keyIdx.Add(1) % int64(len(p.keys))
	key := p.keys[next]
	p.mu.Lock()
	// Recreate client with new key (keeps same URL/model)
	p.Client = NewClient(p.Config.BaseURL, key, p.Config.Model)
	p.mu.Unlock()
	slog.Info("provider: rotated API key", "provider", p.Config.ID, "key_index", next)
	return key
}

// Enabled returns whether this provider is currently active.
func (p *ProviderInstance) Enabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled
}

// SetEnabled activates or deactivates this provider.
func (p *ProviderInstance) SetEnabled(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = v
}

// Test verifies that the provider can respond to a simple prompt.
func (p *ProviderInstance) Test(ctx context.Context) error {
	msgs := []Message{
		{Role: "user", Content: "Reply with 'ok'."},
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	resp, err := p.Client.Chat(ctx, msgs, 0.0)
	if err != nil {
		return fmt.Errorf("provider %s test failed: %w", p.Config.ID, err)
	}
	if resp == "" {
		return fmt.Errorf("provider %s test: empty response", p.Config.ID)
	}
	return nil
}

// Models returns the list of known models for this provider.
// Currently returns the configured model; extendable for discovery.
func (p *ProviderInstance) Models() []string {
	return []string{p.Config.Model}
}

// ──────────────────────────────────────────────
// ProviderRegistry — manages all providers
// ──────────────────────────────────────────────

// ProviderRegistry manages multiple LLM provider instances with lifecycle.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]*ProviderInstance // id → instance
	pool      *Pool                        // synced pool for backward compat
	sessionMu sync.RWMutex
	sessions  map[string]string // session_id → provider_id override
}

// NewProviderRegistry creates a registry, optionally wrapping an existing Pool.
func NewProviderRegistry(pool *Pool) *ProviderRegistry {
	if pool == nil {
		pool = NewPool()
	}
	return &ProviderRegistry{
		providers: make(map[string]*ProviderInstance),
		pool:      pool,
		sessions:  make(map[string]string),
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(cfg ProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cfg.ID == "" {
		return fmt.Errorf("provider: id is required")
	}
	if cfg.BaseURL == "" {
		return fmt.Errorf("provider %s: base_url is required", cfg.ID)
	}
	if len(cfg.APIKeys) == 0 {
		// Allow empty keys for local backends (Ollama, vLLM, etc.)
		cfg.APIKeys = []string{""}
	}

	inst := newProviderInstance(cfg)
	r.providers[cfg.ID] = inst

	// Sync to Pool for backward compatibility
	if cfg.Type == ProviderTypeChat {
		r.pool.Register(cfg.ID, inst.Client)
		// Also register under tier name if specified
		if cfg.Tier != "" {
			r.pool.Register(cfg.Tier, inst.Client)
		}
	}

	slog.Info("provider: registered",
		"id", cfg.ID,
		"type", cfg.Type,
		"model", cfg.Model,
		"tier", cfg.Tier,
		"keys", len(cfg.APIKeys),
	)
	return nil
}

// Get returns a provider by ID.
func (r *ProviderRegistry) Get(id string) *ProviderInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[id]
}

// GetForSession returns the provider for a session, considering overrides.
func (r *ProviderRegistry) GetForSession(sessionID string) *ProviderInstance {
	r.sessionMu.RLock()
	override, ok := r.sessions[sessionID]
	r.sessionMu.RUnlock()

	if ok {
		if p := r.Get(override); p != nil && p.Enabled() {
			return p
		}
	}
	return nil
}

// SetSessionProvider sets a session-level provider override.
func (r *ProviderRegistry) SetSessionProvider(sessionID, providerID string) {
	r.sessionMu.Lock()
	defer r.sessionMu.Unlock()
	if providerID == "" {
		delete(r.sessions, sessionID)
	} else {
		r.sessions[sessionID] = providerID
	}
}

// ClearSessionProvider removes a session-level override.
func (r *ProviderRegistry) ClearSessionProvider(sessionID string) {
	r.SetSessionProvider(sessionID, "")
}

// List returns all registered providers.
func (r *ProviderRegistry) List() []ProviderStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]ProviderStatus, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, ProviderStatus{
			ID:           p.Config.ID,
			DisplayName:  p.Config.DisplayName,
			Type:         p.Config.Type,
			Model:        p.Config.Model,
			BaseURL:      p.Config.BaseURL,
			Enabled:      p.Enabled(),
			Tier:         p.Config.Tier,
			Priority:     p.Config.Priority,
			Capabilities: p.Config.Capabilities,
			KeyCount:     len(p.keys),
			BreakerState: p.Client.Breaker().State(),
		})
	}
	return out
}

// ProviderStatus is the public view of a provider.
type ProviderStatus struct {
	ID           string       `json:"id"`
	DisplayName  string       `json:"display_name,omitempty"`
	Type         ProviderType `json:"type"`
	Model        string       `json:"model"`
	BaseURL      string       `json:"base_url"`
	Enabled      bool         `json:"enabled"`
	Tier         string       `json:"tier,omitempty"`
	Priority     int          `json:"priority"`
	Capabilities []Capability `json:"capabilities,omitempty"`
	KeyCount     int          `json:"key_count"`
	BreakerState string       `json:"breaker_state"`
}

// Enable activates a provider.
func (r *ProviderRegistry) Enable(id string) error {
	p := r.Get(id)
	if p == nil {
		return fmt.Errorf("provider %s not found", id)
	}
	p.SetEnabled(true)
	slog.Info("provider: enabled", "id", id)
	return nil
}

// Disable deactivates a provider.
func (r *ProviderRegistry) Disable(id string) error {
	p := r.Get(id)
	if p == nil {
		return fmt.Errorf("provider %s not found", id)
	}
	p.SetEnabled(false)
	slog.Info("provider: disabled", "id", id)
	return nil
}

// TestProvider verifies connectivity for a specific provider.
func (r *ProviderRegistry) TestProvider(ctx context.Context, id string) error {
	p := r.Get(id)
	if p == nil {
		return fmt.Errorf("provider %s not found", id)
	}
	return p.Test(ctx)
}

// SwitchModel changes the model of a provider at runtime.
func (r *ProviderRegistry) SwitchModel(id, newModel string) error {
	p := r.Get(id)
	if p == nil {
		return fmt.Errorf("provider %s not found", id)
	}
	p.mu.Lock()
	p.Config.Model = newModel
	p.Client = NewClient(p.Config.BaseURL, p.keys[p.keyIdx.Load()%int64(len(p.keys))], newModel)
	p.mu.Unlock()
	// Sync to pool
	r.pool.Register(id, p.Client)
	if p.Config.Tier != "" {
		r.pool.Register(p.Config.Tier, p.Client)
	}
	slog.Info("provider: model switched", "id", id, "model", newModel)
	return nil
}

// Pool returns the underlying LLM pool (backward compat).
func (r *ProviderRegistry) Pool() *Pool {
	return r.pool
}

// ByType returns all enabled providers of a given type.
func (r *ProviderRegistry) ByType(t ProviderType) []*ProviderInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*ProviderInstance
	for _, p := range r.providers {
		if p.Config.Type == t && p.Enabled() {
			out = append(out, p)
		}
	}
	return out
}

// ──────────────────────────────────────────────
// Load providers from LLM_PROVIDERS env var
// ──────────────────────────────────────────────

// LoadProvidersFromEnv reads LLM_PROVIDERS JSON from environment.
// Format: [{"id":"openai","type":"chat","base_url":"...","api_keys":["sk-..."],"model":"gpt-4o","enabled":true}]
func LoadProvidersFromEnv() ([]ProviderConfig, error) {
	raw := os.Getenv("LLM_PROVIDERS")
	if raw == "" {
		return nil, nil
	}
	var configs []ProviderConfig
	if err := json.Unmarshal([]byte(raw), &configs); err != nil {
		return nil, fmt.Errorf("LLM_PROVIDERS: invalid JSON: %w", err)
	}
	return configs, nil
}

// LoadProvidersFromFile reads provider configs from a JSON file.
func LoadProvidersFromFile(path string) ([]ProviderConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// Trim BOM if present
	data = []byte(strings.TrimPrefix(string(data), "\xef\xbb\xbf"))
	var configs []ProviderConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", path, err)
	}
	return configs, nil
}
