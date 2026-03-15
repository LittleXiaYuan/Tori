package models

import (
	"sort"
	"strings"
	"sync"
)

// Capability tags for model selection.
type Capability string

const (
	CapChat       Capability = "chat"
	CapReasoning  Capability = "reasoning"
	CapCoding     Capability = "coding"
	CapVision     Capability = "vision"
	CapAudio      Capability = "audio"
	CapVideo      Capability = "video"
	CapToolUse    Capability = "tool_use"
	CapJSON       Capability = "json_mode"
	CapStreaming  Capability = "streaming"
	CapEmbedding  Capability = "embedding"
	CapLongCtx    Capability = "long_context" // >32K context
	CapMultimodal Capability = "multimodal"
	CapFinetuned  Capability = "finetuned"
)

// Tier represents model quality/cost tiers for routing.
type Tier int

const (
	TierEconomy  Tier = 1 // cheapest, fastest
	TierStandard Tier = 2 // balanced
	TierPremium  Tier = 3 // highest quality
)

// ProviderName identifies the LLM provider.
type ProviderName string

const (
	ProviderOpenAI    ProviderName = "openai"
	ProviderAnthropic ProviderName = "anthropic"
	ProviderGoogle    ProviderName = "google"
	ProviderDeepSeek  ProviderName = "deepseek"
	ProviderMistral   ProviderName = "mistral"
	ProviderOllama    ProviderName = "ollama"
	ProviderZhipu     ProviderName = "zhipu"
	ProviderQwen      ProviderName = "qwen"
	ProviderMoonshot  ProviderName = "moonshot"
	ProviderBaichuan  ProviderName = "baichuan"
	ProviderMinimax   ProviderName = "minimax"
	ProviderYi        ProviderName = "yi"
	ProviderCohere    ProviderName = "cohere"
	ProviderGroq      ProviderName = "groq"
	ProviderTogether  ProviderName = "together"
	ProviderCustom    ProviderName = "custom"
)

// Pricing holds per-token cost info in USD.
type Pricing struct {
	InputPerMToken  float64 `json:"input_per_m_token"`      // cost per 1M input tokens
	OutputPerMToken float64 `json:"output_per_m_token"`     // cost per 1M output tokens
	CachedInput     float64 `json:"cached_input,omitempty"` // discounted cached input
}

// CatalogEntry is an enriched model definition for the model catalog.
type CatalogEntry struct {
	ModelID       string       `json:"model_id"` // provider model name (e.g. "gpt-4o")
	DisplayName   string       `json:"display_name"`
	Provider      ProviderName `json:"provider"`
	Capabilities  []Capability `json:"capabilities"`
	Tier          Tier         `json:"tier"`
	ContextWindow int          `json:"context_window"` // max tokens
	MaxOutput     int          `json:"max_output"`     // max output tokens
	Pricing       Pricing      `json:"pricing"`
	Deprecated    bool         `json:"deprecated,omitempty"`
	Aliases       []string     `json:"aliases,omitempty"` // alternative names
	Notes         string       `json:"notes,omitempty"`
}

