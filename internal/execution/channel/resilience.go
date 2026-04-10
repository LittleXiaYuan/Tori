package channel

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Channel Resilience — circuit breaker + rate limiter + health monitor
//
// Wraps any Channel with:
//   - Circuit breaker: open after N consecutive failures, half-open after cooldown
//   - Rate limiter:    sliding window QoS per-channel
//   - Health monitor:  periodic health checks, metrics collection
//   - Message queue:   offline message buffer for transient failures
// ──────────────────────────────────────────────

// CircuitState represents the circuit breaker state.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"    // healthy, operating normally
	CircuitOpen     CircuitState = "open"      // unhealthy, rejecting calls
	CircuitHalfOpen CircuitState = "half_open" // testing recovery
)

// CircuitBreaker protects a channel from cascading failures.
type CircuitBreaker struct {
	mu             sync.Mutex
	state          CircuitState
	failures       int           // consecutive failures
	maxFailures    int           // threshold to open
	cooldown       time.Duration // time before trying half-open
	lastFailure    time.Time
	successesInHO  int           // successes needed in half-open to close
	hoSuccessCount int
}

// NewCircuitBreaker creates a breaker with sensible defaults.
func NewCircuitBreaker(maxFailures int, cooldown time.Duration) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitBreaker{
		state:         CircuitClosed,
		maxFailures:   maxFailures,
		cooldown:      cooldown,
		successesInHO: 2,
	}
}

// Allow checks if the circuit allows a call through.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) >= cb.cooldown {
			cb.state = CircuitHalfOpen
			cb.hoSuccessCount = 0
			slog.Info("circuit_breaker: half-open")
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

// RecordSuccess records a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	if cb.state == CircuitHalfOpen {
		cb.hoSuccessCount++
		if cb.hoSuccessCount >= cb.successesInHO {
			cb.state = CircuitClosed
			slog.Info("circuit_breaker: closed (recovered)")
		}
	}
}

// RecordFailure records a failed call.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
		slog.Warn("circuit_breaker: open (half-open failed)")
	} else if cb.failures >= cb.maxFailures {
		cb.state = CircuitOpen
		slog.Warn("circuit_breaker: open", "failures", cb.failures)
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState { return cb.state }

// ──────────────────────────────────────────────
// RateLimiter — sliding window per-channel
// ──────────────────────────────────────────────

// RateLimiter limits message send rate per channel.
type RateLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	maxCalls int
	calls    []time.Time
}

// NewRateLimiter creates a limiter (e.g., 30 calls per minute).
func NewRateLimiter(maxCalls int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		window:   window,
		maxCalls: maxCalls,
	}
}

// Allow returns true if the call is within rate limits.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Prune old entries
	valid := rl.calls[:0]
	for _, t := range rl.calls {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.calls = valid

	if len(rl.calls) >= rl.maxCalls {
		return false
	}
	rl.calls = append(rl.calls, now)
	return true
}

// ──────────────────────────────────────────────
// ChannelHealth — per-channel metrics
// ──────────────────────────────────────────────

// ChannelHealth tracks health metrics for a channel.
type ChannelHealth struct {
	mu           sync.RWMutex
	ChannelType  string       `json:"channel_type"`
	State        CircuitState `json:"state"`
	MessagesSent int64        `json:"messages_sent"`
	MessagesRecv int64        `json:"messages_recv"`
	Errors       int64        `json:"errors"`
	LastError    string       `json:"last_error,omitempty"`
	LastActive   time.Time    `json:"last_active"`
	Latency      time.Duration `json:"latency_ms"` // average send latency
	latencySum   time.Duration
	latencyCount int64
}

func newChannelHealth(channelType string) *ChannelHealth {
	return &ChannelHealth{
		ChannelType: channelType,
		State:       CircuitClosed,
		LastActive:  time.Now(),
	}
}

func (h *ChannelHealth) recordSend(latency time.Duration, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		h.Errors++
		h.LastError = err.Error()
	} else {
		h.MessagesSent++
		h.latencySum += latency
		h.latencyCount++
		if h.latencyCount > 0 {
			h.Latency = h.latencySum / time.Duration(h.latencyCount)
		}
	}
	h.LastActive = time.Now()
}

func (h *ChannelHealth) recordRecv() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.MessagesRecv++
	h.LastActive = time.Now()
}

func (h *ChannelHealth) updateState(state CircuitState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.State = state
}

