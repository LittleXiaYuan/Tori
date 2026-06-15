package gateway

// host_adapter.go — Tier 0 microkernel: *Gateway implements packruntime.Host
// (see doc/MICROKERNEL-PACK-BLUEPRINT.md, Phase 0).
//
// This inverts the pack→Gateway dependency: capability packs depend on the
// narrow packruntime.Host contract, and the Gateway satisfies it here by
// delegating to its existing subsystems. It is purely additive — nothing in the
// current boot path changes; new packs can start consuming Host immediately.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"yunque-agent/pkg/packruntime"
)

// gatewayHost is the kernel Host view of a Gateway.
type gatewayHost struct{ g *Gateway }

// Host returns this Gateway as a packruntime.Host (the Tier 0 kernel contract).
func (g *Gateway) Host() packruntime.Host { return gatewayHost{g: g} }

// compile-time assertion that the adapter satisfies the contract.
var _ packruntime.Host = gatewayHost{}

func (h gatewayHost) Handle(pattern string, fn http.HandlerFunc) {
	h.g.mux.HandleFunc(pattern, fn)
}

func (h gatewayHost) RequireAuth(fn http.HandlerFunc) http.HandlerFunc {
	return h.g.requireAuth(fn)
}

func (h gatewayHost) Logger() *slog.Logger { return slog.Default() }

func (h gatewayHost) LLM() packruntime.LLMPort { return hostLLM{g: h.g} }

func (h gatewayHost) KV() packruntime.KVPort { return hostKV{g: h.g} }

func (h gatewayHost) Events() packruntime.EventBus { return hostEvents{} }

// Service is the escape hatch for capabilities not yet exposed as a typed port.
// Reserved for the migration: first-class ports are added per step; until then
// this returns (nil, false) so a pack degrades gracefully instead of coupling to
// the concrete Gateway.
func (h gatewayHost) Service(string) (any, bool) { return nil, false }

// hostLLM bridges packruntime.LLMPort to the gateway's cost-aware LLM call.
type hostLLM struct{ g *Gateway }

func (l hostLLM) Chat(ctx context.Context, system, user string) (string, error) {
	if l.g.llmCall == nil {
		return "", fmt.Errorf("kernel: llm not wired")
	}
	return l.g.llmCall(ctx, system, user)
}

// hostKV bridges packruntime.KVPort to the gateway's pack-scoped Ledger KV.
type hostKV struct{ g *Gateway }

func (k hostKV) Get(ctx context.Context, key string, out any) (bool, error) {
	if k.g.wasmPackKV == nil {
		return false, fmt.Errorf("kernel: kv not wired")
	}
	raw, err := k.g.wasmPackKV.GetRaw(ctx, key)
	if err != nil {
		return false, err
	}
	if len(raw) == 0 {
		return false, nil
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (k hostKV) Put(ctx context.Context, key string, val any) error {
	if k.g.wasmPackKV == nil {
		return fmt.Errorf("kernel: kv not wired")
	}
	return k.g.wasmPackKV.Put(ctx, key, val)
}

// hostEvents is a minimal EventBus that records pack events to the kernel log.
// A richer bus can replace this without changing the pack-facing contract.
type hostEvents struct{}

func (hostEvents) Emit(kind string, payload map[string]any) {
	slog.Debug("kernel pack event", "kind", kind, "payload", payload)
}
