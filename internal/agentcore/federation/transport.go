package federation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"yunque-agent/pkg/safego"
)

// Transport handles the HTTP layer for federation messages.
type Transport struct {
	hub     *Hub
	client  *http.Client
	pending map[string]chan *Message // msg.ID → response channel
}

// NewTransport creates an HTTP transport for the federation hub.
func NewTransport(hub *Hub) *Transport {
	return &Transport{
		hub: hub,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		pending: make(map[string]chan *Message),
	}
}

// SendHTTP delivers a message to a remote peer via HTTP POST.
func (t *Transport) SendHTTP(ctx context.Context, msg Message) (*Message, error) {
	instance := msg.To.Instance()
	if instance == "" || instance == "local" {
		return nil, fmt.Errorf("cannot send to local peer via HTTP")
	}

	url := strings.TrimRight(instance, "/") + "/v1/federation/receive"

	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Fed-From", string(t.hub.LocalID()))

	resp, err := t.client.Do(req)
	if err != nil {
		t.markPeerUnhealthy(msg.To)
		return nil, fmt.Errorf("send to %s: %w", msg.To, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("peer %s returned %d: %s", msg.To, resp.StatusCode, string(bodyBytes))
	}

	var reply Message
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return nil, fmt.Errorf("decode reply: %w", err)
	}

	return &reply, nil
}

// HTTPHandler returns an http.HandlerFunc for receiving federation messages.
// Mount this at /v1/federation/receive.
func (t *Transport) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var msg Message
		if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&msg); err != nil {
			http.Error(w, "invalid message: "+err.Error(), http.StatusBadRequest)
			return
		}

		reply, err := t.hub.Receive(r.Context(), msg)
		if err != nil {
			slog.Warn("federation: receive error", "from", msg.From, "type", msg.Type, "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if reply != nil {
			json.NewEncoder(w).Encode(reply)
		} else {
			json.NewEncoder(w).Encode(map[string]string{"status": "received"})
		}
	}
}

// SendAndWait sends a message and waits for a reply (synchronous RPC).
func (t *Transport) SendAndWait(ctx context.Context, msg Message, timeout time.Duration) (*Message, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return t.SendHTTP(ctx, msg)
}

// DiscoverPeer sends a discover message to a remote instance and registers capabilities.
func (t *Transport) DiscoverPeer(ctx context.Context, agentName, instanceURL string) (*PeerInfo, error) {
	peerID := NewPeerID(agentName, instanceURL)

	msg, err := t.hub.Send(ctx, peerID, MsgDiscover, "")
	if err != nil {
		return nil, err
	}

	reply, err := t.SendHTTP(ctx, *msg)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", peerID, err)
	}

	if reply.Type == MsgCapReply {
		var caps []string
		json.Unmarshal([]byte(reply.Payload), &caps)
		t.hub.AddPeer(peerID, caps)
		info, _ := t.hub.GetPeer(peerID)
		slog.Info("federation: peer discovered", "peer", peerID, "caps", len(caps))
		return info, nil
	}

	return nil, fmt.Errorf("unexpected reply type: %s", reply.Type)
}

// DelegateTask sends a task to a remote peer and waits for the result.
func (t *Transport) DelegateTask(ctx context.Context, to PeerID, task string, timeout time.Duration) (string, error) {
	msg, err := t.hub.Send(ctx, to, MsgTask, task)
	if err != nil {
		return "", err
	}

	reply, err := t.SendAndWait(ctx, *msg, timeout)
	if err != nil {
		return "", err
	}

	if reply.Type == MsgResult {
		return reply.Payload, nil
	}
	return "", fmt.Errorf("unexpected reply: %s", reply.Type)
}

// StartOutboxWorker runs a background goroutine that sends queued outbox messages.
func (t *Transport) StartOutboxWorker(ctx context.Context) {
	safego.Go("federation-outbox-worker", func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				msgs := t.hub.DrainOutbox()
				for _, msg := range msgs {
					if msg.To.Instance() == "local" || msg.To.Instance() == "" {
						continue
					}
					if _, err := t.SendHTTP(ctx, msg); err != nil {
						slog.Warn("federation: outbox send failed", "to", msg.To, "err", err)
					}
				}
			}
		}
	})
}

// markPeerUnhealthy marks a peer as unhealthy after a failed send.
func (t *Transport) markPeerUnhealthy(id PeerID) {
	if info, ok := t.hub.GetPeer(id); ok {
		info.Healthy = false
		// AddPeer will update
		t.hub.AddPeer(id, info.Capabilities)
	}
}
