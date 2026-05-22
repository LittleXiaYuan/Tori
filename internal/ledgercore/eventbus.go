package ledger

import (
	"sync"
	"sync/atomic"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// EventBus provides in-memory pub/sub for real-time event streaming.
// Subscribers receive events matching their filter as they are appended.
//
// Thread-safe for concurrent Publish/Subscribe/Unsubscribe.
type EventBus struct {
	mu      sync.RWMutex
	subs    map[string]*Subscription
	dropped atomic.Int64
}

// EventFilter specifies which events a subscription cares about.
// All non-empty fields must match (AND logic). Empty fields match everything.
type EventFilter struct {
	TaskIDs   []string    // only events for these tasks (empty = all)
	Kinds     []EventKind // only these event kinds (empty = all)
	Actors    []string    // only from these actors (empty = all)
	Reasoning bool        // if true, only reasoning.* events
}

// Subscription represents an active event stream.
type Subscription struct {
	ID     string
	Filter EventFilter
	C      <-chan *Event // read-only channel for the consumer
	ch     chan *Event   // internal writable channel
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[string]*Subscription)}
}

// Subscribe creates a new subscription. The returned channel receives events
// matching the filter. Buffer size determines how many events can queue before
// slow consumers start missing events (dropped, not blocked).
func (eb *EventBus) Subscribe(filter EventFilter, bufSize int) *Subscription {
	if bufSize < 1 {
		bufSize = 64
	}
	ch := make(chan *Event, bufSize)
	sub := &Subscription{
		ID:     ulid.New(),
		Filter: filter,
		C:      ch,
		ch:     ch,
	}
	eb.mu.Lock()
	eb.subs[sub.ID] = sub
	eb.mu.Unlock()
	return sub
}

// Unsubscribe removes a subscription and closes its channel.
func (eb *EventBus) Unsubscribe(sub *Subscription) {
	eb.mu.Lock()
	if _, ok := eb.subs[sub.ID]; ok {
		delete(eb.subs, sub.ID)
		close(sub.ch)
	}
	eb.mu.Unlock()
}

// Publish broadcasts an event to all matching subscribers.
// Non-blocking: if a subscriber's buffer is full, the event is dropped for that subscriber.
func (eb *EventBus) Publish(e *Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, sub := range eb.subs {
		if matchesFilter(e, &sub.Filter) {
			select {
			case sub.ch <- e:
			default:
				eb.dropped.Add(1)
			}
		}
	}
}

// DroppedCount returns the total number of events dropped due to slow subscribers.
func (eb *EventBus) DroppedCount() int64 {
	return eb.dropped.Load()
}

// SubscriberCount returns the number of active subscriptions.
func (eb *EventBus) SubscriberCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.subs)
}

// Close removes all subscriptions and closes their channels.
func (eb *EventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for id, sub := range eb.subs {
		close(sub.ch)
		delete(eb.subs, id)
	}
}

func matchesFilter(e *Event, f *EventFilter) bool {
	if f.Reasoning && !IsReasoningEvent(e.Kind) {
		return false
	}

	if len(f.TaskIDs) > 0 && !containsStr(f.TaskIDs, e.TaskID) {
		return false
	}
	if len(f.Kinds) > 0 && !containsKind(f.Kinds, e.Kind) {
		return false
	}
	if len(f.Actors) > 0 && !containsStr(f.Actors, e.Actor) {
		return false
	}
	return true
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func containsKind(kinds []EventKind, k EventKind) bool {
	for _, v := range kinds {
		if v == k {
			return true
		}
	}
	return false
}
