package circuit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing, reject calls
	StateHalfOpen              // Testing recovery
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	}
	return "unknown"
}

// LLMCallFunc is the function signature for LLM calls.
type LLMCallFunc func(ctx context.Context, system, user string) (string, error)

// Breaker wraps an LLM call with circuit breaker pattern + automatic model fallback.
type Breaker struct {
	mu           sync.Mutex
	state        State
	failures     int
	successes    int
	lastFailure  time.Time
	threshold    int           // failures before opening
	recoveryTime time.Duration // how long to stay open before half-open
	halfOpenMax  int           // successes needed in half-open to close

	primary       LLMCallFunc
	fallbacks     []FallbackEntry
	cachedAnswers map[string]cachedAnswer // query hash -> cached response
}

// FallbackEntry defines a fallback model with its call function and label.
type FallbackEntry struct {
	Label string
	Call  LLMCallFunc
}

type cachedAnswer struct {
	response  string
	timestamp time.Time
}

// Config configures the circuit breaker.
type Config struct {
	FailureThreshold int           // failures before opening (default: 3)
	RecoveryTime     time.Duration // open duration before half-open (default: 30s)
	HalfOpenMax      int           // successes to close from half-open (default: 2)
}

// New creates a circuit breaker wrapping the primary LLM call.
func New(primary LLMCallFunc, cfg Config) *Breaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 3
	}
	if cfg.RecoveryTime <= 0 {
		cfg.RecoveryTime = 30 * time.Second
	}
	if cfg.HalfOpenMax <= 0 {
		cfg.HalfOpenMax = 2
	}
	return &Breaker{
		state:         StateClosed,
		threshold:     cfg.FailureThreshold,
		recoveryTime:  cfg.RecoveryTime,
		halfOpenMax:   cfg.HalfOpenMax,
		primary:       primary,
		cachedAnswers: make(map[string]cachedAnswer),
	}
}

// AddFallback adds a fallback LLM to try when the primary is down.
func (b *Breaker) AddFallback(label string, fn LLMCallFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fallbacks = append(b.fallbacks, FallbackEntry{Label: label, Call: fn})
}

// Call executes the LLM call with circuit breaker protection.
// Order: primary → fallbacks → cache → error
func (b *Breaker) Call(ctx context.Context, system, user string) (string, error) {
	b.mu.Lock()
	state := b.state

	// Check if open circuit should transition to half-open
	if state == StateOpen && time.Since(b.lastFailure) > b.recoveryTime {
		b.state = StateHalfOpen
		state = StateHalfOpen
		slog.Info("circuit breaker: open → half-open")
	}
	b.mu.Unlock()

	switch state {
	case StateClosed, StateHalfOpen:
		resp, err := b.primary(ctx, system, user)
		if err == nil {
			b.onSuccess(user, resp)
			return resp, nil
		}
		b.onFailure(err)

		// Try fallbacks
		for _, fb := range b.fallbacks {
			resp, err := fb.Call(ctx, system, user)
			if err == nil {
				slog.Info("circuit breaker: fallback succeeded", "fallback", fb.Label)
				b.cacheResponse(user, resp)
				return resp, nil
			}
		}

		// Try cache
		if cached, ok := b.getCached(user); ok {
			slog.Info("circuit breaker: serving cached response")
			return cached + "\n\n_(cached response — live service temporarily unavailable)_", nil
		}

		return "", fmt.Errorf("circuit breaker: all models failed: %w", err)

	case StateOpen:
		// Don't even try primary — go straight to fallbacks
		for _, fb := range b.fallbacks {
			resp, err := fb.Call(ctx, system, user)
			if err == nil {
				b.cacheResponse(user, resp)
				return resp, nil
			}
		}
		if cached, ok := b.getCached(user); ok {
			return cached + "\n\n_(cached response — live service temporarily unavailable)_", nil
		}
		return "", fmt.Errorf("circuit breaker open: primary unavailable, no fallbacks available")
	}

	return "", fmt.Errorf("circuit breaker: unknown state")
}

// State returns the current circuit state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Stats returns breaker stats.
func (b *Breaker) Stats() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	return map[string]any{
		"state":        b.state.String(),
		"failures":     b.failures,
		"successes":    b.successes,
		"last_failure": b.lastFailure,
		"cached_items": len(b.cachedAnswers),
		"fallbacks":    len(b.fallbacks),
	}
}

// Reset manually resets the breaker to closed state.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = StateClosed
	b.failures = 0
	b.successes = 0
	slog.Info("circuit breaker: manually reset to closed")
}

func (b *Breaker) onSuccess(query, response string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.successes++
	b.cacheResponseLocked(query, response)

	if b.state == StateHalfOpen && b.successes >= b.halfOpenMax {
		b.state = StateClosed
		b.successes = 0
		slog.Info("circuit breaker: half-open → closed (recovered)")
	}
}

func (b *Breaker) onFailure(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.successes = 0
	b.lastFailure = time.Now()
	slog.Warn("circuit breaker: failure", "count", b.failures, "threshold", b.threshold, "err", err)

	if b.state == StateClosed && b.failures >= b.threshold {
		b.state = StateOpen
		slog.Warn("circuit breaker: closed → open (tripped)")
	} else if b.state == StateHalfOpen {
		b.state = StateOpen
		slog.Warn("circuit breaker: half-open → open (still failing)")
	}
}

func (b *Breaker) cacheResponse(query, response string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cacheResponseLocked(query, response)
}

func (b *Breaker) cacheResponseLocked(query, response string) {
	// Simple LRU: keep max 100 entries
	if len(b.cachedAnswers) > 100 {
		var oldest string
		var oldestTime time.Time
		for k, v := range b.cachedAnswers {
			if oldest == "" || v.timestamp.Before(oldestTime) {
				oldest = k
				oldestTime = v.timestamp
			}
		}
		delete(b.cachedAnswers, oldest)
	}
	key := simpleHash(query)
	b.cachedAnswers[key] = cachedAnswer{response: response, timestamp: time.Now()}
}

func (b *Breaker) getCached(query string) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := simpleHash(query)
	c, ok := b.cachedAnswers[key]
	if !ok {
		return "", false
	}
	// Expire after 1 hour
	if time.Since(c.timestamp) > time.Hour {
		delete(b.cachedAnswers, key)
		return "", false
	}
	return c.response, true
}

func simpleHash(s string) string {
	// FNV-like fast hash for cache keys
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("%x", h)
}
