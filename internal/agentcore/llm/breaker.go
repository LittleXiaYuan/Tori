package llm

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open: LLM service unavailable")

type breakerState int

const (
	stateClosed   breakerState = iota // normal operation
	stateOpen                         // blocking requests
	stateHalfOpen                     // testing recovery
)

// CircuitBreaker implements the circuit breaker pattern for LLM calls.
// Closed → Open after failThreshold consecutive failures.
// Open → HalfOpen after resetTimeout.
// HalfOpen → Closed on success, Open on failure.
type CircuitBreaker struct {
	mu             sync.Mutex
	state          breakerState
	failures       int
	failThreshold  int
	resetTimeout   time.Duration
	lastFailure    time.Time
	halfOpenPassed int
	halfOpenMax    int // successes needed to close from half-open
}

// NewCircuitBreaker creates a circuit breaker.
// failThreshold: consecutive failures before opening.
// resetTimeout: time before attempting half-open recovery.
func NewCircuitBreaker(failThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failThreshold: failThreshold,
		resetTimeout:  resetTimeout,
		halfOpenMax:   2,
	}
}

// Allow checks if a request is allowed through.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		return nil
	case stateOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = stateHalfOpen
			cb.halfOpenPassed = 0
			return nil
		}
		return ErrCircuitOpen
	case stateHalfOpen:
		return nil
	}
	return nil
}

// RecordSuccess records a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	if cb.state == stateHalfOpen {
		cb.halfOpenPassed++
		if cb.halfOpenPassed >= cb.halfOpenMax {
			cb.state = stateClosed
		}
	}
}

// RecordFailure records a failed call.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()
	if cb.state == stateHalfOpen || cb.failures >= cb.failThreshold {
		cb.state = stateOpen
		cb.failures = 0
	}
}

// State returns the current state as a string.
func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case stateClosed:
		return "closed"
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	}
	return "unknown"
}

// Failures returns current consecutive failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}

// Reset forces the circuit breaker back to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = stateClosed
	cb.failures = 0
	cb.halfOpenPassed = 0
}
