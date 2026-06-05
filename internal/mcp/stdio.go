package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
)

// StdioProvider connects to an external MCP server via stdio (JSON-RPC 2.0).
type StdioProvider struct {
	cmd     string
	args    []string
	env     []string
	process *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	mu      sync.Mutex
	reqID   atomic.Int64
	running bool
}

// NewStdioProvider creates a provider that spawns an external MCP server process.
func NewStdioProvider(command string, args []string, env []string) *StdioProvider {
	return &StdioProvider{
		cmd:  command,
		args: args,
		env:  env,
	}
}

type jsonrpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Start launches the MCP server process and performs initialization.
func (s *StdioProvider) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.process = exec.CommandContext(ctx, s.cmd, s.args...)
	if len(s.env) > 0 {
		s.process.Env = s.env
	}

	stdin, err := s.process.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdin pipe: %w", err)
	}
	stdout, err := s.process.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdout pipe: %w", err)
	}
	s.stdin = stdin
	s.stdout = bufio.NewReader(stdout)

	if err := s.process.Start(); err != nil {
		return fmt.Errorf("mcp stdio: start process: %w", err)
	}
	s.running = true

	// Send initialize request
	_, err = s.call("initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "yunque-agent",
			"version": "1.0.0",
		},
	})
	if err != nil {
		s.Stop()
		return fmt.Errorf("mcp stdio: initialize: %w", err)
	}

	// Send initialized notification (no id, no response expected)
	s.notify("notifications/initialized", nil)

	slog.Info("mcp stdio provider started", "cmd", s.cmd)
	return nil
}

// Stop terminates the MCP server process.
func (s *StdioProvider) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.process != nil && s.process.Process != nil {
		s.process.Process.Kill()
		s.process.Wait()
	}
	slog.Info("mcp stdio provider stopped", "cmd", s.cmd)
}

// ListTools queries the MCP server for available tools.
func (s *StdioProvider) ListTools(ctx context.Context) ([]Tool, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil, fmt.Errorf("mcp stdio: not running")
	}

	result, err := s.call("tools/list", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("mcp stdio: parse tools: %w", err)
	}
	return resp.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (s *StdioProvider) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil, fmt.Errorf("mcp stdio: not running")
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
		// Treat raw text as success
		return SuccessResult(string(result)), nil
	}
	return &callResult, nil
}

func (s *StdioProvider) call(method string, params any) (json.RawMessage, error) {
	id := s.reqID.Add(1)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	if _, err := s.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read until we get the response whose id matches this request. A compliant
	// MCP server may interleave server→client notifications (method, no id) and
	// requests (method + id) on stdout between our request and its response —
	// e.g. server-everything emits logging notifications. The previous code
	// assumed the very next line was the response, so a single interleaved
	// notification produced an empty result and "unexpected end of JSON input".
	// Skip anything that is not our response instead of mistaking it for one.
	for {
		line, err := s.stdout.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Classify the message by the presence of id/method without committing
		// to a full decode: notifications have no id, server requests carry a
		// method, only a response has an id and no method.
		var probe struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(line, &probe); err != nil {
			// Not JSON-RPC (e.g. a stray stdout banner) — skip rather than fail.
			slog.Debug("mcp stdio: skipping non-jsonrpc line", "cmd", s.cmd)
			continue
		}
		if probe.ID == nil || probe.Method != "" {
			// Notification (no id) or server→client request (has method): not
			// our response. We don't reply to server requests; skipping keeps
			// the client unblocked and read-only.
			continue
		}
		if *probe.ID != id {
			// Stale/out-of-order response for a different request — skip.
			continue
		}

		var resp jsonrpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (s *StdioProvider) notify(method string, params any) {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	data = append(data, '\n')
	s.stdin.Write(data)
}
