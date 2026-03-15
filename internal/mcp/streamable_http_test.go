package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockMCPStreamableServer creates a test HTTP server that simulates an MCP Streamable HTTP server.
func mockMCPStreamableServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req jsonrpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session", "test-session-123")

		switch req.Method {
		case "initialize":
			resp := jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}`),
			}
			json.NewEncoder(w).Encode(resp)

		case "notifications/initialized":
			w.WriteHeader(http.StatusOK)

		case "tools/list":
			resp := jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"tools":[{"name":"echo","description":"echoes input","inputSchema":{"type":"object","properties":{"msg":{"type":"string"}}}},{"name":"add","description":"adds numbers","inputSchema":{"type":"object"}}]}`),
			}
			json.NewEncoder(w).Encode(resp)

		case "tools/call":
			params, _ := json.Marshal(req.Params)
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			json.Unmarshal(params, &p)

			result := CallResult{
				Content: []ContentBlock{{Type: "text", Text: "called:" + p.Name}},
			}
			resultJSON, _ := json.Marshal(result)

			resp := jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(resultJSON),
			}
			json.NewEncoder(w).Encode(resp)

		default:
			resp := jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &jsonrpcError{Code: -32601, Message: "method not found"},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
}

func TestStreamableHTTPProvider_StartAndList(t *testing.T) {
	srv := mockMCPStreamableServer(t)
	defer srv.Close()

	p := NewStreamableHTTPProvider(srv.URL, nil, 5*time.Second)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	// Verify initialized
	p.mu.Lock()
	if !p.initialized {
		t.Fatal("expected initialized=true")
	}
	if p.sessionID != "test-session-123" {
		t.Errorf("sessionID: got %q, want test-session-123", p.sessionID)
	}
	p.mu.Unlock()

	tools, err := p.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("tools count: got %d, want 2", len(tools))
	}
	if tools[0].Name != "echo" || tools[1].Name != "add" {
		t.Errorf("tools: got %v", tools)
	}
}

func TestStreamableHTTPProvider_CallTool(t *testing.T) {
	srv := mockMCPStreamableServer(t)
	defer srv.Close()

	p := NewStreamableHTTPProvider(srv.URL, nil, 5*time.Second)
	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	result, err := p.CallTool(ctx, "echo", map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result.Content)
	}
	if len(result.Content) == 0 || result.Content[0].Text != "called:echo" {
		t.Fatalf("result: got %v", result.Content)
	}
}

func TestStreamableHTTPProvider_NotInitialized(t *testing.T) {
	p := NewStreamableHTTPProvider("http://localhost:1", nil, time.Second)

	_, err := p.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected error for uninitialized provider")
	}

	_, err = p.CallTool(context.Background(), "echo", nil)
	if err == nil {
		t.Fatal("expected error for uninitialized provider")
	}
}

func TestStreamableHTTPProvider_Stop(t *testing.T) {
	srv := mockMCPStreamableServer(t)
	defer srv.Close()

	p := NewStreamableHTTPProvider(srv.URL, nil, 5*time.Second)
	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	p.Stop()

	p.mu.Lock()
	if p.initialized {
		t.Error("expected initialized=false after stop")
	}
	if p.sessionID != "" {
		t.Error("expected empty sessionID after stop")
	}
	p.mu.Unlock()

	// Calls after stop should fail
	_, err := p.ListTools(ctx)
	if err == nil {
		t.Fatal("expected error after stop")
	}
}

func TestStreamableHTTPProvider_DoubleStart(t *testing.T) {
	srv := mockMCPStreamableServer(t)
	defer srv.Close()

	p := NewStreamableHTTPProvider(srv.URL, nil, 5*time.Second)
	ctx := context.Background()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer p.Stop()

	// Second start should be a no-op
	if err := p.Start(ctx); err != nil {
		t.Fatalf("second Start: %v", err)
	}
}

func TestStreamableHTTPProvider_CustomHeaders(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		resp := jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	headers := map[string]string{"Authorization": "Bearer test-token"}
	p := NewStreamableHTTPProvider(srv.URL, headers, 5*time.Second)

	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	if receivedAuth != "Bearer test-token" {
		t.Errorf("Authorization header: got %q, want %q", receivedAuth, "Bearer test-token")
	}
}

func TestStreamableHTTPProvider_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error:   &jsonrpcError{Code: -32600, Message: "invalid request"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewStreamableHTTPProvider(srv.URL, nil, 5*time.Second)
	err := p.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for RPC error response")
	}
	if p.initialized {
		t.Error("should not be initialized after error")
	}
}

func TestStreamableHTTPProvider_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewStreamableHTTPProvider(srv.URL, nil, 5*time.Second)
	err := p.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestStreamableHTTPProvider_AsProvider(t *testing.T) {
	srv := mockMCPStreamableServer(t)
	defer srv.Close()

	p := NewStreamableHTTPProvider(srv.URL, nil, 5*time.Second)
	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	// Use through Gateway to verify Provider interface
	gw := NewGateway([]Provider{p}, 5*time.Second)
	tools, err := gw.ListTools(ctx)
	if err != nil {
		t.Fatalf("gateway ListTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("tools: got %d, want 2", len(tools))
	}

	result, err := gw.CallTool(ctx, CallRequest{Name: "echo", Arguments: map[string]any{"msg": "test"}})
	if err != nil {
		t.Fatalf("gateway CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
}