// HasCapability checks if the entry supports a capability.
func (e *CatalogEntry) HasCapability(cap Capability) bool {
	for _, c := range e.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// HasAllCapabilities checks if the entry has all specified capabilities.
func (e *CatalogEntry) HasAllCapabilities(caps ...Capability) bool {
	for _, cap := range caps {
		if !e.HasCapability(cap) {
			return false
		}
	}
	return true
}

// EstimateCost calculates the estimated cost for given token counts.
func (e *CatalogEntry) EstimateCost(inputTokens, outputTokens int) float64 {
	in := float64(inputTokens) / 1_000_000 * e.Pricing.InputPerMToken
	out := float64(outputTokens) / 1_000_000 * e.Pricing.OutputPerMToken
	return in + out
}

// Catalog is a structured model database with capability-based routing.
type Catalog struct {
	mu      sync.RWMutex
	entries map[string]*CatalogEntry // keyed by ModelID
	aliases map[string]string        // alias -> ModelID
}

// NewCatalog creates an empty model catalog.
func NewCatalog() *Catalog {
	return &Catalog{
		entries: make(map[string]*CatalogEntry),
		aliases: make(map[string]string),
	}
}

// Add registers a model in the catalog.
func (c *Catalog) Add(entry CatalogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[entry.ModelID] = &entry
	for _, alias := range entry.Aliases {
		c.aliases[alias] = entry.ModelID
	}
}

// Get returns a catalog entry by model ID or alias.
func (c *Catalog) Get(id string) (*CatalogEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if e, ok := c.entries[id]; ok {
		return e, true
	}
	if realID, ok := c.aliases[id]; ok {
		if e, ok := c.entries[realID]; ok {
			return e, true
		}
	}
	return nil, false
}

// FindByCapabilities returns all models that have ALL specified capabilities,
// sorted by tier (economy first) then by cost.
func (c *Catalog) FindByCapabilities(caps ...Capability) []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var results []CatalogEntry
	for _, e := range c.entries {
		if e.Deprecated {
			continue
		}
		if e.HasAllCapabilities(caps...) {
			results = append(results, *e)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Tier != results[j].Tier {
			return results[i].Tier < results[j].Tier
		}
		return results[i].Pricing.InputPerMToken < results[j].Pricing.InputPerMToken
	})
	return results
}

// FindByProvider returns all models from a specific provider.
func (c *Catalog) FindByProvider(provider ProviderName) []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var results []CatalogEntry
	for _, e := range c.entries {
		if e.Provider == provider {
			results = append(results, *e)
		}
	}
	return results
}

// FindByTier returns all models at the given tier.
func (c *Catalog) FindByTier(tier Tier) []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var results []CatalogEntry
	for _, e := range c.entries {
		if e.Tier == tier && !e.Deprecated {
			results = append(results, *e)
		}
	}
	return results
}

// FindCheapest returns the cheapest model with all specified capabilities.
func (c *Catalog) FindCheapest(caps ...Capability) (*CatalogEntry, bool) {
	results := c.FindByCapabilities(caps...)
	if len(results) == 0 {
		return nil, false
	}
	return &results[0], true
}

// FindBest returns the highest tier model with all specified capabilities.
func (c *Catalog) FindBest(caps ...Capability) (*CatalogEntry, bool) {
	results := c.FindByCapabilities(caps...)
	if len(results) == 0 {
		return nil, false
	}
	return &results[len(results)-1], true
}

// Search searches models by name/ID substring.
func (c *Catalog) Search(query string) []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	q := strings.ToLower(query)
	var results []CatalogEntry
	for _, e := range c.entries {
		if strings.Contains(strings.ToLower(e.ModelID), q) ||
			strings.Contains(strings.ToLower(e.DisplayName), q) {
			results = append(results, *e)
		}
	}
	return results
}

// Count returns total entries in the catalog.
func (c *Catalog) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// All returns all catalog entries.
func (c *Catalog) All() []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]CatalogEntry, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, *e)
	}
	return out
}

// Stats returns catalog statistics.
func (c *Catalog) Stats() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	providers := make(map[ProviderName]int)
	tiers := make(map[Tier]int)
	caps := make(map[Capability]int)
	deprecated := 0

	for _, e := range c.entries {
		providers[e.Provider]++
		tiers[e.Tier]++
		if e.Deprecated {
			deprecated++
		}
		for _, cap := range e.Capabilities {
			caps[cap]++
		}
	}

	return map[string]any{
		"total":        len(c.entries),
		"aliases":      len(c.aliases),
		"deprecated":   deprecated,
		"providers":    providers,
		"tiers":        tiers,
		"capabilities": caps,
	}
}

