package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
)

// ctxWithTenant builds a context with the given tenant ID.
func ctxWithTenant(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxTenantKey, id)
}

// extractErrorCode extracts the error code from the apperror JSON response.
func extractErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %v", resp)
	}
	code, _ := errObj["code"].(string)
	return code
}

// ── Request Validation ─────────────────────────────────────

func TestChat_EmptyBody(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader(""))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleChat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChat_InvalidJSON(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader("{invalid"))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleChat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChat_EmptyMessages(t *testing.T) {
	gw, _ := newTestGateway()
	body := `{"messages":[]}`
	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleChat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	code := extractErrorCode(t, w)
	if code != "MESSAGES_REQUIRED" {
		t.Fatalf("expected MESSAGES_REQUIRED, got %v", code)
	}
}

func TestChat_TooManyMessages(t *testing.T) {
	gw, _ := newTestGateway()
	msgs := make([]map[string]string, 101)
	for i := range msgs {
		msgs[i] = map[string]string{"role": "user", "content": "hi"}
	}
	b, _ := json.Marshal(map[string]any{"messages": msgs})
	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader(string(b)))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleChat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	code := extractErrorCode(t, w)
	if code != "TOO_MANY_MESSAGES" {
		t.Fatalf("expected TOO_MANY_MESSAGES, got %v", code)
	}
}

func TestChat_MessageTooLong(t *testing.T) {
	gw, _ := newTestGateway()
	longText := strings.Repeat("x", 32001)
	b, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{{"role": "user", "content": longText}},
	})
	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader(string(b)))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleChat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	code := extractErrorCode(t, w)
	if code != "MESSAGE_TOO_LONG" {
		t.Fatalf("expected MESSAGE_TOO_LONG, got %v", code)
	}
}

// ── Quota ──────────────────────────────────────────────────

