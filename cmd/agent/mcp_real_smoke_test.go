package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcpkg "yunque-agent/internal/mcp"
	"yunque-agent/pkg/cogni"
)

// TestRealMCPServerSmoke drives a REAL MCP server (npx @modelcontextprotocol/
// server-everything) through the exact production transport path used by the
// Cogni runtime: cogni.MCPManager -> StdioMCPConnector -> internal/mcp.
// StdioProvider -> mcpProviderBridge. It proves the per-Cogni MCP wiring works
// against a live server process (connect -> list tools -> call tool), not a fake.
//
// Skipped by default — it spawns a Node process and may download the package on
// first run. Enable with: YQ_REAL_MCP=1 go test ./cmd/agent -run TestRealMCPServerSmoke -v
func TestRealMCPServerSmoke(t *testing.T) {
	if os.Getenv("YQ_REAL_MCP") == "" {
		t.Skip("set YQ_REAL_MCP=1 to run the real MCP server smoke test (spawns npx)")
	}

	// Same connector the agent builds in cogniModule.Init.
	connector := cogni.NewStdioMCPConnector(func(ctx context.Context, def cogni.MCPServerDef) (cogni.MCPConnection, error) {
		provider := mcpkg.NewStdioProvider(def.Command, def.Args, cogni.ResolveEnv(def.Env))
		if err := provider.Start(ctx); err != nil {
			return nil, err
		}
		return &mcpProviderBridge{provider: provider}, nil
	})

	// Drive the whole path from the SHIPPED example declaration so this proves
	// the example actually runs: declaration.mcp → MCPManager.Register →
	// EnsureConnected (real npx) → tool_filter narrowing → CallTool routing.
	declPath := filepath.Join("..", "..", "examples", "cognis", "mcp-demo-assistant.json")
	raw, err := os.ReadFile(declPath)
	if err != nil {
		t.Fatalf("read example declaration: %v", err)
	}
	var decl cogni.Declaration
	if err := json.Unmarshal(raw, &decl); err != nil {
		t.Fatalf("parse example declaration: %v", err)
	}

	mgr := cogni.NewMCPManager(connector)
	defer mgr.CloseAll()

	mgr.Register(decl.ID, decl.MCP)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := mgr.EnsureConnected(ctx, decl.ID); err != nil {
		t.Fatalf("EnsureConnected against real MCP server: %v", err)
	}

	tools := mgr.Tools(decl.ID)
	if len(tools) == 0 {
		t.Fatal("real MCP server returned no tools")
	}
	names := make(map[string]bool, len(tools))
	for _, tl := range tools {
		names[tl.Name] = true
	}
	t.Logf("example cogni %q surfaced %d MCP tools (after tool_filter): %v", decl.ID, len(tools), mcpSmokeNames(tools))

	// The declaration's tool_filter.include must have narrowed the 13-tool server
	// down to exactly its allowlist.
	if len(tools) != len(decl.MCP.ToolFilter.Include) {
		t.Fatalf("tool_filter should narrow to %d tools, got %d: %v",
			len(decl.MCP.ToolFilter.Include), len(tools), mcpSmokeNames(tools))
	}
	if !names["echo"] {
		t.Fatalf("expected 'echo' in filtered tools, got %v", mcpSmokeNames(tools))
	}

	out, err := mgr.CallTool(ctx, decl.ID, "echo", map[string]any{"message": "hello-yunque"})
	if err != nil {
		t.Fatalf("CallTool echo on real server: %v", err)
	}
	t.Logf("real MCP echo result via example cogni: %v", out)
}

func mcpSmokeNames(tools []cogni.MCPToolInfo) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.Name
	}
	return out
}
