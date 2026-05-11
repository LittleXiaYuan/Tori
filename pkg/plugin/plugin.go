package plugin

import (
	"context"
	"fmt"
	"net/http"
	"yunque-agent/pkg/skills"
)

// Plugin is a domain-specific package that bundles skills, prompts, and config.
type Plugin interface {
	// Name returns the plugin identifier.
	Name() string
	// Description returns what this plugin does.
	Description() string
	// Skills returns all skills provided by this plugin.
	Skills() []skills.Skill
	// SystemPrompt returns additional system prompt text for this domain.
	SystemPrompt() string
}

// UITab describes a navigation tab that a plugin registers in the web dashboard.
type UITab struct {
	Key         string `json:"key"`                   // route key, e.g. "qq-analyzer"
	Label       string `json:"label"`                 // display name (zh)
	LabelEn     string `json:"label_en,omitempty"`    // display name (en)
	Icon        string `json:"icon"`                  // lucide-react icon name
	Description string `json:"description,omitempty"` // short description
	Plugin      string `json:"plugin"`                // owning plugin name
}

// UIPlugin extends Plugin with UI tab registration and custom HTTP handlers.
// Plugins that implement this interface can:
//   - Register navigation tabs in the web dashboard
//   - Mount custom API routes under /v1/ext/{plugin-key}/
type UIPlugin interface {
	Plugin
	// UITabs returns the tabs this plugin wants to show in the web dashboard.
	UITabs() []UITab
	// HTTPHandlers returns custom HTTP handlers keyed by relative path.
	// e.g. {"/upload": uploadHandler, "/analyze": analyzeHandler}
	// These get mounted at /v1/ext/{plugin-key}{path}
	HTTPHandlers() map[string]http.HandlerFunc
}

// CognitivePlugin extends Plugin with capabilities that participate in the agent's
// reasoning process: dynamic context injection, message routing, and memory transformation.
type CognitivePlugin interface {
	Plugin

	// DynamicContext returns context text injected into the LLM system prompt on every request.
	// Return "" to skip.
	DynamicContext(ctx context.Context, userMessage string) string

	// ShouldHandle returns a confidence score (0-1) for claiming this message.
	// Score >= 0.7 means the plugin handles it directly via Handle(), bypassing the Planner.
	ShouldHandle(ctx context.Context, message string) float64

	// Handle processes a message claimed by ShouldHandle.
	Handle(ctx context.Context, message string, env *CognitiveEnv) (string, error)

	// OnMemoryExtract transforms facts extracted by the memory pipeline.
	// Return nil to suppress all facts.
	OnMemoryExtract(ctx context.Context, facts []ExtractedFact) []ExtractedFact
}

// CognitiveEnv provides resources to CognitivePlugin.Handle().
type CognitiveEnv struct {
	LLMCall      func(ctx context.Context, system, user string) (string, error)
	MemorySearch func(ctx context.Context, query string) string
	TenantID     string
	UserID       string
	ChannelType  string
	SessionID    string
}

// ExtractedFact is a fact extracted by the memory pipeline.
type ExtractedFact struct {
	Key     string            `json:"key"`
	Value   string            `json:"value"`
	Source  string            `json:"source"`
	Tags    map[string]string `json:"tags,omitempty"`
}

// PluginMemory is a namespaced key-value store for plugin-private data.
// Each CognitivePlugin gets its own isolated namespace so plugins don't
// interfere with each other or with the agent's main memory.
type PluginMemory interface {
	Get(key string) (string, bool)
	Set(key, value string) error
	Delete(key string) error
	List(prefix string) map[string]string
	// Search returns values that semantically match the query.
	// Plugins with rich data can implement vector search; simple implementations
	// can fall back to substring matching.
	Search(query string, limit int) []string
}

// Registry manages loaded plugins.
type Registry struct {
	plugins map[string]*pluginEntry
	slots   map[string]string // slot name -> active plugin name
}

// NewRegistry creates a plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]*pluginEntry),
		slots:   make(map[string]string),
	}
}

// pluginEntry wraps a plugin with enabled state.
type pluginEntry struct {
	plugin  Plugin
	enabled bool
	slot    string // exclusive slot this plugin occupies (empty = none)
}

