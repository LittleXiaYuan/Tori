package cogni

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// StdioMCPConnector implements MCPConnector by spawning MCP server processes
// via stdio transport or connecting via Streamable HTTP, bridging
// cogni.MCPServerDef to the host's MCP infrastructure.
//
// The actual process management is delegated to a factory function provided
// by the host (cmd/agent), keeping this package free of internal/* imports.
type StdioMCPConnector struct {
	factory MCPConnectionFactory
}

// MCPConnectionFactory creates a live MCPConnection from a server definition.
// The host provides this — it bridges to internal/mcp.StdioProvider or
// StreamableHTTPProvider depending on the transport field.
type MCPConnectionFactory func(ctx context.Context, def MCPServerDef) (MCPConnection, error)

// NewStdioMCPConnector creates a connector using the provided factory.
func NewStdioMCPConnector(factory MCPConnectionFactory) *StdioMCPConnector {
	return &StdioMCPConnector{factory: factory}
}

func (c *StdioMCPConnector) Connect(ctx context.Context, def MCPServerDef) (MCPConnection, error) {
	if c.factory == nil {
		return nil, fmt.Errorf("mcp connector: no factory configured")
	}
	return c.factory(ctx, def)
}

// ResolveEnv replaces ${VAR} references in a map with os.Getenv values.
func ResolveEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
			envKey := v[2 : len(v)-1]
			v = os.Getenv(envKey)
		}
		out = append(out, k+"="+v)
	}
	return out
}
