package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// ExtensionRegistry is the central hub for plugin-contributed system extensions.
// Plugins can register new LLM providers, channels, search engines,
// guardrails, embedding models, and TTS/STT engines at runtime.
type ExtensionRegistry struct {
	mu sync.RWMutex

	// Registration callbacks (set by Agent subsystems during init)
	onRegisterProvider  func(cfg ProviderRegistration) error
	onRegisterChannel   func(cfg ChannelRegistration) error
	onRegisterSearch    func(cfg SearchRegistration) error
	onRegisterGuardrail func(cfg GuardrailRegistration) error
	onRegisterEmbedding func(cfg EmbeddingRegistration) error
	onRegisterSpeech    func(cfg SpeechRegistration) error

	// Tracking
	extensions []ExtensionRecord
}

// NewExtensionRegistry creates the extension hub.
func NewExtensionRegistry() *ExtensionRegistry {
	return &ExtensionRegistry{
		extensions: make([]ExtensionRecord, 0),
	}
}

// ExtensionRecord tracks a registered extension for audit/display.
type ExtensionRecord struct {
	PluginName string `json:"plugin_name"`
	Type       string `json:"type"` // "provider", "channel", "search", etc.
	Name       string `json:"name"` // extension identifier
	Status     string `json:"status"` // "active", "failed"
}

// Extensions returns all registered extensions.
func (r *ExtensionRegistry) Extensions() []ExtensionRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ExtensionRecord, len(r.extensions))
	copy(out, r.extensions)
	return out
}

// ── Registration Callback Setters (called by Agent subsystems) ──

func (r *ExtensionRegistry) OnRegisterProvider(fn func(ProviderRegistration) error) {
	r.mu.Lock()
	r.onRegisterProvider = fn
	r.mu.Unlock()
}

func (r *ExtensionRegistry) OnRegisterChannel(fn func(ChannelRegistration) error) {
	r.mu.Lock()
	r.onRegisterChannel = fn
	r.mu.Unlock()
}

func (r *ExtensionRegistry) OnRegisterSearch(fn func(SearchRegistration) error) {
	r.mu.Lock()
	r.onRegisterSearch = fn
	r.mu.Unlock()
}

func (r *ExtensionRegistry) OnRegisterGuardrail(fn func(GuardrailRegistration) error) {
	r.mu.Lock()
	r.onRegisterGuardrail = fn
	r.mu.Unlock()
}

func (r *ExtensionRegistry) OnRegisterEmbedding(fn func(EmbeddingRegistration) error) {
	r.mu.Lock()
	r.onRegisterEmbedding = fn
	r.mu.Unlock()
}

func (r *ExtensionRegistry) OnRegisterSpeech(fn func(SpeechRegistration) error) {
	r.mu.Lock()
	r.onRegisterSpeech = fn
	r.mu.Unlock()
}

// ── Registration Methods (called by plugins via SDK) ──

// RegisterProvider adds a new LLM/Chat provider to the agent.
// This lets plugins add Ollama, vLLM, Claude, Gemini, or any OpenAI-compatible API.
func (r *ExtensionRegistry) RegisterProvider(ctx context.Context, pluginName string, cfg ProviderRegistration) error {
	r.mu.RLock()
	fn := r.onRegisterProvider
	r.mu.RUnlock()

	if fn == nil {
		return fmt.Errorf("provider registration not available")
	}
	cfg.PluginName = pluginName
	if err := fn(cfg); err != nil {
		r.recordExtension(pluginName, "provider", cfg.ID, "failed")
		return err
	}
	r.recordExtension(pluginName, "provider", cfg.ID, "active")
	slog.Info("plugin registered provider", "plugin", pluginName, "provider", cfg.ID, "model", cfg.Model)
	return nil
}

// RegisterChannel adds a new messaging channel adapter.
// This lets plugins add Matrix, IRC, custom webhook, or any messaging protocol.
func (r *ExtensionRegistry) RegisterChannel(ctx context.Context, pluginName string, cfg ChannelRegistration) error {
	r.mu.RLock()
	fn := r.onRegisterChannel
	r.mu.RUnlock()

	if fn == nil {
		return fmt.Errorf("channel registration not available")
	}
	cfg.PluginName = pluginName
	if err := fn(cfg); err != nil {
		r.recordExtension(pluginName, "channel", cfg.Name, "failed")
		return err
	}
	r.recordExtension(pluginName, "channel", cfg.Name, "active")
	slog.Info("plugin registered channel", "plugin", pluginName, "channel", cfg.Name)
	return nil
}

// RegisterSearch adds a new web search engine.
func (r *ExtensionRegistry) RegisterSearch(ctx context.Context, pluginName string, cfg SearchRegistration) error {
	r.mu.RLock()
	fn := r.onRegisterSearch
	r.mu.RUnlock()

	if fn == nil {
		return fmt.Errorf("search registration not available")
	}
	cfg.PluginName = pluginName
	if err := fn(cfg); err != nil {
		r.recordExtension(pluginName, "search", cfg.Name, "failed")
		return err
	}
	r.recordExtension(pluginName, "search", cfg.Name, "active")
	slog.Info("plugin registered search engine", "plugin", pluginName, "engine", cfg.Name)
	return nil
}

