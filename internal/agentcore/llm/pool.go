package llm

import (
	"fmt"
	"log/slog"
	"sync"
)

// Pool manages multiple LLM clients for different model tiers.
// Thread-safe: clients can be registered and retrieved concurrently.
type Pool struct {
	mu      sync.RWMutex
	clients map[string]*Client // tier name or model ID → client
	primary string             // key of the primary/default client
}

// NewPool creates an empty LLM pool.
func NewPool() *Pool {
	return &Pool{
		clients: make(map[string]*Client),
	}
}

// Register adds a client under the given key (e.g. "fast", "smart", "expert").
// The first registered client becomes the primary by default.
func (p *Pool) Register(key string, client *Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clients[key] = client
	if p.primary == "" {
		p.primary = key
	}
	slog.Info("llm pool: registered", "key", key, "model", client.Model())
}

// SetPrimary designates which key is the primary/fallback client.
func (p *Pool) SetPrimary(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.primary = key
}

// Get retrieves a client by key. Returns nil if not found.
func (p *Pool) Get(key string) *Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.clients[key]
}

// GetOrFallback retrieves a client by key, falling back to primary if not found.
func (p *Pool) GetOrFallback(key string) *Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if c, ok := p.clients[key]; ok {
		return c
	}
	return p.clients[p.primary]
}

// Primary returns the primary/default client.
func (p *Pool) Primary() *Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.clients[p.primary]
}

// Has returns true if a client is registered under the given key.
func (p *Pool) Has(key string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.clients[key]
	return ok
}

// Keys returns all registered keys.
func (p *Pool) Keys() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	keys := make([]string, 0, len(p.clients))
	for k := range p.clients {
		keys = append(keys, k)
	}
	return keys
}

// Size returns the number of registered clients.
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clients)
}

// String returns a summary of the pool for logging.
func (p *Pool) String() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return fmt.Sprintf("Pool{size=%d, primary=%q}", len(p.clients), p.primary)
}
