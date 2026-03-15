package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ResponseCache provides a short-TTL cache for LLM responses to avoid
// duplicate calls for identical message sequences within a short window.
type ResponseCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
	maxSize int
}

type cacheEntry struct {
	reply     string
	createdAt time.Time
	hits      int
}

// NewResponseCache creates a cache with the given TTL and max entries.
func NewResponseCache(ttl time.Duration, maxSize int) *ResponseCache {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	if maxSize <= 0 {
		maxSize = 256
	}
	c := &ResponseCache{
		entries: make(map[string]*cacheEntry, maxSize),
		ttl:     ttl,
		maxSize: maxSize,
	}
	go c.evictLoop()
	return c
}

// Get looks up a cached response for the given messages + temperature.
func (c *ResponseCache) Get(msgs []Message, temp float64) (string, bool) {
	key := c.key(msgs, temp)
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Since(e.createdAt) > c.ttl {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return "", false
	}
	c.mu.Lock()
	e.hits++
	c.mu.Unlock()
	return e.reply, true
}

// Put stores a response in the cache.
func (c *ResponseCache) Put(msgs []Message, temp float64, reply string) {
	key := c.key(msgs, temp)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &cacheEntry{
		reply:     reply,
		createdAt: time.Now(),
	}
}

// Stats returns cache statistics.
func (c *ResponseCache) Stats() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	totalHits := 0
	for _, e := range c.entries {
		totalHits += e.hits
	}
	return map[string]any{
		"size":       len(c.entries),
		"max_size":   c.maxSize,
		"ttl_sec":    c.ttl.Seconds(),
		"total_hits": totalHits,
	}
}

func (c *ResponseCache) key(msgs []Message, temp float64) string {
	h := sha256.New()
	for _, m := range msgs {
		h.Write([]byte(m.Role))
		h.Write([]byte(":"))
		h.Write([]byte(m.Content))
		h.Write([]byte("|"))
	}
	h.Write([]byte(fmt.Sprintf("t=%.2f", temp)))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (c *ResponseCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for k, e := range c.entries {
		if oldestKey == "" || e.createdAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.createdAt
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func (c *ResponseCache) evictLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, e := range c.entries {
			if now.Sub(e.createdAt) > c.ttl {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}

// CacheKeyPrefix returns a short prefix for logging.
func CacheKeyPrefix(msgs []Message) string {
	if len(msgs) == 0 {
		return "(empty)"
	}
	last := msgs[len(msgs)-1].Content
	if len(last) > 40 {
		runes := []rune(last)
		if len(runes) > 40 {
			last = string(runes[:40]) + "..."
		}
	}
	return strings.ReplaceAll(last, "\n", " ")
}
