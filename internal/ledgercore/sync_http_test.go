package ledger

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHTTPSyncTransportSendReceive(t *testing.T) {
	var received SyncMessage
	var mu sync.Mutex
	done := make(chan struct{})

	receiver := NewHTTPSyncTransport(HTTPSyncConfig{AuthKey: "test-secret"})
	receiver.OnMessage(func(msg *SyncMessage) {
		mu.Lock()
		received = *msg
		mu.Unlock()
		close(done)
	})

	ts := httptest.NewServer(receiver.Handler())
	defer ts.Close()

	sender := NewHTTPSyncTransport(HTTPSyncConfig{
		Peers:   map[string]string{"peer-1": ts.URL},
		AuthKey: "test-secret",
	})
	defer sender.Close()

	msg := &SyncMessage{
		Type:       SyncPush,
		InstanceID: "inst-sender",
		RequestID:  "req-1",
		Vector:     VersionVector{"inst-sender": 5},
		Events: []*Event{{
			ID:        "evt-1",
			TaskID:    "task-1",
			Kind:      EventStepStarted,
			Seq:       1,
			CreatedAt: time.Now(),
		}},
		Timestamp: time.Now().UnixMilli(),
	}

	ctx := context.Background()
	if err := sender.Send(ctx, "peer-1", msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	mu.Lock()
	defer mu.Unlock()
	if received.Type != SyncPush {
		t.Errorf("expected SyncPush, got %s", received.Type)
	}
	if received.InstanceID != "inst-sender" {
		t.Errorf("expected inst-sender, got %s", received.InstanceID)
	}
	if len(received.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(received.Events))
	}
	if received.Vector["inst-sender"] != 5 {
		t.Errorf("expected vector[inst-sender]=5, got %d", received.Vector["inst-sender"])
	}
}

func TestHTTPSyncTransportAuthReject(t *testing.T) {
	receiver := NewHTTPSyncTransport(HTTPSyncConfig{AuthKey: "secret-123"})
	ts := httptest.NewServer(receiver.Handler())
	defer ts.Close()

	sender := NewHTTPSyncTransport(HTTPSyncConfig{
		Peers:   map[string]string{"peer-1": ts.URL},
		AuthKey: "wrong-key",
	})
	defer sender.Close()

	err := sender.Send(context.Background(), "peer-1", &SyncMessage{
		Type:       SyncPull,
		InstanceID: "attacker",
		Timestamp:  time.Now().UnixMilli(),
	})
	if err == nil {
		t.Fatal("expected error for wrong auth key")
	}
}

func TestHTTPSyncTransportNoAuth(t *testing.T) {
	var gotMsg bool
	receiver := NewHTTPSyncTransport(HTTPSyncConfig{})
	receiver.OnMessage(func(msg *SyncMessage) { gotMsg = true })
	ts := httptest.NewServer(receiver.Handler())
	defer ts.Close()

	sender := NewHTTPSyncTransport(HTTPSyncConfig{
		Peers: map[string]string{"peer-1": ts.URL},
	})
	defer sender.Close()

	err := sender.Send(context.Background(), "peer-1", &SyncMessage{
		Type:      SyncPush,
		Timestamp: time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if !gotMsg {
		t.Error("expected message to be received")
	}
}

func TestHTTPSyncTransportListPeers(t *testing.T) {
	tr := NewHTTPSyncTransport(HTTPSyncConfig{
		Peers: map[string]string{"a": "http://a:9090", "b": "http://b:9090"},
	})
	defer tr.Close()

	peers, err := tr.ListPeers(context.Background())
	if err != nil {
		t.Fatalf("ListPeers failed: %v", err)
	}
	if len(peers) != 2 {
		t.Errorf("expected 2 peers, got %d", len(peers))
	}
}

func TestHTTPSyncTransportDynamicPeers(t *testing.T) {
	tr := NewHTTPSyncTransport(HTTPSyncConfig{})
	defer tr.Close()

	tr.AddPeer("new-peer", "http://new:9090")
	peers, _ := tr.ListPeers(context.Background())
	if len(peers) != 1 || peers[0] != "new-peer" {
		t.Errorf("expected [new-peer], got %v", peers)
	}

	tr.RemovePeer("new-peer")
	peers, _ = tr.ListPeers(context.Background())
	if len(peers) != 0 {
		t.Errorf("expected 0 peers after removal, got %d", len(peers))
	}
}

func TestHTTPSyncTransportUnknownPeer(t *testing.T) {
	tr := NewHTTPSyncTransport(HTTPSyncConfig{})
	defer tr.Close()

	err := tr.Send(context.Background(), "nonexistent", &SyncMessage{})
	if err == nil {
		t.Error("expected error for unknown peer")
	}
}

func TestHTTPSyncTransportMethodNotAllowed(t *testing.T) {
	tr := NewHTTPSyncTransport(HTTPSyncConfig{})
	ts := httptest.NewServer(tr.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ledger/sync")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

func TestHTTPSyncTransportBidirectional(t *testing.T) {
	var msgs1, msgs2 []SyncMessage
	var mu1, mu2 sync.Mutex

	t1 := NewHTTPSyncTransport(HTTPSyncConfig{})
	t1.OnMessage(func(msg *SyncMessage) {
		mu1.Lock()
		msgs1 = append(msgs1, *msg)
		mu1.Unlock()
	})
	ts1 := httptest.NewServer(t1.Handler())
	defer ts1.Close()

	t2 := NewHTTPSyncTransport(HTTPSyncConfig{})
	t2.OnMessage(func(msg *SyncMessage) {
		mu2.Lock()
		msgs2 = append(msgs2, *msg)
		mu2.Unlock()
	})
	ts2 := httptest.NewServer(t2.Handler())
	defer ts2.Close()

	t1.AddPeer("inst-2", ts2.URL)
	t2.AddPeer("inst-1", ts1.URL)
	defer t1.Close()
	defer t2.Close()

	ctx := context.Background()

	if err := t1.Send(ctx, "inst-2", &SyncMessage{Type: SyncPush, InstanceID: "inst-1"}); err != nil {
		t.Fatalf("t1→t2 failed: %v", err)
	}
	if err := t2.Send(ctx, "inst-1", &SyncMessage{Type: SyncPull, InstanceID: "inst-2"}); err != nil {
		t.Fatalf("t2→t1 failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu1.Lock()
	if len(msgs1) != 1 || msgs1[0].InstanceID != "inst-2" {
		t.Errorf("inst-1 should have received 1 msg from inst-2, got %d", len(msgs1))
	}
	mu1.Unlock()

	mu2.Lock()
	if len(msgs2) != 1 || msgs2[0].InstanceID != "inst-1" {
		t.Errorf("inst-2 should have received 1 msg from inst-1, got %d", len(msgs2))
	}
	mu2.Unlock()
}
