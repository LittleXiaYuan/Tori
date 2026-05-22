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
)

// HTTPSyncTransport implements SyncTransport over HTTP.
// Each peer exposes a POST /ledger/sync endpoint to receive messages.
// Peer discovery uses a static list configured at construction time.
type HTTPSyncTransport struct {
	mu       sync.RWMutex
	peers    map[string]string // peerID ???base URL (e.g. "http://host:9090")
	handler  func(msg *SyncMessage)
	client   *http.Client
	mux      *http.ServeMux
	authKey  string // shared secret for peer authentication
	stopCh   chan struct{}
	stopped  bool
}

// HTTPSyncConfig configures the HTTP sync transport.
type HTTPSyncConfig struct {
	Peers      map[string]string // peerID ???baseURL
	AuthKey    string            // shared secret (sent as X-Sync-Key header)
	Timeout    time.Duration     // HTTP client timeout (default: 10s)
}

// NewHTTPSyncTransport creates an HTTP-based SyncTransport.
func NewHTTPSyncTransport(cfg HTTPSyncConfig) *HTTPSyncTransport {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	peers := cfg.Peers
	if peers == nil {
		peers = make(map[string]string)
	}

	t := &HTTPSyncTransport{
		peers:   peers,
		client:  &http.Client{Timeout: timeout},
		mux:     http.NewServeMux(),
		authKey: cfg.AuthKey,
		stopCh:  make(chan struct{}),
	}

	t.mux.HandleFunc("/ledger/sync", t.handleIncoming)

	return t
}

// Handler returns the http.Handler for mounting on an existing server.
// Mount this on your HTTP server so peers can send messages to this instance.
func (t *HTTPSyncTransport) Handler() http.Handler {
	return t.mux
}

// Send sends a sync message to a specific peer via HTTP POST.
func (t *HTTPSyncTransport) Send(ctx context.Context, peerID string, msg *SyncMessage) error {
	t.mu.RLock()
	baseURL, ok := t.peers[peerID]
	t.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown peer: %s", peerID)
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal sync message: %w", err)
	}

	url := baseURL + "/ledger/sync"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.authKey != "" {
		req.Header.Set("X-Sync-Key", t.authKey)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send to %s: %w", peerID, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("peer %s returned %d", peerID, resp.StatusCode)
	}

	return nil
}

// OnMessage registers a handler for incoming sync messages.
func (t *HTTPSyncTransport) OnMessage(handler func(msg *SyncMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = handler
}

// ListPeers returns known peer instance IDs.
func (t *HTTPSyncTransport) ListPeers(_ context.Context) ([]string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ids := make([]string, 0, len(t.peers))
	for id := range t.peers {
		ids = append(ids, id)
	}
	return ids, nil
}

// AddPeer dynamically adds or updates a peer.
func (t *HTTPSyncTransport) AddPeer(peerID, baseURL string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.peers[peerID] = baseURL
}

// RemovePeer removes a peer from the known list.
func (t *HTTPSyncTransport) RemovePeer(peerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.peers, peerID)
}

// Close releases resources.
func (t *HTTPSyncTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.stopped {
		t.stopped = true
		close(t.stopCh)
	}
	return nil
}

// handleIncoming processes incoming sync messages from peers.
func (t *HTTPSyncTransport) handleIncoming(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	if t.authKey != "" && r.Header.Get("X-Sync-Key") != t.authKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var msg SyncMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	t.mu.RLock()
	handler := t.handler
	t.mu.RUnlock()

	if handler != nil {
		handler(&msg)
	} else {
		slog.Warn("sync transport: received message but no handler registered")
	}

	w.WriteHeader(http.StatusAccepted)
}
