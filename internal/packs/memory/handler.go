// Package memorypack mounts the memory HTTP surface (/v1/memory/*) as a Pack
// Runtime backend module.
//
// Migration status: the pack owns route registration + the enable/disable gate.
// The simple read surface (stats / search) has been "filled in" — its handler
// implementations now live here and talk to the memory manager directly
// (decoupled from the gateway), with tenant resolution injected via tenantOf.
// The remaining routes (recall/debug, add, compact, persona, update) depend on
// the orchestrator / pipeline and stay on the gateway bridge for now.
package memorypack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.memory"

// MemoryGateway is the narrow gateway surface the still-bridged memory routes need.
type MemoryGateway interface {
	HandleMemoryPack(w http.ResponseWriter, r *http.Request)
}

// MemoryReader is the narrow read surface the native handlers need.
type MemoryReader interface {
	Stats(tenantID string) map[string]int
	SearchAll(ctx context.Context, tenantID, query string, limit int) ([]memory.Item, error)
}

// Handler is the memory pack's backend module. reader/tenantOf may be nil (e.g.
// in tests that only exercise the route gates); the native handlers degrade to a
// "not configured" response in that case.
type Handler struct {
	gateway  MemoryGateway
	reader   MemoryReader
	tenantOf func(context.Context) string
	host     packruntime.Host
	started  atomic.Bool
	// compact persists/compacts memory via the host pipeline (injected so the
	// pack owns the handler without importing the concrete pipeline).
	compact func(ctx context.Context, tenantID string, targetCount, decayDays int) (any, error)
	// add writes a memory item at a layer (injected; owns short/mid/long routing).
	add func(ctx context.Context, tenantID string, item memory.Item, layer string) error
}

// SetCompact injects the memory-compaction hook used by the native
// /v1/memory/compact handler (de-shelled from the gateway).
func (h *Handler) SetCompact(fn func(ctx context.Context, tenantID string, targetCount, decayDays int) (any, error)) {
	h.compact = fn
}

// SetAdd injects the memory-write hook used by the native /v1/memory/add
// handler. The closure owns the layer routing (short/mid/long) so the pack stays
// decoupled from the concrete memory manager.
func (h *Handler) SetAdd(fn func(ctx context.Context, tenantID string, item memory.Item, layer string) error) {
	h.add = fn
}

// NewHandler builds the memory pack backed only by the gateway bridge.
func NewHandler(gateway MemoryGateway) *Handler { return &Handler{gateway: gateway} }

// NewHandlerWithService builds the memory pack with the read service wired, so
// the stats/search surface is served natively by this package.
func NewHandlerWithService(gateway MemoryGateway, reader MemoryReader, tenantOf func(context.Context) string) *Handler {
	return &Handler{gateway: gateway, reader: reader, tenantOf: tenantOf}
}

// NewWired builds a fully-wired memory pack: native stats/search/add/compact
// (de-shelled from the gateway). It is the single wiring path shared by the host
// bootstrap and tests, so the native handlers behave identically in both. mgr
// backs read + add (short/mid/long routing); pipe backs compact; either may be
// nil (the corresponding native handler then reports "not configured").
func NewWired(gateway MemoryGateway, mgr *memory.Manager, pipe *memory.Pipeline, tenantOf func(context.Context) string) *Handler {
	h := &Handler{gateway: gateway, tenantOf: tenantOf}
	if mgr != nil {
		h.reader = mgr
		h.add = func(ctx context.Context, tenantID string, item memory.Item, layer string) error {
			switch layer {
			case "long":
				return mgr.AddLong(ctx, tenantID, item)
			case "short":
				return mgr.Short.Put(ctx, tenantID, item)
			default:
				return mgr.AddMid(ctx, tenantID, item)
			}
		}
	}
	if pipe != nil {
		h.compact = func(ctx context.Context, tenantID string, targetCount, decayDays int) (any, error) {
			return pipe.Compact(ctx, tenantID, targetCount, decayDays)
		}
	}
	return h
}

// PackID returns the stable manifest id.
func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Memory is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

// Init wires the pack against the kernel Host (deps arrive via narrow interfaces).
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("memory pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts /v1/memory/*. stats/search are served natively; the rest are
// dispatched to the gateway bridge during this migration phase.
func (h *Handler) Routes() []packruntime.BackendRoute {
	bridge := h.gateway.HandleMemoryPack
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch,
	}
	mk := func(path string, handler http.HandlerFunc) packruntime.BackendRoute {
		return packruntime.BackendRoute{Methods: methods, Path: path, Handler: handler}
	}
	return []packruntime.BackendRoute{
		// Native (filled-in) read surface.
		mk("/v1/memory/stats", h.handleStats),
		mk("/v1/memory/search", h.handleSearch),
		// compact + add are de-shelled — served natively here via injected hooks.
		mk("/v1/memory/compact", h.handleCompact),
		mk("/v1/memory/add", h.handleAdd),
		// Still bridged to the gateway (orchestrator / editable-memory dependent).
		mk("/v1/memory/recall/debug", bridge),
		mk("/v1/memory/persona", bridge),
		mk("/v1/memory/update", bridge),
	}
}

// handleAdd writes a memory item natively (de-shelled from the gateway): it
// builds the item and delegates layer routing to the injected add hook.
func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Layer  string `json:"layer"` // "short", "mid", "long"
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Value == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "value is required")
		return
	}
	if h.add == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "memory manager not configured")
		return
	}
	item := memory.Item{Key: req.Key, Value: req.Value, Source: req.Source}
	if err := h.add(r.Context(), h.tenant(r), item, req.Layer); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeStorageError, "memory add failed", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleCompact compacts memory natively (de-shelled from the gateway): it
// delegates to the injected compaction hook. Returns 500 when no hook is wired.
func (h *Handler) handleCompact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var req struct {
		TargetCount int `json:"target_count"`
		DecayDays   int `json:"decay_days"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.TargetCount < 0 {
		req.TargetCount = 0
	}
	if h.compact == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "memory pipeline not configured")
		return
	}
	result, err := h.compact(r.Context(), h.tenant(r), req.TargetCount, req.DecayDays)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "compact failed", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) tenant(r *http.Request) string {
	if h.tenantOf == nil {
		return ""
	}
	return h.tenantOf(r.Context())
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.reader == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "memory not configured"})
		return
	}
	_ = json.NewEncoder(w).Encode(h.reader.Stats(h.tenant(r)))
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.reader == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "memory not configured"})
		return
	}
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Limit <= 0 {
		req.Limit = 10
	}
	items, _ := h.reader.SearchAll(r.Context(), h.tenant(r), req.Query, req.Limit)
	_ = json.NewEncoder(w).Encode(map[string]any{"results": items, "count": len(items)})
}
