package sandbox

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var ErrSandboxCircuitOpen = errors.New("sandbox circuit breaker open: backend unavailable")

type circuitState int

const (
	circuitClosed   circuitState = iota
	circuitOpen
	circuitHalfOpen
)

// CircuitConfig configures the sandbox circuit breaker.
type CircuitConfig struct {
	FailureThreshold int           // consecutive failures to trip open (default 3)
	ProbeInterval    time.Duration // wait before half-open probe (default 30s)
	HalfOpenMax      int           // successes needed to close (default 2)
}

func DefaultCircuitConfig() CircuitConfig {
	return CircuitConfig{
		FailureThreshold: 3,
		ProbeInterval:    30 * time.Second,
		HalfOpenMax:      2,
	}
}

type sandboxBreaker struct {
	mu            sync.Mutex
	state         circuitState
	failures      int
	lastFail      time.Time
	halfOpenCount int
	config        CircuitConfig

	totalTripped   int64
	totalRecovered int64
}

func newSandboxBreaker(cfg CircuitConfig) *sandboxBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 3
	}
	if cfg.ProbeInterval <= 0 {
		cfg.ProbeInterval = 30 * time.Second
	}
	if cfg.HalfOpenMax <= 0 {
		cfg.HalfOpenMax = 2
	}
	return &sandboxBreaker{config: cfg}
}

func (b *sandboxBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case circuitClosed:
		return true
	case circuitOpen:
		if time.Since(b.lastFail) > b.config.ProbeInterval {
			b.state = circuitHalfOpen
			b.halfOpenCount = 0
			return true
		}
		return false
	case circuitHalfOpen:
		return true
	}
	return true
}

func (b *sandboxBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	if b.state == circuitHalfOpen {
		b.halfOpenCount++
		if b.halfOpenCount >= b.config.HalfOpenMax {
			b.state = circuitClosed
			b.totalRecovered++
			slog.Info("sandbox breaker: recovered", "total_recovered", b.totalRecovered)
		}
	}
}

func (b *sandboxBreaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.lastFail = time.Now()
	if b.state == circuitHalfOpen || b.failures >= b.config.FailureThreshold {
		b.state = circuitOpen
		b.failures = 0
		b.totalTripped++
		slog.Warn("sandbox breaker: tripped open", "total_tripped", b.totalTripped)
	}
}

func (b *sandboxBreaker) stats() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := "closed"
	switch b.state {
	case circuitOpen:
		s = "open"
	case circuitHalfOpen:
		s = "half-open"
	}
	return map[string]any{
		"state":           s,
		"failures":        b.failures,
		"total_tripped":   b.totalTripped,
		"total_recovered": b.totalRecovered,
	}
}

// CircuitRunner wraps a Runner with a circuit breaker.
// When the breaker is open, Run returns ErrSandboxCircuitOpen immediately.
type CircuitRunner struct {
	inner   Runner
	breaker *sandboxBreaker
}

// NewCircuitRunner wraps a Runner with circuit breaker protection.
func NewCircuitRunner(inner Runner, cfg CircuitConfig) *CircuitRunner {
	return &CircuitRunner{
		inner:   inner,
		breaker: newSandboxBreaker(cfg),
	}
}

func (r *CircuitRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if !r.breaker.allow() {
		return nil, ErrSandboxCircuitOpen
	}
	result, err := r.inner.Run(ctx, req)
	if err != nil {
		r.breaker.recordFailure()
		return nil, err
	}
	r.breaker.recordSuccess()
	return result, nil
}

func (r *CircuitRunner) Type() string { return r.inner.Type() }

func (r *CircuitRunner) Close() error { return r.inner.Close() }

// BreakerStats returns circuit breaker state for observability.
func (r *CircuitRunner) BreakerStats() map[string]any { return r.breaker.stats() }
