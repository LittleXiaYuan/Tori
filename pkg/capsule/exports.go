package capsule

import (
	"context"
	"net/http"

	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/skills"
)

// Exports is the single declarative surface through which a Capsule contributes
// capabilities to the host.
//
// Compared to the legacy plugin.Plugin interface (which split Skills /
// UIPlugin / ExtensionRegistry into three different callback paths), Exports
// collects every kind of contribution a capsule can make into one value. This
// enables the registry to reason about activation / deactivation atomically
// and makes "dynamic capability surface" (session-level tool filtering) a
// first-class concern.
//
// Any field may be nil / empty; the registry fans out only non-empty entries.
type Exports struct {
	// Skills are the atomic tools the capsule contributes to the planner.
	Skills []skills.Skill

	// SystemPrompt is merged into the planner's system prompt when the
	// capsule is activated. Empty to skip.
	SystemPrompt string

	// UITabs are navigation entries registered in the web dashboard.
	UITabs []UITab

	// HTTPRoutes are custom endpoints mounted under
	// "/v1/ext/{capsuleName}{path}" when the capsule is activated.
	HTTPRoutes map[string]http.HandlerFunc

	// Providers are LLM / embedding / speech providers the capsule adds.
	Providers []ProviderExport

	// Channels are messaging channel adapters.
	Channels []ChannelExport

	// Searches are web search engines.
	Searches []SearchExport

	// Guardrails are safety rules applied pre/post-generation.
	Guardrails []GuardrailExport

	// Embeddings are vector-embedding providers.
	Embeddings []EmbeddingExport

	// Speeches are TTS / STT engines.
	Speeches []SpeechExport

	// Events are subscriptions the capsule takes on host lifecycle events
	// (e.g. "memory.fact.extracted", "session.started").
	Events []EventSubscription

	// Cogni is an inline cognitive declaration for this capsule. If non-nil,
	// it takes precedence over Manifest.Cogni.File. May be nil for capsules
	// that don't participate directly in AI reasoning.
	Cogni *cogni.Declaration
}

// UITab is a navigation entry registered by a capsule.
// Mirrors plugin.UITab but is owned by pkg/capsule to keep the two packages
// loosely coupled.
type UITab struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	LabelEn     string `json:"label_en,omitempty"`
	Icon        string `json:"icon"`
	Description string `json:"description,omitempty"`
	// Capsule is filled in by the registry.
	Capsule string `json:"capsule,omitempty"`
}

// ProviderExport declares a new LLM-compatible provider.
type ProviderExport struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name,omitempty"`
	Type        string   `json:"type"` // "chat" | "embedding" | "tts" | "stt"
	BaseURL     string   `json:"base_url"`
	APIKeys     []string `json:"api_keys,omitempty"`
	Model       string   `json:"model"`
	Tier        string   `json:"tier,omitempty"`
	Priority    int      `json:"priority,omitempty"`
}

// ChannelExport declares a new messaging channel adapter.
type ChannelExport struct {
	Name         string `json:"name"`
	DisplayName  string `json:"display_name,omitempty"`
	WebhookURL   string `json:"webhook_url,omitempty"`
	SendEndpoint string `json:"send_endpoint,omitempty"`
	ConfigJSON   string `json:"config_json,omitempty"`
}

// SearchExport declares a new web search provider.
type SearchExport struct {
	Name       string `json:"name"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key,omitempty"`
	SearchPath string `json:"search_path"`
}

// GuardrailExport declares a new safety rule.
type GuardrailExport struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Phase       string   `json:"phase"` // "input" | "output" | "both"
	Keywords    []string `json:"keywords,omitempty"`
	Patterns    []string `json:"patterns,omitempty"`
	Endpoint    string   `json:"endpoint,omitempty"`
}

// EmbeddingExport declares a new embedding provider.
type EmbeddingExport struct {
	Name       string `json:"name"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key,omitempty"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions,omitempty"`
}

// SpeechExport declares a new TTS or STT engine.
type SpeechExport struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // "tts" | "stt"
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
	Model   string `json:"model,omitempty"`
	Voice   string `json:"voice,omitempty"`
}

// EventSubscription subscribes the capsule to a named host event.
type EventSubscription struct {
	// Event is the host event name (e.g. "memory.fact.extracted").
	Event string

	// Handler is invoked synchronously when the event fires. It must not block.
	Handler func(ctx context.Context, payload map[string]any) error
}

// Empty reports whether the Exports contributes anything at all.
func (e *Exports) Empty() bool {
	if e == nil {
		return true
	}
	return len(e.Skills) == 0 &&
		e.SystemPrompt == "" &&
		len(e.UITabs) == 0 &&
		len(e.HTTPRoutes) == 0 &&
		len(e.Providers) == 0 &&
		len(e.Channels) == 0 &&
		len(e.Searches) == 0 &&
		len(e.Guardrails) == 0 &&
		len(e.Embeddings) == 0 &&
		len(e.Speeches) == 0 &&
		len(e.Events) == 0 &&
		e.Cogni == nil
}
