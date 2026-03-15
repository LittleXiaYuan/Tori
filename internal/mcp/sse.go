package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SSEProvider connects to a remote MCP server via SSE (Server-Sent Events) transport.
// It implements the MCP SSE protocol: POST JSON-RPC to an endpoint, receive responses via SSE stream.
type SSEProvider struct {
	endpoint string
	headers  map[string]string
	client   *http.Client

	mu          sync.Mutex
	reqID       atomic.Int64
	initialized bool
	messagesURL string // discovered from SSE endpoint event

	pendingMu sync.Mutex
	pending   map[int64]chan *jsonrpcResponse

	cancelSSE context.CancelFunc
}

// NewSSEProvider creates a provider that connects to a remote MCP server via SSE.
func NewSSEProvider(endpoint string, headers map[string]string, timeout time.Duration) *SSEProvider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &SSEProvider{
		endpoint: endpoint,
		headers:  headers,
		client:   &http.Client{Timeout: timeout},
		pending:  make(map[int64]chan *jsonrpcResponse),
	}
}

// Start connects to the SSE endpoint, discovers the messages URL, and performs initialization.
func (s *SSEProvider) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	// Connect to SSE stream to discover the messages endpoint
	sseCtx, cancel := context.WithCancel(ctx)
	s.cancelSSE = cancel

	req, err := http.NewRequestWithContext(sseCtx, http.MethodGet, s.endpoint, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("mcp sse: create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		cancel()
		return fmt.Errorf("mcp sse: connect: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		return fmt.Errorf("mcp sse: unexpected status %d", resp.StatusCode)
	}

	// Read the first SSE event to discover the messages URL
	messagesURL, err := s.readEndpointEvent(resp.Body)
	if err != nil {
		resp.Body.Close()
		cancel()
		return fmt.Errorf("mcp sse: discover endpoint: %w", err)
	}

	// Resolve relative URL against the base endpoint
	s.messagesURL = resolveURL(s.endpoint, messagesURL)

	// Start background goroutine to read SSE responses
	go s.readSSELoop(resp.Body)

	s.initialized = true

	// Perform initialization handshake (unlock because call will use pendingMu)
	s.mu.Unlock()
	_, err = s.call("initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "yunque-agent",
			"version": "1.0.0",
		},
	})
	s.mu.Lock()

	if err != nil {
		s.initialized = false
		cancel()
		return fmt.Errorf("mcp sse: initialize: %w", err)
	}

	// Send initialized notification
	s.postNotification("notifications/initialized", nil)

	slog.Info("mcp sse provider started", "endpoint", s.endpoint, "messages_url", s.messagesURL)
	return nil
}

// Stop closes the SSE connection.
func (s *SSEProvider) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelSSE != nil {
		s.cancelSSE()
		s.cancelSSE = nil
	}
	s.initialized = false
	// Unblock any pending requests
	s.pendingMu.Lock()
	for id, ch := range s.pending {
		close(ch)
		delete(s.pending, id)
	}
	s.pendingMu.Unlock()
	slog.Info("mcp sse provider stopped", "endpoint", s.endpoint)
}

// ListTools queries the MCP server for available tools.
func (s *SSEProvider) ListTools(ctx context.Context) ([]Tool, error) {
	s.mu.Lock()
	ok := s.initialized
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("mcp sse: not initialized")
	}

	result, err := s.call("tools/list", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("mcp sse: parse tools: %w", err)
	}
	return resp.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (s *SSEProvider) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	s.mu.Lock()
	ok := s.initialized
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("mcp sse: not initialized")
	}

	result, err := s.call("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	var callResult CallResult
	if err := json.Unmarshal(result, &callResult); err != nil {
		return SuccessResult(string(result)), nil
	}
	return &callResult, nil
}

// call sends a JSON-RPC request via HTTP POST and waits for SSE response.
func (s *SSEProvider) call(method string, params any) (json.RawMessage, error) {
	id := s.reqID.Add(1)

	// Create response channel
	ch := make(chan *jsonrpcResponse, 1)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()

	reqBody := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// POST to messages URL
	httpReq, err := http.NewRequest(http.MethodPost, s.messagesURL, bytes.NewReader(data))
	if err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("create post request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range s.headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("post request: %w", err)
	}
	httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("post returned status %d", httpResp.StatusCode)
	}

	// Wait for SSE response
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	var resp *jsonrpcResponse
	select {
	case resp = <-ch:
	case <-timer.C:
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("mcp sse: timeout waiting for response")
	}

	if resp == nil {
		return nil, fmt.Errorf("mcp sse: connection closed")
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, nil
}

// postNotification sends a one-way JSON-RPC notification.
func (s *SSEProvider) postNotification(method string, params any) {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	data, _ := json.Marshal(msg)
	req, err := http.NewRequest(http.MethodPost, s.messagesURL, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// readEndpointEvent reads SSE events until it finds an "endpoint" event.
func (s *SSEProvider) readEndpointEvent(body io.Reader) (string, error) {
	scanner := bufio.NewScanner(body)
	var eventType, eventData string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			eventData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		} else if line == "" && eventType != "" {
			// Empty line = end of event
			if eventType == "endpoint" {
				return eventData, nil
			}
			eventType = ""
			eventData = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("SSE stream ended without endpoint event")
}

// readSSELoop continuously reads SSE events and dispatches responses to pending requests.
func (s *SSEProvider) readSSELoop(body io.ReadCloser) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var eventType, eventData string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			eventData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		} else if line == "" && eventType != "" {
			if eventType == "message" && eventData != "" {
				s.handleSSEMessage(eventData)
			}
			eventType = ""
			eventData = ""
		}
	}
}

// handleSSEMessage parses a JSON-RPC response from SSE and dispatches it.
func (s *SSEProvider) handleSSEMessage(data string) {
	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		slog.Debug("mcp sse: parse message failed", "err", err, "data", data)
		return
	}

	s.pendingMu.Lock()
	ch, ok := s.pending[resp.ID]
	if ok {
		delete(s.pending, resp.ID)
	}
	s.pendingMu.Unlock()

	if ok && ch != nil {
		ch <- &resp
	}
}

// resolveURL resolves a potentially relative URL against a base URL.
func resolveURL(base, target string) string {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	// Extract scheme + host from base
	idx := strings.Index(base, "://")
	if idx < 0 {
		return target
	}
	afterScheme := base[idx+3:]
	slashIdx := strings.Index(afterScheme, "/")
	if slashIdx < 0 {
		return base + target
	}
	return base[:idx+3+slashIdx] + target
}
