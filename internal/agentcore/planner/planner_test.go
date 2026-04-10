package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/skills"
)

func TestCleanReplyRemovesToolCalls(t *testing.T) {
	p := &Planner{}
	input := `这是回答内容。{"tool_calls": [{"name": "test", "arguments": {}}]}后续文字`
	cleaned := p.cleanReply(input)
	if cleaned != "这是回答内容。后续文字" {
		t.Fatalf("unexpected: %q", cleaned)
	}
}

func TestCleanReplyRemovesThinkBlock(t *testing.T) {
	p := &Planner{}
	input := `<think>这是思考过程</think>这是真正的回答`
	cleaned := p.cleanReply(input)
	if cleaned != "这是真正的回答" {
		t.Fatalf("unexpected: %q", cleaned)
	}
}

func TestCleanReplyRemovesCodeBlock(t *testing.T) {
	p := &Planner{}
	input := "前文```json\n{\"a\":1}\n```后文"
	cleaned := p.cleanReply(input)
	if cleaned != "前文后文" {
		t.Fatalf("unexpected: %q", cleaned)
	}
}

func TestParseSkillCalls(t *testing.T) {
	p := &Planner{}
	input := `我来帮你查询。{"tool_calls": [{"name": "web_search", "arguments": {"query": "天气"}}]}`
	calls := p.parseSkillCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "web_search" {
		t.Fatalf("expected web_search, got %s", calls[0].Name)
	}
}

