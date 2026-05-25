package planner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
)

func TestModelRuntimeServiceClientSelection(t *testing.T) {
	smart := llm.NewClient("http://smart.invalid", "k", "smart-model")
	expert := llm.NewClient("http://expert.invalid", "k", "expert-model")
	pool := llm.NewPool()
	pool.Register("smart", smart)
	pool.Register("expert", expert)
	pool.SetPrimary("smart")

	service := NewModelRuntimeService(smart)
	service.SetPool(pool)

	if got := service.ClientFor(""); got != smart {
		t.Fatal("expected default client without override")
	}
	if got := service.ClientFor("expert"); got != expert {
		t.Fatal("expected expert override client")
	}
	if got := service.ClientFor("missing"); got != smart {
		t.Fatal("expected missing override to fall back to pool primary")
	}
	override := llm.NewClient("http://session.invalid", "k", "session-model")
	if got := service.ClientForRequest(PlanRequest{ClientOverride: override, ModelOverride: "expert"}); got != override {
		t.Fatal("expected session client override to win")
	}
}

func TestModelRuntimeServiceFallbackChain(t *testing.T) {
	smart := llm.NewClient("http://smart.invalid", "k", "smart-model")
	fast := llm.NewClient("http://fast.invalid", "k", "fast-model")
	pool := llm.NewPool()
	pool.Register("smart", smart)
	pool.Register("fast", fast)
	pool.SetPrimary("smart")

	service := NewModelRuntimeService(smart)
	service.SetPool(pool)
	chain := service.FallbackChain("fast")
	if len(chain) < 2 || chain[0] != fast || chain[1] != smart {
		t.Fatalf("unexpected fallback chain %#v", chain)
	}

	service.SetPool(nil)
	chain = service.FallbackChain("fast")
	if len(chain) != 1 || chain[0] != smart {
		t.Fatalf("expected default-only fallback chain, got %#v", chain)
	}
}

func TestModelRuntimeServiceFallbackChainForRequestUsesCapabilityProvider(t *testing.T) {
	smart := llm.NewClient("http://smart.invalid", "k", "smart-model")
	vision := llm.NewClient("http://vision.invalid", "k", "vision-model")
	pool := llm.NewPool()
	pool.Register("smart", smart)
	pool.SetPrimary("smart")

	service := NewModelRuntimeService(smart)
	service.SetPool(pool)
	chain := service.FallbackChainForRequest(
		PlanRequest{},
		[]llm.Message{{Role: "user", ContentParts: []llm.ContentPart{{Type: "image_url", ImageURL: &llm.MediaURL{URL: "data:image/png;base64,AA=="}}}}},
		func(required ...llm.Capability) *llm.ProviderInstance {
			if len(required) != 1 || required[0] != llm.CapVision {
				t.Fatalf("expected vision capability requirement, got %#v", required)
			}
			return &llm.ProviderInstance{
				Config: llm.ProviderConfig{ID: "vision", Model: "vision-model"},
				Client: vision,
			}
		},
	)
	if len(chain) == 0 || chain[0] != vision {
		t.Fatalf("expected capability provider to lead fallback chain, got %#v", chain)
	}
	if len(chain) < 2 || chain[1] != smart {
		t.Fatalf("expected normal fallback chain after capability provider, got %#v", chain)
	}
}

func TestModelRuntimeServiceFallbackChainForRequestPinsSessionOverride(t *testing.T) {
	smart := llm.NewClient("http://smart.invalid", "k", "smart-model")
	override := llm.NewClient("http://override.invalid", "k", "override-model")
	service := NewModelRuntimeService(smart)

	chain := service.FallbackChainForRequest(
		PlanRequest{ClientOverride: override},
		[]llm.Message{{Role: "user", Content: "hello"}},
		func(required ...llm.Capability) *llm.ProviderInstance {
			t.Fatalf("session override should not invoke capability selector, got %#v", required)
			return nil
		},
	)
	if len(chain) != 1 || chain[0] != override {
		t.Fatalf("expected session override-only chain, got %#v", chain)
	}
}

