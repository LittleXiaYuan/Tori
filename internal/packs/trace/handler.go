// Package tracepack mounts execution trace query APIs as a native capability
// pack. It preserves the gateway's user-safe default view and raw audit mode.
package tracepack

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"yunque-agent/internal/agentcore/traceview"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.trace"

type Gateway interface {
	EventTrail() *observe.AuditTrail
}

type Handler struct {
	trailOf func() *observe.AuditTrail
	host    packruntime.Host
	started atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil)
	}
	return NewProvider(gateway.EventTrail)
}

func NewProvider(trailOf func() *observe.AuditTrail) *Handler {
	return &Handler{trailOf: trailOf}
}

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("trace pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/trace/recent", Handler: h.Recent},
		{Method: http.MethodGet, Path: "/v1/trace/task/", Handler: h.ByTask},
		{Method: http.MethodGet, Path: "/v1/trace/", Handler: h.ByID},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/trace/recent", Description: "List recent execution trace events."},
		{Method: http.MethodGet, Path: "/v1/trace/task/{task_id}", Description: "List execution trace events for a task."},
		{Method: http.MethodGet, Path: "/v1/trace/{trace_id}", Description: "List execution trace events for a trace."},
	}
}

func Paths() []string {
	return []string{"/v1/trace/recent", "/v1/trace/task/", "/v1/trace/"}
}

func (h *Handler) trail() *observe.AuditTrail {
	if h.trailOf == nil {
		return nil
	}
	return h.trailOf()
}

func (h *Handler) ByID(w http.ResponseWriter, r *http.Request) {
	trail := h.trail()
	if trail == nil {
		http.Error(w, "audit trail not available", http.StatusServiceUnavailable)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/trace/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "trace_id required", http.StatusBadRequest)
		return
	}
	traceID := parts[0]

	events := trail.QueryByTraceID(traceID)
	responseEvents, raw := traceview.EventsForResponse(r, events)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"trace_id": traceID,
		"count":    len(events),
		"raw":      raw,
		"events":   responseEvents,
	})
}

func (h *Handler) Recent(w http.ResponseWriter, r *http.Request) {
	trail := h.trail()
	if trail == nil {
		http.Error(w, "audit trail not available", http.StatusServiceUnavailable)
		return
	}
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}

	events := trail.Recent(limit)
	responseEvents, raw := traceview.EventsForResponse(r, events)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count":  len(events),
		"raw":    raw,
		"events": responseEvents,
	})
}

func (h *Handler) ByTask(w http.ResponseWriter, r *http.Request) {
	trail := h.trail()
	if trail == nil {
		http.Error(w, "audit trail not available", http.StatusServiceUnavailable)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/trace/task/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}
	taskID := parts[0]

	events := trail.QueryByTaskID(taskID)
	responseEvents, raw := traceview.EventsForResponse(r, events)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"task_id": taskID,
		"count":   len(events),
		"raw":     raw,
		"events":  responseEvents,
	})
}
