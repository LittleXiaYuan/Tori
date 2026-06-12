package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// HTTPStreamTransport implements StreamTransport over HTTP using Server-Sent Events
// for subscriptions and HTTP POST for publishing.
//
// Publishing: POST /ledger/stream/{topic} with JSON body.
// Subscribing: handled in-process via registered handlers; peers push events to us.
type HTTPStreamTransport struct {
	mu      sync.RWMutex
	subs    map[string]streamSub // subID ???subscription
	peers   map[string]string    // peerID ???base URL
	client  *http.Client
	mux     *http.ServeMux
	authKey string
	stopCh  chan struct{}
	stopped bool
}

type streamSub struct {
	topic   string
	handler func(data []byte)
}

// HTTPStreamConfig configures the HTTP stream transport.
type HTTPStreamConfig struct {
	Peers   map[string]string // peerID ???baseURL (for fanout publishing)
	AuthKey string            // shared secret
	Timeout time.Duration     // HTTP client timeout (default: 10s)
}

// NewHTTPStreamTransport creates an HTTP-based StreamTransport.
func NewHTTPStreamTransport(cfg HTTPStreamConfig) *HTTPStreamTransport {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	peers := cfg.Peers
	if peers == nil {
		peers = make(map[string]string)
	}

	t := &HTTPStreamTransport{
		subs:   make(map[string]streamSub),
		peers:  peers,
		client: &http.Client{Timeout: timeout},
		mux:    http.NewServeMux(),
		authKey: cfg.AuthKey,
		stopCh: make(chan struct{}),
	}

	t.mux.HandleFunc("/ledger/stream", t.handleIncoming)

	return t
}

// Handler returns the http.Handler for mounting on an existing server.
func (t *HTTPStreamTransport) Handler() http.Handler {
	return t.mux
}

// Publish sends event data to all known peers for the given topic.
func (t *HTTPStreamTransport) Publish(ctx context.Context, topic string, data []byte) error {
	t.mu.RLock()
	peers := make(map[string]string, len(t.peers))
	for k, v := range t.peers {
		peers[k] = v
	}
	t.mu.RUnlock()

	if len(peers) == 0 {
		return nil
	}

	envelope := streamHTTPEnvelope{
		Topic: topic,
		Data:  data,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal stream envelope: %w", err)
	}

	var firstErr error
	for peerID, baseURL := range peers {
		url := baseURL + "/ledger/stream"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("create request for %s: %w", peerID, err)
			}
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if t.authKey != "" {
			req.Header.Set("X-Sync-Key", t.authKey)
		}

		resp, err := t.client.Do(req)
		if err != nil {
			slog.Warn("stream publish: peer unreachable", "peer", peerID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			slog.Warn("stream publish: peer rejected event", "peer", peerID, "status", resp.StatusCode)
			if firstErr == nil {
				firstErr = fmt.Errorf("peer %s returned status %d", peerID, resp.StatusCode)
			}
		}
	}

	return firstErr
}

// Subscribe registers a handler for incoming events on the given topic.
func (t *HTTPStreamTransport) Subscribe(_ context.Context, topic string, handler func(data []byte)) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	subID := ulid.New()
	t.subs[subID] = streamSub{topic: topic, handler: handler}
	return subID, nil
}

// Unsubscribe removes a subscription.
func (t *HTTPStreamTransport) Unsubscribe(_ context.Context, subID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.subs, subID)
	return nil
}

// AddPeer dynamically adds or updates a peer.
func (t *HTTPStreamTransport) AddPeer(peerID, baseURL string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.peers[peerID] = baseURL
}

// Close releases resources.
func (t *HTTPStreamTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.stopped {
		t.stopped = true
		close(t.stopCh)
	}
	return nil
}

// streamHTTPEnvelope wraps topic + data for HTTP transport.
type streamHTTPEnvelope struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

// handleIncoming processes incoming stream events from peers.
func (t *HTTPStreamTransport) handleIncoming(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	if t.authKey != "" && r.Header.Get("X-Sync-Key") != t.authKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var envelope streamHTTPEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Collect matching handlers under the lock, invoke them after releasing
	// it: a handler that calls Subscribe/Unsubscribe would otherwise deadlock
	// trying to upgrade the read lock we are still holding.
	t.mu.RLock()
	var handlers []func(data []byte)
	for _, sub := range t.subs {
		if topicMatches(sub.topic, envelope.Topic) {
			handlers = append(handlers, sub.handler)
		}
	}
	t.mu.RUnlock()

	for _, h := range handlers {
		h(envelope.Data)
	}

	if len(handlers) == 0 {
		slog.Debug("stream transport: no matching subscribers", "topic", envelope.Topic)
	}

	w.WriteHeader(http.StatusAccepted)
}

// topicMatches checks if a subscription topic pattern matches an event topic.
// Supports exact match and wildcard ">" suffix (matches any subtopic).
func topicMatches(pattern, topic string) bool {
	if pattern == topic {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '>' {
		prefix := pattern[:len(pattern)-1]
		return len(topic) >= len(prefix) && topic[:len(prefix)] == prefix
	}
	return false
}
