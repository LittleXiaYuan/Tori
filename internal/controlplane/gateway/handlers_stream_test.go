package gateway

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/observe"
)

func TestStreamChat_EmptyBody(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader(""))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	if !strings.Contains(w.Body.String(), "BAD_REQUEST") {
		t.Fatalf("expected BAD_REQUEST error event, got %s", w.Body.String())
	}
}

func TestStreamChat_InvalidJSON(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader("{bad json"))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	if !strings.Contains(w.Body.String(), "BAD_REQUEST") {
		t.Fatalf("expected BAD_REQUEST error, got %s", w.Body.String())
	}
}

func TestStreamChat_EmptyMessages(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader(`{"messages":[]}`))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	if !strings.Contains(w.Body.String(), "MESSAGES_REQUIRED") {
		t.Fatalf("expected MESSAGES_REQUIRED error, got %s", w.Body.String())
	}
}

func TestStreamChat_TooManyMessages(t *testing.T) {
	gw, _ := newTestGateway()
	msgs := make([]map[string]string, 101)
	for i := range msgs {
		msgs[i] = map[string]string{"role": "user", "content": "hi"}
	}
	b, _ := json.Marshal(map[string]any{"messages": msgs})
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader(string(b)))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	if !strings.Contains(w.Body.String(), "TOO_MANY_MESSAGES") {
		t.Fatalf("expected TOO_MANY_MESSAGES error, got %s", w.Body.String())
	}
}

func TestStreamChat_MessageTooLong(t *testing.T) {
	gw, _ := newTestGateway()
	longText := strings.Repeat("x", 32001)
	b, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{{"role": "user", "content": longText}},
	})
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader(string(b)))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	if !strings.Contains(w.Body.String(), "MESSAGE_TOO_LONG") {
		t.Fatalf("expected MESSAGE_TOO_LONG error, got %s", w.Body.String())
	}
}

func TestStreamChat_QuotaExceeded(t *testing.T) {
	gw, _ := newTestGateway()
	gw.usage.SetQuota("t-limited", QuotaConfig{MaxChatCalls: 1})
	gw.usage.RecordChat("t-limited", 100)

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t-limited"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestStreamChat_SSEHeaders(t *testing.T) {
	gw, _ := newTestGateway()

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if w.Header().Get("Cache-Control") != "no-cache, no-transform" {
		t.Error("missing Cache-Control: no-cache, no-transform")
	}
	if w.Header().Get("Connection") != "keep-alive" {
		t.Error("missing Connection: keep-alive")
	}
}

func TestStreamChat_PlannerErrorIsFriendly(t *testing.T) {
	gw, _ := newTestGateway()

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	bodyText := w.Body.String()
	if !strings.Contains(bodyText, "LLM_ERROR") {
		t.Fatalf("expected LLM_ERROR event, got %s", bodyText)
	}
	for _, raw := range []string{"all fallback LLM clients failed", "context deadline exceeded", "execution failed", "handoff agent", "EOF"} {
		if strings.Contains(bodyText, raw) {
			t.Fatalf("stream error response should be friendly, found raw %q in %s", raw, bodyText)
		}
	}
	if !strings.Contains(bodyText, "现场") {
		t.Fatalf("expected friendly recovery wording, got %s", bodyText)
	}
}

func TestStreamChatSendsImmediateHeartbeatBeforeSlowWork(t *testing.T) {
	gw, _ := newTestGateway()

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	if got := w.Header().Get("Cache-Control"); got != "no-cache, no-transform" {
		t.Fatalf("Cache-Control = %q, want no-cache, no-transform", got)
	}
	bodyText := w.Body.String()
	if !strings.HasPrefix(bodyText, ": yunque-agent keepalive ") {
		t.Fatalf("expected padded SSE heartbeat before stream work starts, got %s", bodyText)
	}
	if !strings.Contains(bodyText, "event: ping") {
		t.Fatalf("expected ping event to keep stream alive, got %s", bodyText)
	}
}