func TestModelRuntimeServiceFallbackChainForPlannerRequestUsesStrategySelector(t *testing.T) {
	smart := llm.NewClient("http://smart.invalid", "k", "smart-model")
	vision := llm.NewClient("http://vision.invalid", "k", "vision-model")
	service := NewModelRuntimeService(smart)

	reg := llm.NewProviderRegistry(llm.NewPool())
	if err := reg.Register(llm.ProviderConfig{
		ID:           "vision-provider",
		Type:         llm.ProviderTypeChat,
		BaseURL:      "http://vision.invalid",
		APIKeys:      []string{"k"},
		Model:        "vision-model",
		Enabled:      true,
		Capabilities: []llm.Capability{llm.CapChat, llm.CapVision},
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if got := reg.Get("vision-provider"); got != nil {
		got.Client = vision
	}
	strategy := NewRuntimeStrategyService()
	strategy.SetProviderRegistry(reg)

	chain := service.FallbackChainForPlannerRequest(
		PlanRequest{},
		[]llm.Message{{Role: "user", ContentParts: []llm.ContentPart{{Type: "image_url", ImageURL: &llm.MediaURL{URL: "data:image/png;base64,AA=="}}}}},
		strategy,
	)
	if len(chain) == 0 || chain[0] != vision {
		t.Fatalf("expected strategy-selected vision provider first, got %#v", chain)
	}
}

func TestModelRuntimeServiceAdaptiveRoute(t *testing.T) {
	service := NewModelRuntimeService(nil)
	if got := service.AdaptiveRoute(PlanRequest{ModelOverride: "custom"}); got != "custom" {
		t.Fatalf("expected explicit override, got %q", got)
	}
	if got := service.AdaptiveRoute(PlanRequest{Messages: []llm.Message{{Role: "user", Content: "请分析这个架构并重构"}}}); got != "expert" {
		t.Fatalf("expected expert for complex intent, got %q", got)
	}
	if got := service.AdaptiveRoute(PlanRequest{Messages: []llm.Message{{Role: "user", Content: "hello"}}}); got != "fast" {
		t.Fatalf("expected fast for simple message, got %q", got)
	}
}

func TestModelRuntimeServiceThinkingFlagForRequest(t *testing.T) {
	service := NewModelRuntimeService(nil)
	explicitFalse := false
	if got := service.ThinkingFlagForRequest(PlanRequest{ThinkingEnabled: &explicitFalse}); got == nil || *got {
		t.Fatalf("expected explicit false thinking flag to be preserved, got %#v", got)
	}
	got := service.ThinkingFlagForRequest(PlanRequest{Messages: []llm.Message{{Role: "user", Content: "请分析这个问题"}}})
	if got == nil || !*got {
		t.Fatalf("expected complex query to auto-enable thinking, got %#v", got)
	}
	if got := service.ThinkingFlagForRequest(PlanRequest{Messages: []llm.Message{{Role: "user", Content: "hi"}}}); got != nil {
		t.Fatalf("expected simple query to keep nil thinking flag, got %#v", got)
	}
}

func TestModelRuntimeServiceReasoningCallbacksEmitEvents(t *testing.T) {
	service := NewModelRuntimeService(nil)
	var summaries []string
	var streamTypes []string
	cb := service.ReasoningCallbacks(func(summary string, detail map[string]any) {
		summaries = append(summaries, summary)
		if detail != nil {
			streamTypes = append(streamTypes, detail["stream_type"].(string))
		}
	})
	if cb == nil {
		t.Fatal("expected callback builder")
	}
	opts := &llm.ChatWithToolsOpts{}
	cb(opts)
	if opts.OnReasoningDelta == nil || opts.OnReasoning == nil {
		t.Fatalf("expected reasoning callbacks, got %#v", opts)
	}
	opts.OnReasoningDelta("delta")
	opts.OnReasoning("batch")
	if strings.Join(summaries, ",") != "delta,batch" {
		t.Fatalf("unexpected summaries %#v", summaries)
	}
	if strings.Join(streamTypes, ",") != "thinking_delta,reasoning_batch" {
		t.Fatalf("unexpected stream types %#v", streamTypes)
	}
	if got := service.ReasoningCallbacks(nil); got != nil {
		t.Fatal("nil emitter should return nil callback")
	}
}

func TestModelRuntimeServiceChatFallbackDegrades(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary model outage", http.StatusBadGateway)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "fallback ok"}},
			},
		})
	}))
	defer second.Close()

	service := NewModelRuntimeService(nil)
	var fallbackModel string
	var fallbackAttempt int
	reply, err := service.ChatFallback(context.Background(), []*llm.Client{
		llm.NewClient(first.URL, "k", "first-model"),
		llm.NewClient(second.URL, "k", "second-model"),
	}, []llm.Message{{Role: "user", Content: "hello"}}, func(model string, attempt int, err error) {
		fallbackModel = model
		fallbackAttempt = attempt
		if err == nil {
			t.Fatal("expected fallback reason error")
		}
	})
	if err != nil {
		t.Fatalf("chat fallback: %v", err)
	}
	if reply != "fallback ok" {
		t.Fatalf("unexpected reply %q", reply)
	}
	if fallbackModel != "second-model" || fallbackAttempt != 2 {
		t.Fatalf("unexpected fallback event model=%q attempt=%d", fallbackModel, fallbackAttempt)
	}
}

