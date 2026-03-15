package gateway

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a per-tenant token bucket rate limiter.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // tokens per interval
	interval time.Duration
}

type bucket struct {
	tokens    int
	lastFill  time.Time
}

// NewRateLimiter creates a rate limiter allowing `rate` requests per `interval`.
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
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
