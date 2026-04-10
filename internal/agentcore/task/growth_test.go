package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestDynamicSkillExecute(t *testing.T) {
	def := DynamicSkillDef{
		Name:        "greet",
		Description: "打招呼技能",
		Instruction: "你是一个友好的技能，当收到参数时，用中文打招呼。",
	}

	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		if !strings.Contains(system, "友好") {
			t.Fatal("expected instruction in system prompt")
		}
		return "你好，欢迎使用！", nil
	}
	sk := NewDynamicSkill(def, skills.NewRegistry())
	if sk.Name() != "greet" {
		t.Fatalf("expected greet, got %s", sk.Name())
	}

	result, err := sk.Execute(context.Background(), map[string]any{"name": "Alice"}, &skills.Environment{LLMCall: skills.LLMCallFunc(mockLLM)})
	if err != nil {
		t.Fatal(err)
	}
	if result != "你好，欢迎使用！" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestSkillGeneratorFromGap(t *testing.T) {
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{name: "web_search", result: "results"})
	reg.Register(&mockSkill{name: "file_write", result: "wrote"})

	// Mock LLM that generates a skill definition when asked
	callCount := 0
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		if callCount == 1 {
			// First call: generate skill definition
			def := DynamicSkillDef{
				Name:        "email_send",
				Description: "发送邮件通知",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"to":      map[string]any{"type": "string"},
						"subject": map[string]any{"type": "string"},
						"body":    map[string]any{"type": "string"},
					},
					"required": []string{"to", "subject", "body"},
				},
				Instruction: "模拟发送邮件。由于没有真正的邮件服务，返回一个格式化的邮件摘要。",
				ComposedOf:  []string{},
			}
			b, _ := json.Marshal(def)
			return string(b), nil
		}
		// Subsequent calls: execute the generated skill
		return "邮件已发送至 admin@example.com", nil
	}

	gen := NewSkillGenerator(mockLLM, reg, nil)

	gap := &GapRecord{
		ID:         "gap-1",
		TaskID:     "t1",
		StepID:     1,
		StepAction: "send email notification",
		SkillName:  "email_send",
		ErrorMsg:   `skill "email_send" not found`,
		GapType:    GapSkillMissing,
	}

	skill, err := gen.Generate(context.Background(), gap)
	if err != nil {
		t.Fatal(err)
	}

	if skill.Name() != "email_send" {
		t.Fatalf("expected email_send, got %s", skill.Name())
	}

	// Verify it's registered
	if _, ok := reg.Get("email_send"); !ok {
		t.Fatal("generated skill should be registered")
	}

	// Execute the generated skill
	result, err := skill.Execute(context.Background(), map[string]any{
		"to":      "admin@example.com",
		"subject": "Test",
		"body":    "Hello",
	}, &skills.Environment{LLMCall: skills.LLMCallFunc(mockLLM)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "邮件") {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestGrowthLoopIntegrated(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	reg := skills.NewRegistry()
	// No email skill registered — will trigger gap

	callCount := 0
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		switch {
		case callCount == 1:
			// Planning call: plan one step using email_send
			return `[{"action":"send email","skill_name":"email_send","args":{"to":"admin","body":"test"}}]`, nil
		case strings.Contains(system, "技能生成器"):
			// Skill generation call
			def := DynamicSkillDef{
				Name:        "email_send",
				Description: "发送邮件",
				Instruction: "模拟发送邮件，返回发送结果摘要。",
			}
			b, _ := json.Marshal(def)
			return string(b), nil
		case strings.Contains(system, "错误分析"):
			// Gap suggestion
			return "需要安装邮件技能", nil
		default:
			// Dynamic skill execution
			return "邮件已发送成功", nil
		}
	}

	gap := NewGapAnalyzer(mockLLM)
	gen := NewSkillGenerator(mockLLM, reg, nil)

	env := &skills.Environment{LLMCall: skills.LLMCallFunc(mockLLM)}
	runner := NewRunner(s, reg, mockLLM, env)
	runner.SetGapAnalyzer(gap)
	runner.SetSkillGenerator(gen)

	tk, _ := s.Create(CreateRequest{Description: "send email to admin"})
	err := runner.Run(context.Background(), tk.ID)

	// Should succeed because growth loop auto-generated the skill
	if err != nil {
		t.Fatalf("expected success after auto-generation, got: %v", err)
	}

	got, _ := s.Get(tk.ID)
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s (error: %s)", got.Status, got.Error)
	}

	// Verify skill was registered
	if _, ok := reg.Get("email_send"); !ok {
		t.Fatal("email_send should be registered after growth")
	}

	// Verify gap was resolved
	stats := gap.Stats()
	if stats["unresolved"] != 0 {
		t.Fatalf("expected 0 unresolved gaps, got %d", stats["unresolved"])
	}

	// Step should show resolved gap
	if !strings.Contains(got.Steps[0].GapType, "auto_resolved") {
		t.Fatalf("expected auto_resolved in gap_type, got %s", got.Steps[0].GapType)
	}
}

func TestDynamicSkillNameConflict(t *testing.T) {
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{name: "web_search", result: "ok"}) // conflict target

	def := DynamicSkillDef{
		Name:        "web_search", // conflicts with existing
		Description: "另一个搜索技能",
		Instruction: "搜索网络。",
	}
	b, _ := json.Marshal(def)
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		return string(b), nil
	}

	gen := NewSkillGenerator(mockLLM, reg, nil)
	gap := &GapRecord{
		SkillName: "web_search",
		GapType:   GapSkillMissing,
		ErrorMsg:  "not found",
	}

	skill, err := gen.Generate(context.Background(), gap)
	if err != nil {
		t.Fatal(err)
	}

	// Should get _gen suffix to avoid conflict
	if skill.Name() != "web_search_gen" {
		t.Fatalf("expected web_search_gen, got %s", skill.Name())
	}
}
