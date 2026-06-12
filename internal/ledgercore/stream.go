package ledger

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// ── Distributed Event Stream ────────────────────────────────────────────────
// Provides event externalization, cross-instance subscription, and
// ordered delivery guarantees on top of the local EventBus.

// StreamTransport is an interface for external message brokers.
// Implementations can wrap NATS, Redis Streams, Kafka, etc.
type StreamTransport interface {
	// Publish sends an event to the external stream.
	Publish(ctx context.Context, topic string, data []byte) error

	// Subscribe registers a handler for incoming events on a topic pattern.
	// Returns a subscription ID that can be used to Unsubscribe.
	Subscribe(ctx context.Context, topic string, handler func(data []byte)) (string, error)

	// Unsubscribe removes a subscription.
	Unsubscribe(ctx context.Context, subID string) error

	// Close releases resources.
	Close() error
}

// EventStream extends EventBus with distributed capabilities.
type EventStream struct {
	bus        *EventBus
	transport  StreamTransport
	instanceID string
	prefix     string // topic prefix, e.g. "ledger.events"

	mu           sync.RWMutex
	externalSubs map[string]string // filter description ???transport sub ID

	// Deduplication: track recently seen event IDs to avoid reprocessing
	seen    map[string]time.Time
	seenMu  sync.Mutex
	maxSeen int
	seenTTL time.Duration

	// Sequence tracking per remote instance for ordering
	remoteSeqs   map[string]map[string]int64 // instanceID ???taskID ???lastSeq
	remoteSeqsMu sync.RWMutex

	stopCh   chan struct{}
	stopOnce sync.Once
}

// EventStreamConfig configures the distributed event stream.
type EventStreamConfig struct {
	InstanceID string          // Unique ID for this Ledger instance
	Transport  StreamTransport // External transport (nil = local-only)
	Prefix     string          // Topic prefix (default: "ledger.events")
	MaxSeen    int             // Dedup cache size (default: 10000)
	SeenTTL    time.Duration   // Dedup cache TTL (default: 5m)
}

// StreamEnvelope wraps an event with routing metadata for cross-instance delivery.
type StreamEnvelope struct {
	InstanceID string `json:"instance_id"`
	EventID    string `json:"event_id"`
	TaskID     string `json:"task_id"`
	Seq        int64  `json:"seq"`
	Kind       string `json:"kind"`
	Timestamp  int64  `json:"ts"`
	Event      *Event `json:"event"`
}

// NewEventStream creates a distributed event stream.
func NewEventStream(bus *EventBus, cfg EventStreamConfig) *EventStream {
	if cfg.Prefix == "" {
		cfg.Prefix = "ledger.events"
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID = ulid.New()
	}
	if cfg.MaxSeen == 0 {
		cfg.MaxSeen = 10000
	}
	if cfg.SeenTTL == 0 {
		cfg.SeenTTL = 5 * time.Minute
	}

	es := &EventStream{
		bus:          bus,
		transport:    cfg.Transport,
		instanceID:   cfg.InstanceID,
		prefix:       cfg.Prefix,
		externalSubs: make(map[string]string),
		seen:         make(map[string]time.Time),
		maxSeen:      cfg.MaxSeen,
		seenTTL:      cfg.SeenTTL,
		remoteSeqs:   make(map[string]map[string]int64),
		stopCh:       make(chan struct{}),
	}

	// Start dedup cleanup goroutine
	go es.cleanupLoop()

	return es
}