// Register adds a plugin (enabled by default).
func (r *Registry) Register(p Plugin) {
	r.plugins[p.Name()] = &pluginEntry{plugin: p, enabled: true}
}

// RegisterWithSlot adds a plugin with an exclusive slot.
// Returns error if the slot is already occupied by another enabled plugin.
func (r *Registry) RegisterWithSlot(p Plugin, slot string) error {
	if slot != "" {
		if existing, ok := r.slots[slot]; ok && existing != p.Name() {
			return fmt.Errorf("slot %q is occupied by plugin %q", slot, existing)
		}
		r.slots[slot] = p.Name()
	}
	r.plugins[p.Name()] = &pluginEntry{plugin: p, enabled: true, slot: slot}
	return nil
}

// SlotOwner returns the plugin name occupying a given slot.
func (r *Registry) SlotOwner(slot string) (string, bool) {
	name, ok := r.slots[slot]
	return name, ok
}

// Slots returns all occupied slots.
func (r *Registry) Slots() map[string]string {
	out := make(map[string]string, len(r.slots))
	for k, v := range r.slots {
		out[k] = v
	}
	return out
}

// Unregister removes a plugin and frees its slot.
func (r *Registry) Unregister(name string) {
	if e, ok := r.plugins[name]; ok && e.slot != "" {
		delete(r.slots, e.slot)
	}
	delete(r.plugins, name)
}

// SetEnabled enables or disables a plugin by name.
func (r *Registry) SetEnabled(name string, enabled bool) bool {
	e, ok := r.plugins[name]
	if !ok {
		return false
	}
	e.enabled = enabled
	return true
}

// IsEnabled checks if a plugin is enabled.
func (r *Registry) IsEnabled(name string) bool {
	e, ok := r.plugins[name]
	return ok && e.enabled
}

// Get returns a plugin by name (regardless of enabled state).
func (r *Registry) Get(name string) (Plugin, bool) {
	e, ok := r.plugins[name]
	if !ok {
		return nil, false
	}
	return e.plugin, true
}

// All returns all registered plugins (enabled only).
func (r *Registry) All() []Plugin {
	out := make([]Plugin, 0, len(r.plugins))
	for _, e := range r.plugins {
		if e.enabled {
			out = append(out, e.plugin)
		}
	}
	return out
}

// AllIncludeDisabled returns all plugins with their enabled state.
func (r *Registry) AllIncludeDisabled() []PluginInfo {
	out := make([]PluginInfo, 0, len(r.plugins))
	for _, e := range r.plugins {
		info := PluginInfo{
			Name:        e.plugin.Name(),
			Description: e.plugin.Description(),
			Enabled:     e.enabled,
			SkillCount:  len(e.plugin.Skills()),
			Slot:        e.slot,
			Source:      "builtin",
		}
		if sp, ok := e.plugin.(*ScriptPlugin); ok {
			info.Source = "script"
			info.Language = sp.Manifest().Language
		}
		if up, ok := e.plugin.(UIPlugin); ok {
			info.HasUI = true
			info.UITabs = up.UITabs()
		}
		out = append(out, info)
	}
	return out
}

// PluginInfo is metadata about a plugin.
type PluginInfo struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Enabled     bool    `json:"enabled"`
	SkillCount  int     `json:"skill_count"`
	Slot        string  `json:"slot,omitempty"`
	Source      string  `json:"source"`             // "builtin" or "script"
	Language    string  `json:"language,omitempty"` // for script plugins
	HasUI       bool    `json:"has_ui"`             // implements UIPlugin
	IsCognitive bool    `json:"is_cognitive"`       // implements CognitivePlugin
	UITabs      []UITab `json:"ui_tabs,omitempty"`  // registered tabs
}

// AllSkills returns all skills from enabled plugins.
func (r *Registry) AllSkills() []skills.Skill {
	var out []skills.Skill
	for _, e := range r.plugins {
		if e.enabled {
			out = append(out, e.plugin.Skills()...)
		}
	}
	return out
}

