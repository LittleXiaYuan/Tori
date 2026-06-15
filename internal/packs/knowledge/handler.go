// Package knowledgepack mounts the knowledge-base (RAG) HTTP surface as a Pack
// Runtime backend module.
//
// Migration status: the pack owns route registration + the enable/disable gate.
// The read surface (search / sources / stats) has been "filled in" — its handler
// implementations now live in this package and talk to the knowledge store
// directly (decoupled from the gateway). The write/ingest/import surface is
// still served by the gateway during the bridge phase (consumed via the narrow
// KnowledgeGateway interface) and will be filled in later.
package knowledgepack

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/safego"
)

const PackID = "yunque.pack.knowledge"

// KnowledgeGateway is the narrow gateway surface the still-bridged knowledge
// write/ingest/import routes need.
type KnowledgeGateway interface {
	HandleKnowledgePack(w http.ResponseWriter, r *http.Request)
}

// Handler is the knowledge pack's backend module. store may be nil (e.g. in
// tests that only exercise the route gates); the native read handlers degrade to
// a "not configured" response in that case.
type Handler struct {
	gateway KnowledgeGateway
	store   *knowledge.Store
	host    packruntime.Host
	started atomic.Bool
}

// NewHandler builds the knowledge pack backed only by the gateway bridge (no
// native store wiring). Used by tests that exercise route gating.
func NewHandler(gateway KnowledgeGateway) *Handler { return &Handler{gateway: gateway} }

// NewHandlerWithStore builds the knowledge pack with the knowledge store wired,
// so the read surface is served natively by this package.
func NewHandlerWithStore(gateway KnowledgeGateway, store *knowledge.Store) *Handler {
	return &Handler{gateway: gateway, store: store}
}

// PackID returns the stable manifest id.
func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Knowledge is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

// Init wires the pack against the kernel Host (deps arrive via the narrow
// KnowledgeGateway interface + store, not the concrete Gateway).
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("knowledge pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the knowledge surface. The read routes are served by this
// package's native handlers; the write/ingest/import routes are still dispatched
// to the gateway bridge during this migration phase. Methods are declared
// broadly so the manifest's path-only routes keep the original permissive
// (handler-decides) behavior.
func (h *Handler) Routes() []packruntime.BackendRoute {
	bridge := h.gateway.HandleKnowledgePack
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch,
	}
	mk := func(path string, handler http.HandlerFunc) packruntime.BackendRoute {
		return packruntime.BackendRoute{Methods: methods, Path: path, Handler: handler}
	}
	return []packruntime.BackendRoute{
		// Native (filled-in) read surface.
		mk("/v1/knowledge/search", h.handleSearch),
		mk("/v1/knowledge/sources", h.handleSources),
		mk("/v1/knowledge/stats", h.handleStats),
		// /v1/knowledge/ingest is de-shelled — served natively here via the store.
		mk("/v1/knowledge/ingest", h.handleIngest),
		// source delete + update are de-shelled — served natively via the store.
		mk("/v1/knowledge/source", h.handleDelete),
		mk("/v1/knowledge/source/update", h.handleUpdate),
		// Still bridged to the gateway (file upload / remote fetch).
		mk("/v1/knowledge/upload", bridge),
		mk("/v1/knowledge/import-url", bridge),
		mk("/v1/knowledge/import-repo", bridge),
	}
}

// handleIngest ingests inline text/structured content natively (de-shelled from
// the gateway): it writes through the store and triggers an async reindex.
func (h *Handler) handleIngest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		Name    string `json:"name"`
		Trigger string `json:"trigger"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "name and content required"})
		return
	}
	if req.Name == "" {
		req.Name = "inline-text"
	}
	var src *knowledge.Source
	var err error
	if req.Trigger != "" {
		src, err = h.store.IngestStructured(req.Name, req.Trigger, req.Content)
	} else {
		src, err = h.store.IngestText(req.Name, req.Content)
	}
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	safego.Go("knowledge-reindex", func() {
		if err := h.store.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after ingest failed", "err", err)
		}
	})
	_ = json.NewEncoder(w).Encode(map[string]any{"source": src, "stats": h.store.Stats()})
}

// handleUpdate updates a source natively (de-shelled from the gateway).
func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Trigger string `json:"trigger"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "id required"})
		return
	}
	src, err := h.store.UpdateSource(req.ID, req.Name, req.Trigger, req.Content)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	safego.Go("knowledge-reindex", func() {
		if err := h.store.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after update failed", "err", err)
		}
	})
	_ = json.NewEncoder(w).Encode(map[string]any{"source": src, "stats": h.store.Stats()})
}

// handleDelete removes a source natively (de-shelled from the gateway).
func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	sourceID := r.URL.Query().Get("id")
	if sourceID == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "source id required"})
		return
	}
	if ok := h.store.RemoveSource(sourceID); !ok {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "source not found"})
		return
	}
	safego.Go("knowledge-reindex", func() {
		if err := h.store.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after delete failed", "err", err)
		}
	})
	_ = json.NewEncoder(w).Encode(map[string]any{"deleted": sourceID, "stats": h.store.Stats()})
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "query parameter 'q' required"})
		return
	}
	limit := 10
	if n := r.URL.Query().Get("n"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 50 {
		limit = 50
	}
	chunks := h.store.SearchFiltered(query, limit, r.URL.Query().Get("file"), r.URL.Query().Get("lang"))
	_ = json.NewEncoder(w).Encode(map[string]any{"chunks": chunks, "count": len(chunks)})
}

func (h *Handler) handleSources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"sources": h.store.Sources()})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.store == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	_ = json.NewEncoder(w).Encode(h.store.Stats())
}
