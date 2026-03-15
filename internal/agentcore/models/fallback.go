package models

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Auth profile
// ──────────────────────────────────────────────

// AuthProfile represents API credentials for a model provider.
type AuthProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"` // "openai", "anthropic", "deepseek", etc.
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url,omitempty"`
	OrgID    string `json:"org_id,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// ──────────────────────────────────────────────
// Model entry with fallbacks
// ──────────────────────────────────────────────

// ModelEntry represents a model with its provider and fallback chain.
type ModelEntry struct {
	ID           string   `json:"id"`           // e.g. "gpt-4o"
	Provider     string   `json:"provider"`     // primary provider
	ProfileID    string   `json:"profile_id"`   // primary auth profile
	Fallbacks    []string `json:"fallbacks"`    // fallback model IDs in order
	MaxRetries   int      `json:"max_retries"`  // per-model retry count (default 2)
	TimeoutMs    int64    `json:"timeout_ms"`   // per-request timeout
}

// ──────────────────────────────────────────────
// Request / Response for the fallback system
// ──────────────────────────────────────────────

// CompletionRequest is a model-agnostic completion request.
type CompletionRequest struct {
	Model       string         `json:"model"`
	Messages    []ChatMessage  `json:"messages"`
	Temperature *float64       `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

// ChatMessage is a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResponse is a model-agnostic completion response.
type CompletionResponse struct {
	Model      string `json:"model"`       // actual model used
	ProfileID  string `json:"profile_id"`  // actual profile used
	Content    string `json:"content"`
	FinishReason string `json:"finish_reason,omitempty"`
	Usage      *Usage `json:"usage,omitempty"`
	Attempts   int    `json:"attempts"`    // how many attempts were made
}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ──────────────────────────────────────────────
// Provider — interface for LLM backends
// ──────────────────────────────────────────────

// Provider executes completion requests against a specific LLM API.
type Provider interface {
	// ID returns the provider identifier (e.g. "openai", "anthropic").
	ID() string
	// Complete sends a request and returns the response.
	Complete(ctx context.Context, profile *AuthProfile, req *CompletionRequest) (*CompletionResponse, error)
}

// ──────────────────────────────────────────────
// FallbackChain — the core orchestrator
// ──────────────────────────────────────────────

// FallbackChain manages models, profiles, and fallback logic.
type FallbackChain struct {
	mu        sync.RWMutex
	models    map[string]*ModelEntry
	profiles  map[string]*AuthProfile
	providers map[string]Provider
}

// NewFallbackChain creates a new fallback chain manager.
func NewFallbackChain() *FallbackChain {
	return &FallbackChain{
		models:    make(map[string]*ModelEntry),
		profiles:  make(map[string]*AuthProfile),
		providers: make(map[string]Provider),
	}
}

// RegisterProvider adds a provider implementation.
func (fc *FallbackChain) RegisterProvider(p Provider) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.providers[p.ID()] = p
}

// AddProfile adds an auth profile.
func (fc *FallbackChain) AddProfile(p AuthProfile) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.profiles[p.ID] = &p
}

// AddModel adds a model entry.
func (fc *FallbackChain) AddModel(m ModelEntry) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if m.MaxRetries <= 0 {
		m.MaxRetries = 2
	}
	fc.models[m.ID] = &m
}

// ──────────────────────────────────────────────
// Complete with fallback
// ──────────────────────────────────────────────

// Complete executes a request with automatic fallback and profile rotation.
func (fc *FallbackChain) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	// Build attempt chain: primary model + fallbacks
	chain := []string{req.Model}
	if m, ok := fc.models[req.Model]; ok {
		chain = append(chain, m.Fallbacks...)
	}

	var lastErr error
	totalAttempts := 0

	for _, modelID := range chain {
		model := fc.models[modelID]
		if model == nil {
			// Model not in registry, try directly with default profile
			model = &ModelEntry{ID: modelID, MaxRetries: 2}
		}

		// Collect applicable profiles for this model's provider
		profiles := fc.profilesForModel(model)
		if len(profiles) == 0 {
			// Use a nil profile (provider handles default auth)
			profiles = []*AuthProfile{nil}
		}

		for _, profile := range profiles {
			if profile != nil && !profile.Enabled {
				continue
			}

			for attempt := 1; attempt <= model.MaxRetries; attempt++ {
				totalAttempts++

				// Apply per-model timeout
				reqCtx := ctx
				if model.TimeoutMs > 0 {
					var cancel context.CancelFunc
					reqCtx, cancel = context.WithTimeout(ctx, time.Duration(model.TimeoutMs)*time.Millisecond)
					defer cancel()
				}

				// Find provider
				provider := fc.findProvider(model, profile)
				if provider == nil {
					lastErr = fmt.Errorf("no provider for model %q", modelID)
					break
				}

				// Clone request with the current model
				r := *req
				r.Model = modelID

				profileName := "default"
				if profile != nil {
					profileName = profile.Name
				}

				resp, err := provider.Complete(reqCtx, profile, &r)
				if err == nil {
					resp.Attempts = totalAttempts
					if profile != nil {
						resp.ProfileID = profile.ID
					}
					return resp, nil
				}

				lastErr = err
				slog.Warn("fallback: attempt failed",
					"model", modelID,
					"profile", profileName,
					"attempt", attempt,
					"err", err,
				)
			}
		}
	}

	return nil, fmt.Errorf("all models exhausted after %d attempts: %w", totalAttempts, lastErr)
}

// profilesForModel returns auth profiles matching the model's provider.
func (fc *FallbackChain) profilesForModel(model *ModelEntry) []*AuthProfile {
	var profiles []*AuthProfile

	// Primary profile first
	if model.ProfileID != "" {
		if p, ok := fc.profiles[model.ProfileID]; ok {
			profiles = append(profiles, p)
		}
	}

	// Additional profiles for same provider
	for _, p := range fc.profiles {
		if p.Provider == model.Provider && p.ID != model.ProfileID {
			profiles = append(profiles, p)
		}
	}
	return profiles
}

// findProvider locates the provider for a model/profile combination.
func (fc *FallbackChain) findProvider(model *ModelEntry, profile *AuthProfile) Provider {
	// Try model's provider first
	if p, ok := fc.providers[model.Provider]; ok {
		return p
	}
	// Try profile's provider
	if profile != nil {
		if p, ok := fc.providers[profile.Provider]; ok {
			return p
		}
	}
	// Try first available provider
	for _, p := range fc.providers {
		return p
	}
	return nil
}

// ──────────────────────────────────────────────
// Query
// ──────────────────────────────────────────────

// ListModels returns all registered models.
func (fc *FallbackChain) ListModels() []*ModelEntry {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	out := make([]*ModelEntry, 0, len(fc.models))
	for _, m := range fc.models {
		cp := *m
		out = append(out, &cp)
	}
	return out
}

// ListProfiles returns all auth profiles (keys masked).
func (fc *FallbackChain) ListProfiles() []*AuthProfile {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	out := make([]*AuthProfile, 0, len(fc.profiles))
	for _, p := range fc.profiles {
		cp := *p
		if len(cp.APIKey) > 8 {
			cp.APIKey = cp.APIKey[:4] + "..." + cp.APIKey[len(cp.APIKey)-4:]
		}
		out = append(out, &cp)
	}
	return out
}
