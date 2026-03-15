package runtime

import (
	"sync"
)

// ──────────────────────────────────────────────
// Binding — routes inbound messages to the correct AgentRuntime
// Inspired by OpenClaw: channel/account/peer/group → agentId
// ──────────────────────────────────────────────

// BindingKey is the composite key for routing decisions.
type BindingKey struct {
	Channel string // "telegram", "feishu", "discord", "api", etc.
	Account string // bot account / app id
	Peer    string // user or contact id
	Group   string // group / channel id (empty for DM)
}

// Binding maps a key pattern to a target agent.
type Binding struct {
	Key      BindingKey `json:"key"`
	AgentID  string     `json:"agent_id"`
	Priority int        `json:"priority"` // higher = checked first
}

// Router resolves which agent should handle a given inbound context.
type Router struct {
	mu       sync.RWMutex
	bindings []Binding
	pool     *Pool
}

// NewRouter creates a binding router backed by a runtime pool.
func NewRouter(pool *Pool) *Router {
	return &Router{pool: pool}
}

// AddBinding adds a routing rule. More specific bindings should have higher priority.
func (r *Router) AddBinding(b Binding) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = append(r.bindings, b)
	// Keep sorted by priority descending (highest first)
	for i := len(r.bindings) - 1; i > 0; i-- {
		if r.bindings[i].Priority > r.bindings[i-1].Priority {
			r.bindings[i], r.bindings[i-1] = r.bindings[i-1], r.bindings[i]
		}
	}
}

// RemoveBindingsForAgent removes all bindings targeting a specific agent.
func (r *Router) RemoveBindingsForAgent(agentID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.bindings[:0]
	removed := 0
	for _, b := range r.bindings {
		if b.AgentID == agentID {
			removed++
		} else {
			kept = append(kept, b)
		}
	}
	r.bindings = kept
	return removed
}

// Resolve finds the best matching agent runtime for the given context.
// Falls back to the pool's default agent if no binding matches.
func (r *Router) Resolve(key BindingKey) *AgentRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, b := range r.bindings {
		if matchBinding(b.Key, key) {
			if rt, ok := r.pool.Get(b.AgentID); ok {
				return rt
			}
		}
	}
	return r.pool.Default()
}

// Bindings returns a copy of all current bindings.
func (r *Router) Bindings() []Binding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Binding, len(r.bindings))
	copy(out, r.bindings)
	return out
}

// matchBinding checks if a binding key pattern matches the given key.
// Empty fields in the pattern are wildcards (match anything).
func matchBinding(pattern, key BindingKey) bool {
	if pattern.Channel != "" && pattern.Channel != key.Channel {
		return false
	}
	if pattern.Account != "" && pattern.Account != key.Account {
		return false
	}
	if pattern.Peer != "" && pattern.Peer != key.Peer {
		return false
	}
	if pattern.Group != "" && pattern.Group != key.Group {
		return false
	}
	return true
}