// PublishAndBroadcast publishes an event locally and to external transport.
func (es *EventStream) PublishAndBroadcast(ctx context.Context, e *Event) error {
	// Local publish
	es.bus.Publish(e)

	// External broadcast (if transport configured)
	if es.transport == nil {
		return nil
	}

	envelope := StreamEnvelope{
		InstanceID: es.instanceID,
		EventID:    e.ID,
		TaskID:     e.TaskID,
		Seq:        e.Seq,
		Kind:       string(e.Kind),
		Timestamp:  e.CreatedAt.UnixMilli(),
		Event:      e,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	topic := es.topicFor(e)
	return es.transport.Publish(ctx, topic, data)
}

// SubscribeRemote subscribes to events from remote Ledger instances.
// Events are injected into the local EventBus after deduplication.
func (es *EventStream) SubscribeRemote(ctx context.Context, filter EventFilter) error {
	if es.transport == nil {
		return nil
	}

	topic := es.filterToTopic(filter)

	subID, err := es.transport.Subscribe(ctx, topic, func(data []byte) {
		var envelope StreamEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return
		}

		// Skip own events
		if envelope.InstanceID == es.instanceID {
			return
		}

		// Dedup
		if es.isDuplicate(envelope.EventID) {
			return
		}

		// Check sequence ordering. Mark seen only after the event is
		// accepted: marking a rejected out-of-order event would make any
		// later re-delivery look like a duplicate and drop it permanently.
		if !es.checkAndUpdateSeq(envelope.InstanceID, envelope.TaskID, envelope.Seq) {
			return // out-of-order, skip (or could buffer)
		}
		es.markSeen(envelope.EventID)

		// Inject into local bus
		if envelope.Event != nil && matchesFilter(envelope.Event, &filter) {
			es.bus.Publish(envelope.Event)
		}
	})
	if err != nil {
		return err
	}

	es.mu.Lock()
	old, exists := es.externalSubs[topic]
	es.externalSubs[topic] = subID
	es.mu.Unlock()

	// Drop the superseded transport subscription, otherwise its handler keeps
	// firing alongside the new one (duplicate delivery + leak).
	if exists && old != subID {
		_ = es.transport.Unsubscribe(ctx, old)
	}

	return nil
}

// UnsubscribeAll removes all external subscriptions.
func (es *EventStream) UnsubscribeAll(ctx context.Context) {
	es.mu.Lock()
	defer es.mu.Unlock()

	for topic, subID := range es.externalSubs {
		if es.transport != nil {
			es.transport.Unsubscribe(ctx, subID)
		}
		delete(es.externalSubs, topic)
	}
}

// Close shuts down the event stream. Safe to call multiple times.
func (es *EventStream) Close() {
	es.stopOnce.Do(func() {
		close(es.stopCh)
		es.UnsubscribeAll(context.Background())
		if es.transport != nil {
			es.transport.Close()
		}
	})
}

// InstanceID returns this stream's instance identifier.
func (es *EventStream) InstanceID() string {
	return es.instanceID
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (es *EventStream) topicFor(e *Event) string {
	if e.TaskID != "" {
		return es.prefix + ".task." + e.TaskID
	}
	return es.prefix + ".global"
}

func (es *EventStream) filterToTopic(f EventFilter) string {
	if len(f.TaskIDs) == 1 {
		return es.prefix + ".task." + f.TaskIDs[0]
	}
	return es.prefix + ".>" // wildcard for all events
}

func (es *EventStream) isDuplicate(eventID string) bool {
	es.seenMu.Lock()
	defer es.seenMu.Unlock()
	_, exists := es.seen[eventID]
	return exists
}

func (es *EventStream) markSeen(eventID string) {
	es.seenMu.Lock()
	defer es.seenMu.Unlock()
	es.seen[eventID] = time.Now()

	// Evict if too large: TTL pass first, then hard-cap by evicting the
	// oldest entries so the cache stays bounded even when everything is
	// within the TTL window.
	if len(es.seen) > es.maxSeen {
		cutoff := time.Now().Add(-es.seenTTL)
		for id, ts := range es.seen {
			if ts.Before(cutoff) {
				delete(es.seen, id)
			}
		}
		for len(es.seen) > es.maxSeen {
			var oldestID string
			var oldestTS time.Time
			for id, ts := range es.seen {
				if oldestID == "" || ts.Before(oldestTS) {
					oldestID, oldestTS = id, ts
				}
			}
			delete(es.seen, oldestID)
		}
	}
}

func (es *EventStream) checkAndUpdateSeq(instanceID, taskID string, seq int64) bool {
	es.remoteSeqsMu.Lock()
	defer es.remoteSeqsMu.Unlock()

	if _, ok := es.remoteSeqs[instanceID]; !ok {
		es.remoteSeqs[instanceID] = make(map[string]int64)
	}
	lastSeq := es.remoteSeqs[instanceID][taskID]
	if seq <= lastSeq {
		return false // already processed or out of order
	}
	es.remoteSeqs[instanceID][taskID] = seq
	return true
}

func (es *EventStream) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-es.stopCh:
			return
		case <-ticker.C:
			es.seenMu.Lock()
			cutoff := time.Now().Add(-es.seenTTL)
			for id, ts := range es.seen {
				if ts.Before(cutoff) {
					delete(es.seen, id)
				}
			}
			es.seenMu.Unlock()
		}
	}
}
