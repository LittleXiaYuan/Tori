package llm

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Capability — what an endpoint can do
// ──────────────────────────────────────────────

type Capability string

const (
	CapChat       Capability = "chat"
	CapCompletion Capability = "completion"
	CapEmbedding  Capability = "embedding"
	CapVision     Capability = "vision"
	CapTools      Capability = "tools"
	CapThinking   Capability = "thinking"
)

// ──────────────────────────────────────────────
// Endpoint — a configured LLM provider endpoint
// ──────────────────────────────────────────────

// Endpoint represents a single LLM API endpoint with cooldown state.
type Endpoint struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	BaseURL      string       `json:"base_url"`
	APIKey       string       `json:"api_key"`
	Model        string       `json:"model"`
	Priority     int          `json:"priority"` // lower = higher priority
	Capabilities []Capability `json:"capabilities"`
	Enabled      bool         `json:"enabled"`

	// Cooldown state
	mu            sync.RWMutex
	cooldownUntil time.Time
	failCount     int
	successCount  int
	totalLatency  time.Duration
	lastUsed      time.Time
}

// IsCoolingDown returns true if the endpoint is in cooldown.
func (e *Endpoint) IsCoolingDown() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return time.Now().Before(e.cooldownUntil)
}

// HasCapability checks if the endpoint supports a capability.
func (e *Endpoint) HasCapability(cap Capability) bool {
	for _, c := range e.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// RecordSuccess records a successful request.
func (e *Endpoint) RecordSuccess(latency time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.successCount++
	e.totalLatency += latency
	e.lastUsed = time.Now()
	e.failCount = 0 // reset consecutive failures
}

// RecordFailure records a failed request and applies cooldown.
func (e *Endpoint) RecordFailure() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failCount++
	e.lastUsed = time.Now()

	// Exponential backoff cooldown
	cooldown := cooldownDuration(e.failCount)
	e.cooldownUntil = time.Now().Add(cooldown)
	return cooldown
}

// Stats returns endpoint statistics.
func (e *Endpoint) Stats() EndpointStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := EndpointStats{
		ID:           e.ID,
		SuccessCount: e.successCount,
		FailCount:    e.failCount,
		CoolingDown:  time.Now().Before(e.cooldownUntil),
	}
	if e.successCount > 0 {
		s.AvgLatency = e.totalLatency / time.Duration(e.successCount)
	}
	if !e.lastUsed.IsZero() {
		t := e.lastUsed
		s.LastUsed = &t
	}
	return s
}

// EndpointStats holds endpoint performance metrics.
type EndpointStats struct {
	ID           string        `json:"id"`
	SuccessCount int           `json:"success_count"`
	FailCount    int           `json:"fail_count"`
	AvgLatency   time.Duration `json:"avg_latency"`
	CoolingDown  bool          `json:"cooling_down"`
	LastUsed     *time.Time    `json:"last_used,omitempty"`
}

func cooldownDuration(failCount int) time.Duration {
	switch {
	case failCount <= 1:
		return 30 * time.Second
	case failCount <= 3:
		return 2 * time.Minute
	case failCount <= 5:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}

// ──────────────────────────────────────────────
// Router — selects the best endpoint
// ──────────────────────────────────────────────

// EndpointRouter manages endpoint selection with cooldown and capability routing.
type EndpointRouter struct {
	mu        sync.RWMutex
	endpoints []*Endpoint
}

// NewEndpointRouter creates an endpoint router.
func NewEndpointRouter() *EndpointRouter {
	return &EndpointRouter{}
}

// AddEndpoint registers an endpoint.
func (r *EndpointRouter) AddEndpoint(ep *Endpoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.endpoints = append(r.endpoints, ep)
}

// RemoveEndpoint removes an endpoint by ID.
func (r *EndpointRouter) RemoveEndpoint(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, ep := range r.endpoints {
		if ep.ID == id {
			r.endpoints = append(r.endpoints[:i], r.endpoints[i+1:]...)
			return
		}
	}
}

// Select picks the best available endpoint for the required capabilities.
// Selection logic: enabled → not cooling down → has capabilities → lowest priority number.
func (r *EndpointRouter) Select(required ...Capability) (*Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidates []*Endpoint
	for _, ep := range r.endpoints {
		if !ep.Enabled {
			continue
		}
		if ep.IsCoolingDown() {
			continue
		}
		if !hasAllCaps(ep, required) {
			continue
		}
		candidates = append(candidates, ep)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("router: no available endpoint for capabilities %v", required)
	}

	// Sort by priority (lower = better)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority < candidates[j].Priority
	})

	return candidates[0], nil
}

