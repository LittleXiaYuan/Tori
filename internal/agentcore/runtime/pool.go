package runtime

import (
	"fmt"
	"sync"
)

// ──────────────────────────────────────────────
// RuntimePool — manages multiple AgentRuntime instances
// Provides lookup by ID and a default fallback agent.
// ──────────────────────────────────────────────

// Pool manages a set of AgentRuntime instances.
type Pool struct {
	mu        sync.RWMutex
	agents    map[string]*AgentRuntime
	defaultID string // fallback agent when no binding matches
}

// NewPool creates an empty runtime pool.
func NewPool() *Pool {
	return &Pool{agents: make(map[string]*AgentRuntime)}
}

// Register adds an agent runtime to the pool.
func (p *Pool) Register(rt *AgentRuntime) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.agents[rt.ID()] = rt
	// First registered agent becomes the default
	if p.defaultID == "" {
		p.defaultID = rt.ID()
	}
}

// SetDefault sets the default agent ID for unmatched bindings.
func (p *Pool) SetDefault(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultID = id
}

// Get returns a runtime by agent ID.
func (p *Pool) Get(id string) (*AgentRuntime, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	rt, ok := p.agents[id]
	return rt, ok
}

// Default returns the default runtime (fallback).
func (p *Pool) Default() *AgentRuntime {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.defaultID == "" {
		return nil
	}
	return p.agents[p.defaultID]
}

// Resolve returns the runtime for the given ID, falling back to default.
func (p *Pool) Resolve(id string) *AgentRuntime {
	if id != "" {
		if rt, ok := p.Get(id); ok {
			return rt
		}
	}
	return p.Default()
}

// Remove removes an agent runtime from the pool.
func (p *Pool) Remove(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.agents[id]; !ok {
		return fmt.Errorf("agent not found: %s", id)
	}
	delete(p.agents, id)
	if p.defaultID == id {
		p.defaultID = ""
		// Promote any remaining agent as default
		for k := range p.agents {
			p.defaultID = k
			break
		}
	}
	return nil
}

// List returns all registered agent configs.
func (p *Pool) List() []AgentConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]AgentConfig, 0, len(p.agents))
	for _, rt := range p.agents {
		out = append(out, rt.Config)
	}
	return out
}

// Count returns the number of registered agents.
func (p *Pool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.agents)
}
