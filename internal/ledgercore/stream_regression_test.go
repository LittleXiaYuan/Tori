package ledger

// In-package regression tests for the deep-scan stream fixes (commit
// e4b55801): idempotent Close, superseded-subscription release, accept-gated
// dedup marking, and the seen-cache hard cap.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

type recordingTransport struct {
	mu           sync.Mutex
	nextID       int
	handlers     map[string]func([]byte)
	unsubscribed []string
	closed       int
}

func newRecordingTransport() *recordingTransport {
	return &recordingTransport{handlers: make(map[string]func([]byte))}
}

func (rt *recordingTransport) Publish(context.Context, string, []byte) error { return nil }

func (rt *recordingTransport) Subscribe(_ context.Context, _ string, handler func(data []byte)) (string, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.nextID++
	id := fmt.Sprintf("sub-%d", rt.nextID)
	rt.handlers[id] = handler
	return id, nil
}

func (rt *recordingTransport) Unsubscribe(_ context.Context, subID string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	delete(rt.handlers, subID)
	rt.unsubscribed = append(rt.unsubscribed, subID)
	return nil
}

func (rt *recordingTransport) Close() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.closed++
	return nil
}

func (rt *recordingTransport) activeHandlers() []func([]byte) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := make([]func([]byte), 0, len(rt.handlers))
	for _, h := range rt.handlers {
		out = append(out, h)
	}
	return out
}

// Close used to close stopCh unconditionally; a second Close panicked.
func TestEventStreamCloseIsIdempotent(t *testing.T) {
	rt := newRecordingTransport()
	es := NewEventStream(NewEventBus(), EventStreamConfig{InstanceID: "local", Transport: rt})

	es.Close()
	es.Close() // pre-fix: panic (close of closed channel)

	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.closed != 1 {
		t.Fatalf("transport.Close called %d times, want exactly 1", rt.closed)
	}
}

// Re-subscribing the same filter used to overwrite the sub ID without
// releasing the old transport subscription — its handler kept firing
// (duplicate delivery) and leaked.
func TestEventStreamResubscribeReleasesOldSubscription(t *testing.T) {
	rt := newRecordingTransport()
	es := NewEventStream(NewEventBus(), EventStreamConfig{InstanceID: "local", Transport: rt})
	defer es.Close()

	ctx := context.Background()
	filter := EventFilter{TaskIDs: []string{"task-1"}}
	if err := es.SubscribeRemote(ctx, filter); err != nil {
		t.Fatalf("SubscribeRemote: %v", err)
	}
	if err := es.SubscribeRemote(ctx, filter); err != nil {
		t.Fatalf("SubscribeRemote again: %v", err)
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.unsubscribed) != 1 || rt.unsubscribed[0] != "sub-1" {
		t.Fatalf("superseded subscription not released: unsubscribed=%v", rt.unsubscribed)
	}
	if len(rt.handlers) != 1 {
		t.Fatalf("%d transport handlers still registered, want 1", len(rt.handlers))
	}
}

func deliverEnvelope(t *testing.T, handler func([]byte), env StreamEnvelope) {
	t.Helper()
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	handler(data)
}

// Events rejected by the per-instance seq check used to be marked seen first,
// so a later legitimate re-delivery was dropped as a duplicate. Seen must be
// recorded only for accepted events.
func TestEventStreamMarksSeenOnlyAfterAccept(t *testing.T) {
	rt := newRecordingTransport()
	es := NewEventStream(NewEventBus(), EventStreamConfig{InstanceID: "local", Transport: rt})
	defer es.Close()

	if err := es.SubscribeRemote(context.Background(), EventFilter{}); err != nil {
		t.Fatalf("SubscribeRemote: %v", err)
	}
	handlers := rt.activeHandlers()
	if len(handlers) != 1 {
		t.Fatalf("want 1 handler, got %d", len(handlers))
	}
	handler := handlers[0]

	// Accepted: first event for (remote, task-1) with seq 2.
	deliverEnvelope(t, handler, StreamEnvelope{
		InstanceID: "remote", EventID: "evt-accepted", TaskID: "task-1", Seq: 2,
	})
	// Rejected by ordering: seq 1 <= lastSeq 2.
	deliverEnvelope(t, handler, StreamEnvelope{
		InstanceID: "remote", EventID: "evt-rejected", TaskID: "task-1", Seq: 1,
	})

	es.seenMu.Lock()
	defer es.seenMu.Unlock()
	if _, ok := es.seen["evt-accepted"]; !ok {
		t.Fatal("accepted event must be marked seen")
	}
	if _, ok := es.seen["evt-rejected"]; ok {
		t.Fatal("order-rejected event must NOT be marked seen (would block re-delivery)")
	}
}

// The dedup cache only ran a TTL sweep on overflow; with all entries inside
// the TTL window it grew without bound. The hard cap must hold regardless.
func TestEventStreamSeenCacheHardCap(t *testing.T) {
	es := NewEventStream(NewEventBus(), EventStreamConfig{
		InstanceID: "local",
		MaxSeen:    5,
		SeenTTL:    time.Hour, // nothing is TTL-expired during the test
	})
	defer es.Close()

	for i := 0; i < 20; i++ {
		es.markSeen(fmt.Sprintf("evt-%d", i))
	}

	es.seenMu.Lock()
	defer es.seenMu.Unlock()
	if len(es.seen) > 5 {
		t.Fatalf("seen cache grew to %d entries, hard cap is 5", len(es.seen))
	}
}