// SelectWithFallback tries Select, if no result found, picks the endpoint
// with the earliest cooldown expiry (wait for it).
func (r *EndpointRouter) SelectWithFallback(required ...Capability) (*Endpoint, error) {
	ep, err := r.Select(required...)
	if err == nil {
		return ep, nil
	}

	// Find endpoint with earliest cooldown expiry
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best *Endpoint
	var earliest time.Time
	for _, ep := range r.endpoints {
		if !ep.Enabled || !hasAllCaps(ep, required) {
			continue
		}
		ep.mu.RLock()
		cu := ep.cooldownUntil
		ep.mu.RUnlock()
		if best == nil || cu.Before(earliest) {
			best = ep
			earliest = cu
		}
	}

	if best == nil {
		return nil, fmt.Errorf("router: no endpoint supports capabilities %v", required)
	}

	slog.Info("router: all endpoints cooling, using earliest available", "id", best.ID, "available_at", earliest)
	return best, nil
}

func hasAllCaps(ep *Endpoint, caps []Capability) bool {
	for _, c := range caps {
		if !ep.HasCapability(c) {
			return false
		}
	}
	return true
}

// AllStats returns stats for all endpoints.
func (r *EndpointRouter) AllStats() []EndpointStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EndpointStats, len(r.endpoints))
	for i, ep := range r.endpoints {
		out[i] = ep.Stats()
	}
	return out
}

// Endpoints returns all registered endpoints (shallow copy).
func (r *EndpointRouter) Endpoints() []*Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Endpoint, len(r.endpoints))
	copy(out, r.endpoints)
	return out
}

// ──────────────────────────────────────────────
// Dynamic model switch
// ──────────────────────────────────────────────

// ModelSwitch dynamically changes an endpoint's model.
func (r *EndpointRouter) ModelSwitch(endpointID, newModel string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ep := range r.endpoints {
		if ep.ID == endpointID {
			ep.mu.Lock()
			ep.Model = newModel
			ep.mu.Unlock()
			slog.Info("router: model switched", "endpoint", endpointID, "model", newModel)
			return nil
		}
	}
	return fmt.Errorf("router: endpoint %q not found", endpointID)
}

// ──────────────────────────────────────────────
// Execute helper
// ──────────────────────────────────────────────

// DoFunc is a function that uses an endpoint. Returns the result or error.
type DoFunc func(ctx context.Context, ep *Endpoint) error

// ExecuteWithRetry selects an endpoint and retries with fallback on failure.
func (r *EndpointRouter) ExecuteWithRetry(ctx context.Context, maxAttempts int, do DoFunc, caps ...Capability) error {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ep, err := r.Select(caps...)
		if err != nil {
			ep, err = r.SelectWithFallback(caps...)
			if err != nil {
				return err
			}
		}

		start := time.Now()
		err = do(ctx, ep)
		if err == nil {
			ep.RecordSuccess(time.Since(start))
			return nil
		}

		cd := ep.RecordFailure()
		slog.Warn("router: request failed, cooldown applied",
			"endpoint", ep.ID,
			"attempt", attempt,
			"cooldown", cd,
			"err", err,
		)
	}
	return fmt.Errorf("router: all %d attempts failed", maxAttempts)
}
