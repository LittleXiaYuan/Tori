package sessionqueuepack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/session"
	"yunque-agent/pkg/packruntime"
)

func TestSessionQueuePackV2AndRouteSpecs(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := NewProvider(nil)
	if h.PackID() != PackID {
		t.Fatalf("PackID=%q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 2 {
		t.Fatalf("Routes len=%d, want 2", got)
	}
	if got := len(RouteSpecs()); got != 2 {
		t.Fatalf("RouteSpecs len=%d, want 2", got)
	}
	paths := map[string]bool{}
	for _, route := range h.Routes() {
		paths[route.Path] = true
	}
	for _, spec := range RouteSpecs() {
		if !paths[spec.Path] {
			t.Fatalf("route spec path %s has no mounted route", spec.Path)
		}
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestQueueNilManagerPreservesEmptySummary(t *testing.T) {
	h := NewProvider(nil)
	w := httptest.NewRecorder()
	h.Queue(w, httptest.NewRequest(http.MethodGet, "/v1/sessions/queue", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"queues":{}`) {
		t.Fatalf("expected empty queues body, got %s", w.Body.String())
	}
}

func TestQueueListSnapshotAndCancel(t *testing.T) {
	qm := session.NewQueueManager(func(ctx context.Context, entry *session.TaskEntry) (string, error) {
		return "ok", nil
	}, 10)
	defer qm.Shutdown()

	if err := qm.Enqueue(&session.TaskEntry{ID: "task-1", SessionID: "session-a", Prompt: "do it"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	h := NewProvider(func() *session.QueueManager { return qm })

	listRec := httptest.NewRecorder()
	h.Queue(listRec, httptest.NewRequest(http.MethodGet, "/v1/sessions/queue", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	var listBody struct {
		Queues map[string]int `json:"queues"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listBody.Queues["session-a"] != 1 {
		t.Fatalf("unexpected queues: %#v", listBody.Queues)
	}

	snapshotRec := httptest.NewRecorder()
	h.Queue(snapshotRec, httptest.NewRequest(http.MethodGet, "/v1/sessions/queue?id=session-a", nil))
	var snapshotBody struct {
		SessionID string              `json:"session_id"`
		Tasks     []session.TaskEntry `json:"tasks"`
	}
	if err := json.Unmarshal(snapshotRec.Body.Bytes(), &snapshotBody); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshotBody.SessionID != "session-a" || len(snapshotBody.Tasks) != 1 || snapshotBody.Tasks[0].ID != "task-1" {
		t.Fatalf("unexpected snapshot: %#v", snapshotBody)
	}

	cancelRec := httptest.NewRecorder()
	h.Cancel(cancelRec, httptest.NewRequest(http.MethodPost, "/v1/sessions/queue/cancel", strings.NewReader(`{"session_id":"session-a","task_id":"task-1"}`)))
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	if !strings.Contains(cancelRec.Body.String(), `"cancelled":true`) {
		t.Fatalf("expected cancelled=true, got %s", cancelRec.Body.String())
	}
}

func TestCancelNilManagerAndInvalidBody(t *testing.T) {
	h := NewProvider(nil)
	nilRec := httptest.NewRecorder()
	h.Cancel(nilRec, httptest.NewRequest(http.MethodPost, "/v1/sessions/queue/cancel", strings.NewReader(`{}`)))
	if nilRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil manager status=%d body=%s", nilRec.Code, nilRec.Body.String())
	}

	qm := session.NewQueueManager(nil, 10)
	defer qm.Shutdown()
	h = NewProvider(func() *session.QueueManager { return qm })
	badRec := httptest.NewRecorder()
	h.Cancel(badRec, httptest.NewRequest(http.MethodPost, "/v1/sessions/queue/cancel", strings.NewReader(`{`)))
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad body status=%d body=%s", badRec.Code, badRec.Body.String())
	}
}