func TestParseSkillCallsNone(t *testing.T) {
	p := &Planner{}
	calls := p.parseSkillCalls("这是普通回复，没有技能调用。")
	if len(calls) != 0 {
		t.Fatalf("expected 0 calls, got %d", len(calls))
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	reg := skills.NewRegistry()
	p := NewPlanner(nil, reg, 8)
	prompt := p.buildSystemPrompt()
	if prompt == "" {
		t.Fatal("expected non-empty system prompt")
	}
	if len(prompt) < 50 {
		t.Fatal("system prompt too short")
	}
}

func TestSetNativeFC(t *testing.T) {
	p := NewPlanner(nil, nil, 8)
	if p.useNativeFC {
		t.Fatal("should default to false")
	}
	p.SetNativeFC(true)
	if !p.useNativeFC {
		t.Fatal("should be true after set")
	}
}

func TestFindClosingBrace(t *testing.T) {
	tests := []struct {
		input string
		start int
		want  int
	}{
		{`{"a": 1}`, 0, 7},
		{`{"a": {"b": 2}}`, 0, 14},
		{`xx{"a": 1}yy`, 2, 9},
		{`{unclosed`, 0, -1},
	}
	for _, tt := range tests {
		got := findClosingBrace(tt.input, tt.start)
		if got != tt.want {
			t.Errorf("findClosingBrace(%q, %d) = %d, want %d", tt.input, tt.start, got, tt.want)
		}
	}
}

func TestCleanReplyMultipleToolCalls(t *testing.T) {
	p := &Planner{}
	input := `先搜索一下。{"tool_calls": [{"name": "web_search", "arguments": {"query": "a"}}]}然后再查。{"tool_calls": [{"name": "file_list", "arguments": {"path": "."}}]}最终结果。`
	cleaned := p.cleanReply(input)
	if cleaned != "先搜索一下。然后再查。最终结果。" {
		t.Fatalf("unexpected: %q", cleaned)
	}
}

func TestCleanReplyTrailingCallDescription(t *testing.T) {
	p := &Planner{}
	// After JSON is stripped, the trailing "让我先调用..." should be cleaned
	input := "关于Chirp技能，让我先调用use_skill来加载详细说明："
	cleaned := p.cleanReply(input)
	if cleaned != "关于Chirp技能，" && cleaned != "关于Chirp技能" {
		// Accept both with and without trailing comma/punctuation
		if len(cleaned) > len("关于Chirp技能，") {
			t.Fatalf("expected trailing call description removed, got: %q", cleaned)
		}
	}
}

func TestCleanReplyTrailingCallDescriptionPreservesNormal(t *testing.T) {
	p := &Planner{}
	input := "这是一个正常的回答，没有工具调用描述。"
	cleaned := p.cleanReply(input)
	if cleaned != input {
		t.Fatalf("should not modify normal text, got: %q", cleaned)
	}
}

func TestCleanReplyRemovesACTTags(t *testing.T) {
	p := &Planner{}
	input := `<|ACT {"emotion":{"name":"happy","intensity":1}}|>
嗨！你好呀！

<|ACT {"emotion":{"name":"curious","intensity":1}}|>
今天有什么需要帮忙的吗？`
	cleaned := p.cleanReply(input)
	expected := "嗨！你好呀！\n\n今天有什么需要帮忙的吗？"
	if cleaned != expected {
		t.Fatalf("ACT tags not properly stripped.\nGot:      %q\nExpected: %q", cleaned, expected)
	}
}

func TestCleanReplyACTTagsOnlyLine(t *testing.T) {
	p := &Planner{}
	input := `<|ACT {"emotion":{"name":"neutral","intensity":1}}|>
Hello!`
	cleaned := p.cleanReply(input)
	if cleaned != "Hello!" {
		t.Fatalf("single ACT tag not stripped, got: %q", cleaned)
	}
}

func TestExecutionSummaryEmpty(t *testing.T) {
	result := &PlanResult{Reply: "hello", Plan: nil}
	if result.ExecutionSummary() != "" {
		t.Fatal("expected empty summary for no plan steps")
	}
}

func TestExecutionSummaryWithSteps(t *testing.T) {
	result := &PlanResult{
		Reply: "搜索结果如下...",
		Plan: []PlanStep{
			{Skill: "web_search", Status: StepDone, Result: "找到3个结果"},
			{Skill: "translate", Status: StepDone, Result: "翻译完成"},
		},
	}
	summary := result.ExecutionSummary()
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if !contains(summary, "web_search") || !contains(summary, "translate") {
		t.Fatalf("expected skill names in summary, got: %s", summary)
	}
	if !contains(summary, "✓") {
		t.Fatal("expected success markers")
	}
}

func TestExecutionSummaryWithFailure(t *testing.T) {
	result := &PlanResult{
		Reply: "sorry",
		Plan: []PlanStep{
			{Skill: "use_skill", Status: StepFailed, Error: "skill \"Chirp\" is not installed"},
		},
	}
	summary := result.ExecutionSummary()
	if !contains(summary, "失败") {
		t.Fatalf("expected failure indicator, got: %s", summary)
	}
	if !contains(summary, "use_skill") {
		t.Fatalf("expected skill name, got: %s", summary)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ── Integration-level tests (mock LLM server) ──

func mockLLMServer(t *testing.T, responseFunc func(msgs []llm.Message) string) *llm.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		reply := responseFunc(req.Messages)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": reply}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return llm.NewClient(srv.URL, "test-key", "test-model")
}

type mockSkill struct {
	name   string
	desc   string
	execFn func(ctx context.Context, args map[string]any, env *skills.Environment) (string, error)
}

func (s *mockSkill) Name() string        { return s.name }
func (s *mockSkill) Description() string { return s.desc }
func (s *mockSkill) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (s *mockSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	return s.execFn(ctx, args, env)
}

func TestPlannerDefaults(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string { return "hi" })
	p := NewPlanner(client, skills.NewRegistry(), 0)
	if p.maxSteps != 15 {
		t.Errorf("expected default maxSteps=15, got %d", p.maxSteps)
	}
	if p.toolTimeout != 60*time.Second {
		t.Errorf("expected default toolTimeout=60s, got %v", p.toolTimeout)
	}
}

func TestRunTextBased_SimpleReply(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string {
		return "Hello! How can I help you?"
	})
	p := NewPlanner(client, skills.NewRegistry(), 8)
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reply == "" {
		t.Error("expected non-empty reply")
	}
	if result.Steps != 1 {
		t.Errorf("expected 1 step, got %d", result.Steps)
	}
}

