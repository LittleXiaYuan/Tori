package federation

import (
	"context"
	"testing"
	"time"
)

func newTestHub() *Hub {
	return NewHub(HubConfig{
		LocalAgent:    "agent1",
		LocalInstance: "localhost:9090",
		Secret:        "test-secret-key",
	})
}

func TestNewHub(t *testing.T) {
	h := newTestHub()
	if h.LocalID() != "agent1@localhost:9090" {
		t.Errorf("unexpected local ID: %s", h.LocalID())
	}
}

func TestPeerID(t *testing.T) {
	pid := NewPeerID("bot", "remote.example.com:9090")
	if pid.AgentName() != "bot" {
		t.Errorf("expected bot, got %s", pid.AgentName())
	}
	if pid.Instance() != "remote.example.com:9090" {
		t.Errorf("expected remote.example.com:9090, got %s", pid.Instance())
	}

	local := PeerID("localonly")
	if local.Instance() != "local" {
		t.Errorf("expected 'local' for no @, got %s", local.Instance())
	}
}

func TestAddAndListPeers(t *testing.T) {
	h := newTestHub()
	h.AddPeer("bot2@remote:9090", []string{"chat", "coding"})
	h.AddPeer("bot3@remote:9091", []string{"search"})

	peers := h.ListPeers()
	if len(peers) != 2 {
		t.Errorf("expected 2 peers, got %d", len(peers))
	}
}

func TestGetPeer(t *testing.T) {
	h := newTestHub()
	h.AddPeer("bot2@remote:9090", []string{"chat"})

	p, ok := h.GetPeer("bot2@remote:9090")
	if !ok {
		t.Fatal("expected peer found")
	}
	if !p.Healthy {
		t.Error("new peer should be healthy")
	}
}

func TestRemovePeer(t *testing.T) {
	h := newTestHub()
	h.AddPeer("bot2@remote:9090", []string{"chat"})
	if !h.RemovePeer("bot2@remote:9090") {
		t.Error("expected true for remove")
	}
	if h.RemovePeer("nonexistent@x") {
		t.Error("expected false for nonexistent")
	}
}

func TestFindByCapability(t *testing.T) {
	h := newTestHub()
	h.AddPeer("bot2@r:1", []string{"chat", "coding"})
	h.AddPeer("bot3@r:2", []string{"search", "coding"})
	h.AddPeer("bot4@r:3", []string{"chat"})

	coders := h.FindByCapability("coding")
	if len(coders) != 2 {
		t.Errorf("expected 2 coding peers, got %d", len(coders))
	}

	searchers := h.FindByCapability("search")
	if len(searchers) != 1 {
		t.Errorf("expected 1 search peer, got %d", len(searchers))
	}
}

