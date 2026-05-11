package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/pkg/skills"
)

// ── Action Types ─────────────────────────────────────────────

func TestAskAction(t *testing.T) {
	action := AskAction("What color?",
		AskOption{Label: "Red", Value: "red"},
		AskOption{Label: "Blue", Value: "blue", Hint: "Cool color"},
	)
	if action.Kind != ActionAsk {
		t.Fatalf("expected ActionAsk, got %s", action.Kind)
	}
	payload, ok := action.Payload.(AskPayload)
	if !ok {
		t.Fatal("payload type mismatch")
	}
	if payload.Question != "What color?" {
		t.Errorf("unexpected question: %s", payload.Question)
	}
	if len(payload.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(payload.Options))
	}
}

func TestConfirmAction(t *testing.T) {
	action := ConfirmAction("Delete all data?", true)
	if action.Kind != ActionConfirm {
		t.Fatalf("expected ActionConfirm, got %s", action.Kind)
	}
	payload, ok := action.Payload.(ConfirmPayload)
	if !ok {
		t.Fatal("payload type mismatch")
	}
	if !payload.Destructive {
		t.Error("expected destructive=true")
	}
}

func TestFileAction(t *testing.T) {
	action := FileAction("/tmp/report.pdf", "report.pdf", "application/pdf", 1024)
	if action.Kind != ActionShowFile {
		t.Fatalf("expected ActionShowFile, got %s", action.Kind)
	}
	payload, ok := action.Payload.(FilePayload)
	if !ok {
		t.Fatal("payload type mismatch")
	}
	if payload.Size != 1024 {
		t.Errorf("unexpected size: %d", payload.Size)
	}
}

func TestSuggestAction(t *testing.T) {
	action := SuggestAction(
		Suggestion{Label: "Try this", Prompt: "do something"},
		Suggestion{Label: "Or this", Prompt: "do else"},
	)
	if action.Kind != ActionSuggest {
		t.Fatalf("expected ActionSuggest, got %s", action.Kind)
	}
	payload, ok := action.Payload.(SuggestPayload)
	if !ok {
		t.Fatal("payload type mismatch")
	}
	if len(payload.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(payload.Suggestions))
	}
}

func TestActionKindConstants(t *testing.T) {
	kinds := []ActionKind{ActionAsk, ActionConfirm, ActionShowFile, ActionSuggest, ActionProgress, ActionRequestInput}
	seen := make(map[ActionKind]bool)
	for _, k := range kinds {
		if seen[k] {
			t.Errorf("duplicate action kind: %s", k)
		}
		seen[k] = true
		if string(k) == "" {
			t.Error("empty action kind")
		}
	}
}

// ── Interrupt Handling ───────────────────────────────────────

func TestCheckInterrupt_NilRunState(t *testing.T) {
	p := &Planner{}
	interrupted, msgs := p.checkInterrupt(PlanRequest{}, nil)
	if interrupted {
		t.Error("should not interrupt without runState")
	}
	if msgs != nil {
		t.Error("should return nil messages")
	}
}

func TestCheckInterrupt_EmptyTaskID(t *testing.T) {
	p := &Planner{
		runState: func(_ string) *session.RunState { return nil },
	}
	interrupted, msgs := p.checkInterrupt(PlanRequest{TaskID: ""}, nil)
	if interrupted {
		t.Error("should not interrupt with empty task ID")
	}
	if msgs != nil {
		t.Error("should return nil messages")
	}
}

func TestSupplementMessages_Empty(t *testing.T) {
	msgs := supplementMessages(nil)
	if msgs != nil {
		t.Error("expected nil for empty supplements")
	}
	msgs = supplementMessages([]string{})
	if msgs != nil {
		t.Error("expected nil for empty slice")
	}
}

func TestSupplementMessages_NonEmpty(t *testing.T) {
	msgs := supplementMessages([]string{"info1", "info2"})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("expected user role, got %s", msgs[0].Role)
	}
	if !strings.Contains(msgs[0].Content, "info1") || !strings.Contains(msgs[0].Content, "info2") {
		t.Error("supplements not joined correctly")
	}
}

