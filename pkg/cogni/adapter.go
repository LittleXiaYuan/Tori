package cogni

import (
	"yunque-agent/pkg/plugin"
)

// PluginToDeclaration wraps an existing Plugin as a Cogni Declaration so it
// participates in the unified Cogni evaluation pipeline. The generated
// declaration uses AlwaysOn activation with low priority (200) so existing
// Plugin behavior is preserved: their system prompt is always injected,
// and their skills remain visible. Dedicated Cogni declarations with
// higher priority (lower number) can override or narrow the surface.
func PluginToDeclaration(p plugin.Plugin) *Declaration {
	if p == nil {
		return nil
	}

	d := &Declaration{
		ID:          "plugin:" + p.Name(),
		DisplayName: p.Description(),
		Description: "Auto-adapted from Plugin: " + p.Name(),
		Priority:    200,
		Activation: ActivationRules{
			AlwaysOn: true,
		},
		Context: ContextInjection{
			Static: p.SystemPrompt(),
		},
	}

	// Expose only this plugin's own skills.
	var skillNames []string
	for _, s := range p.Skills() {
		skillNames = append(skillNames, s.Name())
	}
	if len(skillNames) > 0 {
		d.Surface = ToolSurface{
			Include: skillNames,
		}
	}

	return d
}

// CognitivePluginToDeclaration wraps a CognitivePlugin with richer
// activation rules derived from ShouldHandle semantics.
func CognitivePluginToDeclaration(cp plugin.CognitivePlugin) *Declaration {
	d := PluginToDeclaration(cp)
	if d == nil {
		return nil
	}
	// CognitivePlugins already have activation logic (ShouldHandle) that
	// runs inside the planner. By keeping AlwaysOn, their DynamicContext
	// is always injected, matching the existing CognitivePlugin contract.
	// The cognitiveContext planner callback handles ShouldHandle separately.
	return d
}

// RegisterPlugins bulk-registers existing Plugins into a Cogni Registry
// with source "plugin-adapter" so they're distinguishable from file-loaded
// declarations during reload (file-sourced entries are keyed by "file:").
func RegisterPlugins(reg *Registry, plugins []plugin.Plugin) {
	for _, p := range plugins {
		d := PluginToDeclaration(p)
		if d == nil {
			continue
		}
		_ = reg.Add(d, "plugin-adapter")
	}
}
