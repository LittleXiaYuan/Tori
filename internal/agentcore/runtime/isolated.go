package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/review"
	"yunque-agent/internal/execution/sandbox"
)

// IsolatedRunner provides sandboxed command execution with review gate integration.
type IsolatedRunner struct {
	mu       sync.Mutex
	policy   sandbox.Policy
	gate     *review.IntelligentGate
	stats    RunnerStats
}

// RunnerStats tracks execution statistics.
type RunnerStats struct {
	TotalRuns      int64         `json:"total_runs"`
	Blocked        int64         `json:"blocked"`
	Succeeded      int64         `json:"succeeded"`
	Failed         int64         `json:"failed"`
	AvgDuration    time.Duration `json:"avg_duration"`
}

// NewIsolatedRunner creates a runner with the given sandbox policy.
func NewIsolatedRunner(level SandboxLevel) *IsolatedRunner {
	var policy sandbox.Policy
	switch level {
	case SandboxStrict:
		policy = sandbox.DefaultPolicy()
		policy.AllowNetwork = false
	case SandboxBasic:
		policy = sandbox.PersonalPolicy()
	default:
		policy = sandbox.Policy{MaxDuration: 5 * time.Minute, MaxOutputBytes: 512 * 1024}
	}
	return &IsolatedRunner{policy: policy}
}

// SetReviewGate attaches a review gate for pre-execution checks.
func (ir *IsolatedRunner) SetReviewGate(gate *review.IntelligentGate) {
	ir.gate = gate
}

// RunCommand executes a command within the sandbox, subject to review.
func (ir *IsolatedRunner) RunCommand(ctx context.Context, command, tenantID string) (*sandbox.Result, error) {
	// 审查阶段
	if ir.gate != nil {
		verdict := ir.gate.ReviewDetailed(ctx, "shell_exec", command, tenantID)
		if !verdict.Allowed {
			ir.mu.Lock()
			ir.stats.Blocked++
			ir.mu.Unlock()
			return nil, fmt.Errorf("blocked by review gate: %s", verdict.Reason)
		}
	}

	// 策略检查
	if !ir.policy.AllowNetwork && containsNetworkCmd(command) {
		ir.mu.Lock()
		ir.stats.Blocked++
		ir.mu.Unlock()
		return nil, fmt.Errorf("network commands blocked by sandbox policy")
	}

	// 执行
	start := time.Now()
	sb, err := sandbox.New("", ir.policy)
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	result, err := sb.Exec(ctx, command)
	dur := time.Since(start)

	ir.mu.Lock()
	ir.stats.TotalRuns++
	if err != nil {
		ir.stats.Failed++
	} else {
		ir.stats.Succeeded++
	}
	ir.stats.AvgDuration = (ir.stats.AvgDuration*time.Duration(ir.stats.TotalRuns-1) + dur) / time.Duration(ir.stats.TotalRuns)
	ir.mu.Unlock()

	return result, err
}

// Stats returns execution statistics.
func (ir *IsolatedRunner) Stats() RunnerStats {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	return ir.stats
}

// containsNetworkCmd checks for common network-related commands.
func containsNetworkCmd(cmd string) bool {
	netCmds := []string{"curl", "wget", "ssh", "scp", "rsync", "nc ", "netcat", "telnet", "ftp "}
	for _, nc := range netCmds {
		if contains(cmd, nc) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// AgentPool manages multiple AgentRuntime instances for multi-agent scenarios.
type AgentPool struct {
	mu      sync.RWMutex
	agents  map[string]*AgentRuntime
	factory AgentFactory
}

// AgentFactory creates agent runtimes on demand.
type AgentFactory func(config AgentConfig) (*AgentRuntime, error)

// NewAgentPool creates an agent pool.
func NewAgentPool() *AgentPool {
	return &AgentPool{
		agents: make(map[string]*AgentRuntime),
	}
}

// SetFactory sets the agent creation function.
func (ap *AgentPool) SetFactory(fn AgentFactory) { ap.factory = fn }

// Get returns an agent by ID.
func (ap *AgentPool) Get(id string) (*AgentRuntime, bool) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()
	a, ok := ap.agents[id]
	return a, ok
}

// GetOrCreate returns an existing agent or creates one.
func (ap *AgentPool) GetOrCreate(config AgentConfig) (*AgentRuntime, error) {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if a, ok := ap.agents[config.ID]; ok {
		return a, nil
	}
	if ap.factory == nil {
		return nil, fmt.Errorf("no agent factory configured")
	}
	a, err := ap.factory(config)
	if err != nil {
		return nil, err
	}
	ap.agents[config.ID] = a
	slog.Info("agent pool: created runtime", "id", config.ID, "name", config.Name)
	return a, nil
}

// Remove shuts down and removes an agent.
func (ap *AgentPool) Remove(id string) bool {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	if _, ok := ap.agents[id]; !ok {
		return false
	}
	delete(ap.agents, id)
	return true
}

// List returns all agent IDs.
func (ap *AgentPool) List() []string {
	ap.mu.RLock()
	defer ap.mu.RUnlock()
	ids := make([]string, 0, len(ap.agents))
	for id := range ap.agents {
		ids = append(ids, id)
	}
	return ids
}

// RunOnAgent routes a request to a specific agent.
func (ap *AgentPool) RunOnAgent(ctx context.Context, agentID string, msgs []llm.Message) (*planner.PlanResult, error) {
	a, ok := ap.Get(agentID)
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	return a.Run(ctx, msgs)
}
