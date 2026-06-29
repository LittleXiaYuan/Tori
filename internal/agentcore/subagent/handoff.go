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
//
// partial is any work the subagent completed before a timeout/cancel — e.g.
// research already gathered, a half-generated document. When err is a timeout
// (#33), the parent planner feeds partial into its own PartialPlanResultForRequest
// so the user sees "已部分执行" instead of an empty failure. Empty partial
// means the subagent produced nothing usable before the timeout.
type RunFunc func(ctx context.Context, agentName string, input string, providerOverride string) (reply string, partial string, err error)

// HandoffConfig defines a named subagent that can be delegated to.
type HandoffConfig struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Skills      []string `json:"skills,omitempty"`      // scoped skill names (empty = all)
	ProviderID  string   `json:"provider_id,omitempty"` // per-agent LLM override
	SystemNote  string   `json:"system_note,omitempty"` // extra system prompt for this agent
}

// HandoffResult is the result of a handoff execution.
//
// PartialResult carries any work the subagent completed before a timeout/cancel
// (#33). Empty on success (Reply holds the full result) or when the subagent
// produced nothing usable. On timeout, Reply is empty and PartialResult holds
// the recoverable evidence so the parent planner can surface it via
// PartialPlanResultForRequest instead of losing the subagent's progress.
type HandoffResult struct {
	AgentName     string        `json:"agent_name"`
	AgentID       string        `json:"agent_id"`
	Reply         string        `json:"reply"`
	PartialResult string        `json:"partial_result,omitempty"`
	Duration      time.Duration `json:"duration_ms"`
}

// HandoffRegistry manages named subagent configurations and their execution.
type HandoffRegistry struct {
	mu              sync.RWMutex
	configs         map[string]*HandoffConfig // name → config
	mgr             *Manager
	runFn           RunFunc // injected planner.Run wrapper
	contextPreparer func(ctx context.Context, agentName string) string // shared session/project context injector
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

// SetContextPreparer injects a function that returns shared session/project
// context prepended to every sub-agent's input. This solves the "blank-slate
// executor" problem: sub-agents no longer need the parent to manually encode
// who the user is, what project they're in, or what goal is being pursued.
func (hr *HandoffRegistry) SetContextPreparer(fn func(ctx context.Context, agentName string) string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.contextPreparer = fn
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

	// Prepend shared session/project context so the sub-agent doesn't start
	// with a blank slate. The preparer is set by init_planner.go via
	// SetContextPreparer and returns a compact context block that covers
	// who the user is, the active project, and the current goal.
	actualInput := input
	hr.mu.RLock()
	preparer := hr.contextPreparer
	hr.mu.RUnlock()
	if preparer != nil {
		if sharedCtx := preparer(ctx, agentName); sharedCtx != "" {
			actualInput = sharedCtx + "\n\n---\n\n" + input
		}
	}

	t0 := time.Now()
	reply, partial, err := runFn(ctx, agentName, actualInput, provider)
	dur := time.Since(t0)

	if err != nil {
		slog.Warn("handoff: agent failed", "agent", agentName, "err", err, "duration", dur, "has_partial", partial != "")
		// #33: return the result with PartialResult populated so the parent
		// planner can surface recoverable evidence on timeout instead of losing
		// the subagent's progress. The error still propagates so the parent
		// knows the handoff did not complete nominally.
		return &HandoffResult{
			AgentName:     agentName,
			AgentID:       sa.ID,
			PartialResult: partial,
			Duration:      dur,
		}, fmt.Errorf("handoff agent %q execution failed: %w", agentName, err)
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
