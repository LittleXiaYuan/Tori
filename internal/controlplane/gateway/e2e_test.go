package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

// mockLLMServer creates a test HTTP server that mimics OpenAI chat completions API.
func mockLLMServer(reply string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": reply}},
			},
		})
	}))
}

// newE2EGateway creates a gateway backed by a mock LLM.
func newE2EGateway(mockURL string) (*Gateway, *tenant.Manager) {
	llmClient := llm.NewClient(mockURL, "test-key", "test-model")
	reg := skills.NewRegistry()
	p := planner.NewPlanner(llmClient, reg, 8)
	tm := tenant.NewManager()
	short := memory.NewShortTerm(1 * time.Hour)
	mid := memory.NewMidTerm()
	long := memory.NewLongTerm()
	mm := memory.NewManager(short, mid, long)
	sched := scheduler.New(func(ctx context.Context, job scheduler.Job) {})
	cs := session.NewStore(50)
	pr := plugin.NewRegistry()
	jwtCfg := &JWTConfig{Secret: "e2e-test-secret", Issuer: "e2e", Expiration: time.Hour}
	// Register the migrated core packs with an enabled registry so the e2e
	// gateway matches production after the pack-route migration (otherwise
	// /v1/{skills,memory,tasks,...} fall through to the SPA catch-all).
	gw := NewFromConfig(GatewayConfig{
		Planner:   p,
		Tenants:   tm,
		Memory:    mm,
		Skills:    reg,
		Scheduler: sched,
		ConvStore: cs,
		Plugins:   pr,
		JWTConfig: jwtCfg,
		Packs:     newMigrationPackRegistry(),
	})
	registerMigrationPacks(gw)
	gw.SetLedgerHealthChecker(fakeHealthChecker{})
	return gw, tm
}

func authedRequest(method, path, body string, apiKey string) *http.Request {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, path, reader)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// --- E2E Test Suite ---

