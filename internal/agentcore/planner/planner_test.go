package planner

import (
	"testing"
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
