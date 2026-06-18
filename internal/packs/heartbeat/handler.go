package heartbeatpack

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/runtime/heartbeat"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.heartbeat"

type Gateway interface {
	HeartbeatService() *heartbeat.Service
}

type Handler struct {
	heartbeatOf func() *heartbeat.Service
	host        packruntime.Host
	started     atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil)
	}
	return NewProvider(gateway.HeartbeatService)
}

func NewProvider(heartbeat func() *heartbeat.Service) *Handler {
	return &Handler{heartbeatOf: heartbeat}
}

func (h *Handler) PackID() string { return PackID }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("heartbeat pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPut}, Path: "/v1/heartbeat", Handler: h.Status},
		{Method: http.MethodPost, Path: "/v1/heartbeat/trigger", Handler: h.Trigger},
		{Method: http.MethodGet, Path: "/v1/heartbeat/logs", Handler: h.Logs},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/heartbeat", Description: "Read heartbeat running status."},
		{Method: http.MethodPut, Path: "/v1/heartbeat", Description: "Update heartbeat enabled state or interval."},
		{Method: http.MethodPost, Path: "/v1/heartbeat/trigger", Description: "Trigger one heartbeat run immediately."},
		{Method: http.MethodGet, Path: "/v1/heartbeat/logs", Description: "List recent heartbeat logs."},
	}
}

func Paths() []string {
	return []string{"/v1/heartbeat", "/v1/heartbeat/trigger", "/v1/heartbeat/logs"}
}

func (h *Handler) heartbeat() *heartbeat.Service {
	if h.heartbeatOf == nil {
		return nil
	}
	return h.heartbeatOf()
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	hb := h.heartbeat()
	if hb == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "heartbeat not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"running": hb.IsRunning()})
	case http.MethodPut:
		var req struct {
			Enabled  *bool `json:"enabled"`
			Interval *int  `json:"interval_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
			return
		}
		if req.Enabled != nil {
			hb.SetEnabled(r.Context(), *req.Enabled)
		}
		if req.Interval != nil && *req.Interval > 0 {
			hb.SetInterval(r.Context(), time.Duration(*req.Interval)*time.Minute)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT")
	}
}

func (h *Handler) Trigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	hb := h.heartbeat()
	if hb == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "heartbeat not configured")
		return
	}
	entry := hb.Trigger(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entry)
}

func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	hb := h.heartbeat()
	if hb == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "heartbeat not configured")
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	logs := hb.Logs(limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logs)
}
