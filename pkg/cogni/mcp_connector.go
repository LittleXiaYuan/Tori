package cogni

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
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
	return c.connectWithRetry(ctx, def)
}

const (
	mcpMaxRetries     = 3
	mcpInitialBackoff = 500 * time.Millisecond
	mcpMaxBackoff     = 5 * time.Second
)

func (c *StdioMCPConnector) connectWithRetry(ctx context.Context, def MCPServerDef) (MCPConnection, error) {
	var lastErr error
	backoff := mcpInitialBackoff

	for attempt := 0; attempt <= mcpMaxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("mcp connector: retrying connection",
				"server", def.Command, "attempt", attempt, "backoff", backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > mcpMaxBackoff {
				backoff = mcpMaxBackoff
			}
		}

		conn, err := c.factory(ctx, def)
		if err == nil {
			if attempt > 0 {
				slog.Info("mcp connector: reconnected after retry", "server", def.Command, "attempts", attempt+1)
			}
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("mcp connector: failed after %d attempts: %w", mcpMaxRetries+1, lastErr)
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
