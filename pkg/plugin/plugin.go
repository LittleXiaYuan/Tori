package plugin

import (
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
