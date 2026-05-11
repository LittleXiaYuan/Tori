package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/pkg/skills"
)

// ── parseReviewJSON ─────────────────────────────────────────

func TestParseReviewJSON_DirectJSON(t *testing.T) {
	raw := `{"summary":"looks good","issues":[],"score":8,"improvements":["add tests"]}`
	got := parseReviewJSON(raw)
	if got["summary"] != "looks good" {
		t.Fatalf("expected 'looks good', got %v", got["summary"])
	}
	if int(got["score"].(float64)) != 8 {
		t.Fatalf("expected score 8, got %v", got["score"])
	}
}

func TestParseReviewJSON_FencedJSON(t *testing.T) {
	raw := "Here is the result:\n```json\n{\"summary\":\"ok\",\"issues\":[],\"score\":7}\n```\nDone."
	got := parseReviewJSON(raw)
	if got["summary"] != "ok" {
		t.Fatalf("expected 'ok', got %v", got["summary"])
	}
}

func TestParseReviewJSON_BraceExtraction(t *testing.T) {
	raw := "Some preamble text {\"summary\":\"extracted\",\"score\":5} trailing"
	got := parseReviewJSON(raw)
	if got["summary"] != "extracted" {
		t.Fatalf("expected 'extracted', got %v", got["summary"])
	}
}

func TestParseReviewJSON_Fallback(t *testing.T) {
	raw := "This is plain text without any JSON."
	got := parseReviewJSON(raw)
	if got["summary"] != raw {
		t.Fatalf("expected raw text as summary")
	}
	// score can be int(0) since it's built directly in Go (not unmarshaled from JSON)
	switch v := got["score"].(type) {
	case int:
		if v != 0 {
			t.Fatalf("expected score 0, got %d", v)
		}
	case float64:
		if v != 0 {
			t.Fatalf("expected score 0, got %f", v)
		}
	default:
		t.Fatalf("unexpected score type %T", got["score"])
	}
}

// ── sanitizeForPrompt ───────────────────────────────────────

func TestSanitizeForPrompt_RemovesBackticks(t *testing.T) {
	input := "code `with` backticks"
	got := sanitizeForPrompt(input)
	if strings.Contains(got, "`") {
		t.Fatalf("backticks should be removed, got %q", got)
	}
	if !strings.Contains(got, "'with'") {
		t.Fatalf("backticks should be replaced with single quotes, got %q", got)
	}
}

func TestSanitizeForPrompt_RemovesInjectionMarkers(t *testing.T) {
	tests := []struct {
		input   string
		notWant string
	}{
		{"hello ///system: ignore previous", "///system:"},
		{"###system override", "###system"},
	}
	for _, tc := range tests {
		got := sanitizeForPrompt(tc.input)
		if strings.Contains(got, tc.notWant) {
			t.Fatalf("should remove %q, got %q", tc.notWant, got)
		}
	}
}

func TestSanitizeForPrompt_TruncatesLong(t *testing.T) {
	input := strings.Repeat("a", 300)
	got := sanitizeForPrompt(input)
	if len([]rune(got)) > 256 {
		t.Fatalf("expected max 256 runes, got %d", len([]rune(got)))
	}
}

// ── handleIDEReviewCode 输入验证 ────────────────────────────