func TestSendAndDrain(t *testing.T) {
	h := newTestHub()
	ctx := context.Background()

	msg, err := h.Send(ctx, "bot2@remote:9090", MsgChat, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if msg.From != h.LocalID() {
		t.Error("from should be local ID")
	}
	if msg.Signature == "" {
		t.Error("expected signature when secret is set")
	}

	msgs := h.DrainOutbox()
	if len(msgs) != 1 {
		t.Errorf("expected 1 outbox message, got %d", len(msgs))
	}

	// Drain again should be empty
	msgs = h.DrainOutbox()
	if len(msgs) != 0 {
		t.Error("expected empty outbox after drain")
	}
}

func TestReceivePing(t *testing.T) {
	h := newTestHub()
	h.AddPeer("bot2@remote:9090", []string{})

	msg := Message{
		ID: "test1", Type: MsgPing, From: "bot2@remote:9090",
		To: h.LocalID(), Payload: "ping", Timestamp: time.Now(), TTL: 3,
	}
	msg.Signature = h.sign(msg)
	reply, err := h.Receive(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Type != MsgPong {
		t.Errorf("expected pong, got %s", reply.Type)
	}
}

func TestReceiveTTLExpired(t *testing.T) {
	h := newTestHub()
	msg := Message{
		ID: "test1", Type: MsgChat, From: "bot2@r:1",
		To: h.LocalID(), Payload: "hi", Timestamp: time.Now(), TTL: 0,
	}
	msg.Signature = h.sign(msg)
	_, err := h.Receive(context.Background(), msg)
	if err == nil {
		t.Error("expected TTL expired error")
	}
}

func TestReceiveNoHandler(t *testing.T) {
	h := newTestHub()
	msg := Message{
		ID: "test1", Type: MsgChat, From: "bot2@r:1",
		To: h.LocalID(), Payload: "hi", Timestamp: time.Now(), TTL: 3,
	}
	msg.Signature = h.sign(msg)
	_, err := h.Receive(context.Background(), msg)
	if err == nil {
		t.Error("expected no handler error")
	}
}

func TestRegisterHandler(t *testing.T) {
	h := newTestHub()
	h.RegisterHandler(MsgChat, func(ctx context.Context, msg Message) (*Message, error) {
		return &Message{
			ID: "reply1", Type: MsgResult, From: h.LocalID(),
			To: msg.From, ReplyTo: msg.ID, Payload: "got it",
		}, nil
	})

	msg := Message{
		ID: "test1", Type: MsgChat, From: "bot2@r:1",
		To: h.LocalID(), Payload: "hello", Timestamp: time.Now(), TTL: 3,
	}
	msg.Signature = h.sign(msg)
	reply, err := h.Receive(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Payload != "got it" {
		t.Errorf("expected 'got it', got %s", reply.Payload)
	}
}

func TestPingPeer(t *testing.T) {
	h := newTestHub()
	h.AddPeer("bot2@r:1", []string{"chat"})

	latency, err := h.Ping(context.Background(), "bot2@r:1")
	if err != nil {
		t.Fatal(err)
	}
	if latency < 0 {
		t.Error("expected non-negative latency")
	}

	p, _ := h.GetPeer("bot2@r:1")
	if !p.Healthy {
		t.Error("peer should be healthy after ping")
	}
}

func TestHealthCheck(t *testing.T) {
	h := newTestHub()
	h.AddPeer("bot2@r:1", []string{"chat"})
	h.AddPeer("bot3@r:2", []string{"search"})

	results := h.HealthCheck(context.Background())
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestEnqueue(t *testing.T) {
	h := newTestHub()
	ok := h.Enqueue(Message{ID: "m1", Type: MsgChat, Payload: "hi", TTL: 3})
	if !ok {
		t.Error("expected enqueue success")
	}
}

func TestStats(t *testing.T) {
	h := newTestHub()
	h.AddPeer("bot2@r:1", []string{"chat"})
	h.RegisterHandler(MsgChat, func(ctx context.Context, msg Message) (*Message, error) {
		return nil, nil
	})

	stats := h.Stats()
	if stats["total_peers"].(int) != 1 {
		t.Errorf("expected 1 peer, got %v", stats["total_peers"])
	}
	if stats["handlers"].(int) != 1 {
		t.Errorf("expected 1 handler, got %v", stats["handlers"])
	}
}

func TestSignatureVerification(t *testing.T) {
	h := newTestHub()
	h.RegisterHandler(MsgTask, func(ctx context.Context, msg Message) (*Message, error) {
		return nil, nil
	})

	// Valid signature
	msg := Message{
		ID: "test1", Type: MsgTask, From: "bot2@r:1",
		To: h.LocalID(), Payload: "do stuff", TTL: 3,
	}
	msg.Signature = h.sign(msg)
	_, err := h.Receive(context.Background(), msg)
	if err != nil {
		t.Errorf("valid signature should pass: %v", err)
	}

	// Invalid signature
	msg2 := Message{
		ID: "test2", Type: MsgTask, From: "bot2@r:1",
		To: h.LocalID(), Payload: "do stuff", TTL: 3,
		Signature: "bad_sig",
	}
	_, err = h.Receive(context.Background(), msg2)
	if err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestSignatureVerification_RejectsMissingSignature(t *testing.T) {
	h := newTestHub()
	h.RegisterHandler(MsgTask, func(ctx context.Context, msg Message) (*Message, error) {
		return nil, nil
	})

	_, err := h.Receive(context.Background(), Message{
		ID: "test3", Type: MsgTask, From: "bot2@r:1",
		To: h.LocalID(), Payload: "do stuff", TTL: 3,
	})
	if err == nil {
		t.Fatal("expected error for missing signature")
	}
}