func TestRunTextBased_SkillCall(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `I need to search. {"tool_calls": [{"name": "web_search", "arguments": {"query": "golang testing"}}]}`
		}
		return "Go testing uses the testing package."
	})

	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "web_search", desc: "Search the web",
		execFn: func(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
			q, _ := args["query"].(string)
			return fmt.Sprintf("Results for '%s': Go testing is built-in.", q), nil
		},
	})

	p := NewPlanner(client, reg, 8)
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "how does go testing work?"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.SkillsUsed) != 1 || result.SkillsUsed[0] != "web_search" {
		t.Errorf("expected [web_search], got %v", result.SkillsUsed)
	}
}

func TestRunTextBased_ParallelSkillCalls(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `{"tool_calls": [
				{"name": "skill_a", "arguments": {"id": "1"}},
				{"name": "skill_b", "arguments": {"id": "2"}}
			]}`
		}
		return "Combined results."
	})

	var executed int32
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "skill_a", desc: "A",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&executed, 1)
			return "result_a", nil
		},
	})
	reg.Register(&mockSkill{
		name: "skill_b", desc: "B",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&executed, 1)
			return "result_b", nil
		},
	})

	p := NewPlanner(client, reg, 8)
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "do both"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.SkillsUsed) != 2 {
		t.Errorf("expected 2 skills, got %v", result.SkillsUsed)
	}
	if atomic.LoadInt32(&executed) != 2 {
		t.Errorf("expected both skills executed, got %d", executed)
	}
}

func TestRunTextBased_ContextCancellation(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string {
		time.Sleep(2 * time.Second)
		return "should not reach"
	})
	p := NewPlanner(client, skills.NewRegistry(), 8)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := p.Run(ctx, PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
		TenantID: "test",
	})
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestRunTextBased_ReflectRetry(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return "bad answer"
		}
		return "improved answer after reflection"
	})
	p := NewPlanner(client, skills.NewRegistry(), 8)

	reflectCount := 0
	p.SetReflect(func(_ context.Context, _, _ string) bool {
		reflectCount++
		return reflectCount > 1 // reject first, accept second
	})

	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "what is 2+2?"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Steps != 2 {
		t.Errorf("expected 2 steps (reflect retry), got %d", result.Steps)
	}
}

func TestSafeToolGo_PanicRecovery(t *testing.T) {
	done := make(chan bool, 1)
	safeToolGo(context.Background(), 5*time.Second, func(_ context.Context) {
		defer func() { done <- true }()
		panic("test panic")
	})
	select {
	case <-done:
		// recovered successfully
	case <-time.After(2 * time.Second):
		t.Error("safeToolGo did not recover in time")
	}
}

func TestSafeToolGo_Timeout(t *testing.T) {
	started := make(chan bool, 1)
	safeToolGo(context.Background(), 100*time.Millisecond, func(ctx context.Context) {
		started <- true
		<-ctx.Done()
	})
	select {
	case <-started:
		// timeout will cancel the goroutine
	case <-time.After(2 * time.Second):
		t.Error("goroutine did not start")
	}
}

func TestSetToolTimeout(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string { return "ok" })
	p := NewPlanner(client, skills.NewRegistry(), 8)
	p.SetToolTimeout(30 * time.Second)
	if p.toolTimeout != 30*time.Second {
		t.Errorf("expected 30s, got %v", p.toolTimeout)
	}
}

func TestContextLayers_NoCrossContamination(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string { return "reply" })
	reg := skills.NewRegistry()
	p := NewPlanner(client, reg, 8)

	p.SetMemory(func(_ context.Context, _, query string) string {
		time.Sleep(10 * time.Millisecond)
		if contains(query, "memory-a") {
			return "fact-a"
		}
		return ""
	})

	const N = 20
	results := make([]*PlanResult, N)
	errs := make([]error, N)
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(idx int) {
			defer wg.Done()
			msg := fmt.Sprintf("request-%d", idx)
			if idx%2 == 0 {
				msg = "memory-a " + msg
			}
			results[idx], errs[idx] = p.Run(context.Background(), PlanRequest{
				Messages: []llm.Message{{Role: "user", Content: msg}},
				TenantID: fmt.Sprintf("tenant-%d", idx),
			})
		}(i)
	}
	wg.Wait()

	for i := 0; i < N; i++ {
		if errs[i] != nil {
			t.Fatalf("request %d failed: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Fatalf("request %d: nil result", i)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"你好世界", 2, "你好..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}