func TestChat_QuotaExceeded(t *testing.T) {
	gw, _ := newTestGateway()
	// Set a tight quota and exhaust it
	gw.usage.SetQuota("t-limited", QuotaConfig{MaxChatCalls: 1})
	gw.usage.RecordChat("t-limited", 100) // exhaust the 1-call limit

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t-limited"))
	w := httptest.NewRecorder()
	gw.handleChat(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	code := extractErrorCode(t, w)
	if code != "QUOTA_EXCEEDED" {
		t.Fatalf("expected QUOTA_EXCEEDED, got %v", code)
	}
}

// ── Sticker Probability ────────────────────────────────────

func TestStickerSendProb(t *testing.T) {
	tests := []struct {
		freq float64
		want float64
	}{
		{0, 0},
		{-1, 0},
		{1, 0.25},
		{2, 0.50},
		{3, 0.80},
		{10, 0.80},
	}
	for _, tt := range tests {
		got := stickerSendProb(tt.freq)
		if got != tt.want {
			t.Errorf("stickerSendProb(%v) = %v, want %v", tt.freq, got, tt.want)
		}
	}
}

// ── Valid request (planner will fail but that's expected with mock LLM) ──

func TestChat_ValidRequest_PlannerError(t *testing.T) {
	gw, _ := newTestGateway()
	body := `{"messages":[{"role":"user","content":"hello world"}]}`
	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleChat(w, req)
	// Planner will fail because LLM client points to localhost:0, but validation should pass.
	// We expect 502 (llm_error) rather than 400 (validation error).
	if w.Code == http.StatusBadRequest {
		t.Fatal("validation should have passed; got 400 instead of 502")
	}
	code := extractErrorCode(t, w)
	if code != "LLM_ERROR" {
		t.Fatalf("expected LLM_ERROR, got %v", code)
	}
}

func TestChat_SessionManagement(t *testing.T) {
	gw, _ := newTestGateway()
	// Send with session_id — should create session automatically
	body := `{"messages":[{"role":"user","content":"test session"}],"session_id":"sess-1"}`
	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleChat(w, req)
	// Verify session was created
	stored := gw.convStore.Get("sess-1")
	if len(stored) == 0 {
		t.Fatal("expected session to be created with user message")
	}
	if stored[0].Content != "test session" {
		t.Fatalf("expected stored message 'test session', got %q", stored[0].Content)
	}
}

func TestChat_MethodNotAllowed(t *testing.T) {
	gw, _ := newTestGateway()
	// handleChat is exposed on POST; GET should be routed differently or 405
	req := httptest.NewRequest("GET", "/v1/chat", nil)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	// Use ServeHTTP to test routing
	gw.ServeHTTP(w, req)
	// GET on /v1/chat should not match POST handler
	if w.Code == http.StatusOK {
		t.Fatal("GET /v1/chat should not return 200")
	}
}

func TestTryHandleSlashCommand_NavigateWithoutBrowserExtension(t *testing.T) {
	gw, _ := newTestGateway()
	resp, handled, err := gw.tryHandleSlashCommand(context.Background(), planner.PlanRequest{
		TenantID: "t1",
		Messages: []llm.Message{{Role: "user", Content: "/navigate 查找关于雷军的资料"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected slash command to be handled")
	}
	if resp == nil || !strings.Contains(resp.Result.Reply, "浏览器扩展未连接") {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestNormalizeNavigateTarget_SearchQuery(t *testing.T) {
	got := normalizeNavigateTarget("查找关于雷军的资料")
	if !strings.HasPrefix(got, "https://www.bing.com/search?q=") {
		t.Fatalf("expected search URL, got %q", got)
	}
}

func TestDetectBrowserIntentHint_WithURLExploreRequest(t *testing.T) {
	hint := detectBrowserIntentHint("使用 My Browser 访问 https://www.youtube.com/channel/abc ，然后彻底探索并告诉我发生了什么")
	if hint == "" {
		t.Fatal("expected browser intent hint")
	}
	if !strings.Contains(hint, "browser_navigate") {
		t.Fatalf("unexpected hint: %s", hint)
	}
}

func TestDetectBrowserIntentHint_CurrentPageRequest(t *testing.T) {
	hint := detectBrowserIntentHint("帮我提取当前页面内容并总结一下")
	if hint == "" {
		t.Fatal("expected browser intent hint for current page")
	}
}

func TestSummarizeBrowserSlashReply_MarkIncludesCount(t *testing.T) {
	reply := summarizeBrowserSlashReply("browser_mark_elements", nil, BrowserResult{OK: true, Total: 8})
	if !strings.Contains(reply, "8") {
		t.Fatalf("expected element count in reply, got %q", reply)
	}
}

func TestSummarizeBrowserSlashArtifact_ContentPreview(t *testing.T) {
	summary := summarizeBrowserSlashArtifact("browser_get_content", map[string]any{"url": "https://example.com"}, BrowserResult{OK: true, Content: "hello world", Title: "Example"})
	if summary["skill"] != "browser_get_content" {
		t.Fatalf("unexpected skill: %#v", summary)
	}
	if summary["text_length"] != len("hello world") {
		t.Fatalf("unexpected text_length: %#v", summary)
	}
	if summary["preview"] != "hello world" {
		t.Fatalf("unexpected preview: %#v", summary)
	}
	if summary["url"] != "https://example.com" {
		t.Fatalf("unexpected url: %#v", summary)
	}
	if summary["next_command"] != "/mark" {
		t.Fatalf("unexpected next_command: %#v", summary)
	}
	if summary["next_label"] != "Mark interactive elements" {
		t.Fatalf("unexpected next_label: %#v", summary)
	}
}

func TestSummarizeBrowserPlanArtifact_UsesLatestBrowserStep(t *testing.T) {
	summary := summarizeBrowserPlanArtifact([]planner.PlanStep{
		{Skill: "web_search", Status: planner.StepDone, Result: `{"ok":true}`},
		{Skill: "browser_navigate", Status: planner.StepDone, Args: map[string]any{"url": "https://example.com"}, Result: `{"ok":true,"url":"https://example.com","title":"Example"}`},
		{Skill: "browser_get_content", Status: planner.StepDone, Args: map[string]any{"url": "https://example.com"}, Result: `{"ok":true,"url":"https://example.com","title":"Example","content":"hello from browser"}`},
	})
	if summary == nil {
		t.Fatal("expected browser summary")
	}
	if summary["skill"] != "browser_get_content" {
		t.Fatalf("unexpected skill: %#v", summary)
	}
	if summary["preview"] != "hello from browser" {
		t.Fatalf("unexpected preview: %#v", summary)
	}
	if summary["next_command"] != "/mark" {
		t.Fatalf("unexpected next_command: %#v", summary)
	}
}

func TestSummarizeBrowserPlanArtifact_HandlesFailedBrowserStep(t *testing.T) {
	summary := summarizeBrowserPlanArtifact([]planner.PlanStep{
		{Skill: "browser_click", Status: planner.StepFailed, Args: map[string]any{"selector": "button.buy"}, Error: "element not found"},
	})
	if summary == nil {
		t.Fatal("expected browser summary")
	}
	if summary["error"] != "element not found" {
		t.Fatalf("unexpected error summary: %#v", summary)
	}
}
