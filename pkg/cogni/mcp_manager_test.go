package cogni

import (
	"context"
	"fmt"
	"testing"
)

type mockConnection struct {
	tools  []MCPToolInfo
	closed bool
}

func (m *mockConnection) ListTools(ctx context.Context) ([]MCPToolInfo, error) {
	return m.tools, nil
}

func (m *mockConnection) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	return map[string]any{"tool": name, "called": true}, nil
}

func (m *mockConnection) Close() error {
	m.closed = true
	return nil
}

type mockConnector struct {
	connections map[string]*mockConnection
}

func (c *mockConnector) Connect(ctx context.Context, def MCPServerDef) (MCPConnection, error) {
	if conn, ok := c.connections[def.Name]; ok {
		return conn, nil
	}
	return nil, fmt.Errorf("unknown server: %s", def.Name)
}

func TestMCPManager_RegisterAndConnect(t *testing.T) {
	connector := &mockConnector{
		connections: map[string]*mockConnection{
			"github": {
				tools: []MCPToolInfo{
					{Server: "github", Name: "github_list_prs"},
					{Server: "github", Name: "github_create_issue"},
				},
			},
		},
	}

	mgr := NewMCPManager(connector)
	mgr.Register("code-reviewer", MCPConfig{
		Servers: []MCPServerDef{{Name: "github"}},
	})

	err := mgr.EnsureConnected(context.Background(), "code-reviewer")
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	tools := mgr.Tools("code-reviewer")
	if len(tools) != 2 {
		t.Errorf("tools = %d, want 2", len(tools))
	}
}

func TestMCPManager_ToolFilter_Include(t *testing.T) {
	connector := &mockConnector{
		connections: map[string]*mockConnection{
			"github": {
				tools: []MCPToolInfo{
					{Server: "github", Name: "github_list_prs"},
					{Server: "github", Name: "github_create_issue"},
					{Server: "github", Name: "read_file"},
				},
			},
		},
	}

	mgr := NewMCPManager(connector)
	mgr.Register("reviewer", MCPConfig{
		Servers: []MCPServerDef{{Name: "github"}},
		ToolFilter: &MCPToolFilter{
			Include: []string{"github_*"},
		},
	})

	mgr.EnsureConnected(context.Background(), "reviewer")

	tools := mgr.Tools("reviewer")
	if len(tools) != 2 {
		t.Errorf("tools = %d, want 2 (github_* only)", len(tools))
	}
}

func TestMCPManager_ToolFilter_Exclude(t *testing.T) {
	connector := &mockConnector{
		connections: map[string]*mockConnection{
			"srv": {
				tools: []MCPToolInfo{
					{Server: "srv", Name: "safe_tool"},
					{Server: "srv", Name: "shell_exec"},
					{Server: "srv", Name: "file_write"},
				},
			},
		},
	}

	mgr := NewMCPManager(connector)
	mgr.Register("safe-agent", MCPConfig{
		Servers: []MCPServerDef{{Name: "srv"}},
		ToolFilter: &MCPToolFilter{
			Exclude: []string{"shell_exec", "file_write"},
		},
	})

	mgr.EnsureConnected(context.Background(), "safe-agent")

	tools := mgr.Tools("safe-agent")
	if len(tools) != 1 || tools[0].Name != "safe_tool" {
		t.Errorf("tools = %v, want [safe_tool]", tools)
	}
}

func TestMCPManager_CallTool(t *testing.T) {
	connector := &mockConnector{
		connections: map[string]*mockConnection{
			"srv": {
				tools: []MCPToolInfo{
					{Server: "srv", Name: "my_tool"},
				},
			},
		},
	}

	mgr := NewMCPManager(connector)
	mgr.Register("agent", MCPConfig{
		Servers: []MCPServerDef{{Name: "srv"}},
	})
	mgr.EnsureConnected(context.Background(), "agent")

	result, err := mgr.CallTool(context.Background(), "agent", "my_tool", nil)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if m, ok := result.(map[string]any); !ok || m["tool"] != "my_tool" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestMCPManager_CallTool_NotFound(t *testing.T) {
	mgr := NewMCPManager(nil)
	mgr.Register("agent", MCPConfig{})
	mgr.connections["agent"].ready = true

	_, err := mgr.CallTool(context.Background(), "agent", "missing", nil)
	if err == nil {
		t.Error("expected error for missing tool")
	}
}

func TestMCPManager_Unregister(t *testing.T) {
	conn := &mockConnection{tools: []MCPToolInfo{{Server: "s", Name: "t"}}}
	connector := &mockConnector{connections: map[string]*mockConnection{"s": conn}}

	mgr := NewMCPManager(connector)
	mgr.Register("agent", MCPConfig{Servers: []MCPServerDef{{Name: "s"}}})
	mgr.EnsureConnected(context.Background(), "agent")

	mgr.Unregister("agent")
	if !conn.closed {
		t.Error("connection should be closed on unregister")
	}
	if tools := mgr.Tools("agent"); tools != nil {
		t.Error("tools should be nil after unregister")
	}
}

func TestMCPManager_Status(t *testing.T) {
	connector := &mockConnector{
		connections: map[string]*mockConnection{
			"srv": {tools: []MCPToolInfo{{Server: "srv", Name: "t"}}},
		},
	}

	mgr := NewMCPManager(connector)
	mgr.Register("agent", MCPConfig{
		Servers: []MCPServerDef{{Name: "srv"}},
	})
	mgr.EnsureConnected(context.Background(), "agent")

	statuses := mgr.Status("agent")
	if len(statuses) != 1 {
		t.Fatalf("statuses = %d, want 1", len(statuses))
	}
	if !statuses[0].Connected {
		t.Error("should be connected")
	}
}

func TestMCPManager_NotRegistered(t *testing.T) {
	mgr := NewMCPManager(nil)
	err := mgr.EnsureConnected(context.Background(), "unknown")
	if err == nil {
		t.Error("expected error for unregistered cogni")
	}
}

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		want     bool
	}{
		{"exact", []string{"exact"}, true},
		{"wildcard", []string{"github_*"}, false},
		{"github_pr", []string{"github_*"}, true},
		{"no match", []string{"other"}, false},
	}
	for _, tt := range tests {
		if got := matchesAny(tt.name, tt.patterns); got != tt.want {
			t.Errorf("matchesAny(%q, %v) = %v, want %v", tt.name, tt.patterns, got, tt.want)
		}
	}
}

type failingConnector struct{}

func (failingConnector) Connect(ctx context.Context, def MCPServerDef) (MCPConnection, error) {
	return nil, fmt.Errorf("dial %s: connection refused", def.Name)
}

func TestMCPManager_AllServersFail(t *testing.T) {
	mgr := NewMCPManager(failingConnector{})
	mgr.Register("broken", MCPConfig{Servers: []MCPServerDef{{Name: "a"}, {Name: "b"}}})

	// Regression: a cogni whose every MCP server is unreachable must report an
	// error, not silently look "connected".
	err := mgr.EnsureConnected(context.Background(), "broken")
	if err == nil {
		t.Fatal("expected error when all MCP servers fail to connect")
	}

	// And it must stay not-ready so a later call can retry the dead servers.
	mgr.mu.RLock()
	st := mgr.connections["broken"]
	mgr.mu.RUnlock()
	st.mu.Lock()
	ready := st.ready
	st.mu.Unlock()
	if ready {
		t.Error("state should not be ready after a total connect failure")
	}
}
