package cogni

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// MCPServerDef declares an MCP server connection specific to a Cogni.
type MCPServerDef struct {
	Name      string            `json:"name" yaml:"name"`
	Transport string            `json:"transport,omitempty" yaml:"transport,omitempty"` // stdio | sse | streamable_http
	Command   string            `json:"command,omitempty" yaml:"command,omitempty"`
	Args      []string          `json:"args,omitempty" yaml:"args,omitempty"`
	URL       string            `json:"url,omitempty" yaml:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Headers   map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Timeout   int               `json:"timeout,omitempty" yaml:"timeout,omitempty"` // seconds
}

// MCPConfig is the per-Cogni MCP configuration.
type MCPConfig struct {
	Servers    []MCPServerDef `json:"servers,omitempty" yaml:"servers,omitempty"`
	ToolFilter *MCPToolFilter `json:"tool_filter,omitempty" yaml:"tool_filter,omitempty"`
}

// MCPToolFilter restricts which MCP tools are visible to the Cogni.
type MCPToolFilter struct {
	Include []string `json:"include,omitempty" yaml:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

// MCPToolInfo describes a tool provided by a connected MCP server.
type MCPToolInfo struct {
	Server      string         `json:"server"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// MCPServerStatus reports the connection state of a Cogni's MCP server.
type MCPServerStatus struct {
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	ToolCount int    `json:"tool_count"`
	Error     string `json:"error,omitempty"`
}

// MCPConnector is the interface the host provides to actually connect
// to MCP servers. This decouples the cogni package from the mcp package.
type MCPConnector interface {
	Connect(ctx context.Context, def MCPServerDef) (MCPConnection, error)
}

// MCPConnection represents a live connection to an MCP server.
type MCPConnection interface {
	ListTools(ctx context.Context) ([]MCPToolInfo, error)
	CallTool(ctx context.Context, name string, args map[string]any) (any, error)
	Close() error
}

// MCPManager manages per-Cogni MCP server connections with lazy initialization.
// Each Cogni gets its own isolated set of MCP connections.
type MCPManager struct {
	mu          sync.RWMutex
	connector   MCPConnector
	connections map[string]*cogniMCPState // keyed by cogni ID
}

type cogniMCPState struct {
	mu      sync.Mutex
	config  MCPConfig
	servers map[string]MCPConnection // keyed by server name
	tools   []MCPToolInfo
	ready   bool
}

func NewMCPManager(connector MCPConnector) *MCPManager {
	return &MCPManager{
		connector:   connector,
		connections: make(map[string]*cogniMCPState),
	}
}

// Register declares the MCP config for a Cogni. Does not connect yet (lazy).
func (m *MCPManager) Register(cogniID string, cfg MCPConfig) {
	m.mu.Lock()

	old, hasOld := m.connections[cogniID]
	newState := &cogniMCPState{
		config:  cfg,
		servers: make(map[string]MCPConnection),
	}
	m.connections[cogniID] = newState
	m.mu.Unlock()

	if hasOld {
		old.mu.Lock()
		old.close()
		old.mu.Unlock()
	}
	slog.Debug("mcp_manager: registered",
		"cogni", cogniID,
		"servers", len(cfg.Servers),
	)
}

// Unregister closes all MCP connections for a Cogni and removes it.
func (m *MCPManager) Unregister(cogniID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.connections[cogniID]; ok {
		state.close()
		delete(m.connections, cogniID)
	}
}

// EnsureConnected lazily connects all servers for a Cogni if not already done.
func (m *MCPManager) EnsureConnected(ctx context.Context, cogniID string) error {
	m.mu.RLock()
	state, ok := m.connections[cogniID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("mcp_manager: cogni %q not registered", cogniID)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.ready {
		return nil
	}

	if m.connector == nil {
		return fmt.Errorf("mcp_manager: no connector configured")
	}

	var connectErrs []error
	for _, def := range state.config.Servers {
		if _, exists := state.servers[def.Name]; exists {
			continue
		}

		conn, err := m.connector.Connect(ctx, def)
		if err != nil {
			slog.Warn("mcp_manager: connect failed",
				"cogni", cogniID,
				"server", def.Name,
				"err", err,
			)
			connectErrs = append(connectErrs, fmt.Errorf("%s: %w", def.Name, err))
			continue
		}
		state.servers[def.Name] = conn

		tools, err := conn.ListTools(ctx)
		if err != nil {
			slog.Warn("mcp_manager: list tools failed",
				"cogni", cogniID,
				"server", def.Name,
				"err", err,
			)
			connectErrs = append(connectErrs, fmt.Errorf("%s: list tools: %w", def.Name, err))
			continue
		}
		// Stamp the owning server name so CallTool can route back to the right
		// connection. A connection bridge maps the provider's tools generically
		// and usually can't know which configured server it represents, so the
		// manager — which iterates per def — fills it in. Respect a Server the
		// connection already set (e.g. multiplexed/aggregating connections).
		for i := range tools {
			if tools[i].Server == "" {
				tools[i].Server = def.Name
			}
		}
		state.tools = append(state.tools, applyToolFilter(tools, state.config.ToolFilter)...)
	}

	// Only mark ready once every configured server is connected. A partial
	// connection stays not-ready so a later EnsureConnected retries the
	// missing servers instead of freezing a degraded surface as "done".
	state.ready = len(state.servers) == len(state.config.Servers)

	// Surface a hard failure when nothing connected at all. Previously this
	// path always returned nil and set ready=true, so a cogni whose every MCP
	// server was unreachable looked successfully connected to callers.
	if len(state.config.Servers) > 0 && len(state.servers) == 0 {
		return fmt.Errorf("mcp_manager: cogni %q: all %d MCP server(s) failed to connect: %w",
			cogniID, len(state.config.Servers), errors.Join(connectErrs...))
	}

	slog.Info("mcp_manager: connected",
		"cogni", cogniID,
		"servers", len(state.servers),
		"configured", len(state.config.Servers),
		"tools", len(state.tools),
	)
	return nil
}

// Tools returns the filtered tool list for a Cogni.
func (m *MCPManager) Tools(cogniID string) []MCPToolInfo {
	m.mu.RLock()
	state, ok := m.connections[cogniID]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	return state.tools
}

// Status returns connection status for all servers of a Cogni.
func (m *MCPManager) Status(cogniID string) []MCPServerStatus {
	m.mu.RLock()
	state, ok := m.connections[cogniID]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	var out []MCPServerStatus
	for _, def := range state.config.Servers {
		st := MCPServerStatus{Name: def.Name}
		if conn, ok := state.servers[def.Name]; ok && conn != nil {
			st.Connected = true
			if tools, err := conn.ListTools(context.Background()); err == nil {
				st.ToolCount = len(tools)
			}
		}
		out = append(out, st)
	}
	return out
}

// CallTool routes a tool call to the correct MCP server for a Cogni.
func (m *MCPManager) CallTool(ctx context.Context, cogniID, toolName string, args map[string]any) (any, error) {
	m.mu.RLock()
	state, ok := m.connections[cogniID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("mcp_manager: cogni %q not registered", cogniID)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	for _, t := range state.tools {
		if t.Name == toolName {
			conn, ok := state.servers[t.Server]
			if !ok {
				return nil, fmt.Errorf("mcp_manager: server %q not connected", t.Server)
			}
			return conn.CallTool(ctx, toolName, args)
		}
	}
	return nil, fmt.Errorf("mcp_manager: tool %q not found for cogni %q", toolName, cogniID)
}

// CloseAll shuts down all MCP connections for all Cognis.
func (m *MCPManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, state := range m.connections {
		state.close()
		delete(m.connections, id)
	}
}

func (s *cogniMCPState) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, conn := range s.servers {
		if err := conn.Close(); err != nil {
			slog.Warn("mcp_manager: close error", "server", name, "err", err)
		}
	}
	s.servers = make(map[string]MCPConnection)
	s.tools = nil
	s.ready = false
}

func applyToolFilter(tools []MCPToolInfo, filter *MCPToolFilter) []MCPToolInfo {
	if filter == nil {
		return tools
	}

	var result []MCPToolInfo
	for _, t := range tools {
		if len(filter.Include) > 0 && !matchesAny(t.Name, filter.Include) {
			continue
		}
		if matchesAny(t.Name, filter.Exclude) {
			continue
		}
		result = append(result, t)
	}
	return result
}

func matchesAny(name string, patterns []string) bool {
	for _, p := range patterns {
		if p == name {
			return true
		}
		// Simple wildcard: "github_*" matches "github_create_issue"
		if len(p) > 0 && p[len(p)-1] == '*' {
			prefix := p[:len(p)-1]
			if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}