// ── Template Detection ───────────────────────────────────────

func TestDetectPlaceholders(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"Hello {{name}}, your order {{order_id}} is ready.", 2},
		{"No placeholders here.", 0},
		{"{{single}}", 1},
		{"{{a}} {{b}} {{c}} {{d}}", 4},
		{"nested {{outer}} and {{inner}}", 2},
	}
	for _, tc := range cases {
		got := detectPlaceholders(tc.input)
		if len(got) != tc.want {
			t.Errorf("detectPlaceholders(%q): got %d placeholders, want %d", tc.input[:min(40, len(tc.input))], len(got), tc.want)
		}
	}
}

// ── PlanResult Methods ───────────────────────────────────────

func TestPlanResult_ExecutionSummary_AllStatuses(t *testing.T) {
	result := &PlanResult{
		Reply: "done",
		Plan: []PlanStep{
			{Skill: "s1", Status: StepDone, Result: "ok"},
			{Skill: "s2", Status: StepFailed, Error: "timeout"},
			{Skill: "s3", Status: StepSkipped},
			{Skill: "s4", Status: StepRunning},
		},
	}
	summary := result.ExecutionSummary()
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if !strings.Contains(summary, "s1") {
		t.Error("should mention done skill")
	}
	if !strings.Contains(summary, "s2") {
		t.Error("should mention failed skill")
	}
}

func TestPlanResult_JSON(t *testing.T) {
	result := PlanResult{
		Reply:      "hello",
		SkillsUsed: []string{"web_search"},
		Steps:      2,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded PlanResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Reply != "hello" {
		t.Errorf("reply mismatch: %s", decoded.Reply)
	}
}

// ── Planner Configuration ────────────────────────────────────

func TestPlannerSetters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model")
	p := NewPlanner(client, skills.NewRegistry(), 8)

	p.SetNativeFC(true)
	if !p.useNativeFC {
		t.Error("SetNativeFC failed")
	}

	p.SetToolTimeout(45 * time.Second)
	if p.toolTimeout != 45*time.Second {
		t.Errorf("SetToolTimeout failed: %v", p.toolTimeout)
	}

	p.maxSteps = 20
	if p.maxSteps != 20 {
		t.Errorf("maxSteps set failed: %d", p.maxSteps)
	}
}

func TestPlannerLLMClientFor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model")
	p := NewPlanner(client, skills.NewRegistry(), 8)

	got := p.LLMClientFor("")
	if got == nil {
		t.Error("LLMClientFor('') should return primary client")
	}

	got = p.LLMClientFor("unknown-tier")
	if got == nil {
		t.Error("LLMClientFor with unknown tier should still return a client")
	}
}

// ── Multi-Step Execution ─────────────────────────────────────

func TestRunTextBased_MaxStepsEnforced(t *testing.T) {
	callCount := int32(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		reply := fmt.Sprintf(`{"tool_calls": [{"name": "loop_skill", "arguments": {"n": "%d"}}]}`, n)
		if n > 5 {
			reply = "Final answer after many steps."
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": reply}},
			},
		})
	}))
	defer srv.Close()

	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "loop_skill", desc: "loops",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			return "continue", nil
		},
	})

	client := llm.NewClient(srv.URL, "test-key", "test-model")
	p := NewPlanner(client, reg, 3)

	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "loop forever"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Steps > 4 {
		t.Errorf("expected steps capped at ~3, got %d", result.Steps)
	}
}

func TestRunTextBased_SkillError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		var reply string
		if callCount == 1 {
			reply = `{"tool_calls": [{"name": "bad_skill", "arguments": {}}]}`
		} else {
			reply = "The skill failed, but here is a fallback answer."
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": reply}},
			},
		})
	}))
	defer srv.Close()

	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "bad_skill", desc: "fails",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			return "", fmt.Errorf("skill execution failed: internal error")
		},
	})

	client := llm.NewClient(srv.URL, "test-key", "test-model")
	p := NewPlanner(client, reg, 8)

	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "try the bad skill"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reply == "" {
		t.Error("expected non-empty reply even after skill failure")
	}
}

