package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// StreamableHTTPProvider connects to an MCP server via Streamable HTTP transport.
// Unlike SSE, each JSON-RPC request is a POST and the response is returned directly
// in the HTTP response body (Content-Type: application/json).
type StreamableHTTPProvider struct {
	url     string
	headers map[string]string
	client  *http.Client

	mu          sync.Mutex
	reqID       atomic.Int64
	initialized bool
	sessionID   string // MCP session ID from Mcp-Session header
}

// NewStreamableHTTPProvider creates a provider using the Streamable HTTP transport.
func NewStreamableHTTPProvider(url string, headers map[string]string, timeout time.Duration) *StreamableHTTPProvider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &StreamableHTTPProvider{
		url:     url,
		headers: headers,
		client:  &http.Client{Timeout: timeout},
	}
}

// Start performs the MCP initialization handshake.
func (p *StreamableHTTPProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	result, sessionID, err := p.post(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "yunque-agent",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return fmt.Errorf("mcp streamable-http: initialize: %w", err)
	}
	_ = result
	p.sessionID = sessionID

	// Send initialized notification
	p.notify(ctx, "notifications/initialized", nil)

	p.initialized = true
	slog.Info("mcp streamable-http provider started", "url", p.url, "session", p.sessionID)
	return nil
}

// Stop marks the provider as not initialized.
func (p *StreamableHTTPProvider) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.initialized = false
	p.sessionID = ""
	slog.Info("mcp streamable-http provider stopped", "url", p.url)
}

// ListTools queries the MCP server for available tools.
func (p *StreamableHTTPProvider) ListTools(ctx context.Context) ([]Tool, error) {
	p.mu.Lock()
	ok := p.initialized
	p.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("mcp streamable-http: not initialized")
	}

	result, _, err := p.post(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("mcp streamable-http: parse tools: %w", err)
	}
	return resp.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (p *StreamableHTTPProvider) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	p.mu.Lock()
	ok := p.initialized
	p.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("mcp streamable-http: not initialized")
	}

	result, _, err := p.post(ctx, "tools/call", map[string]any{
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

// post sends a JSON-RPC request and reads the response.
// Returns the result, any session ID from the response, and an error.
func (p *StreamableHTTPProvider) post(ctx context.Context, method string, params any) (json.RawMessage, string, error) {
	id := p.reqID.Add(1)
	reqBody := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	p.mu.Lock()
	sid := p.sessionID
	p.mu.Unlock()
	if sid != "" {
		httpReq.Header.Set("Mcp-Session", sid)
	}
	for k, v := range p.headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("post request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("post returned status %d", httpResp.StatusCode)
	}

	// Extract session ID
	sessionID := httpResp.Header.Get("Mcp-Session")

	// Read response body
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(httpResp.Body); err != nil {
		return nil, sessionID, fmt.Errorf("read response: %w", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		return nil, sessionID, fmt.Errorf("parse response: %w", err)
	}
	if resp.Error != nil {
		return nil, sessionID, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, sessionID, nil
}

// notify sends a one-way notification (no response expected).
func (p *StreamableHTTPProvider) notify(ctx context.Context, method string, params any) {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	data, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	p.mu.Lock()
	notifySID := p.sessionID
	p.mu.Unlock()
	if notifySID != "" {
		req.Header.Set("Mcp-Session", notifySID)
	}
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}
	resp, err := p.client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