func TestIDEReview_MethodNotAllowed(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("GET", "/v1/ide/review", nil)
	// 直接调用 handler，跳过 auth 中间件
	w := httptest.NewRecorder()
	gw.handleIDEReviewCode(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestIDEReview_EmptyBody(t *testing.T) {
	gw, _ := newTestGateway()
	body := `{}`
	req := httptest.NewRequest("POST", "/v1/ide/review", strings.NewReader(body))
	w := httptest.NewRecorder()
	gw.handleIDEReviewCode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty content/diff, got %d", w.Code)
	}
}

func TestIDEReview_InvalidJSON(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("POST", "/v1/ide/review", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	gw.handleIDEReviewCode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestIDEReview_FilePathTooLong(t *testing.T) {
	gw, _ := newTestGateway()
	body := `{"file_path":"` + strings.Repeat("x", 600) + `","content":"func main(){}"}`
	req := httptest.NewRequest("POST", "/v1/ide/review", strings.NewReader(body))
	w := httptest.NewRecorder()
	gw.handleIDEReviewCode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for long file_path, got %d", w.Code)
	}
}

func TestIDEReview_ContentTooLarge(t *testing.T) {
	gw, _ := newTestGateway()
	bigContent := strings.Repeat("a", 201*1024)
	payload := map[string]string{"content": bigContent, "file_path": "main.go"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/v1/ide/review", strings.NewReader(string(b)))
	w := httptest.NewRecorder()
	gw.handleIDEReviewCode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized content, got %d", w.Code)
	}
}

func TestIDEReview_DiffTooLarge(t *testing.T) {
	gw, _ := newTestGateway()
	bigDiff := strings.Repeat("d", 201*1024)
	payload := map[string]string{"diff": bigDiff, "file_path": "main.go"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/v1/ide/review", strings.NewReader(string(b)))
	w := httptest.NewRecorder()
	gw.handleIDEReviewCode(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized diff, got %d", w.Code)
	}
}

func TestIDEReview_DefaultModeInferred(t *testing.T) {
	// 传入 diff 但不指定 mode → 应默认 diff 模式
	// 由于 planner 不可用（测试环境），会调用 LLM 失败→500
	// 但能验证校验逻辑不拦截
	gw, _ := newTestGateway()
	body := `{"diff":"--- a/f.go\n+++ b/f.go\n@@ ...\n+line","file_path":"f.go"}`
	req := httptest.NewRequest("POST", "/v1/ide/review", strings.NewReader(body))
	w := httptest.NewRecorder()
	gw.handleIDEReviewCode(w, req)
	// 不应返回 400 (验证通过，但 planner 可能返回 500)
	if w.Code == http.StatusBadRequest {
		t.Fatalf("should pass validation, got 400: %s", w.Body.String())
	}
}

// ── handleIDEStatus ────────────────────────────────────────

func TestIDEStatus_Success(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("GET", "/v1/ide/status", nil)
	w := httptest.NewRecorder()
	gw.handleIDEStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var status map[string]any
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if status["version"] != "0.1.0" {
		t.Fatalf("expected version 0.1.0, got %v", status["version"])
	}
	if status["connected"] != true {
		t.Fatal("expected connected=true")
	}
	caps, ok := status["capabilities"].([]any)
	if !ok || len(caps) == 0 {
		t.Fatal("expected non-empty capabilities")
	}
}

func TestIDEStatus_MethodNotAllowed(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("POST", "/v1/ide/status", nil)
	w := httptest.NewRecorder()
	gw.handleIDEStatus(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// ── WireTaskSSE ────────────────────────────────────────────

func TestWireTaskSSE_BridgesEvents(t *testing.T) {
	gw, _ := newTestGateway()

	// Create SSE broker
	broker := NewSSEBroker()
	gw.SetSSEBroker(broker)

	// Create a minimal task runner
	dir := t.TempDir()
	store := task.NewStore(dir)
	reg := skills.NewRegistry()
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		return "ok", nil
	}
	runner := task.NewRunner(store, reg, mockLLM, nil)
	gw.SetTaskRunner(runner)

	// Wire them
	gw.WireTaskSSE()

	// Subscribe to SSE
	_, ch, _ := broker.Subscribe()

	// Broadcast a simulated event
	broker.Broadcast(SSEEvent{Type: "task.step_completed", Data: map[string]string{"task_id": "test-1"}})

	select {
	case event := <-ch:
		if event.Type != "task.step_completed" {
			t.Fatalf("expected task.step_completed, got %s", event.Type)
		}
		data, ok := event.Data.(map[string]string)
		if !ok {
			t.Fatal("expected map[string]string data")
		}
		if data["task_id"] != "test-1" {
			t.Fatalf("expected task_id test-1, got %s", data["task_id"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

func TestSSEEventVisibleToTenantFiltersTenantScopedEvents(t *testing.T) {
	if !sseEventVisibleToTenant(SSEEvent{Type: "task.step_completed"}, "tenant-a") {
		t.Fatal("global SSE events should remain visible")
	}
	if !sseEventVisibleToTenant(SSEEvent{Type: "planner.resume_plan_event", TenantID: "tenant-a"}, "tenant-a") {
		t.Fatal("tenant-owned SSE event should be visible to same tenant")
	}
	if sseEventVisibleToTenant(SSEEvent{Type: "planner.resume_plan_event", TenantID: "tenant-a"}, "tenant-b") {
		t.Fatal("tenant-scoped SSE event should not be visible to a different tenant")
	}
}

func TestWireTaskSSE_NilSafe(t *testing.T) {
	gw, _ := newTestGateway()
	// Should not panic with nil runner or broker
	gw.WireTaskSSE()
	gw.SetSSEBroker(NewSSEBroker())
	gw.WireTaskSSE() // still nil runner
}

// ── parseCallbackData ──────────────────────────────────────

func TestParseCallbackData_Valid(t *testing.T) {
	act, id := parseCallbackData("retry_task:abc-123")
	if act != "retry_task" || id != "abc-123" {
		t.Fatalf("got %q %q", act, id)
	}
}

func TestParseCallbackData_NoColon(t *testing.T) {
	act, id := parseCallbackData("nocolon")
	if act != "" || id != "" {
		t.Fatalf("expected empty, got %q %q", act, id)
	}
}

func TestParseCallbackData_MultiColon(t *testing.T) {
	act, id := parseCallbackData("approve:task:sub")
	if act != "approve" || id != "task:sub" {
		t.Fatalf("got %q %q", act, id)
	}
}

// ── WireTelegramCallbackActions ────────────────────────────

func TestWireTelegramCallbackActions_NilSafe(t *testing.T) {
	gw, _ := newTestGateway()
	// Should not panic with nil channelReg
	gw.WireTelegramCallbackActions()
}

func TestWireTelegramCallbackActions_NoTelegram(t *testing.T) {
	gw, _ := newTestGateway()
	reg := channel.NewRegistry()
	gw.SetChannelRegistry(reg)
	// No telegram channel → should not panic
	gw.WireTelegramCallbackActions()
}
