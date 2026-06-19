// Package sessionqueuepack mounts per-session task queue inspection as a native
// capability pack.
package sessionqueuepack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/session"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.session-queue"

type Gateway interface {
	QueueManager() *session.QueueManager
}

type Handler struct {
	queueOf func() *session.QueueManager
	host    packruntime.Host
	started atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil)
	}
	return NewProvider(gateway.QueueManager)
}

func NewProvider(queueOf func() *session.QueueManager) *Handler {
	return &Handler{queueOf: queueOf}
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
		h.host.Logger().Info("session queue pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/sessions/queue", Handler: h.Queue},
		{Method: http.MethodPost, Path: "/v1/sessions/queue/cancel", Handler: h.Cancel},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/sessions/queue", Description: "List all session queues or inspect one session queue with ?id=."},
		{Method: http.MethodPost, Path: "/v1/sessions/queue/cancel", Description: "Cancel a queued task for a session."},
	}
}

func Paths() []string {
	return []string{"/v1/sessions/queue", "/v1/sessions/queue/cancel"}
}

func (h *Handler) queue() *session.QueueManager {
	if h.queueOf == nil {
		return nil
	}
	return h.queueOf()
}

func (h *Handler) Queue(w http.ResponseWriter, r *http.Request) {
	qm := h.queue()
	if qm == nil {
		writeJSON(w, map[string]any{"queues": map[string]int{}})
		return
	}

	sessionID := r.URL.Query().Get("id")
	if sessionID != "" {
		snapshot := qm.SessionSnapshot(sessionID)
		writeJSON(w, map[string]any{
			"session_id": sessionID,
			"tasks":      snapshot,
		})
		return
	}

	writeJSON(w, map[string]any{"queues": qm.AllSessions()})
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	qm := h.queue()
	if qm == nil {
		http.Error(w, `{"error":"queue not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		TaskID    string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"cancelled": qm.Cancel(req.SessionID, req.TaskID)})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