func TestModelRuntimeServiceRequestFallbackWrappers(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary model outage", http.StatusBadGateway)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "request fallback ok"}},
			},
		})
	}))
	defer second.Close()

	fast := llm.NewClient(first.URL, "k", "fast-model")
	smart := llm.NewClient(second.URL, "k", "smart-model")
	pool := llm.NewPool()
	pool.Register("fast", fast)
	pool.Register("smart", smart)
	pool.SetPrimary("smart")

	service := NewModelRuntimeService(smart)
	service.SetPool(pool)
	var fallbackAttempt int
	reply, err := service.ChatFallbackForRequest(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, []llm.Message{{Role: "user", Content: "hi"}}, nil, func(model string, attempt int, err error) {
		fallbackAttempt = attempt
		if model != "smart-model" || err == nil {
			t.Fatalf("unexpected fallback event model=%q attempt=%d err=%v", model, attempt, err)
		}
	})
	if err != nil {
		t.Fatalf("request fallback chat: %v", err)
	}
	if reply != "request fallback ok" || fallbackAttempt != 2 {
		t.Fatalf("unexpected request fallback reply=%q attempt=%d", reply, fallbackAttempt)
	}

	full, err := service.ChatFallbackFullForRequest(context.Background(), PlanRequest{}, []llm.Message{{Role: "user", Content: "hi"}}, nil, nil)
	if err != nil {
		t.Fatalf("request fallback full chat: %v", err)
	}
	if full.Content != "request fallback ok" {
		t.Fatalf("unexpected full fallback content %q", full.Content)
	}
}

