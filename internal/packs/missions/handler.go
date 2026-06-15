// Package missionspack mounts the mission-parsing HTTP surface
// (/v1/missions/*) as a v2 capability pack (Tier 0 microkernel). Native pack:
// it owns NL mission-intent parsing, reaching the planner only through a narrow
// accessor (no planner import). Split out of the misnamed registerTaskRoutes.
package missionspack

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.missions"

// Gateway is the narrow host surface the missions pack needs. Returning any keeps
// the pack decoupled from the planner package; the result is JSON-encoded as-is.
type Gateway interface {
	ParseMissionIntent(ctx context.Context, description string) (any, error)
}

// Handler is the missions pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the missions pack backed by the host's mission-parse accessor.
func New(gw Gateway) *Handler { return &Handler{gw: gw} }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("missions pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the mission surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodPost, Path: "/v1/missions/parse", Handler: h.handleParse},
	}
}

func (h *Handler) handleParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if h.gw == nil {
		http.Error(w, "mission parsing not available", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		http.Error(w, "description is required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	result, err := h.gw.ParseMissionIntent(ctx, req.Description)
	if err != nil {
		slog.Error("mission parse: failed", "err", err)
		http.Error(w, "failed to parse mission intent", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
