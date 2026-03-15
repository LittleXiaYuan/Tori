package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// ServerConfig describes a single external MCP server to connect to.
type ServerConfig struct {
	// Transport type: "stdio", "sse", or "streamable_http"
	Transport string `json:"transport"`

	// For stdio: command and arguments
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`

	// For sse / streamable_http: URL and optional headers
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	// Timeout for HTTP transports (seconds). Default: 30.
	Timeout int `json:"timeout,omitempty"`

	// Active flag (default true if omitted)
	Active *bool `json:"active,omitempty"`
}

// MCPConfig is the top-level config loaded from data/mcp.json.
type MCPConfig struct {
	Servers map[string]ServerConfig `json:"mcpServers"`
}

// LoadConfig reads MCP server configuration from a JSON file.
// The format is compatible with the MCP standard mcpServers format.
func LoadConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse mcp config: %w", err)
	}
	return &cfg, nil
}

// ConnectAll creates providers from config, starts them, and adds them to the gateway.
// Returns the count of successfully connected servers.
func ConnectAll(ctx context.Context, cfg *MCPConfig, gw *Gateway) int {
	if cfg == nil || len(cfg.Servers) == 0 {
		return 0
	}

	connected := 0
	for name, sc := range cfg.Servers {
		// Skip inactive servers
		if sc.Active != nil && !*sc.Active {
			slog.Info("mcp server skipped (inactive)", "name", name)
			continue
		}

		provider, err := createProvider(ctx, name, sc)
		if err != nil {
			slog.Warn("mcp server connect failed", "name", name, "err", err)
			continue
		}

		gw.AddProvider(provider)
		connected++
		slog.Info("mcp server connected", "name", name, "transport", sc.Transport)
	}
	return connected
}

// createProvider creates and starts a Provider from a ServerConfig.
func createProvider(ctx context.Context, name string, sc ServerConfig) (Provider, error) {
	transport := strings.ToLower(strings.TrimSpace(sc.Transport))
	if transport == "" {
		// Auto-detect: if URL is set use streamable_http, if command is set use stdio
		if sc.URL != "" {
			transport = "streamable_http"
		} else if sc.Command != "" {
			transport = "stdio"
		} else {
			return nil, fmt.Errorf("server %q: transport not specified and cannot be auto-detected", name)
		}
	}

	timeout := time.Duration(sc.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	switch transport {
	case "stdio":
		if sc.Command == "" {
			return nil, fmt.Errorf("server %q: stdio transport requires command", name)
		}
		p := NewStdioProvider(sc.Command, sc.Args, sc.Env)
		if err := p.Start(ctx); err != nil {
			return nil, err
		}
		return p, nil

	case "sse":
		if sc.URL == "" {
			return nil, fmt.Errorf("server %q: sse transport requires url", name)
		}
		p := NewSSEProvider(sc.URL, sc.Headers, timeout)
		if err := p.Start(ctx); err != nil {
			return nil, err
		}
		return p, nil

	case "streamable_http":
		if sc.URL == "" {
			return nil, fmt.Errorf("server %q: streamable_http transport requires url", name)
		}
		p := NewStreamableHTTPProvider(sc.URL, sc.Headers, timeout)
		if err := p.Start(ctx); err != nil {
			return nil, err
		}
		return p, nil

	default:
		return nil, fmt.Errorf("server %q: unknown transport %q", name, transport)
	}
}