// Snapshot returns a copy of the health metrics.
func (h *ChannelHealth) Snapshot() ChannelHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return ChannelHealth{
		ChannelType:  h.ChannelType,
		State:        h.State,
		MessagesSent: h.MessagesSent,
		MessagesRecv: h.MessagesRecv,
		Errors:       h.Errors,
		LastError:    h.LastError,
		LastActive:   h.LastActive,
		Latency:      h.Latency,
	}
}

// ──────────────────────────────────────────────
// ResilientChannel — wraps a Channel with resilience
// ──────────────────────────────────────────────

// ResilientChannel wraps a Channel with circuit breaker, rate limiting, and health tracking.
type ResilientChannel struct {
	inner   Channel
	breaker *CircuitBreaker
	limiter *RateLimiter
	health  *ChannelHealth
	queue   *OfflineQueue // buffered messages when circuit is open
}

// NewResilientChannel creates a resilient wrapper around any Channel.
func NewResilientChannel(ch Channel, maxFailures int, rateLimit int) *ResilientChannel {
	return &ResilientChannel{
		inner:   ch,
		breaker: NewCircuitBreaker(maxFailures, 30*time.Second),
		limiter: NewRateLimiter(rateLimit, time.Minute),
		health:  newChannelHealth(ch.Type()),
		queue:   NewOfflineQueue(1000),
	}
}

func (rc *ResilientChannel) Type() string { return rc.inner.Type() }

func (rc *ResilientChannel) Start(ctx context.Context, handler func(Message) Reply) error {
	// Wrap handler with recv metrics
	wrapped := func(msg Message) Reply {
		rc.health.recordRecv()
		return handler(msg)
	}
	return rc.inner.Start(ctx, wrapped)
}

func (rc *ResilientChannel) Send(ctx context.Context, target string, reply Reply) error {
	if !rc.breaker.Allow() {
		// Buffer the message for retry later
		rc.queue.Push(target, reply)
		return nil // silently queued
	}

	if !rc.limiter.Allow() {
		rc.queue.Push(target, reply)
		return nil
	}

	start := time.Now()
	err := rc.inner.Send(ctx, target, reply)
	latency := time.Since(start)

	if err != nil {
		rc.breaker.RecordFailure()
		rc.health.recordSend(latency, err)
		rc.health.updateState(rc.breaker.State())
		rc.queue.Push(target, reply) // buffer for retry
		return err
	}

	rc.breaker.RecordSuccess()
	rc.health.recordSend(latency, nil)
	rc.health.updateState(rc.breaker.State())
	return nil
}

// Health returns current health metrics.
func (rc *ResilientChannel) Health() ChannelHealth { return rc.health.Snapshot() }

// DrainQueue retries buffered messages. Call periodically.
func (rc *ResilientChannel) DrainQueue(ctx context.Context) int {
	if !rc.breaker.Allow() {
		return 0
	}
	sent := 0
	for {
		target, reply, ok := rc.queue.Pop()
		if !ok {
			break
		}
		if err := rc.inner.Send(ctx, target, reply); err != nil {
			rc.breaker.RecordFailure()
			rc.queue.Push(target, reply) // put back
			break
		}
		rc.breaker.RecordSuccess()
		sent++
	}
	return sent
}

// ──────────────────────────────────────────────
// OfflineQueue — buffer for failed sends
// ──────────────────────────────────────────────

type queuedMessage struct {
	Target  string
	Reply   Reply
	QueuedAt time.Time
}

// OfflineQueue buffers messages when a channel is unavailable.
type OfflineQueue struct {
	mu      sync.Mutex
	items   []queuedMessage
	maxSize int
}

// NewOfflineQueue creates a bounded offline message buffer.
func NewOfflineQueue(maxSize int) *OfflineQueue {
	return &OfflineQueue{maxSize: maxSize}
}

// Push adds a message to the queue.
func (q *OfflineQueue) Push(target string, reply Reply) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) >= q.maxSize {
		q.items = q.items[1:] // drop oldest
	}
	q.items = append(q.items, queuedMessage{
		Target:   target,
		Reply:    reply,
		QueuedAt: time.Now(),
	})
}

// Pop removes and returns the oldest queued message.
func (q *OfflineQueue) Pop() (target string, reply Reply, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return "", Reply{}, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item.Target, item.Reply, true
}

// Len returns the queue depth.
func (q *OfflineQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
