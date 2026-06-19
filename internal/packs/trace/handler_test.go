package tracepack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/packruntime"
)

func TestTracePackV2AndRouteSpecs(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := NewProvider(nil)
	if h.PackID() != PackID {
		t.Fatalf("PackID=%q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 3 {
		t.Fatalf("Routes len=%d, want 3", got)
	}
	if got := len(RouteSpecs()); got != 3 {
		t.Fatalf("RouteSpecs len=%d, want 3", got)
	}
	paths := map[string]bool{}
	for _, route := range h.Routes() {
		paths[route.Path] = true
	}
	for _, spec := range RouteSpecs() {
		path := spec.Path
		path = strings.Replace(path, "{task_id}", "", 1)
		path = strings.Replace(path, "{trace_id}", "", 1)
		if !paths[path] {
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

func TestTraceEndpointsSanitizeUserVisibleEventsByDefault(t *testing.T) {
	trail := observe.NewAuditTrail(20)
	h := NewProvider(func() *observe.AuditTrail { return trail })

	rawSummary := `handoff agent "file_exec" execution failed: context deadline exceeded`
	rawDetail := `all fallback LLM clients failed (FC): EOF`
	event := observe.NewEvent("trace-user-safe", observe.DomainPlanner, observe.EventHandoffDone, rawSummary)
	event.Detail = observe.HandoffDetail{Agent: "file_exec", Error: rawDetail}
	event.Meta.TaskID = "task-user-safe"
	trail.Record(event)

	for _, tc := range []struct {
		name string
		path string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"by-id", "/v1/trace/trace-user-safe", h.ByID},
		{"by-task", "/v1/trace/task/task-user-safe", h.ByTask},
		{"recent", "/v1/trace/recent?limit=5", h.Recent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			tc.call(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
			}
			body := strings.ToLower(w.Body.String())
			for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "all fallback", "eof"} {
				if strings.Contains(body, banned) {
					t.Fatalf("trace response should be friendly by default; found %q in %s", banned, w.Body.String())
				}
			}
			if !(strings.Contains(w.Body.String(), "现场已保留") || strings.Contains(w.Body.String(), "已保留现场")) {
				t.Fatalf("expected friendly recovery wording, got %s", w.Body.String())
			}
			if !strings.Contains(w.Body.String(), `"raw":false`) {
				t.Fatalf("expected raw=false marker, got %s", w.Body.String())
			}
		})
	}
}

func TestTraceEndpointRawModePreservesAuditEvents(t *testing.T) {
	trail := observe.NewAuditTrail(20)
	h := NewProvider(func() *observe.AuditTrail { return trail })

	rawSummary := `handoff agent "file_exec" execution failed: context deadline exceeded`
	rawDetail := `all fallback LLM clients failed (FC): EOF`
	event := observe.NewEvent("trace-raw-mode", observe.DomainPlanner, observe.EventHandoffDone, rawSummary)
	event.Detail = observe.HandoffDetail{Agent: "file_exec", Error: rawDetail}
	trail.Record(event)

	req := httptest.NewRequest(http.MethodGet, "/v1/trace/trace-raw-mode?raw=1", nil)
	w := httptest.NewRecorder()
	h.ByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := strings.ToLower(w.Body.String())
	for _, want := range []string{"handoff agent", "context deadline exceeded", "all fallback", "eof", `"raw":true`} {
		if !strings.Contains(body, want) {
			t.Fatalf("raw trace response should preserve %q, got %s", want, w.Body.String())
		}
	}
}

func TestTraceEndpointSanitizesModelSwitchReasonByDefault(t *testing.T) {
	trail := observe.NewAuditTrail(20)
	h := NewProvider(func() *observe.AuditTrail { return trail })

	rawReason := `Post "https://api.moonshot.ai/v1/chat/completions": EOF`
	event := observe.NewEvent("trace-model-switch", observe.DomainPlanner, observe.EventPlan, "模型暂时没有回应，正在换用 qwen3.5:4b 继续。")
	event.Detail = planner.ModelFallbackDetail{Model: "qwen3.5:4b", Attempt: 2, Reason: rawReason}
	trail.Record(event)

	req := httptest.NewRequest(http.MethodGet, "/v1/trace/trace-model-switch", nil)
	w := httptest.NewRecorder()
	h.ByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := strings.ToLower(w.Body.String())
	for _, banned := range []string{"api.moonshot.ai", "eof", "chat/completions"} {
		if strings.Contains(body, banned) {
			t.Fatalf("trace response should hide raw model switch reason %q, got %s", banned, w.Body.String())
		}
	}
	if !(strings.Contains(w.Body.String(), "现场已保留") || strings.Contains(w.Body.String(), "已保留现场")) {
		t.Fatalf("expected friendly model switch reason, got %s", w.Body.String())
	}
}
