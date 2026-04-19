// Package server implements an MCP Server that exposes dispatch tools
// for external workers (Cursor, Claude Code, Windsurf, etc.) to claim
// and execute tasks orchestrated by the yunque planner.
//
// Wire protocol: Streamable HTTP — each JSON-RPC 2.0 request is a POST,
// response is returned in the HTTP body (application/json).
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// ToolHandler processes a single tool call and returns a result.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// ToolDef describes a tool the server exposes to MCP clients.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Handler     ToolHandler    `json:"-"`
}

// Server is a lightweight MCP server that exposes registered tools
// via the JSON-RPC 2.0 protocol used by MCP.
type Server struct {
	name    string
	version string

	mu    sync.RWMutex
	tools map[string]ToolDef
}

// New creates a new MCP server with the given identity.
func New(name, version string) *Server {
	return &Server{
		name:    name,
		version: version,
		tools:   make(map[string]ToolDef),
	}
}

// RegisterTool adds a tool to the server.
func (s *Server) RegisterTool(def ToolDef) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[def.Name] = def
}

// ──────────────────────────────────────────────────────────────
// JSON-RPC types
// ──────────────────────────────────────────────────────────────

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HandleRequest processes a raw JSON-RPC request and returns a response.
func (s *Server) HandleRequest(ctx context.Context, raw []byte) ([]byte, error) {
	var req jsonRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return s.marshalError(nil, -32700, "parse error")
	}
	if req.JSONRPC != "2.0" {
		return s.marshalError(req.ID, -32600, "invalid jsonrpc version")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized", "notifications/cancelled":
		return nil, nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.marshalResult(req.ID, map[string]any{"resources": []any{}})
	case "resources/read":
		return s.marshalError(req.ID, -32602, "no resources available")
	case "resources/templates/list":
		return s.marshalResult(req.ID, map[string]any{"resourceTemplates": []any{}})
	case "prompts/list":
		return s.marshalResult(req.ID, map[string]any{"prompts": []any{}})
	case "prompts/get":
		return s.marshalError(req.ID, -32602, "no prompts available")
	case "completion/complete":
		return s.marshalResult(req.ID, map[string]any{"completion": map[string]any{"values": []any{}}})
	case "ping":
		return s.marshalResult(req.ID, map[string]any{})
	default:
		slog.Debug("mcp server: unknown method", "method", req.Method)
		return s.marshalError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleInitialize(req jsonRPCRequest) ([]byte, error) {
	result := map[string]any{
		"protocolVersion": "2025-11-05",
		"capabilities": map[string]any{
			"tools":     map[string]any{"listChanged": false},
			"resources": map[string]any{"subscribe": false, "listChanged": false},
			"prompts":   map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":    s.name,
			"version": s.version,
		},
	}
	return s.marshalResult(req.ID, result)
}

func (s *Server) handleToolsList(req jsonRPCRequest) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	toolList := make([]map[string]any, 0, len(s.tools))
	for _, t := range s.tools {
		toolList = append(toolList, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return s.marshalResult(req.ID, map[string]any{"tools": toolList})
}

func (s *Server) handleToolsCall(ctx context.Context, req jsonRPCRequest) ([]byte, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.marshalError(req.ID, -32602, "invalid params")
		}
	}

	s.mu.RLock()
	tool, ok := s.tools[params.Name]
	s.mu.RUnlock()

	if !ok {
		return s.marshalError(req.ID, -32602, fmt.Sprintf("tool not found: %s", params.Name))
	}

	result, err := tool.Handler(ctx, params.Arguments)
	if err != nil {
		return s.marshalResult(req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		})
	}

	text := "ok"
	if result != nil {
		switch v := result.(type) {
		case string:
			text = v
		default:
			b, _ := json.Marshal(v)
			text = string(b)
		}
	}
	return s.marshalResult(req.ID, map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	})
}

func (s *Server) marshalResult(id json.RawMessage, result any) ([]byte, error) {
	resp := jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
	return json.Marshal(resp)
}

func (s *Server) marshalError(id json.RawMessage, code int, message string) ([]byte, error) {
	resp := jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &jsonRPCError{Code: code, Message: message}}
	return json.Marshal(resp)
}
