package mcp

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type registryEntry struct {
	provider Provider
	tool     Tool
}

// Registry manages tool providers and their tools with thread-safe access.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]registryEntry
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]registryEntry)}
}

// Register adds a tool with its provider. Returns error if name already taken.
func (r *Registry) Register(provider Provider, tool Tool) error {
	name := strings.TrimSpace(tool.Name)
	if name == "" {
		return fmt.Errorf("tool name is required")
	}
	if provider == nil {
		return fmt.Errorf("provider is required for tool %s", name)
	}
	if tool.InputSchema == nil {
		tool.InputSchema = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}
	tool.Name = name
	r.entries[name] = registryEntry{provider: provider, tool: tool}
	return nil
}

// Unregister removes a tool by name.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, strings.TrimSpace(name))
}

// Lookup finds a tool's provider and descriptor by name.
func (r *Registry) Lookup(name string) (Provider, Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[strings.TrimSpace(name)]
	if !ok {
		return nil, Tool{}, false
	}
	return entry.provider, entry.tool, true
}

// List returns all registered tools sorted by name.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	tools := make([]Tool, 0, len(names))
	for _, name := range names {
		tools = append(tools, r.entries[name].tool)
	}
	return tools
}

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// Clear removes all entries.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = make(map[string]registryEntry)
}