func TestModelRuntimeServiceChatWithToolsFallbackForRequestWiresReasoningAndThinking(t *testing.T) {
	var thinkingEnabled *bool
	toolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ThinkingEnabled  *bool `json:"thinking_enabled"`
			EnableThinking   *bool `json:"enable_thinking"`
			IncludeReasoning *bool `json:"include_reasoning"`
			Thinking         struct {
				Type string `json:"type"`
			} `json:"thinking"`
			ReasoningEffort string `json:"reasoning_effort"`
			Reasoning       struct {
				Type string `json:"type"`
			} `json:"reasoning"`
			Messages []llm.Message `json:"messages"`
			Tools    []any         `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch {
		case payload.ThinkingEnabled != nil:
			thinkingEnabled = payload.ThinkingEnabled
		case payload.EnableThinking != nil:
			thinkingEnabled = payload.EnableThinking
		case payload.IncludeReasoning != nil:
			thinkingEnabled = payload.IncludeReasoning
		case payload.Thinking.Type == "enabled":
			enabled := true
			thinkingEnabled = &enabled
		case payload.ReasoningEffort != "":
			enabled := true
			thinkingEnabled = &enabled
		case payload.Reasoning.Type == "enabled":
			enabled := true
			thinkingEnabled = &enabled
		}
		if len(payload.Tools) != 1 {
			t.Fatalf("expected one tool definition, got %#v", payload.Tools)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":              "assistant",
						"content":           "no tool needed",
						"reasoning_content": "reasoned",
					},
				},
			},
		})
	}))
	defer toolServer.Close()

	service := NewModelRuntimeService(llm.NewClient(toolServer.URL, "k", "kimi-k2-test"))
	var reasoningEvents int
	explicitThinking := true
	reply, calls, reasoning, err := service.ChatWithToolsFallbackForRequest(
		context.Background(),
		PlanRequest{ThinkingEnabled: &explicitThinking},
		[]llm.Message{{Role: "user", Content: "请分析这个问题"}},
		[]llm.FunctionDef{{Name: "noop", Description: "noop", Parameters: map[string]any{"type": "object"}}},
		nil,
		func(summary string, detail map[string]any) {
			reasoningEvents++
		},
		nil,
	)
	if err != nil {
		t.Fatalf("request tools fallback: %v", err)
	}
	if reply != "no tool needed" || len(calls) != 0 || reasoning != "reasoned" {
		t.Fatalf("unexpected tools fallback reply=%q calls=%#v reasoning=%q", reply, calls, reasoning)
	}
	if thinkingEnabled == nil || !*thinkingEnabled {
		t.Fatalf("expected thinking flag to be wired, got %#v", thinkingEnabled)
	}
	if reasoningEvents != 1 {
		t.Fatalf("expected reasoning callback from response, got %d", reasoningEvents)
	}
}

func TestModelRuntimeServiceChatForRequestUsesSessionOverride(t *testing.T) {
	defaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "default"}},
			},
		})
	}))
	defer defaultServer.Close()
	overrideServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model    string        `json:"model"`
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Model != "override-model" {
			t.Fatalf("expected override model, got %q", payload.Model)
		}
		if len(payload.Messages) != 1 || payload.Messages[0].Content != "hello" {
			t.Fatalf("unexpected messages %#v", payload.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "override"}},
			},
		})
	}))
	defer overrideServer.Close()

	service := NewModelRuntimeService(llm.NewClient(defaultServer.URL, "k", "default-model"))
	reply, err := service.ChatForRequest(context.Background(), PlanRequest{
		ClientOverride: llm.NewClient(overrideServer.URL, "k", "override-model"),
	}, []llm.Message{{Role: "user", Content: "hello"}}, 0.3)
	if err != nil {
		t.Fatalf("chat for request: %v", err)
	}
	if reply != "override" {
		t.Fatalf("expected override reply, got %q", reply)
	}
}

func TestModelRuntimeServiceChatForRequestTierUsesTierUnlessSessionOverride(t *testing.T) {
	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "fast"}},
			},
		})
	}))
	defer fastServer.Close()
	expertServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "expert"}},
			},
		})
	}))
	defer expertServer.Close()
	overrideServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "override"}},
			},
		})
	}))
	defer overrideServer.Close()

	fast := llm.NewClient(fastServer.URL, "k", "fast-model")
	expert := llm.NewClient(expertServer.URL, "k", "expert-model")
	pool := llm.NewPool()
	pool.Register("fast", fast)
	pool.Register("expert", expert)
	pool.SetPrimary("fast")
	service := NewModelRuntimeService(fast)
	service.SetPool(pool)

	reply, err := service.ChatForRequestTier(context.Background(), PlanRequest{}, "expert", []llm.Message{{Role: "user", Content: "hello"}}, 0.7)
	if err != nil {
		t.Fatalf("tier chat: %v", err)
	}
	if reply != "expert" {
		t.Fatalf("expected expert tier reply, got %q", reply)
	}

	reply, err = service.ChatForRequestTier(context.Background(), PlanRequest{
		ClientOverride: llm.NewClient(overrideServer.URL, "k", "override-model"),
	}, "expert", []llm.Message{{Role: "user", Content: "hello"}}, 0.7)
	if err != nil {
		t.Fatalf("override tier chat: %v", err)
	}
	if reply != "override" {
		t.Fatalf("expected session override reply, got %q", reply)
	}
}

func TestModelRuntimeServiceChatForRequestReportsMissingClient(t *testing.T) {
	service := NewModelRuntimeService(nil)
	_, err := service.ChatForRequest(context.Background(), PlanRequest{}, []llm.Message{{Role: "user", Content: "hello"}}, 0.7)
	if err == nil || !strings.Contains(err.Error(), "planner LLM client not configured") {
		t.Fatalf("expected missing client error, got %v", err)
	}
	_, err = service.ChatForRequestTier(context.Background(), PlanRequest{}, "expert", []llm.Message{{Role: "user", Content: "hello"}}, 0.7)
	if err == nil || !strings.Contains(err.Error(), "planner LLM client not configured") {
		t.Fatalf("expected missing tier client error, got %v", err)
	}
}

func TestModelRuntimeServiceAnalyzeUploadedFile(t *testing.T) {
	var capturedPreview string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(payload.Messages) != 2 {
			t.Fatalf("expected system/user messages, got %#v", payload.Messages)
		}
		capturedPreview = payload.Messages[1].Content
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": `{"file_kind":"docx","is_template":false,"summary":"报名表","suggestions":["填充字段"]}`}},
			},
		})
	}))
	defer srv.Close()

	analysis, err := NewModelRuntimeService(llm.NewClient(srv.URL, "k", "test-model")).AnalyzeUploadedFile(context.Background(), "报名表.docx", "姓名：{{name}}\n学校：{{school}}")
	if err != nil {
		t.Fatalf("analyze upload: %v", err)
	}
	if analysis.FileKind != "docx" || analysis.Summary != "报名表" {
		t.Fatalf("unexpected analysis %#v", analysis)
	}
	if !analysis.IsTemplate {
		t.Fatalf("placeholder detection should force template=true: %#v", analysis)
	}
	if strings.Join(analysis.Placeholders, ",") != "name,school" {
		t.Fatalf("unexpected placeholders %#v", analysis.Placeholders)
	}
	if !strings.Contains(capturedPreview, "报名表.docx") || !strings.Contains(capturedPreview, "{{name}}") {
		t.Fatalf("unexpected upload prompt preview %q", capturedPreview)
	}
}

func TestModelRuntimeServiceAnalyzeUploadedFileFallbackOnInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "not-json"}},
			},
		})
	}))
	defer srv.Close()

	analysis, err := NewModelRuntimeService(llm.NewClient(srv.URL, "k", "test-model")).AnalyzeUploadedFile(context.Background(), "合同模板.txt", "正文")
	if err != nil {
		t.Fatalf("analyze upload fallback: %v", err)
	}
	if analysis.FileKind != "unknown" || !analysis.IsTemplate {
		t.Fatalf("expected filename-template fallback, got %#v", analysis)
	}
}

func TestModelRuntimeServiceGenerateConversationTitle(t *testing.T) {
	var capturedModel string
	var capturedUser string
	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model    string        `json:"model"`
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		capturedModel = payload.Model
		if len(payload.Messages) != 2 {
			t.Fatalf("expected title messages, got %#v", payload.Messages)
		}
		capturedUser = payload.Messages[1].Content
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "《项目复盘标题》"}},
			},
		})
	}))
	defer fastServer.Close()

	service := NewModelRuntimeService(nil)
	pool := llm.NewPool()
	pool.Register("fast", llm.NewClient(fastServer.URL, "k", "fast-model"))
	pool.SetPrimary("fast")
	service.SetPool(pool)

	title := service.GenerateConversationTitle(context.Background(), strings.Repeat("用", 320), "助手回答")
	if title != "项目复盘标题" {
		t.Fatalf("unexpected title %q", title)
	}
	if capturedModel != "fast-model" {
		t.Fatalf("expected fast model, got %q", capturedModel)
	}
	if strings.Count(capturedUser, "用") != 301 {
		t.Fatalf("expected user message to be rune-truncated at 300 characters plus prompt label, got %d", strings.Count(capturedUser, "用"))
	}
}

func TestModelRuntimeServiceParseMissionIntent(t *testing.T) {
	var capturedModel string
	var capturedUser string
	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model    string        `json:"model"`
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		capturedModel = payload.Model
		if len(payload.Messages) != 2 || !strings.Contains(payload.Messages[0].Content, "mission intent classifier") {
			t.Fatalf("unexpected mission prompt %#v", payload.Messages)
		}
		capturedUser = payload.Messages[1].Content
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": `{"type":"cron","name":"每日站会","description":"每天提醒我站会","config":{"cron_expr":"0 9 * * *","message":"站会"},"confidence":0.91,"explanation":"contains daily schedule"}`}},
			},
		})
	}))
	defer fastServer.Close()

	service := NewModelRuntimeService(nil)
	pool := llm.NewPool()
	pool.Register("fast", llm.NewClient(fastServer.URL, "k", "fast-model"))
	pool.SetPrimary("fast")
	service.SetPool(pool)

	result, err := service.ParseMissionIntent(context.Background(), "每天九点提醒我站会")
	if err != nil {
		t.Fatalf("parse mission: %v", err)
	}
	if result.Type != "cron" || result.Name != "每日站会" || result.Config["cron_expr"] != "0 9 * * *" {
		t.Fatalf("unexpected mission parse result %#v", result)
	}
	if capturedModel != "fast-model" || capturedUser != "每天九点提醒我站会" {
		t.Fatalf("unexpected mission model/user model=%q user=%q", capturedModel, capturedUser)
	}
}

func TestModelRuntimeServiceParseMissionIntentFallbackOnInvalidJSON(t *testing.T) {
	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "not-json"}},
			},
		})
	}))
	defer fastServer.Close()

	service := NewModelRuntimeService(llm.NewClient(fastServer.URL, "k", "fast-model"))
	result, err := service.ParseMissionIntent(context.Background(), "整理收件箱")
	if err != nil {
		t.Fatalf("parse mission fallback: %v", err)
	}
	if result.Type != "task" || result.Name != "整理收件箱" || result.Config["goal"] != "整理收件箱" || result.Confidence != 0.3 {
		t.Fatalf("unexpected fallback result %#v", result)
	}
}

func TestModelRuntimeServiceModelIDAndCacheStats(t *testing.T) {
	primary := llm.NewClient("http://primary.invalid", "k", "primary-model")
	expert := llm.NewClient("http://expert.invalid", "k", "expert-model")
	pool := llm.NewPool()
	pool.Register("primary", primary)
	pool.Register("expert", expert)
	pool.SetPrimary("primary")

	service := NewModelRuntimeService(primary)
	service.SetPool(pool)
	if got := service.ModelIDForTier("expert"); got != "expert-model" {
		t.Fatalf("expected expert model id, got %q", got)
	}
	if got := service.ModelIDForTier("missing"); got != "primary-model" {
		t.Fatalf("expected missing tier to fall back to primary model id, got %q", got)
	}
	if stats := service.DefaultResponseCacheStats(); stats == nil || stats["size"] == nil {
		t.Fatalf("expected default cache stats, got %#v", stats)
	}
	var nilService *ModelRuntimeService
	if got := nilService.ModelIDForTier("fast"); got != "" {
		t.Fatalf("nil service model id = %q", got)
	}
	if stats := nilService.DefaultResponseCacheStats(); stats != nil {
		t.Fatalf("nil service cache stats = %#v", stats)
	}
}

func TestModelRuntimeServiceHealth(t *testing.T) {
	var nilService *ModelRuntimeService
	if health := nilService.Health(); health.Configured {
		t.Fatalf("nil service health should be unconfigured, got %#v", health)
	}

	empty := NewModelRuntimeService(nil)
	if health := empty.Health(); health.Configured {
		t.Fatalf("empty service health should be unconfigured, got %#v", health)
	}

	client := llm.NewClient("http://primary.invalid", "k", "primary-model")
	client.Breaker().RecordFailure()
	service := NewModelRuntimeService(client)
	health := service.Health()
	if !health.Configured {
		t.Fatalf("expected configured health, got %#v", health)
	}
	if health.BreakerState != "closed" {
		t.Fatalf("expected closed breaker, got %#v", health)
	}
	if health.Failures != 1 {
		t.Fatalf("expected one recorded failure, got %#v", health)
	}
}

func TestNilModelRuntimeServiceIsNoop(t *testing.T) {
	var service *ModelRuntimeService
	if got := service.DefaultClient(); got != nil {
		t.Fatalf("nil service default client = %#v", got)
	}
	if got := service.Pool(); got != nil {
		t.Fatalf("nil service pool = %#v", got)
	}
	if got := service.ClientFor("smart"); got != nil {
		t.Fatalf("nil service client = %#v", got)
	}
	if got := service.Health(); got.Configured {
		t.Fatalf("nil service health = %#v", got)
	}
	if got := service.FallbackChain("smart"); got != nil {
		t.Fatalf("nil service fallback chain = %#v", got)
	}
}
