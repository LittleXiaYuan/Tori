package plugin

import (
	"context"
	"log/slog"
	"sync"
)

// Hook event names for Agent lifecycle.
const (
	HookAgentStart   = "agent.start"
	HookAgentStop    = "agent.stop"
	HookChatBefore   = "chat.before"
	HookChatAfter    = "chat.after"
	HookMemoryExtract = "memory.extract"
	HookPluginLoad   = "plugin.load"
	HookPluginUnload = "plugin.unload"
)

// HookPayload carries event data to hook handlers.
type HookPayload struct {
	Event  string         `json:"event"`
	Data   map[string]any `json:"data,omitempty"`
}

// HookHandler processes a hook event. Returns modified data or error.
type HookHandler func(ctx context.Context, payload HookPayload) error

// hookEntry ties a handler to its source plugin.
type hookEntry struct {
	pluginName string
	handler    HookHandler
}

// HookManager dispatches lifecycle events to registered plugin handlers.
type HookManager struct {
	mu       sync.RWMutex
	handlers map[string][]hookEntry // event -> handlers
}

// NewHookManager creates a hook manager.
func NewHookManager() *HookManager {
	return &HookManager{handlers: make(map[string][]hookEntry)}
}

// Register adds a hook handler for an event from a specific plugin.
func (hm *HookManager) Register(event, pluginName string, handler HookHandler) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.handlers[event] = append(hm.handlers[event], hookEntry{
		pluginName: pluginName,
		handler:    handler,
	})
	slog.Debug("hook registered", "event", event, "plugin", pluginName)
}

// UnregisterPlugin removes all hooks for a given plugin.
func (hm *HookManager) UnregisterPlugin(pluginName string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	for event, entries := range hm.handlers {
		filtered := entries[:0]
		for _, e := range entries {
			if e.pluginName != pluginName {
				filtered = append(filtered, e)
			}
		}
		hm.handlers[event] = filtered
	}
}

// Emit fires an event to all registered handlers sequentially.
// Returns on first error (fail-fast). Use EmitAll for best-effort.
func (hm *HookManager) Emit(ctx context.Context, event string, data map[string]any) error {
	hm.mu.RLock()
	entries := make([]hookEntry, len(hm.handlers[event]))
	copy(entries, hm.handlers[event])
	hm.mu.RUnlock()

	if len(entries) == 0 {
		return nil
	}

	payload := HookPayload{Event: event, Data: data}
	for _, e := range entries {
		if err := e.handler(ctx, payload); err != nil {
			slog.Warn("hook handler error", "event", event, "plugin", e.pluginName, "err", err)
			return err
		}
	}
	return nil
}

// EmitAll fires an event to all handlers, collecting errors but not stopping.
func (hm *HookManager) EmitAll(ctx context.Context, event string, data map[string]any) []error {
	hm.mu.RLock()
	entries := make([]hookEntry, len(hm.handlers[event]))
	copy(entries, hm.handlers[event])
	hm.mu.RUnlock()

	payload := HookPayload{Event: event, Data: data}
	var errs []error
	for _, e := range entries {
		if err := e.handler(ctx, payload); err != nil {
			slog.Warn("hook handler error", "event", event, "plugin", e.pluginName, "err", err)
			errs = append(errs, err)
		}
	}
	return errs
}

// Events returns all registered event names.
func (hm *HookManager) Events() []string {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	out := make([]string, 0, len(hm.handlers))
	for event, entries := range hm.handlers {
		if len(entries) > 0 {
			out = append(out, event)
		}
	}
	return out
}

// HandlersFor returns plugin names registered for a given event.
func (hm *HookManager) HandlersFor(event string) []string {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	entries := hm.handlers[event]
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.pluginName
	}
	return out
}
