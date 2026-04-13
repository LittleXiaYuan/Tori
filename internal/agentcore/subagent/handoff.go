package subagent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// RunFunc is the function signature for running a planner request within a subagent.
// It receives the subagent's isolated context and returns the reply.
type RunFunc func(ctx context.Context, agentName string, input string, providerOverride string) (string, error)

// HandoffConfig defines a named subagent that can be delegated to.
type HandoffConfig struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Skills      []string `json:"skills,omitempty"`      // scoped skill names (empty = all)
	ProviderID  string   `json:"provider_id,omitempty"` // per-agent LLM override
	SystemNote  string   `json:"system_note,omitempty"` // extra system prompt for this agent
}

// HandoffResult is the result of a handoff execution.
type HandoffResult struct {
	AgentName string        `json:"agent_name"`
	AgentID   string        `json:"agent_id"`
	Reply     string        `json:"reply"`
	Duration  time.Duration `json:"duration_ms"`
}

// HandoffRegistry manages named subagent configurations and their execution.
type HandoffRegistry struct {
	mu      sync.RWMutex
	configs map[string]*HandoffConfig // name → config
	mgr     *Manager
	runFn   RunFunc // injected planner.Run wrapper
}

// NewHandoffRegistry creates a handoff registry backed by the given subagent manager.
func NewHandoffRegistry(mgr *Manager) *HandoffRegistry {
	return &HandoffRegistry{
		configs: make(map[string]*HandoffConfig),
		mgr:     mgr,
	}
}

// SetRunFunc injects the planner-loop execution function.
func (hr *HandoffRegistry) SetRunFunc(fn RunFunc) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.runFn = fn
}

// Register adds a named subagent configuration.
func (hr *HandoffRegistry) Register(cfg HandoffConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("handoff config name is required")
	}
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.configs[cfg.Name] = &cfg
	slog.Info("handoff agent registered", "name", cfg.Name, "provider", cfg.ProviderID)
	return nil
}

// Unregister removes a named subagent configuration.
func (hr *HandoffRegistry) Unregister(name string) bool {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	if _, ok := hr.configs[name]; !ok {
		return false
	}
	delete(hr.configs, name)
	return true
}

// Get returns a handoff config by name.
func (hr *HandoffRegistry) Get(name string) (*HandoffConfig, bool) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	cfg, ok := hr.configs[name]
	if !ok {
		return nil, false
	}
	cp := *cfg
	return &cp, true
}

// List returns all registered handoff configs.
func (hr *HandoffRegistry) List() []HandoffConfig {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	out := make([]HandoffConfig, 0, len(hr.configs))
	for _, cfg := range hr.configs {
		out = append(out, *cfg)
	}
	return out
}

// Execute delegates a task to a named subagent.
// It spawns (or reuses) a subagent instance, runs the task, and returns the result.
// parentProvider is the model/provider the parent planner is using; exec agents
// inherit it when no per-agent ProviderID is configured.
func (hr *HandoffRegistry) Execute(ctx context.Context, parentID, agentName, input, parentProvider string) (*HandoffResult, error) {
	hr.mu.RLock()
	cfg, ok := hr.configs[agentName]
	runFn := hr.runFn
	hr.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("handoff agent %q not registered", agentName)
	}
	if runFn == nil {
		return nil, fmt.Errorf("handoff run function not set")
	}

	// Spawn a fresh subagent for this delegation
	sa, err := hr.mgr.Spawn(parentID, agentName, cfg.Description, cfg.Skills)
	if err != nil {
		return nil, fmt.Errorf("spawn subagent %q: %w", agentName, err)
	}
	defer hr.mgr.Destroy(sa.ID) // cleanup after execution

	slog.Info("handoff: delegating task", "agent", agentName, "subagent_id", sa.ID, "parent", parentID)

	provider := cfg.ProviderID
	if provider == "" {
		provider = parentProvider
	}

	t0 := time.Now()
	reply, err := runFn(ctx, agentName, input, provider)
	dur := time.Since(t0)

	if err != nil {
		slog.Warn("handoff: agent failed", "agent", agentName, "err", err, "duration", dur)
		return nil, fmt.Errorf("handoff agent %q execution failed: %w", agentName, err)
	}

	slog.Info("handoff: agent completed", "agent", agentName, "duration", dur)
	return &HandoffResult{
		AgentName: agentName,
		AgentID:   sa.ID,
		Reply:     reply,
		Duration:  dur,
	}, nil
}

// ToolNames returns skill names formatted as "transfer_to_{name}" for LLM discovery.
func (hr *HandoffRegistry) ToolNames() []string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	names := make([]string, 0, len(hr.configs))
	for name := range hr.configs {
		names = append(names, "transfer_to_"+name)
	}
	return names
}

// ToolDefinitions returns skill parameter definitions for all handoff agents.
// These are formatted for injection into the LLM function calling schema.
func (hr *HandoffRegistry) ToolDefinitions() []map[string]any {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	defs := make([]map[string]any, 0, len(hr.configs))
	for name, cfg := range hr.configs {
		desc := cfg.Description
		if desc == "" {
			desc = fmt.Sprintf("委派任务给 %s 子Agent", name)
		}
		defs = append(defs, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "transfer_to_" + name,
				"description": desc,
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"input": map[string]any{
							"type":        "string",
							"description": "要委派给子Agent的任务描述",
						},
					},
					"required": []string{"input"},
				},
			},
		})
	}
	return defs
}

// IsHandoffCall checks if a skill name is a handoff tool call.
func (hr *HandoffRegistry) IsHandoffCall(skillName string) (agentName string, ok bool) {
	const prefix = "transfer_to_"
	if !strings.HasPrefix(skillName, prefix) {
		return "", false
	}
	name := skillName[len(prefix):]
	hr.mu.RLock()
	_, exists := hr.configs[name]
	hr.mu.RUnlock()
	return name, exists
}