// CombinedPrompt merges system prompts from enabled plugins.
func (r *Registry) CombinedPrompt() string {
	var result string
	for _, e := range r.plugins {
		if e.enabled {
			if sp := e.plugin.SystemPrompt(); sp != "" {
				result += "\n\n## " + e.plugin.Name() + " 领域能力\n" + sp
			}
		}
	}
	return result
}

// AllUITabs returns all UI tabs from enabled plugins that implement UIPlugin.
func (r *Registry) AllUITabs() []UITab {
	var tabs []UITab
	for _, e := range r.plugins {
		if !e.enabled {
			continue
		}
		if up, ok := e.plugin.(UIPlugin); ok {
			for _, t := range up.UITabs() {
				t.Plugin = e.plugin.Name()
				tabs = append(tabs, t)
			}
		}
	}
	return tabs
}

// AllHTTPHandlers returns all HTTP handlers from enabled plugins that implement UIPlugin.
// Returns a map of full path -> handler: "/v1/ext/{pluginKey}{relativePath}" -> handler.
func (r *Registry) AllHTTPHandlers() map[string]http.HandlerFunc {
	out := make(map[string]http.HandlerFunc)
	for _, e := range r.plugins {
		if !e.enabled {
			continue
		}
		if up, ok := e.plugin.(UIPlugin); ok {
			for path, handler := range up.HTTPHandlers() {
				fullPath := "/v1/ext/" + e.plugin.Name() + path
				out[fullPath] = handler
			}
		}
	}
	return out
}

// ── CognitivePlugin Registry Methods ──

// AllCognitive returns all enabled CognitivePlugins.
func (r *Registry) AllCognitive() []CognitivePlugin {
	var out []CognitivePlugin
	for _, e := range r.plugins {
		if e.enabled {
			if cp, ok := e.plugin.(CognitivePlugin); ok {
				out = append(out, cp)
			}
		}
	}
	return out
}

// RouteMessage checks all CognitivePlugins and returns the one that wants to handle
// the message with the highest confidence. Returns nil if no plugin claims it (score < 0.7).
func (r *Registry) RouteMessage(ctx context.Context, message string) (CognitivePlugin, float64) {
	var best CognitivePlugin
	bestScore := 0.0
	for _, e := range r.plugins {
		if !e.enabled {
			continue
		}
		cp, ok := e.plugin.(CognitivePlugin)
		if !ok {
			continue
		}
		score := cp.ShouldHandle(ctx, message)
		if score > bestScore {
			bestScore = score
			best = cp
		}
	}
	if bestScore < 0.7 {
		return nil, 0
	}
	return best, bestScore
}

// CollectDynamicContext gathers context text from all CognitivePlugins for a given
// user message. Returns concatenated context ready for injection into the system prompt.
func (r *Registry) CollectDynamicContext(ctx context.Context, userMessage string) string {
	var parts []string
	for _, e := range r.plugins {
		if !e.enabled {
			continue
		}
		cp, ok := e.plugin.(CognitivePlugin)
		if !ok {
			continue
		}
		if ctxText := cp.DynamicContext(ctx, userMessage); ctxText != "" {
			parts = append(parts, "## "+e.plugin.Name()+" 认知上下文\n"+ctxText)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "\n\n"
		}
		result += p
	}
	return result
}

// TransformFacts passes extracted facts through all CognitivePlugins' OnMemoryExtract.
// Plugins are called in registration order; each sees the output of the previous.
func (r *Registry) TransformFacts(ctx context.Context, facts []ExtractedFact) []ExtractedFact {
	for _, e := range r.plugins {
		if !e.enabled {
			continue
		}
		cp, ok := e.plugin.(CognitivePlugin)
		if !ok {
			continue
		}
		facts = cp.OnMemoryExtract(ctx, facts)
		if facts == nil {
			return nil
		}
	}
	return facts
}

// PluginInfoEx extends PluginInfo with cognitive capability flags.
func (r *Registry) AllIncludeDisabledEx() []PluginInfo {
	out := r.AllIncludeDisabled()
	for i, info := range out {
		if e, ok := r.plugins[info.Name]; ok {
			if _, isCognitive := e.plugin.(CognitivePlugin); isCognitive {
				out[i].IsCognitive = true
			}
		}
	}
	return out
}
