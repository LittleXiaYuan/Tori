package ledger

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHTTPStreamPublishSubscribe(t *testing.T) {
	var received []byte
	var mu sync.Mutex
	done := make(chan struct{})

	receiver := NewHTTPStreamTransport(HTTPStreamConfig{})
	receiver.Subscribe(context.Background(), "ledger.events.>", func(data []byte) {
		mu.Lock()
		received = data
		mu.Unlock()
		close(done)
	})

	ts := httptest.NewServer(receiver.Handler())
	defer ts.Close()

	sender := NewHTTPStreamTransport(HTTPStreamConfig{
		Peers: map[string]string{"r": ts.URL},
	})
	defer sender.Close()

	payload := map[string]string{"event_id": "evt-1", "kind": "step_started"}
	data, _ := json.Marshal(payload)

	if err := sender.Publish(context.Background(), "ledger.events.task.123", data); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	mu.Lock()
	defer mu.Unlock()
	var got map[string]string
	if err := json.Unmarshal(received, &got); err != nil {
		t.Fatalf("unmarshal received: %v", err)
	}
	if got["event_id"] != "evt-1" {
		t.Errorf("expected event_id=evt-1, got %s", got["event_id"])
	}
}

func TestHTTPStreamTopicFiltering(t *testing.T) {
	var count int
	var mu sync.Mutex

	receiver := NewHTTPStreamTransport(HTTPStreamConfig{})
	receiver.Subscribe(context.Background(), "ledger.events.task.abc", func(data []byte) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	ts := httptest.NewServer(receiver.Handler())
	defer ts.Close()

	sender := NewHTTPStreamTransport(HTTPStreamConfig{
		Peers: map[string]string{"r": ts.URL},
	})
	defer sender.Close()

	ctx := context.Background()
	sender.Publish(ctx, "ledger.events.task.abc", []byte(`{}`))
	sender.Publish(ctx, "ledger.events.task.xyz", []byte(`{}`))
	sender.Publish(ctx, "ledger.events.global", []byte(`{}`))

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	if count != 1 {
		t.Errorf("expected 1 matching event, got %d", count)
	}
	mu.Unlock()
}

func TestHTTPStreamWildcard(t *testing.T) {
	var count int
	var mu sync.Mutex

	receiver := NewHTTPStreamTransport(HTTPStreamConfig{})
	receiver.Subscribe(context.Background(), "ledger.events.>", func(data []byte) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	ts := httptest.NewServer(receiver.Handler())
	defer ts.Close()

	sender := NewHTTPStreamTransport(HTTPStreamConfig{
		Peers: map[string]string{"r": ts.URL},
	})
	defer sender.Close()

	ctx := context.Background()
	sender.Publish(ctx, "ledger.events.task.abc", []byte(`{}`))
	sender.Publish(ctx, "ledger.events.global", []byte(`{}`))
	sender.Publish(ctx, "other.topic", []byte(`{}`))

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	if count != 2 {
		t.Errorf("expected 2 wildcard matches, got %d", count)
	}
	mu.Unlock()
}

func TestHTTPStreamUnsubscribe(t *testing.T) {
	var count int
	var mu sync.Mutex

	receiver := NewHTTPStreamTransport(HTTPStreamConfig{})
	subID, _ := receiver.Subscribe(context.Background(), "ledger.events.>", func(data []byte) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	ts := httptest.NewServer(receiver.Handler())
	defer ts.Close()

	sender := NewHTTPStreamTransport(HTTPStreamConfig{
		Peers: map[string]string{"r": ts.URL},
	})
	defer sender.Close()

	ctx := context.Background()
	sender.Publish(ctx, "ledger.events.x", []byte(`{}`))
	time.Sleep(50 * time.Millisecond)

	receiver.Unsubscribe(ctx, subID)
	sender.Publish(ctx, "ledger.events.y", []byte(`{}`))
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if count != 1 {
		t.Errorf("expected 1 event (after unsub), got %d", count)
	}
	mu.Unlock()
}

func TestHTTPStreamNoPeers(t *testing.T) {
	sender := NewHTTPStreamTransport(HTTPStreamConfig{})
	defer sender.Close()

	err := sender.Publish(context.Background(), "test", []byte(`{}`))
	if err != nil {
		t.Errorf("publish with no peers should succeed (no-op), got: %v", err)
	}
}

func TestTopicMatches(t *testing.T) {
	tests := []struct {
		pattern, topic string
		want           bool
	}{
		{"ledger.events.task.abc", "ledger.events.task.abc", true},
		{"ledger.events.task.abc", "ledger.events.task.xyz", false},
		{"ledger.events.>", "ledger.events.task.abc", true},
		{"ledger.events.>", "ledger.events.global", true},
		{"ledger.events.>", "other.topic", false},
		{"ledger.>", "ledger.events.x", true},
		{"exact", "exact", true},
		{"exact", "other", false},
	}

	for _, tt := range tests {
		got := topicMatches(tt.pattern, tt.topic)
		if got != tt.want {
			t.Errorf("topicMatches(%q, %q) = %v, want %v", tt.pattern, tt.topic, got, tt.want)
		}
	}
}
