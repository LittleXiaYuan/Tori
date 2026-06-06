package cogni

import (
	"context"
	"fmt"
	"testing"
	"time"
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

// TestMCPManager_ReRegisterDoesNotDeadlock guards the self-deadlock fix:
// Register closing the old state must NOT hold old.mu while calling close()
// (close() locks it itself). Re-register happens on every runtime sync.
func TestMCPManager_ReRegisterDoesNotDeadlock(t *testing.T) {
	connector := &mockConnector{connections: map[string]*mockConnection{
		"github": {tools: []MCPToolInfo{{Server: "github", Name: "t1"}}},
	}}
	mgr := NewMCPManager(connector)
	cfg := MCPConfig{Servers: []MCPServerDef{{Name: "github"}}}
	mgr.Register("r", cfg)
	if err := mgr.EnsureConnected(context.Background(), "r"); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// A CHANGED config forces the old.close() teardown path (the deadlock site),
	// past the idempotent-skip guard.
	cfg2 := MCPConfig{
		Servers:    []MCPServerDef{{Name: "github"}},
		ToolFilter: &MCPToolFilter{Include: []string{"t1"}},
	}
	done := make(chan struct{})
	go func() {
		mgr.Register("r", cfg2) // hasOld=true → old.close(); must not deadlock
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("re-register deadlocked")
	}
	if err := mgr.EnsureConnected(context.Background(), "r"); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
}

// TestMCPManager_ReRegisterSameConfigKeepsConnection guards the idempotency
// optimization: re-registering the identical config (the common per-sync case)
// must keep the live connection instead of tearing it down and respawning.
func TestMCPManager_ReRegisterSameConfigKeepsConnection(t *testing.T) {
	conn := &mockConnection{tools: []MCPToolInfo{{Server: "github", Name: "t1"}}}
	connector := &mockConnector{connections: map[string]*mockConnection{"github": conn}}
	mgr := NewMCPManager(connector)
	cfg := MCPConfig{Servers: []MCPServerDef{{Name: "github"}}}
	mgr.Register("r", cfg)
	if err := mgr.EnsureConnected(context.Background(), "r"); err != nil {
		t.Fatalf("connect: %v", err)
	}

	mgr.Register("r", cfg) // identical → must be a no-op
	if conn.closed {
		t.Fatal("identical re-register should not close the live connection")
	}
	if got := len(mgr.Tools("r")); got != 1 {
		t.Fatalf("tools lost after idempotent re-register: %d", got)
	}
}

type flakyListConn struct {
	calls int
	tools []MCPToolInfo
}

func (f *flakyListConn) ListTools(context.Context) ([]MCPToolInfo, error) {
	f.calls++
	if f.calls == 1 {
		return nil, fmt.Errorf("transient list failure")
	}
	return f.tools, nil
}
func (f *flakyListConn) CallTool(context.Context, string, map[string]any) (any, error) {
	return nil, nil
}
func (f *flakyListConn) Close() error { return nil }

type flakyListConnector struct{ conn MCPConnection }

func (c flakyListConnector) Connect(context.Context, MCPServerDef) (MCPConnection, error) {
	return c.conn, nil
}

// TestMCPManager_ListToolsFailureThenRetrySucceeds guards the stuck-server fix:
// a ListTools failure after a successful connect must drop the half-connected
// server so a later EnsureConnected retries it (instead of stranding it
// tool-less forever).
func TestMCPManager_ListToolsFailureThenRetrySucceeds(t *testing.T) {
	conn := &flakyListConn{tools: []MCPToolInfo{{Server: "s", Name: "ok_tool"}}}
	mgr := NewMCPManager(flakyListConnector{conn: conn})
	mgr.Register("f", MCPConfig{Servers: []MCPServerDef{{Name: "s"}}})

	if err := mgr.EnsureConnected(context.Background(), "f"); err == nil {
		t.Fatal("expected first connect to fail on ListTools")
	}
	if got := len(mgr.Tools("f")); got != 0 {
		t.Fatalf("expected no tools after failure, got %d", got)
	}

	if err := mgr.EnsureConnected(context.Background(), "f"); err != nil {
		t.Fatalf("retry should succeed, got %v", err)
	}
	if got := len(mgr.Tools("f")); got != 1 {
		t.Fatalf("expected 1 tool after retry, got %d", got)
	}
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