// ── Prompt Builder ───────────────────────────────────────────

func TestBuildSystemPrompt_WithSkills(t *testing.T) {
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "calculator", desc: "Performs calculations",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			return "42", nil
		},
	})

	p := NewPlanner(nil, reg, 8)
	prompt := p.buildSystemPrompt()
	if !strings.Contains(prompt, "calculator") {
		t.Error("system prompt should list registered skills")
	}
}

func TestBuildSystemPrompt_Empty(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	prompt := p.buildSystemPrompt()
	if prompt == "" {
		t.Error("system prompt should not be empty even without skills")
	}
}

// ── PlanRequest Fields ───────────────────────────────────────

func TestPlanRequest_WithTaskContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		hasContext := false
		for _, m := range req.Messages {
			if strings.Contains(m.Content, "task-context-data") {
				hasContext = true
			}
		}
		reply := "without context"
		if hasContext {
			reply = "with task context"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": reply}},
			},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model")
	p := NewPlanner(client, skills.NewRegistry(), 8)

	result, err := p.Run(context.Background(), PlanRequest{
		Messages:    []llm.Message{{Role: "user", Content: "do task"}},
		TenantID:    "test",
		TaskID:      "task-123",
		TaskContext: "task-context-data: step 1 done",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reply == "" {
		t.Error("expected non-empty reply")
	}
}

func TestPlanRequest_WithModelOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "expert reply"}},
			},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model")
	p := NewPlanner(client, skills.NewRegistry(), 8)

	result, err := p.Run(context.Background(), PlanRequest{
		Messages:      []llm.Message{{Role: "user", Content: "expert question"}},
		TenantID:      "test",
		ModelOverride: "expert",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reply == "" {
		t.Error("expected non-empty reply")
	}
}

func TestBuildMessages_WithBeliefContext(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	p.SetBeliefContext(func(_ context.Context, message, tenantID, channel string) string {
		return "belief-context-data:" + message + ":" + tenantID + ":" + channel
	})

	msgs, _ := p.BuildMessages(context.Background(), PlanRequest{
		Messages:    []llm.Message{{Role: "user", Content: "hello world"}},
		TenantID:    "tenant-a",
		ChannelType: "web",
	})

	found := false
	for _, m := range msgs {
		if strings.Contains(m.Content, "belief-context-data:hello world:tenant-a:web") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("belief context not injected into messages: %#v", msgs)
	}
}

func TestBuildMessages_UsesQueryAwareStrategyContext(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	p.SetStrategyContext(func() string {
		return "general strategy should not be used"
	})
	p.SetStrategyContextFor(func(query string) string {
		if query != "请做 code review" {
			t.Fatalf("strategy query = %q", query)
		}
		return "scoped code review strategy"
	})

	msgs, _ := p.BuildMessages(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "请做 code review"}},
		TenantID: "tenant-a",
	})

	foundScoped := false
	for _, m := range msgs {
		if strings.Contains(m.Content, "scoped code review strategy") {
			foundScoped = true
		}
		if strings.Contains(m.Content, "general strategy should not be used") {
			t.Fatalf("query-aware strategy should take precedence: %#v", msgs)
		}
	}
	if !foundScoped {
		t.Fatalf("query-aware strategy context not injected: %#v", msgs)
	}
}

func TestBuildMessages_FallsBackToGeneralStrategyContext(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	p.SetStrategyContext(func() string {
		return "general strategy fallback"
	})
	p.SetStrategyContextFor(func(query string) string {
		return ""
	})

	msgs, _ := p.BuildMessages(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "普通问题"}},
		TenantID: "tenant-a",
	})

	for _, m := range msgs {
		if strings.Contains(m.Content, "general strategy fallback") {
			return
		}
	}
	t.Fatalf("general strategy fallback not injected: %#v", msgs)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
