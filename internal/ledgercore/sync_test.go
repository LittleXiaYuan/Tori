package ledger

import (
	"context"
	"testing"
	"time"
)

// ── VersionVector Tests ─────────────────────────────────────────────────────

func TestVersionVectorMerge(t *testing.T) {
	v1 := VersionVector{"a": 3, "b": 1}
	v2 := VersionVector{"a": 1, "b": 5, "c": 2}
	merged := v1.Merge(v2)

	if merged["a"] != 3 {
		t.Errorf("expected a=3, got %d", merged["a"])
	}
	if merged["b"] != 5 {
		t.Errorf("expected b=5, got %d", merged["b"])
	}
	if merged["c"] != 2 {
		t.Errorf("expected c=2, got %d", merged["c"])
	}
}

func TestVersionVectorIncrement(t *testing.T) {
	vv := VersionVector{"inst1": 5}
	vv2 := vv.Increment("inst1")
	if vv2["inst1"] != 6 {
		t.Errorf("expected inst1=6, got %d", vv2["inst1"])
	}
	// Original should be unchanged
	if vv["inst1"] != 5 {
		t.Errorf("original mutated: expected 5, got %d", vv["inst1"])
	}
}

func TestVersionVectorHappensBefore(t *testing.T) {
	v1 := VersionVector{"a": 1, "b": 2}
	v2 := VersionVector{"a": 2, "b": 3}
	v3 := VersionVector{"a": 2, "b": 1}

	if !v1.HappensBefore(v2) {
		t.Error("v1 should happen-before v2")
	}
	if v2.HappensBefore(v1) {
		t.Error("v2 should NOT happen-before v1")
	}
	if v1.HappensBefore(v3) {
		t.Error("v1 should NOT happen-before v3 (concurrent)")
	}
}

func TestVersionVectorConcurrent(t *testing.T) {
	v1 := VersionVector{"a": 2, "b": 1}
	v2 := VersionVector{"a": 1, "b": 2}

	if !v1.Concurrent(v2) {
		t.Error("v1 and v2 should be concurrent")
	}

	v3 := VersionVector{"a": 1, "b": 1}
	v4 := VersionVector{"a": 2, "b": 2}
	if v3.Concurrent(v4) {
		t.Error("v3 should happen-before v4, not concurrent")
	}
}

// ── EventStream Tests ───────────────────────────────────────────────────────

func TestEventStreamDedup(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	es := NewEventStream(bus, EventStreamConfig{
		InstanceID: "test-1",
		MaxSeen:    100,
		SeenTTL:    time.Minute,
	})
	defer es.Close()

	// First time ???not a duplicate
	if es.isDuplicate("evt-001") {
		t.Error("event should not be duplicate initially")
	}

	es.markSeen("evt-001")

	// Second time ???duplicate
	if !es.isDuplicate("evt-001") {
		t.Error("event should be duplicate after markSeen")
	}
}

func TestEventStreamSeqTracking(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	es := NewEventStream(bus, EventStreamConfig{InstanceID: "test-1"})
	defer es.Close()

	// Seq 1 from remote ???accept
	if !es.checkAndUpdateSeq("remote-1", "task-1", 1) {
		t.Error("seq 1 should be accepted")
	}
	// Seq 2 from remote ???accept
	if !es.checkAndUpdateSeq("remote-1", "task-1", 2) {
		t.Error("seq 2 should be accepted")
	}
	// Seq 1 again from remote ???reject (already processed)
	if es.checkAndUpdateSeq("remote-1", "task-1", 1) {
		t.Error("seq 1 should be rejected (duplicate)")
	}
	// Seq 3 from different instance ???accept
	if !es.checkAndUpdateSeq("remote-2", "task-1", 3) {
		t.Error("seq 3 from remote-2 should be accepted")
	}
}

func TestEventStreamLocalPublish(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	sub := bus.Subscribe(EventFilter{}, 10)

	es := NewEventStream(bus, EventStreamConfig{
		InstanceID: "test-1",
		// No transport ???local only
	})
	defer es.Close()

	event := &Event{
		ID:        "evt-1",
		TaskID:    "task-1",
		Kind:      EventStepStarted,
		Actor:     "agent",
		Seq:       1,
		CreatedAt: time.Now(),
	}

	err := es.PublishAndBroadcast(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case received := <-sub.C:
		if received.ID != "evt-1" {
			t.Errorf("expected evt-1, got %s", received.ID)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

// ── SyncEngine Tests ────────────────────────────────────────────────────────

type mockSyncTransport struct {
	handler  func(msg *SyncMessage)
	messages []*SyncMessage
	peers    []string
}

func (m *mockSyncTransport) Send(_ context.Context, peerID string, msg *SyncMessage) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockSyncTransport) OnMessage(handler func(msg *SyncMessage)) {
	m.handler = handler
}

func (m *mockSyncTransport) ListPeers(_ context.Context) ([]string, error) {
	return m.peers, nil
}

func (m *mockSyncTransport) Close() error { return nil }

func TestSyncEngineLocalEvent(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	transport := &mockSyncTransport{peers: []string{"peer-1"}}

	se := NewSyncEngine(SyncEngineConfig{
		InstanceID: "inst-1",
		Bus:        bus,
		Transport:  transport,
	})

	event := &Event{
		ID:        "evt-1",
		TaskID:    "task-1",
		Kind:      EventStepStarted,
		Seq:       1,
		CreatedAt: time.Now(),
	}

	se.OnLocalEvent(context.Background(), event)

	vv := se.GetVersionVector()
	if vv["inst-1"] != 1 {
		t.Errorf("expected vector[inst-1]=1, got %d", vv["inst-1"])
	}

	// Should have pushed to peer
	if len(transport.messages) != 1 {
		t.Errorf("expected 1 push message, got %d", len(transport.messages))
	}
	if transport.messages[0].Type != SyncPush {
		t.Errorf("expected SyncPush, got %s", transport.messages[0].Type)
	}
}

func TestSyncEnginePullFromPeer(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	transport := &mockSyncTransport{peers: []string{"peer-1"}}

	se := NewSyncEngine(SyncEngineConfig{
		InstanceID: "inst-1",
		Bus:        bus,
		Transport:  transport,
	})

	se.pullFromPeer(context.Background(), "peer-1")

	if len(transport.messages) != 1 {
		t.Fatalf("expected 1 pull message, got %d", len(transport.messages))
	}
	if transport.messages[0].Type != SyncPull {
		t.Errorf("expected SyncPull, got %s", transport.messages[0].Type)
	}
}

func TestSyncMessageMarshal(t *testing.T) {
	msg := &SyncMessage{
		Type:       SyncPush,
		InstanceID: "inst-1",
		RequestID:  "req-1",
		Vector:     VersionVector{"inst-1": 5, "inst-2": 3},
		Events: []*Event{
			{ID: "e1", TaskID: "t1", Kind: EventTaskCreated, Seq: 1, CreatedAt: time.Now()},
		},
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := msg.Marshal()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	decoded, err := UnmarshalSyncMessage(data)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Type != SyncPush {
		t.Errorf("expected SyncPush, got %s", decoded.Type)
	}
	if decoded.Vector["inst-1"] != 5 {
		t.Errorf("expected vector[inst-1]=5, got %d", decoded.Vector["inst-1"])
	}
	if len(decoded.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(decoded.Events))
	}
}
