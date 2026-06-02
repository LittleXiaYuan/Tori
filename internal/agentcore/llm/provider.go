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

// ProviderSource indicates where this provider configuration came from.
type ProviderSource string

const (
	ProviderSourceDirect ProviderSource = "direct" // user-provided API key
	ProviderSourceTori   ProviderSource = "tori"   // auto-discovered via Tori binding
	ProviderSourceLocal  ProviderSource = "local"  // local model (Ollama, vLLM, etc.)
)

// ProviderMode determines LLM routing strategy.
type ProviderMode string

const (
	ProviderModeLocal  ProviderMode = "local"  // only use direct/local providers
	ProviderModeTori   ProviderMode = "tori"   // only use Tori-sourced providers
	ProviderModeHybrid ProviderMode = "hybrid" // prefer direct, fallback to Tori
)

// ProviderConfig is the configuration for a single LLM provider instance.
type ProviderConfig struct {
	ID            string         `json:"id"`
	DisplayName   string         `json:"display_name,omitempty"`
	Type          ProviderType   `json:"type"`             // "chat", "embedding", etc.
	Source        ProviderSource `json:"source,omitempty"` // "direct", "tori", "local"
	BaseURL       string         `json:"base_url"`
	APIKeys       []string       `json:"api_keys"` // supports key rotation
	Model         string         `json:"model"`
	Enabled       bool           `json:"enabled"`
	Priority      int            `json:"priority,omitempty"`       // lower = higher priority
	Capabilities  []Capability   `json:"capabilities,omitempty"`   // chat, tools, vision, etc.
	Tier          string         `json:"tier,omitempty"`           // "fast", "smart", "expert"
	PresetID      string         `json:"preset_id,omitempty"`      // links to a provider preset template
	Dialect       Dialect        `json:"dialect,omitempty"`        // API dialect: "" = OpenAI, "anthropic" = Claude
	ContextWindow int            `json:"context_window,omitempty"` // in K tokens (e.g. 128 = 128K), 0 = 128K default
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
	var client *Client
	if cfg.Dialect == DialectAnthropic || isAnthropicURL(cfg.BaseURL) {
		client = NewClaudeClient(cfg.BaseURL, key, cfg.Model)
		cfg.Dialect = DialectAnthropic
	} else {
		client = NewClient(cfg.BaseURL, key, cfg.Model)
	}
	if cfg.ContextWindow > 0 {
		client.SetContextWindow(cfg.ContextWindow)
	}
	p := &ProviderInstance{
		Config:  cfg,
		Client:  client,
		keys:    cfg.APIKeys,
		enabled: cfg.Enabled,
	}
	return p
}

func isAnthropicURL(url string) bool {
	return strings.Contains(url, "anthropic.com") || strings.Contains(url, "claude.ai")
}

// RotateKey switches to the next API key (round-robin).
func (p *ProviderInstance) RotateKey() string {
	if len(p.keys) <= 1 {
		return ""
	}
	next := p.keyIdx.Add(1) % int64(len(p.keys))
	key := p.keys[next]
	p.mu.Lock()
	if p.Config.Dialect == DialectAnthropic {
		p.Client = NewClaudeClient(p.Config.BaseURL, key, p.Config.Model)
	} else {
		p.Client = NewClient(p.Config.BaseURL, key, p.Config.Model)
	}
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
	mu           sync.RWMutex
	providers    map[string]*ProviderInstance // id → instance
	pool         *Pool                        // synced pool for backward compat
	sessionMu    sync.RWMutex
	sessions     map[string]string // session_id → provider_id override
	mode         ProviderMode
	persistStore PersistStore // KV store for persistence (preferred)
	persistPath  string       // file path for JSON fallback
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
		mode:      ProviderModeHybrid,
	}
}

// PersistStore is a minimal interface for persisting provider configs.
type PersistStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// SetPersistStore sets a KV store for automatic persistence on Register/Delete.
func (r *ProviderRegistry) SetPersistStore(store PersistStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.persistStore = store
}

// SetPersistPath sets the file path for automatic persistence (legacy fallback).
func (r *ProviderRegistry) SetPersistPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.persistPath = path
}

// LoadFromStore loads providers and mode from the configured KV store.
func (r *ProviderRegistry) LoadFromStore() (int, error) {
	count, _, err := r.LoadFromStoreFiltered(nil)
	return count, err
}