func TestStreamChatStepEventsAreFriendly(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]any{{
						"id":   "call-1",
						"type": "function",
						"function": map[string]any{
							"name":      "missing_tool",
							"arguments": "{}",
						},
					}},
				},
				"finish_reason": "tool_calls",
			}},
		})
	}))
	defer mock.Close()
	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("stream-step-friendly")
	gw.planner = planner.NewPlanner(llm.NewClient(mock.URL, "test-key", "test-model"), gw.registry, 1)
	gw.planner.SetNativeFC(true)

	body := `{"messages":[{"role":"user","content":"调用一个不存在的工具"}]}`
	req := authedRequest(http.MethodPost, "/v1/chat/stream", body, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 stream response, got %d body=%s", w.Code, w.Body.String())
	}
	bodyText := strings.ToLower(w.Body.String())
	for _, raw := range []string{"unknown skill", "执行失败: unknown skill"} {
		if strings.Contains(bodyText, raw) {
			t.Fatalf("stream step event should hide raw tool failure %q, got %s", raw, w.Body.String())
		}
	}
	if !strings.Contains(w.Body.String(), "所需工具暂时不可用") {
		t.Fatalf("expected friendly tool-unavailable wording, got %s", w.Body.String())
	}
}

func TestStreamChatDocumentIntentAddsRoutingHint(t *testing.T) {
	var mu sync.Mutex
	var capturedBodies []string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]string `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload.Messages)
		mu.Lock()
		capturedBodies = append(capturedBodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer mock.Close()

	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("stream-document-routing")

	body := `{"messages":[{"role":"user","content":"请读取这个 xls 文件，并提取表格字段"}]}`
	req := authedRequest(http.MethodPost, "/v1/chat/stream", body, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 stream response, got %d body=%s", w.Code, w.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(capturedBodies) == 0 {
		t.Fatalf("expected stream request to reach mock LLM")
	}
	firstPrompt := capturedBodies[0]
	if !strings.Contains(firstPrompt, "[Document routing]") || !strings.Contains(firstPrompt, "document_parse") {
		t.Fatalf("expected document routing hint in stream planner prompt, got %s", firstPrompt)
	}
}

func TestAgenticChat_PlannerErrorIsFriendly(t *testing.T) {
	gw, _ := newTestGateway()

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/agentic", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleAgenticChat(w, req)

	bodyText := w.Body.String()
	if !strings.Contains(bodyText, "PLANNER_ERROR") {
		t.Fatalf("expected PLANNER_ERROR event, got %s", bodyText)
	}
	for _, raw := range []string{"all fallback LLM clients failed", "context deadline exceeded", "execution failed", "handoff agent", "EOF"} {
		if strings.Contains(bodyText, raw) {
			t.Fatalf("agentic error response should be friendly, found raw %q in %s", raw, bodyText)
		}
	}
	if !strings.Contains(bodyText, "现场") {
		t.Fatalf("expected friendly recovery wording, got %s", bodyText)
	}
}

func TestAgenticChatSendsImmediateHeartbeatBeforeSlowWork(t *testing.T) {
	gw, _ := newTestGateway()

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/agentic", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleAgenticChat(w, req)

	if got := w.Header().Get("Cache-Control"); got != "no-cache, no-transform" {
		t.Fatalf("Cache-Control = %q, want no-cache, no-transform", got)
	}
	bodyText := w.Body.String()
	if !strings.HasPrefix(bodyText, ": yunque-agent keepalive ") {
		t.Fatalf("expected padded SSE heartbeat before work starts, got %s", bodyText)
	}
	if !strings.Contains(bodyText, "event: ping") {
		t.Fatalf("expected ping event to keep client stream alive, got %s", bodyText)
	}
}

func TestAgenticChatAttachmentContextIsHiddenFromConversationStore(t *testing.T) {
	gw, _ := newTestGateway()
	rawDoc := "[Parsed document: 申请表.docx]\nWorkspace path: uploads/申请表.docx\n\n公司名称\t云鸢科技\n联系电话\t已填写"
	payload := map[string]any{
		"session_id": "agentic-attachment-visible-session",
		"messages": []map[string]string{
			{"role": "user", "content": "请读取并处理附件"},
		},
		"attachments": []map[string]string{
			{
				"name":     "申请表.docx",
				"mime":     "text/plain; charset=utf-8",
				"data_b64": base64.StdEncoding.EncodeToString([]byte(rawDoc)),
			},
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest("POST", "/v1/chat/agentic", strings.NewReader(string(bodyBytes)))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleAgenticChat(w, req)

	history := gw.convStore.Get("agentic-attachment-visible-session")
	if len(history) == 0 {
		t.Fatalf("expected user message to be persisted")
	}
	persisted := history[0].Content
	for _, raw := range []string{"[Parsed document:", "公司名称\t云鸢科技", "联系电话\t已填写", "Workspace path:"} {
		if strings.Contains(persisted, raw) {
			t.Fatalf("conversation history should not expose full parsed attachment %q in %q", raw, persisted)
		}
	}
	if !strings.Contains(persisted, "附件已读取") || !strings.Contains(persisted, "申请表.docx") {
		t.Fatalf("expected compact attachment summary in conversation history, got %q", persisted)
	}
	if len(history) < 2 || history[1].Role != "system" || !strings.Contains(history[1].Content, "公司名称\t云鸢科技") {
		t.Fatalf("expected hidden attachment context to be retained for follow-up turns, got %+v", history)
	}

	req = httptest.NewRequest("GET", "/v1/conversations/messages?session_id=agentic-attachment-visible-session", nil)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w = httptest.NewRecorder()
	gw.handleConversationMessages(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected conversation messages 200, got %d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "公司名称\t云鸢科技") || strings.Contains(w.Body.String(), "[隐藏附件上下文]") {
		t.Fatalf("conversation messages API should hide attachment context, got %s", w.Body.String())
	}
}

func TestAgenticChatParsedAttachmentReachesPlannerPrompt(t *testing.T) {
	var mu sync.Mutex
	var capturedBodies []string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]string `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload.Messages)
		mu.Lock()
		capturedBodies = append(capturedBodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "已读取附件。"}}},
		})
	}))
	defer mock.Close()

	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("agentic-parsed-attachment")
	rawDoc := "[Parsed document: 申请表.docx]\nWorkspace path: uploads/申请表.docx\n\n公司名称\t云鸢科技\n联系电话\t13864841667"
	payload := map[string]any{
		"mode": "chat",
		"messages": []map[string]string{
			{"role": "user", "content": "请读取附件，并告诉我公司名称"},
		},
		"attachments": []map[string]string{
			{
				"name":     "申请表.docx",
				"mime":     "text/plain; charset=utf-8",
				"data_b64": base64.StdEncoding.EncodeToString([]byte(rawDoc)),
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := authedRequest(http.MethodPost, "/v1/chat/agentic", string(body), tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(capturedBodies) == 0 {
		t.Fatalf("expected request to reach mock LLM")
	}
	firstPrompt := capturedBodies[0]
	for _, want := range []string{"[Document routing]", "[Parsed document: 申请表.docx]", "公司名称\\t云鸢科技", "联系电话\\t13864841667"} {
		if !strings.Contains(firstPrompt, want) {
			t.Fatalf("expected parsed attachment content %q in planner prompt, got %s", want, firstPrompt)
		}
	}
}

func TestAgenticChatAttachmentMetadataContextIsHiddenFromConversationStore(t *testing.T) {
	gw, _ := newTestGateway()
	rawDoc := "[Attachment file: 申请表.pdf]\nWorkspace path: uploads/申请表.pdf\nStatus: ready\nNote: 附件已添加，但当前本地解析器还不能直接展开 .pdf 正文；配置文档解析后端后会自动提取正文。"
	payload := map[string]any{
		"session_id": "agentic-attachment-metadata-session",
		"messages": []map[string]string{
			{"role": "user", "content": "请读取并处理附件"},
		},
		"attachments": []map[string]string{
			{
				"name":     "申请表.pdf",
				"mime":     "text/x-yunque-attachment-metadata; charset=utf-8",
				"data_b64": base64.StdEncoding.EncodeToString([]byte(rawDoc)),
			},
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest("POST", "/v1/chat/agentic", strings.NewReader(string(bodyBytes)))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleAgenticChat(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	history := gw.convStore.Get("agentic-attachment-metadata-session")
	if len(history) < 2 {
		t.Fatalf("expected visible and hidden messages, got %+v", history)
	}
	persisted := history[0].Content
	if strings.Contains(persisted, "[Attachment file:") || strings.Contains(persisted, "Workspace path: uploads/申请表.pdf") {
		t.Fatalf("conversation history should not expose hidden attachment metadata, got %q", persisted)
	}
	if !strings.Contains(persisted, "附件已读取") || !strings.Contains(persisted, "申请表.pdf") {
		t.Fatalf("expected compact attachment summary in visible history, got %q", persisted)
	}
	if history[1].Role != "system" || !strings.Contains(history[1].Content, "[Attachment file: 申请表.pdf]") || !strings.Contains(persisted, "正文未直接展开") {
		t.Fatalf("expected hidden metadata context to be retained, got %+v", history)
	}
}

func TestTruncateUTF8ByBytesKeepsAttachmentContextValid(t *testing.T) {
	body := strings.Repeat("云雀", 50000)
	got := truncateUTF8ByBytes(body, 64*1024, "\n...[truncated]")
	if !strings.HasSuffix(got, "\n...[truncated]") {
		t.Fatalf("expected truncation suffix")
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Fatalf("truncated attachment context should not contain replacement rune")
	}
	if !strings.Contains(got, "云雀") {
		t.Fatalf("expected retained unicode content")
	}
}

func TestAgenticChatAttachmentContextCarriesIntoFollowUpPrompt(t *testing.T) {
	var mu sync.Mutex
	var capturedBodies []string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]string `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload.Messages)
		mu.Lock()
		capturedBodies = append(capturedBodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer mock.Close()

	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("agentic-attachment-follow-up")
	rawDoc := "[Parsed document: 申请表.docx]\n\n公司名称\t云鸢科技"
	first := map[string]any{
		"session_id": "agentic-attachment-follow-up-session",
		"mode":       "chat",
		"messages": []map[string]string{
			{"role": "user", "content": "请读取附件"},
		},
		"attachments": []map[string]string{
			{
				"name":     "申请表.docx",
				"mime":     "text/plain; charset=utf-8",
				"data_b64": base64.StdEncoding.EncodeToString([]byte(rawDoc)),
			},
		},
	}
	firstBody, _ := json.Marshal(first)
	req := authedRequest(http.MethodPost, "/v1/chat/agentic", string(firstBody), tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	second := map[string]any{
		"session_id": "agentic-attachment-follow-up-session",
		"mode":       "chat",
		"messages": []map[string]string{
			{"role": "user", "content": "请读取附件\n\n[附件已读取]\n- 申请表.docx：内容已作为隐藏上下文提供给模型。"},
			{"role": "assistant", "content": "已读取。"},
			{"role": "user", "content": "继续根据刚才附件提取公司名称"},
		},
	}
	secondBody, _ := json.Marshal(second)
	req = authedRequest(http.MethodPost, "/v1/chat/agentic", string(secondBody), tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	mu.Lock()
	defer mu.Unlock()
	if len(capturedBodies) < 2 {
		t.Fatalf("expected both turns to reach mock LLM, captured=%d bodies=%v", len(capturedBodies), capturedBodies)
	}
	followUpPrompt := capturedBodies[len(capturedBodies)-1]
	if !strings.Contains(followUpPrompt, "公司名称") || !strings.Contains(followUpPrompt, "云鸢科技") {
		t.Fatalf("expected hidden attachment context in follow-up prompt, got %s", followUpPrompt)
	}
}

func TestAgenticChatAttachmentMetadataCarriesIntoFollowUpPrompt(t *testing.T) {
	var mu sync.Mutex
	var capturedBodies []string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]string `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload.Messages)
		mu.Lock()
		capturedBodies = append(capturedBodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer mock.Close()

	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("agentic-attachment-metadata-follow-up")
	rawDoc := "[Attachment file: 申请表.pdf]\nWorkspace path: uploads/申请表.pdf\nStatus: ready\nNote: 附件已添加，但当前本地解析器还不能直接展开 .pdf 正文；配置文档解析后端后会自动提取正文。"
	first := map[string]any{
		"session_id": "agentic-attachment-metadata-follow-up-session",
		"mode":       "chat",
		"messages": []map[string]string{
			{"role": "user", "content": "请读取附件"},
		},
		"attachments": []map[string]string{
			{
				"name":     "申请表.pdf",
				"mime":     "text/x-yunque-attachment-metadata; charset=utf-8",
				"data_b64": base64.StdEncoding.EncodeToString([]byte(rawDoc)),
			},
		},
	}
	firstBody, _ := json.Marshal(first)
	req := authedRequest(http.MethodPost, "/v1/chat/agentic", string(firstBody), tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	second := map[string]any{
		"session_id": "agentic-attachment-metadata-follow-up-session",
		"mode":       "chat",
		"messages": []map[string]string{
			{"role": "user", "content": "请读取附件\n\n[附件已读取]\n- 申请表.pdf：已记录文件信息，正文未直接展开。"},
			{"role": "assistant", "content": "已记录。"},
			{"role": "user", "content": "继续根据刚才附件处理"},
		},
	}
	secondBody, _ := json.Marshal(second)
	req = authedRequest(http.MethodPost, "/v1/chat/agentic", string(secondBody), tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	mu.Lock()
	defer mu.Unlock()
	if len(capturedBodies) < 2 {
		t.Fatalf("expected both turns to reach mock LLM, captured=%d bodies=%v", len(capturedBodies), capturedBodies)
	}
	followUpPrompt := capturedBodies[len(capturedBodies)-1]
	if !strings.Contains(followUpPrompt, "[Attachment file: 申请表.pdf]") || !strings.Contains(followUpPrompt, "正文未直接展开") {
		t.Fatalf("expected hidden attachment metadata in follow-up prompt, got %s", followUpPrompt)
	}
	if strings.Contains(followUpPrompt, "[Parsed document: 申请表.pdf]") {
		t.Fatalf("metadata follow-up should not be represented as parsed document, got %s", followUpPrompt)
	}
}

func TestAgenticChatAttachmentMetadataAddsDocumentRoutingHint(t *testing.T) {
	var mu sync.Mutex
	var capturedBodies []string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]string `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload.Messages)
		mu.Lock()
		capturedBodies = append(capturedBodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer mock.Close()

	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("agentic-attachment-routing")
	rawDoc := "[Attachment file: 申请表.pdf]\nWorkspace path: uploads/申请表.pdf\nStatus: ready\nNote: 附件已添加，但当前本地解析器还不能直接展开 .pdf 正文；配置文档解析后端后会自动提取正文。"
	payload := map[string]any{
		"session_id": "agentic-attachment-routing-session",
		"mode":       "chat",
		"messages": []map[string]string{
			{"role": "user", "content": "请读取附件"},
		},
		"attachments": []map[string]string{
			{
				"name":     "申请表.pdf",
				"mime":     "text/x-yunque-attachment-metadata; charset=utf-8",
				"data_b64": base64.StdEncoding.EncodeToString([]byte(rawDoc)),
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := authedRequest(http.MethodPost, "/v1/chat/agentic", string(body), tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	mu.Lock()
	defer mu.Unlock()
	if len(capturedBodies) == 0 {
		t.Fatalf("expected request to reach mock LLM")
	}
	firstPrompt := capturedBodies[0]
	if !strings.Contains(firstPrompt, "[Document routing]") || !strings.Contains(firstPrompt, "may not be parsed yet") {
		t.Fatalf("expected document routing hint for metadata attachment, got %s", firstPrompt)
	}
	if !strings.Contains(firstPrompt, "[Attachment file: 申请表.pdf]") {
		t.Fatalf("expected attachment metadata in prompt, got %s", firstPrompt)
	}
}

func TestFriendlyAgentEventForStreamSanitizesSummaryAndDetail(t *testing.T) {
	rawSummary := `handoff agent "file_exec" execution failed: context deadline exceeded`
	rawDetail := `all fallback LLM clients failed (FC): EOF`
	event := observe.NewEvent("trace-friendly-event", observe.DomainPlanner, observe.EventHandoffDone, rawSummary)
	event.Detail = observe.HandoffDetail{Agent: "file_exec", Error: rawDetail}

	got := friendlyAgentEventForStream(event)
	if got.Summary == rawSummary || strings.Contains(got.Summary, "handoff agent") || strings.Contains(got.Summary, "context deadline exceeded") {
		t.Fatalf("expected friendly summary, got %q", got.Summary)
	}
	detail, ok := got.Detail.(observe.HandoffDetail)
	if !ok {
		t.Fatalf("expected handoff detail, got %T", got.Detail)
	}
	if detail.Error == rawDetail || strings.Contains(detail.Error, "fallback") || strings.Contains(detail.Error, "EOF") {
		t.Fatalf("expected friendly detail error, got %q", detail.Error)
	}
	if event.Summary != rawSummary {
		t.Fatalf("original event should stay raw for audit trail, got %q", event.Summary)
	}
	origDetail := event.Detail.(observe.HandoffDetail)
	if origDetail.Error != rawDetail {
		t.Fatalf("original detail should stay raw for audit trail, got %q", origDetail.Error)
	}
}

func TestFriendlyAgentEventForStreamSanitizesNestedDetail(t *testing.T) {
	raw := `tool panic: nil pointer while calling nested tool`
	event := observe.NewEvent("trace-nested-friendly-event", observe.DomainPlanner, observe.EventToolResult, "工具执行遇到问题")
	event.Detail = map[string]any{
		"skill": "nested_tool",
		"result": map[string]any{
			"stderr": raw,
			"items": []any{
				map[string]any{"reason": `all fallback LLM clients failed (FC): EOF`},
			},
		},
	}

	got := friendlyAgentEventForStream(event)
	body, _ := json.Marshal(got.Detail)
	bodyText := strings.ToLower(string(body))
	for _, banned := range []string{"tool panic", "nil pointer", "all fallback", "eof"} {
		if strings.Contains(bodyText, banned) {
			t.Fatalf("nested stream detail should be friendly, found %q in %s", banned, string(body))
		}
	}
	if !strings.Contains(string(body), "已保留现场") && !strings.Contains(string(body), "现场") {
		t.Fatalf("expected friendly recovery wording in nested detail, got %s", string(body))
	}

	origBody, _ := json.Marshal(event.Detail)
	if !strings.Contains(string(origBody), raw) {
		t.Fatalf("original nested detail should stay raw for audit trail, got %s", string(origBody))
	}
}

func TestStreamChat_SessionCreation(t *testing.T) {
	gw, _ := newTestGateway()

	body := `{"messages":[{"role":"user","content":"stream session test"}],"session_id":"stream-sess-1"}`
	req := httptest.NewRequest("POST", "/v1/chat/stream", strings.NewReader(body))
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handleStreamChat(w, req)

	stored := gw.convStore.Get("stream-sess-1")
	if len(stored) == 0 {
		t.Fatal("expected session to be created with user message")
	}
	if stored[0].Content != "stream session test" {
		t.Fatalf("expected stored message, got %q", stored[0].Content)
	}
}

func TestStreamChat_ViaE2E(t *testing.T) {
	streamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", 500)
			return
		}
		chunks := []string{
			`{"choices":[{"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":"test"},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":""},"finish_reason":"stop"}]}`,
		}
		for _, c := range chunks {
			w.Write([]byte("data: " + c + "\n\n"))
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer streamServer.Close()

	gw, tm := newE2EGateway(streamServer.URL)
	tn := tm.Register("stream-e2e")

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := authedRequest("POST", "/v1/chat/stream", body, tn.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
