package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/observe"
)

func TestTraceEndpointsSanitizeUserVisibleEventsByDefault(t *testing.T) {
	gw, tm := newTestGatewayMigrationEnabled()
	tenant := tm.Register("trace-sanitized")
	trail := observe.NewAuditTrail(20)
	gw.SetEventTrail(trail)

	rawSummary := `handoff agent "file_exec" execution failed: context deadline exceeded`
	rawDetail := `all fallback LLM clients failed (FC): EOF`
	event := observe.NewEvent("trace-user-safe", observe.DomainPlanner, observe.EventHandoffDone, rawSummary)
	event.Detail = observe.HandoffDetail{Agent: "file_exec", Error: rawDetail}
	event.Meta.TaskID = "task-user-safe"
	trail.Record(event)

	for _, path := range []string{
		"/v1/trace/trace-user-safe",
		"/v1/trace/task/task-user-safe",
		"/v1/trace/recent?limit=5",
	} {
		req := authedRequest(http.MethodGet, path, "", tenant.APIKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d body=%s", path, w.Code, w.Body.String())
		}
		body := strings.ToLower(w.Body.String())
		for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "all fallback", "eof"} {
			if strings.Contains(body, banned) {
				t.Fatalf("%s: trace response should be friendly by default; found %q in %s", path, banned, w.Body.String())
			}
		}
		if !(strings.Contains(w.Body.String(), "现场已保留") || strings.Contains(w.Body.String(), "已保留现场")) {
			t.Fatalf("%s: expected friendly recovery wording, got %s", path, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"raw":false`) {
			t.Fatalf("%s: expected raw=false marker, got %s", path, w.Body.String())
		}
	}
}

func TestTraceEndpointsRawModePreservesAuditEvents(t *testing.T) {
	gw, tm := newTestGatewayMigrationEnabled()
	tenant := tm.Register("trace-raw")
	trail := observe.NewAuditTrail(20)
	gw.SetEventTrail(trail)

	rawSummary := `handoff agent "file_exec" execution failed: context deadline exceeded`
	rawDetail := `all fallback LLM clients failed (FC): EOF`
	event := observe.NewEvent("trace-raw-mode", observe.DomainPlanner, observe.EventHandoffDone, rawSummary)
	event.Detail = observe.HandoffDetail{Agent: "file_exec", Error: rawDetail}
	event.Meta.TaskID = "task-raw-mode"
	trail.Record(event)

	req := authedRequest(http.MethodGet, "/v1/trace/trace-raw-mode?raw=1", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
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

func TestTraceEndpointsSanitizeModelSwitchReasonByDefault(t *testing.T) {
	gw, tm := newTestGatewayMigrationEnabled()
	tenant := tm.Register("trace-model-switch-safe")
	trail := observe.NewAuditTrail(20)
	gw.SetEventTrail(trail)

	rawReason := `Post "https://api.moonshot.ai/v1/chat/completions": EOF`
	event := observe.NewEvent("trace-model-switch", observe.DomainPlanner, observe.EventPlan, "模型暂时没有回应，正在换用 qwen3.5:4b 继续。")
	event.Detail = planner.ModelFallbackDetail{Model: "qwen3.5:4b", Attempt: 2, Reason: rawReason}
	event.Meta.TaskID = "task-model-switch"
	trail.Record(event)

	req := authedRequest(http.MethodGet, "/v1/trace/trace-model-switch", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
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