// LoadBuiltinCatalog populates the catalog with well-known models.
func (c *Catalog) LoadBuiltinCatalog() {
	builtins := []CatalogEntry{
		// OpenAI
		{ModelID: "gpt-4o", DisplayName: "GPT-4o", Provider: ProviderOpenAI, Tier: TierPremium,
			Capabilities:  []Capability{CapChat, CapVision, CapToolUse, CapJSON, CapStreaming, CapMultimodal},
			ContextWindow: 128000, MaxOutput: 16384,
			Pricing: Pricing{InputPerMToken: 2.5, OutputPerMToken: 10.0, CachedInput: 1.25},
			Aliases: []string{"gpt4o"}},
		{ModelID: "gpt-4o-mini", DisplayName: "GPT-4o Mini", Provider: ProviderOpenAI, Tier: TierEconomy,
			Capabilities:  []Capability{CapChat, CapVision, CapToolUse, CapJSON, CapStreaming, CapMultimodal},
			ContextWindow: 128000, MaxOutput: 16384,
			Pricing: Pricing{InputPerMToken: 0.15, OutputPerMToken: 0.6, CachedInput: 0.075}},
		{ModelID: "o1", DisplayName: "O1", Provider: ProviderOpenAI, Tier: TierPremium,
			Capabilities:  []Capability{CapChat, CapReasoning, CapCoding, CapVision, CapToolUse, CapStreaming},
			ContextWindow: 200000, MaxOutput: 100000,
			Pricing: Pricing{InputPerMToken: 15.0, OutputPerMToken: 60.0, CachedInput: 7.5}},
		{ModelID: "o3-mini", DisplayName: "O3 Mini", Provider: ProviderOpenAI, Tier: TierStandard,
			Capabilities:  []Capability{CapChat, CapReasoning, CapCoding, CapToolUse, CapStreaming},
			ContextWindow: 200000, MaxOutput: 100000,
			Pricing: Pricing{InputPerMToken: 1.1, OutputPerMToken: 4.4, CachedInput: 0.55}},

		// Anthropic
		{ModelID: "claude-sonnet-4-20250514", DisplayName: "Claude Sonnet 4", Provider: ProviderAnthropic, Tier: TierPremium,
			Capabilities:  []Capability{CapChat, CapVision, CapToolUse, CapJSON, CapStreaming, CapCoding, CapReasoning, CapMultimodal},
			ContextWindow: 200000, MaxOutput: 64000,
			Pricing: Pricing{InputPerMToken: 3.0, OutputPerMToken: 15.0, CachedInput: 0.3},
			Aliases: []string{"claude-sonnet-4", "sonnet-4"}},
		{ModelID: "claude-3-5-haiku-20241022", DisplayName: "Claude 3.5 Haiku", Provider: ProviderAnthropic, Tier: TierEconomy,
			Capabilities:  []Capability{CapChat, CapVision, CapToolUse, CapJSON, CapStreaming, CapCoding},
			ContextWindow: 200000, MaxOutput: 8192,
			Pricing: Pricing{InputPerMToken: 0.8, OutputPerMToken: 4.0, CachedInput: 0.08},
			Aliases: []string{"claude-haiku", "haiku"}},

		// Google
		{ModelID: "gemini-2.5-pro", DisplayName: "Gemini 2.5 Pro", Provider: ProviderGoogle, Tier: TierPremium,
			Capabilities:  []Capability{CapChat, CapVision, CapAudio, CapVideo, CapToolUse, CapJSON, CapStreaming, CapReasoning, CapMultimodal, CapLongCtx},
			ContextWindow: 1048576, MaxOutput: 65536,
			Pricing: Pricing{InputPerMToken: 1.25, OutputPerMToken: 10.0}},
		{ModelID: "gemini-2.0-flash", DisplayName: "Gemini 2.0 Flash", Provider: ProviderGoogle, Tier: TierEconomy,
			Capabilities:  []Capability{CapChat, CapVision, CapAudio, CapToolUse, CapJSON, CapStreaming, CapMultimodal, CapLongCtx},
			ContextWindow: 1048576, MaxOutput: 8192,
			Pricing: Pricing{InputPerMToken: 0.1, OutputPerMToken: 0.4}},

		// DeepSeek
		{ModelID: "deepseek-chat", DisplayName: "DeepSeek V3", Provider: ProviderDeepSeek, Tier: TierStandard,
			Capabilities:  []Capability{CapChat, CapToolUse, CapJSON, CapStreaming, CapCoding},
			ContextWindow: 64000, MaxOutput: 8192,
			Pricing: Pricing{InputPerMToken: 0.27, OutputPerMToken: 1.1, CachedInput: 0.07},
			Aliases: []string{"deepseek-v3"}},
		{ModelID: "deepseek-reasoner", DisplayName: "DeepSeek R1", Provider: ProviderDeepSeek, Tier: TierStandard,
			Capabilities:  []Capability{CapChat, CapReasoning, CapCoding, CapStreaming},
			ContextWindow: 64000, MaxOutput: 8192,
			Pricing: Pricing{InputPerMToken: 0.55, OutputPerMToken: 2.19, CachedInput: 0.14},
			Aliases: []string{"deepseek-r1"}},

		// Qwen
		{ModelID: "qwen-max", DisplayName: "Qwen Max", Provider: ProviderQwen, Tier: TierPremium,
			Capabilities:  []Capability{CapChat, CapToolUse, CapJSON, CapStreaming, CapCoding, CapLongCtx},
			ContextWindow: 131072, MaxOutput: 8192,
			Pricing: Pricing{InputPerMToken: 2.4, OutputPerMToken: 9.6}},
		{ModelID: "qwen-turbo", DisplayName: "Qwen Turbo", Provider: ProviderQwen, Tier: TierEconomy,
			Capabilities:  []Capability{CapChat, CapToolUse, CapJSON, CapStreaming, CapLongCtx},
			ContextWindow: 131072, MaxOutput: 8192,
			Pricing: Pricing{InputPerMToken: 0.3, OutputPerMToken: 0.6}},

		// Zhipu
		{ModelID: "glm-4-plus", DisplayName: "GLM-4 Plus", Provider: ProviderZhipu, Tier: TierPremium,
			Capabilities:  []Capability{CapChat, CapToolUse, CapJSON, CapStreaming, CapCoding},
			ContextWindow: 128000, MaxOutput: 4096,
			Pricing: Pricing{InputPerMToken: 7.14, OutputPerMToken: 7.14}},

		// Moonshot
		{ModelID: "moonshot-v1-128k", DisplayName: "Moonshot V1 128K", Provider: ProviderMoonshot, Tier: TierStandard,
			Capabilities:  []Capability{CapChat, CapToolUse, CapStreaming, CapLongCtx},
			ContextWindow: 128000, MaxOutput: 4096,
			Pricing: Pricing{InputPerMToken: 8.57, OutputPerMToken: 8.57}},

		// Mistral
		{ModelID: "mistral-large-latest", DisplayName: "Mistral Large", Provider: ProviderMistral, Tier: TierPremium,
			Capabilities:  []Capability{CapChat, CapToolUse, CapJSON, CapStreaming, CapCoding, CapReasoning},
			ContextWindow: 128000, MaxOutput: 8192,
			Pricing: Pricing{InputPerMToken: 2.0, OutputPerMToken: 6.0}},

		// Groq (fast inference)
		{ModelID: "llama-3.3-70b-versatile", DisplayName: "Llama 3.3 70B (Groq)", Provider: ProviderGroq, Tier: TierEconomy,
			Capabilities:  []Capability{CapChat, CapToolUse, CapJSON, CapStreaming},
			ContextWindow: 128000, MaxOutput: 32768,
			Pricing: Pricing{InputPerMToken: 0.59, OutputPerMToken: 0.79}},
	}

	for _, entry := range builtins {
		c.Add(entry)
	}
}
