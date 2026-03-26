package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// SSE Event Stream — real-time task/workflow/approval events
//
// GET /v1/events/stream
//
// Streams Server-Sent Events for:
//   - task.* : task lifecycle events
//   - workflow.* : workflow execution events
//   - approval.* : new approval requests
//
// Usage:
//   const es = new EventSource("/v1/events/stream");
//   es.addEventListener("task.step_completed", (e) => { ... });
//   es.addEventListener("approval.request", (e) => { ... });
// ──────────────────────────────────────────────

// SSEBroker manages SSE client connections and event broadcasting.
type SSEBroker struct {
	mu      sync.RWMutex
	clients map[string]chan SSEEvent // clientID → event channel
	counter int
}

// SSEEvent is an event to be sent to all SSE clients.
type SSEEvent struct {
	Type string `json:"type"` // e.g. "task.step_completed", "approval.request"
	Data any    `json:"data"`
}

// NewSSEBroker creates a new SSE broker.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[string]chan SSEEvent),
	}
}

// Subscribe adds a new SSE client and returns its event channel + cleanup func.
func (b *SSEBroker) Subscribe() (string, <-chan SSEEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.counter++
	id := fmt.Sprintf("sse-%d", b.counter)
	ch := make(chan SSEEvent, 64)
	b.clients[id] = ch
	cleanup := func() {
		b.mu.Lock()
		delete(b.clients, id)
		b.mu.Unlock()
		// Drain remaining events
		for range ch {
		}
	}
	return id, ch, cleanup
}

// Broadcast sends an event to all connected clients.
func (b *SSEBroker) Broadcast(event SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for id, ch := range b.clients {
		select {
		case ch <- event:
		default:
			slog.Warn("sse: client buffer full, dropping event", "client", id, "type", event.Type)
		}
	}
}

// ClientCount returns the number of connected SSE clients.
func (b *SSEBroker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// handleSSEStream is the SSE endpoint.
// GET /v1/events/stream
func (g *Gateway) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	if g.sseBroker == nil {
		http.Error(w, "SSE not available", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx proxy support

	clientID, ch, cleanup := g.sseBroker.Subscribe()
	defer cleanup()

	slog.Info("sse: client connected", "client", clientID)

	// Send initial heartbeat
	fmt.Fprintf(w, "event: connected\ndata: {\"client_id\":%q,\"time\":%q}\n\n",
		clientID, time.Now().Format(time.RFC3339))
	flusher.Flush()

	// Keep-alive ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			slog.Info("sse: client disconnected", "client", clientID)
			return

		case event := <-ch:
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()

		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive %s\n\n", time.Now().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}
