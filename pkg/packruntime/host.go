package packruntime

// host.go — Tier 0 microkernel contract (see doc/MICROKERNEL-PACK-BLUEPRINT.md).
//
// Host is the narrow kernel interface a capability pack depends on, instead of
// reaching into the concrete *gateway.Gateway. It inverts the dependency: packs
// import this contract; the Gateway implements it (internal/controlplane/gateway
// /host_adapter.go). Ports are deliberately minimal — extend by adding new
// methods/ports (never change existing signatures), or use Service() as the
// escape hatch for capabilities not yet first-classed.

import (
	"context"
	"log/slog"
	"net/http"
)

// Host is the kernel surface exposed to capability packs.
type Host interface {
	// Handle mounts an HTTP handler on the kernel mux.
	Handle(pattern string, h http.HandlerFunc)
	// RequireAuth wraps a handler with the kernel's standard auth middleware.
	RequireAuth(h http.HandlerFunc) http.HandlerFunc
	// Logger returns the kernel logger.
	Logger() *slog.Logger

	// LLM returns the shared LLM port (foreground chat tier).
	LLM() LLMPort
	// KV returns a pack-scoped persistent key/value port.
	KV() KVPort
	// Events returns the kernel event bus port.
	Events() EventBus

	// Service is the escape hatch for capabilities not yet exposed as a typed
	// port. Returns (nil, false) when the named service is unavailable. New
	// first-class ports are added per migration step; until then a pack can
	// resolve a host service by name without coupling to *gateway.Gateway.
	Service(name string) (any, bool)
}

// LLMPort is the minimal LLM capability a pack may use. It mirrors
// workflow.LLMCallFunc so the Gateway can satisfy it with zero glue.
type LLMPort interface {
	Chat(ctx context.Context, system, user string) (string, error)
}

// KVPort is a pack-scoped persistent key/value store backed by the kernel's
// Ledger KV. Values are JSON-encoded by the host adapter.
type KVPort interface {
	Get(ctx context.Context, key string, out any) (found bool, err error)
	Put(ctx context.Context, key string, val any) error
}

// EventBus lets a pack emit kernel events (observability / cross-pack signals).
type EventBus interface {
	Emit(kind string, payload map[string]any)
}
