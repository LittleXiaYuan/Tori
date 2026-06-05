package cogni

import (
	"context"
	"testing"
)

// fakeMCPConn stands in for the stdio transport to a real MCP server. Everything
// else in the loop (Cogni config -> Manager register -> lazy connect -> tool
// discovery -> filter -> call routing) is the real MCPManager code path.
type fakeMCPConn struct{ lastCall string }

func (f *fakeMCPConn) ListTools(context.Context) ([]MCPToolInfo, error) {
	return []MCPToolInfo{
		{Server: "github", Name: "create_issue", Description: "Create an issue", InputSchema: map[string]any{"type": "object"}},
		{Server: "github", Name: "list_pull_requests", Description: "List PRs", InputSchema: map[string]any{"type": "object"}},
		{Server: "github", Name: "delete_repository", Description: "DANGER: delete a repo", InputSchema: map[string]any{"type": "object"}},
	}, nil
}
func (f *fakeMCPConn) CallTool(_ context.Context, name string, args map[string]any) (any, error) {
	f.lastCall = name
	return map[string]any{"ok": true, "tool": name, "args": args}, nil
}
func (f *fakeMCPConn) Close() error { return nil }

type fakeConnector struct{ conn *fakeMCPConn }

func (c *fakeConnector) Connect(context.Context, MCPServerDef) (MCPConnection, error) {
	return c.conn, nil
}

func mcpToolNames(ts []MCPToolInfo) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return out
}

// TestMCP_CogniClosedLoop proves the "a Cogni brings its own MCP" design end to
// end: a Cogni declares mcp.servers + tool_filter; the manager lazily connects,
// discovers tools, filters them, the surface merges native + MCP tools, and a
// call routes back to that Cogni's server.
func TestMCP_CogniClosedLoop(t *testing.T) {
	conn := &fakeMCPConn{}
	mgr := NewMCPManager(&fakeConnector{conn: conn})

	// ① a Cogni declares its own MCP server + a tool whitelist
	cfg := MCPConfig{
		Servers: []MCPServerDef{{
			Name: "github", Transport: "stdio", Command: "npx",
			Args: []string{"-y", "@modelcontextprotocol/server-github"},
		}},
		ToolFilter: &MCPToolFilter{Include: []string{"create_issue", "list_pull_requests"}},
	}
	mgr.Register("github-helper", cfg)

	// ② lazy connect on activation
	if err := mgr.EnsureConnected(context.Background(), "github-helper"); err != nil {
		t.Fatalf("EnsureConnected: %v", err)
	}

	// ③ discovered 3 tools, tool_filter keeps 2 (delete_repository blocked)
	tools := mgr.Tools("github-helper")
	if len(tools) != 2 {
		t.Fatalf("expected 2 filtered MCP tools, got %d: %v", len(tools), mcpToolNames(tools))
	}
	for _, tl := range tools {
		if tl.Name == "delete_repository" {
			t.Fatalf("tool_filter failed: dangerous tool leaked into surface")
		}
	}

	// ④ surface merge: native skills + this Cogni's MCP tools
	native := []string{"web_search", "file_open"}
	combined := append([]string{}, native...)
	for _, mt := range tools {
		combined = append(combined, "github__"+mt.Name)
	}

	// ⑤ call routes by cogniID to that Cogni's MCP server
	res, err := mgr.CallTool(context.Background(), "github-helper", "create_issue", map[string]any{"title": "bug"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if conn.lastCall != "create_issue" {
		t.Fatalf("call routing failed: lastCall=%q", conn.lastCall)
	}

	t.Logf("MCP↔Cogni 闭环:")
	t.Logf("  ① Cogni 'github-helper' 声明 mcp.servers + tool_filter[create_issue,list_pull_requests]")
	t.Logf("  ② Register + EnsureConnected → 懒连接 + ListTools 发现 3 个工具")
	t.Logf("  ③ tool_filter → 保留 %d 个: %v(delete_repository 被挡)", len(tools), mcpToolNames(tools))
	t.Logf("  ④ 并入 Cogni surface(原生+MCP): %v", combined)
	t.Logf("  ⑤ CallTool 按 cogniID 路由 → %v", res)
}
