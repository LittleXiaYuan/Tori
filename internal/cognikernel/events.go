package cognikernel

import (
	"sync"
	"time"
)

// EventType classifies kernel-level events.
type EventType string

const (
	EventConversationEnded EventType = "conversation_ended"
	EventReflectCompleted  EventType = "reflect_completed"
	EventIdleDetected      EventType = "idle_detected"
	EventDreamCompleted    EventType = "dream_completed"
	EventSecurityAlert     EventType = "security_alert"
	EventExperienceAdded   EventType = "experience_added"
	EventSkillGrown        EventType = "skill_grown"
)

// Event is a kernel-level event that flows between loops.
type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// ConversationEndData carries context from a completed conversation
// into the reflective loop.
type ConversationEndData struct {
	TenantID   string   `json:"tenant_id"`
	SessionID  string   `json:"session_id"`
	UserIntent string   `json:"user_intent"`
	AgentReply string   `json:"agent_reply"`
	SkillsUsed []string `json:"skills_used"`
	ModelTier  string   `json:"model_tier"`
	TaskID     string   `json:"task_id,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// IdleData carries context for idle-triggered events.
type IdleData struct {
	TenantID string `json:"tenant_id"`
}

// ReflectResult is the output of a reflective loop cycle.
type ReflectResult struct {
	Satisfied        bool    `json:"satisfied"`
	Quality          int     `json:"quality"`
	ExperiencesAdded int     `json:"experiences_added"`
	DistilledRules   int     `json:"distilled_rules"`
	MemoryUpdates    int     `json:"memory_updates"`
	Score            float64 `json:"score"`
}

// DreamResult is the output of a dreaming loop cycle.
type DreamResult struct {
	ThoughtsGenerated int `json:"thoughts_generated"`
	ExplorationsRun   int `json:"explorations_run"`
	SkillsSuggested   int `json:"skills_suggested"`
	FactsDiscovered   int `json:"facts_discovered"`
}

// EventHandler processes a kernel event.
type EventHandler func(Event)

// EventBus is a publish-subscribe event bus for kernel-internal communication.
// It connects the three loops and the immune system.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]EventHandler
	buffer      []Event
	bufferSize  int
}

// NewEventBus creates an event bus with a ring buffer of the given size.
func NewEventBus(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 128
	}
	return &EventBus{
		subscribers: make(map[EventType][]EventHandler),
		buffer:      make([]Event, 0, bufferSize),
		bufferSize:  bufferSize,
	}
}

// Subscribe registers a handler for the given event type.
func (b *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

// Publish dispatches an event to all subscribers of its type.
// Events are also appended to the ring buffer for diagnostic inspection.
// Handlers are called with a defensive copy of the slice and panic recovery
// so a misbehaving handler never crashes the bus or corrupts state.
func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	src := b.subscribers[event.Type]
	handlers := make([]EventHandler, len(src))
	copy(handlers, src)
	b.mu.RUnlock()

	// Buffer for diagnostics
	b.mu.Lock()
	b.buffer = append(b.buffer, event)
	if len(b.buffer) > b.bufferSize {
		b.buffer = b.buffer[len(b.buffer)-b.bufferSize:]
	}
	b.mu.Unlock()

	for _, h := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log but don't propagate — one panicking handler must not
					// prevent other handlers from running.
					_ = r // logged by caller or slog in production
				}
			}()
			h(event)
		}()
	}
}

// hasSubscribers returns true if at least one handler is registered for the type.
func (b *EventBus) hasSubscribers(eventType EventType) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[eventType]) > 0
}

// RecentEvents returns the last N events from the ring buffer.
func (b *EventBus) RecentEvents(n int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n <= 0 || n > len(b.buffer) {
		n = len(b.buffer)
	}
	start := len(b.buffer) - n
	out := make([]Event, n)
	copy(out, b.buffer[start:])
	return out
}
