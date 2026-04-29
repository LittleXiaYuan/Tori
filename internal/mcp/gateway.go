package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Gateway federates tools from multiple providers with caching.
type Gateway struct {
	providers []Provider
	cacheTTL  time.Duration
	mu        sync.Mutex
	cached    *cachedRegistry
}

type cachedRegistry struct {
	registry  *Registry
	expiresAt time.Time
}

// NewGateway creates a tool gateway from a list of providers.
func NewGateway(providers []Provider, cacheTTL time.Duration) *Gateway {
	filtered := make([]Provider, 0, len(providers))
	for _, p := range providers {
		if p != nil {
			filtered = append(filtered, p)
		}
	}
	if cacheTTL <= 0 {
		cacheTTL = 10 * time.Second
	}
	return &Gateway{
		providers: filtered,
		cacheTTL:  cacheTTL,
	}
}

// ListTools returns all tools from all providers.
func (g *Gateway) ListTools(ctx context.Context) ([]Tool, error) {
	reg, err := g.getRegistry(ctx, false)
	if err != nil {
		return nil, err
	}
	return reg.List(), nil
}

// CallTool dispatches a tool call to the owning provider.
func (g *Gateway) CallTool(ctx context.Context, req CallRequest) (*CallResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ErrorResult("tool name is required"), nil
	}

	reg, err := g.getRegistry(ctx, false)
	if err != nil {
		return nil, err
	}

	provider, _, ok := reg.Lookup(name)
	if !ok {
		// Cache might be stale — force refresh once
		reg, err = g.getRegistry(ctx, true)
		if err != nil {
			return nil, err
		}
		provider, _, ok = reg.Lookup(name)
		if !ok {
			return ErrorResult("tool not found: " + name), nil
		}
	}

	args := req.Arguments
	if args == nil {
		args = map[string]any{}
	}

	result, err := provider.CallTool(ctx, name, args)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if result == nil {
		return SuccessResult(nil), nil
	}
	return result, nil
}

// AddProvider appends a provider and invalidates cache.
func (g *Gateway) AddProvider(p Provider) {
	if p == nil {
		return
	}
	g.mu.Lock()
	g.providers = append(g.providers, p)
	g.cached = nil
	g.mu.Unlock()
}

// ProtocolInfo returns MCP protocol metadata.
func (g *Gateway) ProtocolInfo() map[string]any {
	return map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities": map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":    "yunque-mcp-gateway",
			"version": "1.0.0",
		},
	}
}

func (g *Gateway) getRegistry(ctx context.Context, force bool) (*Registry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !force && g.cached != nil && time.Now().Before(g.cached.expiresAt) {
		return g.cached.registry, nil
	}

	reg := NewRegistry()
	for _, p := range g.providers {
		tools, err := p.ListTools(ctx)
		if err != nil {
			slog.Warn("mcp gateway: list tools failed", "err", err)
			continue
		}
		for _, tool := range tools {
			if err := reg.Register(p, tool); err != nil {
				slog.Debug("mcp gateway: skip tool", "tool", tool.Name, "err", err)
			}
		}
	}

	g.cached = &cachedRegistry{
		registry:  reg,
		expiresAt: time.Now().Add(g.cacheTTL),
	}
	return reg, nil
}

// ToolCount returns total registered tools.
func (g *Gateway) ToolCount(ctx context.Context) int {
	reg, err := g.getRegistry(ctx, false)
	if err != nil {
		return 0
	}
	return reg.Count()
}

// ToolsAsSkillDefs converts MCP tools to the format needed by the planner.
func (g *Gateway) ToolsAsSkillDefs(ctx context.Context) []map[string]any {
	tools, err := g.ListTools(ctx)
	if err != nil {
		return nil
	}
	defs := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		defs = append(defs, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.InputSchema,
		})
	}
	return defs
}

// WrapAsSkill creates a skill-compatible wrapper for a single MCP tool.
func (g *Gateway) WrapAsSkill(toolName string) func(ctx context.Context, args map[string]any) (string, error) {
	return func(ctx context.Context, args map[string]any) (string, error) {
		result, err := g.CallTool(ctx, CallRequest{Name: toolName, Arguments: args})
		if err != nil {
			return "", err
		}
		if result.IsError {
			if len(result.Content) > 0 {
				return "", fmt.Errorf("%s", result.Content[0].Text)
			}
			return "", fmt.Errorf("MCP tool %s returned error with no details", toolName)
		}
		var parts []string
		for _, c := range result.Content {
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
		return strings.Join(parts, "\n"), nil
	}
}
