// Package cronpack mounts the cron HTTP surface (/v1/cron/*) as a v2 capability
// pack (Tier 0 microkernel). Native pack: it owns the cron list/add/remove/run
// handlers, talking to the cron manager through a narrow accessor. Split out of
// the gateway's mixed registerTriggerRoutes grab-bag (triggers/cron/files).
package cronpack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.cron"

// Gateway is the narrow host surface the cron pack needs.
type Gateway interface {
	CronManager() *cron.Manager
}

// Handler is the cron pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the cron pack backed by the host's cron-manager accessor.
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
		h.host.Logger().Info("cron pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) mgr() *cron.Manager {
	if h.gw == nil {
		return nil
	}
	return h.gw.CronManager()
}

// Routes mounts the cron surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	m := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}
	mk := func(path string, fn http.HandlerFunc) packruntime.BackendRoute {
		return packruntime.BackendRoute{Methods: m, Path: path, Handler: fn}
	}
	return []packruntime.BackendRoute{
		mk("/v1/cron/list", h.handleList),
		mk("/v1/cron/add", h.handleAdd),
		mk("/v1/cron/remove", h.handleRemove),
		mk("/v1/cron/run", h.handleRun),
	}
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mgr := h.mgr()
	if mgr == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"jobs": mgr.List()})
}

func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mgr := h.mgr()
	if mgr == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	var req struct {
		Name     string        `json:"name"`
		Schedule cron.Schedule `json:"schedule"`
		Payload  cron.Payload  `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "name, schedule, and payload required"})
		return
	}
	id, err := mgr.Add(req.Name, req.Schedule, req.Payload)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	job, _ := mgr.Get(id)
	_ = json.NewEncoder(w).Encode(map[string]any{"job": job})
}

func (h *Handler) handleRemove(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mgr := h.mgr()
	if mgr == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "job id required"})
		return
	}
	if err := mgr.Remove(id); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mgr := h.mgr()
	if mgr == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "job id required"})
		return
	}
	rec, err := mgr.RunNow(id)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"run": rec})
}
