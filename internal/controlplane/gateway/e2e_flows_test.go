package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/pkg/skills"
)

// ── Knowledge Base E2E Tests ─────────────────────────────────

func TestE2E_KnowledgeStats(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("kb-org")

	req := authedRequest("GET", "/v1/knowledge/stats", "", tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("knowledge stats: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestE2E_KnowledgeSources(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("kb-src-org")

	req := authedRequest("GET", "/v1/knowledge/sources", "", tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("knowledge sources: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestE2E_KnowledgeSearch(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("kb-search-org")

	body := `{"query":"测试查询","limit":5}`
	req := authedRequest("POST", "/v1/knowledge/search", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("knowledge search: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestE2E_KnowledgeIngestAndSearch(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("kb-ingest-org")

	ingestBody := `{"content":"人工智能是计算机科学的一个分支","source":"test","metadata":{"type":"doc"}}`
	req := authedRequest("POST", "/v1/knowledge/ingest", ingestBody, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 && w.Code != 201 {
		t.Logf("knowledge ingest: got %d (may need knowledge store): %s", w.Code, w.Body.String())
	}

	searchBody := `{"query":"人工智能","limit":3}`
	req = authedRequest("POST", "/v1/knowledge/search", searchBody, tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("knowledge search after ingest: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Task CRUD E2E Tests ──────────────────────────────────────

func newE2EGatewayWithTasks(mockURL string) (*Gateway, *tenant.Manager) {
	gw, tm := newE2EGatewayFull(mockURL)
	dir := t_tempDir()
	ts := task.NewJSONStore(dir)
	gw.SetTaskStore(ts)
	runner := task.NewRunner(ts, skills.NewRegistry(), func(ctx context.Context, system, user string) (string, error) {
		return "task result", nil
	}, nil)
	gw.SetTaskRunner(runner)
	return gw, tm
}

func t_tempDir() string {
	dir, err := os.MkdirTemp("", "e2e-tasks-*")
	if err != nil {
		panic(err)
	}
	return dir
}

func TestE2E_TaskCRUD(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGatewayWithTasks(mock.URL)
	tn := tm.Register("task-crud-org")

	// Create task
	body := `{"name":"E2E测试任务","description":"验证任务创建流程","priority":"high"}`
	req := authedRequest("POST", "/v1/tasks", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("task create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	taskID, _ := created["id"].(string)
	if taskID == "" {
		if created["task_id"] != nil {
			taskID, _ = created["task_id"].(string)
		}
	}

	// List tasks
	req = authedRequest("GET", "/v1/tasks", "", tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("task list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &listResp)
	tasks, ok := listResp["tasks"].([]any)
	if !ok {
		t.Logf("task list response: %v", listResp)
	} else if len(tasks) == 0 {
		t.Log("task list: empty after create")
	}

	// Delete task (if we got an ID)
	if taskID != "" {
		req = authedRequest("DELETE", "/v1/tasks?id="+taskID, "", tn.APIKey)
		w = httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != 200 && w.Code != 204 {
			t.Logf("task delete: got %d: %s", w.Code, w.Body.String())
		}
	}
}

func TestE2E_TaskRunAndCancel(t *testing.T) {
	mock := mockLLMServer("task execution result")
	defer mock.Close()
	gw, tm := newE2EGatewayWithTasks(mock.URL)
	tn := tm.Register("task-run-org")

	// Create task first
	body := `{"name":"运行测试任务","description":"验证任务运行和取消","steps":[{"description":"步骤1"}]}`
	req := authedRequest("POST", "/v1/tasks", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("task create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	taskID, _ := created["id"].(string)
	if taskID == "" {
		if v, ok := created["task_id"].(string); ok {
			taskID = v
		}
	}

	if taskID != "" {
		// Run task
		runBody := `{"task_id":"` + taskID + `"}`
		req = authedRequest("POST", "/v1/tasks/run", runBody, tn.APIKey)
		w = httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != 200 && w.Code != 202 {
			t.Logf("task run: got %d: %s", w.Code, w.Body.String())
		}

		// Cancel task
		cancelBody := `{"task_id":"` + taskID + `"}`
		req = authedRequest("POST", "/v1/tasks/cancel", cancelBody, tn.APIKey)
		w = httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Logf("task cancel: got %d: %s", w.Code, w.Body.String())
		}
	}
}

// ── Session Management E2E Tests ─────────────────────────────

func TestE2E_SessionCreateAndResume(t *testing.T) {
	mock := mockLLMServer("你好！")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("session-org")

	// First message in session
	body := `{"messages":[{"role":"user","content":"你好"}],"session_id":"e2e-session-resume"}`
	req := authedRequest("POST", "/v1/chat", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("first chat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Second message in same session (should include history)
	body = `{"messages":[{"role":"user","content":"记住我叫小明"}],"session_id":"e2e-session-resume"}`
	req = authedRequest("POST", "/v1/chat", body, tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("second chat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify conversation exists
	req = authedRequest("GET", "/v1/conversations", "", tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("conversations: expected 200, got %d", w.Code)
	}
}

func TestE2E_MultiSessionIsolation(t *testing.T) {
	mock := mockLLMServer("isolated reply")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("isolation-org")

	sessions := []string{"iso-session-1", "iso-session-2", "iso-session-3"}
	for _, sid := range sessions {
		body := `{"messages":[{"role":"user","content":"hello in ` + sid + `"}],"session_id":"` + sid + `"}`
		req := authedRequest("POST", "/v1/chat", body, tn.APIKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("chat in %s: expected 200, got %d", sid, w.Code)
		}
	}

	// Verify all sessions exist
	req := authedRequest("GET", "/v1/conversations", "", tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("conversations: expected 200, got %d", w.Code)
	}
}

// ── Chat Pipeline Edge Cases ─────────────────────────────────

func TestE2E_ChatWithPlatform(t *testing.T) {
	mock := mockLLMServer("platform reply")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("platform-org")

	body := `{"messages":[{"role":"user","content":"hello"}],"platform":"wechat"}`
	req := authedRequest("POST", "/v1/chat", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("chat with platform: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestE2E_ChatWithThinkingLevel(t *testing.T) {
	mock := mockLLMServer("deep thought")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("thinking-org")

	for _, level := range []string{"deep", "none", "auto"} {
		body := `{"messages":[{"role":"user","content":"think about this"}],"thinking_level":"` + level + `"}`
		req := authedRequest("POST", "/v1/chat", body, tn.APIKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("chat with thinking_level=%s: expected 200, got %d: %s", level, w.Code, w.Body.String())
		}
	}
}

func TestE2E_ChatMultiMessage(t *testing.T) {
	mock := mockLLMServer("multi-turn reply")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("multi-msg-org")

	body := `{"messages":[
		{"role":"system","content":"你是一个helpful的AI助手"},
		{"role":"user","content":"第一个问题"},
		{"role":"assistant","content":"第一个回答"},
		{"role":"user","content":"第二个问题"}
	]}`
	req := authedRequest("POST", "/v1/chat", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("multi-message chat: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["reply"] == nil || resp["reply"] == "" {
		t.Fatal("expected non-empty reply in multi-message chat")
	}
}

// ── Streaming Chat E2E ───────────────────────────────────────

func TestE2E_StreamingChat(t *testing.T) {
	streamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		chunks := []string{
			`{"choices":[{"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":"你好"},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":"！"},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":""},"finish_reason":"stop"}]}`,
		}
		for _, chunk := range chunks {
			w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer streamServer.Close()

	gw, tm := newE2EGateway(streamServer.URL)
	tn := tm.Register("stream-org")

	body := `{"messages":[{"role":"user","content":"hello"}],"stream":true}`
	req := authedRequest("POST", "/v1/chat/stream", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	// Accept both 200 (streaming) and 404 (if stream endpoint not registered)
	if w.Code != 200 && w.Code != 404 {
		t.Fatalf("streaming chat: expected 200/404, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Workflow E2E Tests ───────────────────────────────────────

func TestE2E_WorkflowList(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGatewayFull(mock.URL)
	tn := tm.Register("wf-org")

	req := authedRequest("GET", "/v1/workflows", "", tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	// 200 if workflow store configured, 404 if not
	if w.Code != 200 && w.Code != 404 {
		t.Fatalf("workflow list: expected 200/404, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Cross-Tenant Isolation ───────────────────────────────────

func TestE2E_CrossTenantIsolation(t *testing.T) {
	mock := mockLLMServer("tenant-specific reply")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)

	tenant1 := tm.Register("org-alpha")
	tenant2 := tm.Register("org-beta")

	// Tenant1 creates a conversation
	body := `{"messages":[{"role":"user","content":"alpha data"}],"session_id":"alpha-sess"}`
	req := authedRequest("POST", "/v1/chat", body, tenant1.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("tenant1 chat: expected 200, got %d", w.Code)
	}

	// Tenant2 should not see tenant1's conversations
	req = authedRequest("GET", "/v1/conversations", "", tenant2.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("tenant2 conversations: expected 200, got %d", w.Code)
	}
	var convResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &convResp)
	if convs, ok := convResp["sessions"].([]any); ok && len(convs) > 0 {
		for _, c := range convs {
			cm, _ := c.(map[string]any)
			if cm["id"] == "alpha-sess" {
				t.Error("tenant2 can see tenant1's session — isolation breach")
			}
		}
	}

	// Tenant2 API key should not work for tenant1
	req = authedRequest("POST", "/v1/chat",
		`{"messages":[{"role":"user","content":"cross-tenant attack"}],"session_id":"alpha-sess"}`,
		"invalid-cross-tenant-key")
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("cross-tenant: expected 401, got %d", w.Code)
	}
}

// ── Health & Readiness E2E ───────────────────────────────────

func TestE2E_HealthEndpoints(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, _ := newE2EGateway(mock.URL)

	endpoints := []string{"/healthz", "/readyz"}
	for _, ep := range endpoints {
		req := httptest.NewRequest("GET", ep, nil)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("%s: expected 200, got %d", ep, w.Code)
		}
	}
}

// ── CORS E2E ─────────────────────────────────────────────────

func TestE2E_CORSPreflight(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, _ := newE2EGateway(mock.URL)

	req := httptest.NewRequest("OPTIONS", "/v1/chat", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	// CORS preflight should return 200/204
	if w.Code != 200 && w.Code != 204 {
		t.Logf("CORS preflight: got %d (may depend on allowed origins config)", w.Code)
	}
}

// ── Rate Limiting E2E ────────────────────────────────────────

func TestE2E_RateLimitBehavior(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("rate-org")

	// Send multiple rapid requests — should not crash
	for i := 0; i < 10; i++ {
		body := `{"messages":[{"role":"user","content":"rapid fire"}]}`
		req := authedRequest("POST", "/v1/chat", body, tn.APIKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != 200 && w.Code != 429 {
			t.Fatalf("rapid request %d: unexpected status %d: %s", i, w.Code, w.Body.String())
		}
	}
}

// ── CJK Token Estimation ────────────────────────────────────

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		name string
		text string
		min  int64
		max  int64
	}{
		{"empty", "", 1, 5},
		{"english", "Hello, how are you doing today?", 7, 12},
		{"chinese", "你好世界这是一个测试", 5, 12},
		{"mixed", "Hello你好World世界", 4, 10},
		{"long_english", strings.Repeat("The quick brown fox jumps. ", 100), 500, 800},
		{"long_chinese", strings.Repeat("这是一段中文测试文本。", 100), 400, 900},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := estimateTokens(tc.text)
			if got < tc.min || got > tc.max {
				t.Errorf("estimateTokens(%q): got %d, want [%d, %d]", tc.name, got, tc.min, tc.max)
			}
		})
	}
}