func TestE2E_FullChatFlow(t *testing.T) {
	mock := mockLLMServer("你好！我是Yunque Agent，很高兴为你服务。")
	defer mock.Close()

	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("e2e-org")

	body := `{"messages":[{"role":"user","content":"你好"}]}`
	req := authedRequest("POST", "/v1/chat", body, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("chat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	reply, ok := resp["reply"].(string)
	if !ok || reply == "" {
		t.Fatalf("chat: expected non-empty reply, got %v", resp)
	}
	if !strings.Contains(reply, "Yunque") {
		t.Logf("chat reply: %s", reply)
	}
}

func TestE2E_TenantCRUD(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	admin := tm.Register("admin-org")

	// Create tenant
	req := authedRequest("POST", "/v1/tenants", `{"name":"test-tenant"}`, admin.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("create tenant: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}

	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	if created["name"] != "test-tenant" {
		t.Fatalf("tenant name mismatch: %v", created)
	}
	if created["api_key"] == nil || created["api_key"] == "" {
		t.Fatal("missing api_key in created tenant")
	}

	// List tenants
	req = authedRequest("GET", "/v1/tenants", "", admin.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list tenants: expected 200, got %d", w.Code)
	}
}

func TestE2E_SkillsList(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("skills-org")

	req := authedRequest("GET", "/v1/skills", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("skills: expected 200, got %d", w.Code)
	}
}

func TestE2E_MemoryOps(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("mem-org")

	// Stats
	req := authedRequest("GET", "/v1/memory/stats", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("memory stats: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Add memory
	req = authedRequest("POST", "/v1/memory/add", `{"tier":"mid","key":"test-key","value":"test-value"}`, tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("memory add: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}

	// Search memory
	req = authedRequest("POST", "/v1/memory/search", `{"query":"test","limit":5}`, tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("memory search: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestE2E_MetricsEndpoints(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("met-org")

	// JSON metrics
	req := authedRequest("GET", "/v1/metrics", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("metrics: expected 200, got %d", w.Code)
	}

	// Prometheus (auth required since security hardening)
	req = authedRequest("GET", "/v1/metrics/prometheus", "", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("prometheus: expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "yunque_") {
		t.Fatal("prometheus output should contain yunque_ prefix")
	}
}

func TestE2E_VersionEndpoint(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, _ := newE2EGateway(mock.URL)

	req := httptest.NewRequest("GET", "/v1/version", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("version: expected 200, got %d", w.Code)
	}
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v)
	if v["version"] == nil {
		t.Fatal("version field missing")
	}
}

func TestE2E_JWTTokenFlow(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("jwt-org")

	// Get JWT token
	req := authedRequest("POST", "/v1/token", `{}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("token: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var tokenResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &tokenResp)
	token, ok := tokenResp["token"].(string)
	if !ok || token == "" {
		t.Fatal("expected JWT token in response")
	}

	// Use JWT to access skills
	req = httptest.NewRequest("GET", "/v1/skills", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("skills with JWT: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestE2E_PluginManagement(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("plugin-org")

	// List plugins
	req := authedRequest("GET", "/v1/plugins", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("plugins: expected 200, got %d", w.Code)
	}
}

func TestE2E_ConversationList(t *testing.T) {
	mock := mockLLMServer("hello")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("conv-org")

	// First send a chat to create a session
	body := `{"messages":[{"role":"user","content":"hi"}],"session_id":"e2e-sess-1"}`
	req := authedRequest("POST", "/v1/chat", body, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("chat for conv: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// List conversations
	req = authedRequest("GET", "/v1/conversations", "", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("conversations: expected 200, got %d", w.Code)
	}
}

func TestE2E_SystemInfo(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("sys-org")

	req := authedRequest("GET", "/v1/system/info", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("system info: expected 200, got %d", w.Code)
	}
}

func TestE2E_UnauthorizedAccess(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, _ := newE2EGateway(mock.URL)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/skills"},
		{"POST", "/v1/chat"},
		{"GET", "/v1/tenants"},
		{"GET", "/v1/memory/stats"},
		{"GET", "/v1/metrics"},
		{"GET", "/v1/plugins"},
		{"GET", "/v1/conversations"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Errorf("%s %s: expected 401, got %d", ep.method, ep.path, w.Code)
		}
	}
}

func TestE2E_InvalidAPIKey(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, _ := newE2EGateway(mock.URL)

	req := authedRequest("GET", "/v1/skills", "", "invalid-key-12345")
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401 with invalid key, got %d", w.Code)
	}
}

func TestE2E_ChatValidation_EmptyMessages(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("val-org")

	req := authedRequest("POST", "/v1/chat", `{"messages":[]}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("empty messages: expected 400, got %d", w.Code)
	}
}

func TestE2E_ChatValidation_NoMessages(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("val2-org")

	req := authedRequest("POST", "/v1/chat", `{}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("no messages field: expected 400, got %d", w.Code)
	}
}

func TestE2E_ChatValidation_MessageTooLong(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("val3-org")

	// Build a message with >32000 chars
	longContent := strings.Repeat("a", 33000)
	body := `{"messages":[{"role":"user","content":"` + longContent + `"}]}`
	req := authedRequest("POST", "/v1/chat", body, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("long message: expected 400, got %d", w.Code)
	}
}

func TestE2E_RequestIDHeader(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, _ := newE2EGateway(mock.URL)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	rid := w.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatal("missing X-Request-ID")
	}
	// Verify format: timestamp-counter
	if !strings.Contains(rid, "-") {
		t.Fatalf("unexpected request ID format: %s", rid)
	}
}

func TestE2E_SecurityHeaders(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, _ := newE2EGateway(mock.URL)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing X-Content-Type-Options: nosniff")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing X-Frame-Options: DENY")
	}
}

func TestE2E_SchedulerOps(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("sched-org")

	// List jobs
	req := authedRequest("GET", "/v1/scheduler/jobs", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("scheduler jobs: expected 200, got %d", w.Code)
	}
}

// ── Phase 5 E2E Tests ────────────────────────────────────────

func newE2EGatewayFull(mockURL string) (*Gateway, *tenant.Manager) {
	gw, tm := newE2EGateway(mockURL)
	cs := session.NewStore(50)
	threadMgr := task.NewThreadManager(cs)
	gw.SetThreadManager(threadMgr)
	trigRT := trigger.NewRuntime(nil, nil)
	trigRT.Start()
	gw.SetTriggerRuntime(trigRT)
	ct := costtrack.New()
	gw.SetCostTracker(ct)
	pool := llm.NewPool()
	pool.Register("primary", llm.NewClient(mockURL, "test-key", "test-model"))
	provReg := llm.NewProviderRegistry(pool)
	gw.SetProviderRegistry(provReg)
	return gw, tm
}

func TestE2E_TaskThreadCRUD(t *testing.T) {
	mock := mockLLMServer("thread reply")
	defer mock.Close()
	gw, tm := newE2EGatewayFull(mock.URL)
	tn := tm.Register("thread-org")

	// POST: create thread with message
	body := `{"task_id":"test-task-1","content":"hello from thread"}`
	req := authedRequest("POST", "/v1/tasks/threads", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("thread POST: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// GET: retrieve thread info
	req = authedRequest("GET", "/v1/tasks/threads?id=test-task-1", "", tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("thread GET: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["task_id"] != "test-task-1" {
		t.Fatalf("thread: unexpected task_id: %v", resp)
	}
}

func TestE2E_TriggerCRUD(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGatewayFull(mock.URL)
	tn := tm.Register("trigger-org")

	// Create trigger
	body := `{"name":"e2e-trigger","kind":"event","event":"task_completed","action":{"type":"log","message":"done!"}}`
	req := authedRequest("POST", "/v1/triggers", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("trigger POST: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	trigID, _ := createResp["id"].(string)
	if trigID == "" {
		t.Fatal("trigger create: expected id in response")
	}

	// List triggers
	req = authedRequest("GET", "/v1/triggers", "", tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("trigger list: expected 200, got %d", w.Code)
	}
	var listResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &listResp)
	triggers, ok := listResp["triggers"].([]any)
	if !ok || len(triggers) == 0 {
		t.Fatal("trigger list: expected non-empty triggers array")
	}

	// Get specific trigger
	req = authedRequest("GET", "/v1/triggers?id="+trigID, "", tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("trigger get: expected 200, got %d", w.Code)
	}

	// Emit event
	body = `{"event":"task_completed","text":"task xyz done"}`
	req = authedRequest("POST", "/v1/triggers/emit", body, tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("trigger emit: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete trigger
	req = authedRequest("DELETE", "/v1/triggers?id="+trigID, "", tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("trigger delete: expected 200, got %d", w.Code)
	}
}

func TestE2E_TriggerEmitFire(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()

	var fired int
	handler := func(ctx context.Context, trig *trigger.Trigger, ev *trigger.EventPayload) error {
		fired++
		return nil
	}

	gw, tm := newE2EGateway(mock.URL)
	tn := tm.Register("emit-org")
	trigRT := trigger.NewRuntime(handler, nil)
	trigRT.Start()
	defer trigRT.Stop()
	gw.SetTriggerRuntime(trigRT)

	// Register a trigger directly
	trigRT.Register(trigger.Trigger{
		Name:   "test-fire",
		Kind:   trigger.KindEvent,
		Event:  trigger.EventTaskCompleted,
		Action: trigger.Action{Type: trigger.ActionLog},
	})

	// Emit via API
	body := `{"event":"task_completed","text":"done"}`
	req := authedRequest("POST", "/v1/triggers/emit", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("emit: expected 200, got %d", w.Code)
	}
	if fired != 1 {
		t.Errorf("expected handler fired 1 time, got %d", fired)
	}
}

func TestE2E_CostEndpoints(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGatewayFull(mock.URL)
	tn := tm.Register("cost-org")

	// Record some cost
	ct := costtrack.New()
	ct.RecordExt(costtrack.RecordOpts{
		Model: "test-model", TokensIn: 100, TokensOut: 50,
		SessionID: "s1", TenantID: "cost-org", TaskID: "t1", SkillName: "test-skill",
	})
	gw.SetCostTracker(ct)

	// Get cost summary
	req := authedRequest("GET", "/v1/cost/summary", "", tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("cost summary: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Get cost breakdown by model
	req = authedRequest("GET", "/v1/cost/breakdown?by=model", "", tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("cost breakdown: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestE2E_ProviderLocalDiscover(t *testing.T) {
	// Create a mock Ollama server for testing
	ollamaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "llama3:8b", "model": "llama3:8b", "size": 4700000000},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ollamaMock.Close()

	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGatewayFull(mock.URL)
	tn := tm.Register("local-org")

	// Discover local models
	body := `{"base_url":"` + ollamaMock.URL + `"}`
	req := authedRequest("POST", "/api/providers/local/discover", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("local discover: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var disc map[string]any
	json.Unmarshal(w.Body.Bytes(), &disc)
	if disc["available"] != true {
		t.Fatalf("expected available=true, got %v", disc)
	}
	models, ok := disc["models"].([]any)
	if !ok || len(models) == 0 {
		t.Fatal("expected models in discover response")
	}
}

func TestE2E_ProviderLocalRegister(t *testing.T) {
	ollamaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "qwen2:7b", "model": "qwen2:7b", "size": 3900000000},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ollamaMock.Close()

	mock := mockLLMServer("ok")
	defer mock.Close()
	gw, tm := newE2EGatewayFull(mock.URL)
	tn := tm.Register("reg-org")

	// Register local backend
	body := `{"base_url":"` + ollamaMock.URL + `","tier":"fast","backend":"ollama"}`
	req := authedRequest("POST", "/api/providers/local/register", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("local register: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var reg map[string]any
	json.Unmarshal(w.Body.Bytes(), &reg)
	if reg["ok"] != true {
		t.Fatalf("expected ok=true, got %v", reg)
	}
	if reg["provider_id"] != "local-ollama" {
		t.Errorf("expected provider_id=local-ollama, got %v", reg["provider_id"])
	}

	// Verify provider appears in list
	req = authedRequest("GET", "/api/providers", "", tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("provider list: expected 200, got %d", w.Code)
	}
	var listResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &listResp)
	providers, ok := listResp["providers"].([]any)
	if !ok {
		t.Fatal("expected providers array")
	}
	found := false
	for _, p := range providers {
		pm, _ := p.(map[string]any)
		if pm["id"] == "local-ollama" {
			found = true
		}
	}
	if !found {
		t.Error("local-ollama not found in provider list")
	}
}

func TestE2E_ChatWithTaskThread(t *testing.T) {
	mock := mockLLMServer("task context reply")
	defer mock.Close()
	gw, tm := newE2EGatewayFull(mock.URL)
	tn := tm.Register("chat-task-org")

	// Chat with task_id
	body := `{"messages":[{"role":"user","content":"work on task"}],"task_id":"my-task-1"}`
	req := authedRequest("POST", "/v1/chat", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("chat with task: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var chatResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &chatResp)
	reply, _ := chatResp["reply"].(string)
	if reply == "" {
		t.Fatal("expected non-empty reply")
	}

	// Thread should now exist
	req = authedRequest("GET", "/v1/tasks/threads?id=my-task-1", "", tn.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("thread get: expected 200, got %d", w.Code)
	}
}
