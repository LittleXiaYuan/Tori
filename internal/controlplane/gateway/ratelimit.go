package gateway

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a per-tenant token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int // tokens per interval
	interval time.Duration
	stopCh   chan struct{} // signal to stop cleanup goroutine
}

type bucket struct {
	tokens   int
	lastFill time.Time
}

// NewRateLimiter creates a rate limiter allowing `rate` requests per `interval`.
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
	// Periodically clean up stale buckets to prevent memory leaks
	go rl.cleanup()
	return rl
}

// Stop halts the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	select {
	case <-rl.stopCh:
	default:
		close(rl.stopCh)
	}
}

// cleanup removes inactive buckets every 10 minutes.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-30 * time.Minute)
			for k, b := range rl.buckets {
				if b.lastFill.Before(cutoff) {
					delete(rl.buckets, k)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Allow checks if a request from the given key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: rl.rate, lastFill: time.Now()}
		rl.buckets[key] = b
	}

	// Refill tokens based on elapsed time
	elapsed := time.Since(b.lastFill)
	if elapsed >= rl.interval {
		periods := int(elapsed / rl.interval)
		b.tokens += periods * rl.rate
		if b.tokens > rl.rate {
			b.tokens = rl.rate
		}
		b.lastFill = b.lastFill.Add(time.Duration(periods) * rl.interval)
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

// Middleware wraps an http.HandlerFunc with rate limiting by tenant.
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := tenantFromCtx(r.Context())
		if key == "" {
			key = r.RemoteAddr
		}
		if !rl.Allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next(w, r)
	}
}
