// Package memorypack mounts the memory HTTP surface (/v1/memory/*) as a Pack
// Runtime backend module.
//
// Migration status: fully de-shelled. The pack owns route registration, the
// enable/disable gate AND every handler implementation. The read surface
// (stats/search), the write surface (add/compact) and the orchestrator-backed
// surface (recall/debug, persona, update) all run natively here, talking to the
// memory manager / pipeline / orchestrator through narrow injected deps — no
// gateway bridge remains.
package memorypack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.memory"

// MemoryReader is the narrow read surface the native stats/search handlers need.
type MemoryReader interface {
	Stats(tenantID string) map[string]int
	SearchAll(ctx context.Context, tenantID, query string, limit int) ([]memory.Item, error)
}

// Handler is the memory pack's backend module. Any injected dep may be nil (e.g.
// in tests that only exercise the route gates); the native handlers then degrade
// to a "not configured" response in that case.
type Handler struct {
	reader   MemoryReader
	tenantOf func(context.Context) string
	host     packruntime.Host
	started  atomic.Bool
	// orchOf resolves the memory orchestrator lazily (late-bound) so the wiring
	// order between the gateway and this pack does not matter.
	orchOf func() *memory.Orchestrator
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

// NewWired builds a fully-wired memory pack with every route served natively.
// mgr backs read + add (short/mid/long routing); pipe backs compact; orchOf
// resolves the orchestrator for recall/debug + persona; any may be nil (the
// corresponding native handler then reports "not configured"). It is the single
// wiring path shared by the host bootstrap and tests, so the native handlers
// behave identically in both.
func NewWired(mgr *memory.Manager, pipe *memory.Pipeline, orchOf func() *memory.Orchestrator, tenantOf func(context.Context) string) *Handler {
	h := &Handler{tenantOf: tenantOf, orchOf: orchOf}
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

// Routes mounts /v1/memory/* — every route is served natively by this pack.
func (h *Handler) Routes() []packruntime.BackendRoute {
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch,
	}
	mk := func(path string, handler http.HandlerFunc) packruntime.BackendRoute {
		return packruntime.BackendRoute{Methods: methods, Path: path, Handler: handler}
	}
	return []packruntime.BackendRoute{
		mk("/v1/memory/stats", h.handleStats),
		mk("/v1/memory/search", h.handleSearch),
		mk("/v1/memory/compact", h.handleCompact),
		mk("/v1/memory/add", h.handleAdd),
		mk("/v1/memory/recall/debug", h.handleRecallDebug),
		mk("/v1/memory/persona", h.handlePersonaGet),
		mk("/v1/memory/update", h.handlePersonaUpdate),
	}
}

// orchestrator resolves the late-bound memory orchestrator (may be nil).
func (h *Handler) orchestrator() *memory.Orchestrator {
	if h.orchOf == nil {
		return nil
	}
	return h.orchOf()
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

// handleRecallDebug exposes what the orchestrator recalls for a query (native,
// de-shelled from the gateway), scoped to the caller's tenant: per-item score
// breakdown (layer, raw/final score, access count, age) plus the final compiled
// context string that would be injected into the planner.
func (h *Handler) handleRecallDebug(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator()
	if orch == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "memory orchestrator not configured")
		return
	}
	tid := h.tenant(r)
	query := r.URL.Query().Get("q")
	limit := 10
	if r.Method == http.MethodPost {
		var req struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Query != "" {
			query = req.Query
		}
		if req.Limit > 0 {
			limit = req.Limit
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if query == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "query is required (q= or body.query)")
		return
	}

	items := orch.Recall(r.Context(), tid, query, limit)
	type debugItem struct {
		Content     string  `json:"content"`
		Source      string  `json:"source"`
		Category    string  `json:"category,omitempty"`
		Score       float64 `json:"score"`
		RawScore    float64 `json:"raw_score"`
		AccessCount int     `json:"access_count"`
		AgeSeconds  float64 `json:"age_seconds"`
	}
	out := make([]debugItem, 0, len(items))
	for _, it := range items {
		out = append(out, debugItem{
			Content:     it.Content,
			Source:      it.Source,
			Category:    it.Category,
			Score:       it.Score,
			RawScore:    it.RawScore,
			AccessCount: it.AccessCount,
			AgeSeconds:  it.Age.Seconds(),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tenant":           tid,
		"query":            query,
		"items":            out,
		"count":            len(out),
		"compiled_context": orch.CompileContext(r.Context(), tid, query),
	})
}

func (h *Handler) handlePersonaGet(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator()
	if orch == nil || orch.Editable() == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "memory orchestrator not configured")
		return
	}
	blocks := orch.Editable().AllBlocks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"blocks": blocks})
}

func (h *Handler) handlePersonaUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	orch := h.orchestrator()
	if orch == nil || orch.Editable() == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "memory orchestrator not configured")
		return
	}
	var req struct {
		ID      string `json:"id"`
		Label   string `json:"label"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeInvalidField, "invalid request")
		return
	}

	editable := orch.Editable()
	if req.ID == "" {
		if req.Content == "" {
			apperror.WriteCode(w, apperror.CodeInvalidField, "content cannot be empty for new block")
			return
		}
		label := fmt.Sprintf("%s-%d", req.Label, time.Now().UnixNano())
		editable.AddBlock(label, req.Content, 0)
	} else {
		targetLabel := req.Label
		if targetLabel == "" {
			for _, b := range editable.AllBlocks() {
				if b.ID == req.ID {
					targetLabel = b.Label
					break
				}
			}
		}

		if targetLabel == "" {
			apperror.WriteCode(w, apperror.CodeNotFound, "memory block not found")
			return
		}

		if req.Content == "" {
			editable.RemoveBlock(targetLabel)
		} else {
			res := editable.Edit(memory.EditRequest{
				BlockLabel: targetLabel,
				Op:         memory.OpRethink,
				NewText:    req.Content,
			})
			if !res.Success {
				apperror.WriteCode(w, apperror.CodeInternal, res.Error)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
