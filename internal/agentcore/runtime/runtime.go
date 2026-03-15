package runtime

import (
	"context"
	"log/slog"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/execution/sandbox"
)

// AgentConfig describes how to construct an AgentRuntime.
type AgentConfig struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	PersonaPrompt string            `json:"persona_prompt,omitempty"`
	SandboxLevel  SandboxLevel      `json:"sandbox_level,omitempty"` // "off", "basic", "strict"
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// SandboxLevel controls execution isolation.
type SandboxLevel string

const (
	SandboxOff    SandboxLevel = "off"
	SandboxBasic  SandboxLevel = "basic"  // commands allowlisted, no network
	SandboxStrict SandboxLevel = "strict" // docker isolation
)

// AgentRuntime is an isolated execution context for a single agent.
// Each runtime owns its own planner, memory manager, session store, and sandbox policy.
// This is the core building block for multi-agent collaboration.
type AgentRuntime struct {
	Config       AgentConfig
	Planner      *planner.Planner
	Memory       *memory.Manager
	Orchestrator *memory.Orchestrator
	Sessions     *session.Store
	SandboxPol   sandbox.Policy
}

// Run executes a plan request within this agent's isolated context.
func (r *AgentRuntime) Run(ctx context.Context, msgs []llm.Message) (*planner.PlanResult, error) {
	result, err := r.Planner.Run(ctx, planner.PlanRequest{
		Messages: msgs,
		TenantID: r.Config.ID,
	})
	if err != nil {
		slog.Warn("agent runtime error", "agent", r.Config.ID, "err", err)
		return nil, err
	}
	return result, nil
}

// ID returns the agent identifier.
func (r *AgentRuntime) ID() string { return r.Config.ID }

// Name returns the agent display name.
func (r *AgentRuntime) Name() string { return r.Config.Name }