// LoadFromStoreFiltered loads persisted providers and optionally skips entries.
// Skipped entries are not removed from persistence; callers can use this for
// feature gates such as "do not load local desktop models unless explicitly
// enabled" without destructively editing user configuration.
func (r *ProviderRegistry) LoadFromStoreFiltered(include func(ProviderConfig) bool) (int, int, error) {
	if r.persistStore == nil {
		return 0, 0, nil
	}
	var modeStr string
	if found, err := r.persistStore.Get(context.Background(), "mode", &modeStr); err == nil && found && modeStr != "" {
		switch ProviderMode(modeStr) {
		case ProviderModeLocal, ProviderModeTori, ProviderModeHybrid:
			r.mu.Lock()
			r.mode = ProviderMode(modeStr)
			r.mu.Unlock()
			slog.Info("provider: restored mode from store", "mode", modeStr)
		}
	}
	var configs []ProviderConfig
	found, err := r.persistStore.Get(context.Background(), "all", &configs)
	if err != nil || !found {
		return 0, 0, err
	}
	count := 0
	skipped := 0
	for _, cfg := range configs {
		if cfg.ID == "primary" {
			continue
		}
		if include != nil && !include(cfg) {
			cfg.Enabled = false
			_ = r.register(cfg, false)
			skipped++
			continue
		}
		if err := r.register(cfg, false); err == nil {
			count++
		}
	}
	return count, skipped, nil
}

// persist saves all non-primary providers to the configured store or file.
func (r *ProviderRegistry) persist() {
	var configs []ProviderConfig
	for _, p := range r.providers {
		if p.Config.ID == "primary" {
			continue
		}
		configs = append(configs, p.Config)
	}
	if r.persistStore != nil {
		if err := r.persistStore.Put(context.Background(), "all", configs); err != nil {
			slog.Warn("provider: persist to store error", "err", err)
		}
		return
	}
	if r.persistPath == "" {
		return
	}
	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		slog.Warn("provider: persist marshal error", "err", err)
		return
	}
	if err := os.WriteFile(r.persistPath, data, 0644); err != nil {
		slog.Warn("provider: persist write error", "err", err, "path", r.persistPath)
	}
}

// Mode returns the current provider routing mode.
func (r *ProviderRegistry) Mode() ProviderMode {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mode
}

// SetMode changes the provider routing mode and persists it.
func (r *ProviderRegistry) SetMode(m ProviderMode) {
	r.mu.Lock()
	r.mode = m
	if r.persistStore != nil {
		_ = r.persistStore.Put(context.Background(), "mode", string(m))
	}
	r.mu.Unlock()
	slog.Info("provider mode changed", "mode", m)
}

// ResolveForMode returns the best provider matching the current mode and type.
// Hybrid: prefer direct/local sources, fall back to tori.
func (r *ProviderRegistry) ResolveForMode(t ProviderType) *ProviderInstance {
	r.mu.RLock()
	mode := r.mode
	providers := make([]*ProviderInstance, 0)
	for _, p := range r.providers {
		if p.Config.Type == t && p.Enabled() {
			providers = append(providers, p)
		}
	}
	r.mu.RUnlock()

	if len(providers) == 0 {
		return nil
	}

	var directProviders, toriProviders []*ProviderInstance
	for _, p := range providers {
		switch p.Config.Source {
		case ProviderSourceTori:
			toriProviders = append(toriProviders, p)
		default:
			directProviders = append(directProviders, p)
		}
	}

	switch mode {
	case ProviderModeLocal:
		return bestByPriority(directProviders)
	case ProviderModeTori:
		return bestByPriority(toriProviders)
	default: // hybrid
		if p := bestByPriority(directProviders); p != nil {
			return p
		}
		return bestByPriority(toriProviders)
	}
}

func bestByPriority(providers []*ProviderInstance) *ProviderInstance {
	if len(providers) == 0 {
		return nil
	}
	best := providers[0]
	for _, p := range providers[1:] {
		if p.Config.Priority < best.Config.Priority {
			best = p
		}
	}
	return best
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(cfg ProviderConfig) error {
	return r.register(cfg, true)
}

func (r *ProviderRegistry) register(cfg ProviderConfig, shouldPersist bool) error {
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

	// Sync enabled chat providers to Pool for backward compatibility.
	if cfg.Type == ProviderTypeChat && cfg.Enabled {
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
	if shouldPersist {
		r.persist()
	}
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
			Source:       p.Config.Source,
			Model:        p.Config.Model,
			BaseURL:      p.Config.BaseURL,
			Enabled:      p.Enabled(),
			Tier:         p.Config.Tier,
			Priority:     p.Config.Priority,
			Capabilities: p.Config.Capabilities,
			KeyCount:     len(p.keys),
			BreakerState: p.Client.Breaker().State(),
			PresetID:     p.Config.PresetID,
		})
	}
	return out
}

