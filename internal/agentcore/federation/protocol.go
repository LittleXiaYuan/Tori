package federation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"yunque-agent/pkg/safego"
)

// MessageType classifies federation messages.
type MessageType string

const (
	MsgChat     MessageType = "chat"      // agent-to-agent conversation
	MsgTask     MessageType = "task"      // delegate a task
	MsgResult   MessageType = "result"    // task result
	MsgPing     MessageType = "ping"      // health check
	MsgPong     MessageType = "pong"      // health response
	MsgDiscover MessageType = "discover"  // capability discovery
	MsgCapReply MessageType = "cap_reply" // capability reply
)

// Message is the wire format for the OpenFang-inspired federation protocol (OFP).
type Message struct {
	ID        string      `json:"id"`
	Type      MessageType `json:"type"`
	From      PeerID      `json:"from"`               // sender agent@instance
	To        PeerID      `json:"to"`                 // receiver agent@instance
	ReplyTo   string      `json:"reply_to,omitempty"` // correlates responses
	Payload   string      `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
	Signature string      `json:"signature,omitempty"` // HMAC-SHA256
	TTL       int         `json:"ttl"`                 // hop limit
}

// PeerID uniquely identifies an agent on a specific instance.
// Format: "agent_name@instance_url"
type PeerID string

// AgentName extracts the agent name from a PeerID.
func (p PeerID) AgentName() string {
	parts := strings.SplitN(string(p), "@", 2)
	return parts[0]
}

// Instance extracts the instance URL from a PeerID.
func (p PeerID) Instance() string {
	parts := strings.SplitN(string(p), "@", 2)
	if len(parts) < 2 {
		return "local"
	}
	return parts[1]
}

// NewPeerID creates a PeerID from agent name and instance URL.
func NewPeerID(agent, instance string) PeerID {
	return PeerID(agent + "@" + instance)
}

// PeerInfo describes a known remote agent peer.
type PeerInfo struct {
	ID           PeerID        `json:"id"`
	Capabilities []string      `json:"capabilities"`
	LastSeen     time.Time     `json:"last_seen"`
	Healthy      bool          `json:"healthy"`
	Latency      time.Duration `json:"latency_ms"`
}

// Hub is the federation hub that manages peer connections and message routing.
type Hub struct {
	mu       sync.RWMutex
	localID  PeerID
	peers    map[PeerID]*PeerInfo
	handlers map[MessageType]MessageHandler
	secret   string // shared HMAC secret
	inbox    chan Message
	outbox   chan Message
	httpCli  *http.Client
}

// MessageHandler processes incoming federation messages.
type MessageHandler func(ctx context.Context, msg Message) (*Message, error)

// HubConfig configures the federation hub.
type HubConfig struct {
	LocalAgent    string // local agent name
	LocalInstance string // local instance URL
	Secret        string // shared HMAC secret for message signing
	BufferSize    int    // channel buffer size (default 256)
}

// NewHub creates a new federation hub.
func NewHub(cfg HubConfig) *Hub {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 256
	}
	return &Hub{
		localID:  NewPeerID(cfg.LocalAgent, cfg.LocalInstance),
		peers:    make(map[PeerID]*PeerInfo),
		handlers: make(map[MessageType]MessageHandler),
		secret:   cfg.Secret,
		inbox:    make(chan Message, cfg.BufferSize),
		outbox:   make(chan Message, cfg.BufferSize),
		httpCli:  &http.Client{Timeout: 10 * time.Second},
	}
}

// LocalID returns this hub's peer ID.
func (h *Hub) LocalID() PeerID { return h.localID }

// RegisterHandler sets the handler for a message type.
func (h *Hub) RegisterHandler(typ MessageType, handler MessageHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[typ] = handler
}

// AddPeer registers a known remote peer.
func (h *Hub) AddPeer(id PeerID, capabilities []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.peers[id] = &PeerInfo{
		ID:           id,
		Capabilities: capabilities,
		LastSeen:     time.Now(),
		Healthy:      true,
	}
}

// RemovePeer removes a peer.
func (h *Hub) RemovePeer(id PeerID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.peers[id]; !ok {
		return false
	}
	delete(h.peers, id)
	return true
}

// GetPeer returns peer info.
func (h *Hub) GetPeer(id PeerID) (*PeerInfo, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	p, ok := h.peers[id]
	if !ok {
		return nil, false
	}
	copy := *p
	return &copy, true
}

// ListPeers returns all known peers.
func (h *Hub) ListPeers() []PeerInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]PeerInfo, 0, len(h.peers))
	for _, p := range h.peers {
		out = append(out, *p)
	}
	return out
}

// FindByCapability returns peers that advertise a specific capability.
func (h *Hub) FindByCapability(cap string) []PeerInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []PeerInfo
	for _, p := range h.peers {
		if !p.Healthy {
			continue
		}
		for _, c := range p.Capabilities {
			if c == cap {
				out = append(out, *p)
				break
			}
		}
	}
	return out
}

// Send creates and enqueues a message to a remote peer.
func (h *Hub) Send(ctx context.Context, to PeerID, typ MessageType, payload string) (*Message, error) {
	msg := Message{
		ID:        fmt.Sprintf("ofp_%d", time.Now().UnixNano()),
		Type:      typ,
		From:      h.localID,
		To:        to,
		Payload:   payload,
		Timestamp: time.Now(),
		TTL:       3,
	}

	if h.secret != "" {
		msg.Signature = h.sign(msg)
	}

	select {
	case h.outbox <- msg:
		return &msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, fmt.Errorf("outbox full")
	}
}

// Receive processes an incoming message, verifies signature, and dispatches to handler.
func (h *Hub) Receive(ctx context.Context, msg Message) (*Message, error) {
	// TTL check
	if msg.TTL <= 0 {
		return nil, fmt.Errorf("message TTL expired")
	}

	// Signature verification (before TTL decrement so hash matches).
	// When a shared secret is configured, every inbound message must be signed.
	if h.secret != "" {
		if msg.Signature == "" {
			return nil, fmt.Errorf("missing message signature")
		}
		expected := h.sign(msg)
		if !hmac.Equal([]byte(msg.Signature), []byte(expected)) {
			return nil, fmt.Errorf("invalid message signature")
		}
	}
	msg.TTL--

	// Update peer last seen
	h.mu.Lock()
	if p, ok := h.peers[msg.From]; ok {
		p.LastSeen = time.Now()
		p.Healthy = true
	}
	h.mu.Unlock()

	// Built-in ping/pong
	if msg.Type == MsgPing {
		reply := Message{
			ID:        fmt.Sprintf("ofp_%d", time.Now().UnixNano()),
			Type:      MsgPong,
			From:      h.localID,
			To:        msg.From,
			ReplyTo:   msg.ID,
			Payload:   "pong",
			Timestamp: time.Now(),
			TTL:       3,
		}
		return &reply, nil
	}

	// Dispatch to registered handler (takes priority over built-in defaults)
	h.mu.RLock()
	handler, ok := h.handlers[msg.Type]
	h.mu.RUnlock()

	// Built-in discover fallback (only when no custom handler is registered)
	if !ok && msg.Type == MsgDiscover {
		capJSON, _ := json.Marshal([]string{})
		reply := Message{
			ID:        fmt.Sprintf("ofp_%d", time.Now().UnixNano()),
			Type:      MsgCapReply,
			From:      h.localID,
			To:        msg.From,
			ReplyTo:   msg.ID,
			Payload:   string(capJSON),
			Timestamp: time.Now(),
			TTL:       3,
		}
		return &reply, nil
	}

	if !ok {
		return nil, fmt.Errorf("no handler for message type: %s", msg.Type)
	}

	return handler(ctx, msg)
}

// Ping sends a ping to a peer and measures latency.
func (h *Hub) Ping(ctx context.Context, peer PeerID) (time.Duration, error) {
	start := time.Now()
	msg := Message{
		ID:        fmt.Sprintf("ofp_%d", time.Now().UnixNano()),
		Type:      MsgPing,
		From:      h.localID,
		To:        peer,
		Payload:   "ping",
		Timestamp: time.Now(),
		TTL:       3,
	}
	if h.secret != "" {
		msg.Signature = h.sign(msg)
	}

	inbound := Message{
		ID: msg.ID, Type: MsgPing, From: peer, To: h.localID,
		Payload: "ping", Timestamp: time.Now(), TTL: 3,
	}
	if h.secret != "" {
		inbound.Signature = h.sign(inbound)
	}

	reply, err := h.Receive(ctx, inbound)
	latency := time.Since(start)

	if err != nil {
		h.mu.Lock()
		if p, ok := h.peers[peer]; ok {
			p.Healthy = false
		}
		h.mu.Unlock()
		return 0, err
	}

	if reply != nil && reply.Type == MsgPong {
		h.mu.Lock()
		if p, ok := h.peers[peer]; ok {
			p.Latency = latency
			p.Healthy = true
			p.LastSeen = time.Now()
		}
		h.mu.Unlock()
	}

	return latency, nil
}

// DrainOutbox returns and clears pending outbound messages.
func (h *Hub) DrainOutbox() []Message {
	var msgs []Message
	for {
		select {
		case msg := <-h.outbox:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

// HealthCheck pings all peers and updates their health status.
func (h *Hub) HealthCheck(ctx context.Context) map[PeerID]bool {
	peers := h.ListPeers()
	results := make(map[PeerID]bool, len(peers))
	for _, p := range peers {
		_, err := h.Ping(ctx, p.ID)
		results[p.ID] = err == nil
	}
	return results
}

// Stats returns hub statistics.
func (h *Hub) Stats() map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	healthy := 0
	for _, p := range h.peers {
		if p.Healthy {
			healthy++
		}
	}
	return map[string]any{
		"local_id":       string(h.localID),
		"total_peers":    len(h.peers),
		"healthy_peers":  healthy,
		"handlers":       len(h.handlers),
		"outbox_pending": len(h.outbox),
		"inbox_pending":  len(h.inbox),
	}
}

// sign computes HMAC-SHA256 for a message (excludes the signature field itself).
func (h *Hub) sign(msg Message) string {
	// Clear signature before computing
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%d",
		msg.ID, msg.Type, msg.From, msg.To, msg.Payload, msg.TTL)
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// StartWorker runs a background worker that processes inbox messages.
func (h *Hub) StartWorker(ctx context.Context) {
	safego.Go("federation-inbox-worker", func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-h.inbox:
				reply, err := h.Receive(ctx, msg)
				if err != nil {
					slog.Warn("federation: message processing failed", "type", msg.Type, "from", msg.From, "err", err)
					continue
				}
				if reply != nil {
					select {
					case h.outbox <- *reply:
					default:
						slog.Warn("federation: outbox full, dropping reply")
					}
				}
			}
		}
	})
}

// Enqueue adds a message to the inbox for async processing.
func (h *Hub) Enqueue(msg Message) bool {
	select {
	case h.inbox <- msg:
		return true
	default:
		return false
	}
}
