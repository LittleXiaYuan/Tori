package multiagent

import (
	"log/slog"
	"sync"
)

// ──────────────────────────────────────────────
// MessageBus — in-process pub/sub for agent communication
//
// Each agent subscribes to messages addressed to its role ID.
// Broadcast messages (to="*") are delivered to all subscribers.
// The bus is in-process only (no network). For distributed agents,
// a future adapter can bridge to NATS/Redis Pub/Sub.
// ──────────────────────────────────────────────

// Handler processes a received message. Return a reply or nil.
type Handler func(msg Message) *Message

// Bus is an in-process message bus for inter-agent communication.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler // roleID → handlers
	history  []Message            // append-only message log
	maxHist  int
}

// NewBus creates a message bus.
func NewBus(maxHistory int) *Bus {
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	return &Bus{
		handlers: make(map[string][]Handler),
		maxHist:  maxHistory,
	}
}

// Subscribe registers a handler for messages addressed to the given role.
func (b *Bus) Subscribe(roleID string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[roleID] = append(b.handlers[roleID], handler)
}

// Publish sends a message. If to="*", it's broadcast to all.
// Returns any replies collected synchronously.
func (b *Bus) Publish(msg Message) []Message {
	b.mu.Lock()
	b.history = append(b.history, msg)
	if len(b.history) > b.maxHist {
		b.history = b.history[len(b.history)-b.maxHist:]
	}

	// Collect handlers to call
	var targets []Handler
	if msg.To == "*" {
		// Broadcast to all
		for _, handlers := range b.handlers {
			targets = append(targets, handlers...)
		}
	} else {
		targets = append(targets, b.handlers[msg.To]...)
	}
	b.mu.Unlock()

	var replies []Message
	for _, h := range targets {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("multiagent bus: handler panic", "error", r, "from", msg.From, "to", msg.To)
				}
			}()
			if reply := h(msg); reply != nil {
				b.mu.Lock()
				b.history = append(b.history, *reply)
				b.mu.Unlock()
				replies = append(replies, *reply)
			}
		}()
	}
	return replies
}

// History returns the recent message history.
func (b *Bus) History(limit int) []Message {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if limit <= 0 || limit > len(b.history) {
		limit = len(b.history)
	}
	out := make([]Message, limit)
	copy(out, b.history[len(b.history)-limit:])
	return out
}

// Clear resets the message history.
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.history = nil
}