// ProviderStatus is the public view of a provider.
type ProviderStatus struct {
	ID           string         `json:"id"`
	DisplayName  string         `json:"display_name,omitempty"`
	Type         ProviderType   `json:"type"`
	Source       ProviderSource `json:"source,omitempty"`
	Model        string         `json:"model"`
	BaseURL      string         `json:"base_url"`
	Enabled      bool           `json:"enabled"`
	Tier         string         `json:"tier,omitempty"`
	Priority     int            `json:"priority"`
	Capabilities []Capability   `json:"capabilities,omitempty"`
	KeyCount     int            `json:"key_count"`
	BreakerState string         `json:"breaker_state"`
	PresetID     string         `json:"preset_id,omitempty"`
}

// Delete removes a provider from the registry.
func (r *ProviderRegistry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.providers[id]
	if !ok {
		return fmt.Errorf("provider %s not found", id)
	}
	delete(r.providers, id)
	if p.Config.Tier != "" {
		r.pool.Unregister(p.Config.Tier)
	}
	r.pool.Unregister(id)
	slog.Info("provider: deleted", "id", id)
	r.persist()
	return nil
}

// Enable activates a provider.
func (r *ProviderRegistry) Enable(id string) error {
	r.mu.Lock()
	p, ok := r.providers[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("provider %s not found", id)
	}
	p.SetEnabled(true)
	p.Config.Enabled = true
	if p.Config.Type == ProviderTypeChat {
		r.pool.Register(p.Config.ID, p.Client)
		if p.Config.Tier != "" {
			r.pool.Register(p.Config.Tier, p.Client)
		}
	}
	r.persist()
	r.mu.Unlock()
	slog.Info("provider: enabled", "id", id)
	return nil
}

// Disable deactivates a provider.
func (r *ProviderRegistry) Disable(id string) error {
	r.mu.Lock()
	p, ok := r.providers[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("provider %s not found", id)
	}
	p.SetEnabled(false)
	p.Config.Enabled = false
	r.pool.Unregister(id)
	if p.Config.Tier != "" {
		r.pool.Unregister(p.Config.Tier)
	}
	r.persist()
	r.mu.Unlock()
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
	r.mu.Lock()
	p, ok := r.providers[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("provider %s not found", id)
	}
	p.mu.Lock()
	p.Config.Model = newModel
	key := p.keys[p.keyIdx.Load()%int64(len(p.keys))]
	if p.Config.Dialect == DialectAnthropic {
		p.Client = NewClaudeClient(p.Config.BaseURL, key, newModel)
	} else {
		p.Client = NewClient(p.Config.BaseURL, key, newModel)
	}
	p.mu.Unlock()
	if p.Config.Enabled {
		r.pool.Register(id, p.Client)
		if p.Config.Tier != "" {
			r.pool.Register(p.Config.Tier, p.Client)
		}
	}
	r.persist()
	r.mu.Unlock()
	slog.Info("provider: model switched", "id", id, "model", newModel)
	return nil
}

// Pool returns the underlying LLM pool (backward compat).
func (r *ProviderRegistry) Pool() *Pool {
	return r.pool
}

// ResetAllBreakers resets circuit breakers on all provider clients.
func (r *ProviderRegistry) ResetAllBreakers() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, p := range r.providers {
		if p.Client != nil && p.Client.Breaker() != nil {
			p.Client.Breaker().Reset()
			count++
		}
	}
	return count
}

// SelectByCapability returns the best enabled chat provider that has all required capabilities.
// Falls back to bestByPriority among matching candidates.
func (r *ProviderRegistry) SelectByCapability(required ...Capability) *ProviderInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var candidates []*ProviderInstance
	for _, p := range r.providers {
		if !p.Enabled() || p.Config.Type != ProviderTypeChat {
			continue
		}
		if hasAllCapsSlice(p.Config.Capabilities, required) {
			candidates = append(candidates, p)
		}
	}
	return bestByPriority(candidates)
}

func hasAllCapsSlice(have []Capability, need []Capability) bool {
	for _, n := range need {
		found := false
		for _, h := range have {
			if h == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
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