// RegisterGuardrail adds a new safety guardrail rule.
func (r *ExtensionRegistry) RegisterGuardrail(ctx context.Context, pluginName string, cfg GuardrailRegistration) error {
	r.mu.RLock()
	fn := r.onRegisterGuardrail
	r.mu.RUnlock()

	if fn == nil {
		return fmt.Errorf("guardrail registration not available")
	}
	cfg.PluginName = pluginName
	if err := fn(cfg); err != nil {
		r.recordExtension(pluginName, "guardrail", cfg.Name, "failed")
		return err
	}
	r.recordExtension(pluginName, "guardrail", cfg.Name, "active")
	slog.Info("plugin registered guardrail", "plugin", pluginName, "rule", cfg.Name)
	return nil
}

// RegisterEmbedding adds a new vector embedding provider.
func (r *ExtensionRegistry) RegisterEmbedding(ctx context.Context, pluginName string, cfg EmbeddingRegistration) error {
	r.mu.RLock()
	fn := r.onRegisterEmbedding
	r.mu.RUnlock()

	if fn == nil {
		return fmt.Errorf("embedding registration not available")
	}
	cfg.PluginName = pluginName
	if err := fn(cfg); err != nil {
		r.recordExtension(pluginName, "embedding", cfg.Name, "failed")
		return err
	}
	r.recordExtension(pluginName, "embedding", cfg.Name, "active")
	slog.Info("plugin registered embedding", "plugin", pluginName, "model", cfg.Name)
	return nil
}

// RegisterSpeech adds a new TTS or STT engine.
func (r *ExtensionRegistry) RegisterSpeech(ctx context.Context, pluginName string, cfg SpeechRegistration) error {
	r.mu.RLock()
	fn := r.onRegisterSpeech
	r.mu.RUnlock()

	if fn == nil {
		return fmt.Errorf("speech registration not available")
	}
	cfg.PluginName = pluginName
	if err := fn(cfg); err != nil {
		r.recordExtension(pluginName, "speech", cfg.Name, "failed")
		return err
	}
	r.recordExtension(pluginName, "speech", cfg.Name, "active")
	slog.Info("plugin registered speech engine", "plugin", pluginName, "engine", cfg.Name)
	return nil
}

func (r *ExtensionRegistry) recordExtension(pluginName, typ, name, status string) {
	r.mu.Lock()
	r.extensions = append(r.extensions, ExtensionRecord{
		PluginName: pluginName, Type: typ, Name: name, Status: status,
	})
	r.mu.Unlock()
}

// ── Registration Data Structures ──

// ProviderRegistration describes a new LLM provider to register.
type ProviderRegistration struct {
	PluginName  string   `json:"plugin_name,omitempty"`
	ID          string   `json:"id"`                     // unique provider ID
	DisplayName string   `json:"display_name,omitempty"`
	Type        string   `json:"type"`                   // "chat", "embedding", "tts", "stt"
	BaseURL     string   `json:"base_url"`               // OpenAI-compatible API endpoint
	APIKeys     []string `json:"api_keys,omitempty"`     // supports key rotation
	Model       string   `json:"model"`                  // default model name
	Tier        string   `json:"tier,omitempty"`         // "fast", "smart", "expert"
	Priority    int      `json:"priority,omitempty"`     // lower = higher priority
}

// ChannelRegistration describes a new messaging channel adapter.
type ChannelRegistration struct {
	PluginName   string `json:"plugin_name,omitempty"`
	Name         string `json:"name"`          // channel type identifier
	DisplayName  string `json:"display_name"`
	WebhookURL   string `json:"webhook_url"`   // plugin's webhook endpoint for incoming messages
	SendEndpoint string `json:"send_endpoint"` // plugin's HTTP endpoint for sending messages
	ConfigJSON   string `json:"config_json,omitempty"` // arbitrary config
}

// SearchRegistration describes a new search engine.
type SearchRegistration struct {
	PluginName string `json:"plugin_name,omitempty"`
	Name       string `json:"name"`         // search engine identifier
	BaseURL    string `json:"base_url"`     // search API endpoint
	APIKey     string `json:"api_key,omitempty"`
	SearchPath string `json:"search_path"`  // API path for search (e.g. "/search")
}

// GuardrailRegistration describes a new safety rule.
type GuardrailRegistration struct {
	PluginName  string   `json:"plugin_name,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Phase       string   `json:"phase"` // "input", "output", "both"
	Keywords    []string `json:"keywords,omitempty"`     // block these keywords
	Patterns    []string `json:"patterns,omitempty"`     // block these regex patterns
	Endpoint    string   `json:"endpoint,omitempty"`     // custom check endpoint on the plugin
}

// EmbeddingRegistration describes a new vector embedding provider.
type EmbeddingRegistration struct {
	PluginName string `json:"plugin_name,omitempty"`
	Name       string `json:"name"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key,omitempty"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions,omitempty"`
}

// SpeechRegistration describes a new TTS or STT engine.
type SpeechRegistration struct {
	PluginName string `json:"plugin_name,omitempty"`
	Name       string `json:"name"`
	Type       string `json:"type"`     // "tts" or "stt"
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key,omitempty"`
	Model      string `json:"model,omitempty"`
	Voice      string `json:"voice,omitempty"` // for TTS: default voice ID
}
