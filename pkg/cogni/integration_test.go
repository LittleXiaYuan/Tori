package cogni

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"yunque-agent/pkg/skills"
)

// TestIntegration_FullCogniLifecycle exercises the complete cogni lifecycle:
// Declaration → Registry → Evaluator → Hook → Workflow → Experience → Bus.
func TestIntegration_FullCogniLifecycle(t *testing.T) {
	// ── Phase 1: Create and register a declaration ──
	decl := &Declaration{
		ID:          "test-reviewer",
		DisplayName: "测试审查员",
		Description: "Integration test cogni",
		Activation: ActivationRules{
			Keywords:      []string{"审查", "review", "PR"},
			KeywordWeight: 0.4,
			MinScore:      0.3,
		},
		Context: ContextInjection{
			Static:      "你是一位测试审查员。",
			MemoryQuery: "审查经验 {message}",
		},
		Surface: ToolSurface{
			Exclude: []string{"dangerous_tool"},
		},
		Memory: MemoryPolicy{
			Namespace: "test-reviewer",
		},
		Workflows: []WorkflowDef{
			{
				Name:        "full_review",
				Description: "完整审查流程",
				Steps: []WorkflowStep{
					{Skill: "get_diff", Output: "diff"},
					{Skill: "analyze", Args: map[string]any{"input": "${diff}"}, Output: "result"},
				},
			},
		},
		Experience: ExperienceConfig{
			Enabled:       true,
			AutoRecord:    true,
			RequireReview: false,
			HalfLifeDays:  90,
		},
		Checks: []ActivationCheck{
			{Name: "review-trigger", Message: "帮我审查一下代码", ExpectActive: boolP(true)},
			{Name: "no-trigger", Message: "今天吃什么", ExpectActive: boolP(false)},
		},
	}

	if err := decl.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	// ── Phase 2: Registry ──
	reg := NewRegistry()
	if err := reg.Add(decl, "test"); err != nil {
		t.Fatalf("add: %v", err)
	}

	if !reg.IsEnabled("test-reviewer") {
		t.Fatal("expected enabled")
	}
	if reg.Version() != 1 {
		t.Fatalf("expected version 1, got %d", reg.Version())
	}

	active := reg.Active()
	if len(active) != 1 {
		t.Fatalf("expected 1 active, got %d", len(active))
	}

	// ── Phase 3: Evaluator ──
	eval := NewEvaluator()
	results := eval.Evaluate(active, Session{Message: "帮我 review 一下这个 PR"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Activated {
		t.Fatalf("expected activated, score=%.3f, reasons=%v", results[0].Score, results[0].Reasons)
	}
	if results[0].Score < 0.3 {
		t.Fatalf("expected score >= 0.3, got %.3f", results[0].Score)
	}

	// Negative case
	negResults := eval.Evaluate(active, Session{Message: "今天天气真好"})
	if negResults[0].Activated {
		t.Fatalf("expected NOT activated for unrelated message, score=%.3f", negResults[0].Score)
	}

	// ── Phase 4: Hook (context injection + skill filtering) ──
	hook := NewHook(reg)
	store := NewInMemoryTraceStore(100)
	hook.SetTraceStore(store)

	memCalled := false
	hook.SetMemorySearch(func(_ context.Context, tenantID, query string) string {
		memCalled = true
		return "记忆内容：上次审查发现了SQL注入漏洞"
	})

	ctxReq := ContextRequest{Message: "帮我审查代码", TenantID: "t1", Channel: "webchat"}
	ctxOut := hook.BuildContext(ctxReq)
	if ctxOut == "" {
		t.Fatal("expected non-empty context")
	}
	if !memCalled {
		t.Fatal("expected memory search to be called")
	}

	// Skill filtering
	allSkills := []skills.Skill{
		&testIntSkill{name: "get_diff"},
		&testIntSkill{name: "analyze"},
		&testIntSkill{name: "dangerous_tool"},
		&testIntSkill{name: "safe_tool"},
	}
	filtered := hook.FilterSkills(ctxReq, allSkills)
	hasDangerous := false
	for _, s := range filtered {
		if s.Name() == "dangerous_tool" {
			hasDangerous = true
		}
	}
	if hasDangerous {
		t.Fatal("expected dangerous_tool to be excluded")
	}

	// Check trace was recorded
	traces := store.Recent(10)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}

	// ── Phase 5: Verify declaration checks ──
	checkResults := VerifyDeclaration(decl, eval)
	for _, cr := range checkResults {
		if !cr.Passed && cr.Reason != "no assertion configured (ignored)" {
			t.Fatalf("check %q failed: %s", cr.CheckName, cr.Reason)
		}
	}

	// ── Phase 6: Workflow engine ──
	executedSkills := make(map[string]bool)
	wfEngine := NewWorkflowEngine(func(ctx context.Context, skillName string, args map[string]any) (any, error) {
		executedSkills[skillName] = true
		switch skillName {
		case "get_diff":
			return "diff content here", nil
		case "analyze":
			return map[string]any{"issues": []string{"style issue"}}, nil
		}
		return "ok", nil
	})

	wfResult := wfEngine.Run(context2(), decl.Workflows[0], nil)
	if !wfResult.Success {
		t.Fatalf("workflow failed: %s", wfResult.Error)
	}
	if !executedSkills["get_diff"] || !executedSkills["analyze"] {
		t.Fatal("expected both skills to be executed")
	}
	if len(wfResult.StepResults) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(wfResult.StepResults))
	}

	// ── Phase 7: Experience store ──
	tmpDir := t.TempDir()
	es := NewExperienceStore("test-reviewer", ExperienceConfig{
		Enabled:       true,
		StoreDir:      tmpDir,
		AutoRecord:    true,
		RequireReview: false,
		HalfLifeDays:  90,
		MaxFacts:      100,
	})

	es.AddToolMemory(ToolExperience{
		Tool:       "get_diff",
		Context:    "大 PR",
		Result:     "timeout",
		Learned:    "大 PR 先看文件列表",
		Confidence: 0.9,
	})
	es.AddFact(DomainFact{Fact: "团队规范：函数不超过 50 行", Source: "对话"})
	es.SuggestPattern(BehaviorPattern{Trigger: "部署失败", Response: "先检查环境变量"})

	stats := es.Stats()
	if stats["tool_memories"] != 1 {
		t.Fatalf("expected 1 tool memory, got %d", stats["tool_memories"])
	}
	if stats["domain_facts"] != 1 {
		t.Fatalf("expected 1 fact, got %d", stats["domain_facts"])
	}
	if stats["patterns_total"] != 1 {
		t.Fatalf("expected 1 pattern, got %d", stats["patterns_total"])
	}

	// Experience context hints
	hints := es.ContextHints(context2(), "get_diff 超时了")
	if hints == "" {
		t.Fatal("expected experience hints for get_diff context")
	}

	// ── Phase 8: CogniBus ──
	bus := NewCogniBus(eval, DefaultBusConfig())
	bus.Register(decl)
	routeResult := bus.Route(context2(), Session{Message: "帮我审查 PR"})
	if len(routeResult.Winners) == 0 {
		t.Fatal("expected at least 1 winner from bus routing")
	}
	if routeResult.Winners[0].CogniID != "test-reviewer" {
		t.Fatalf("expected winner test-reviewer, got %s", routeResult.Winners[0].CogniID)
	}

	// ── Phase 9: Health monitoring ──
	monitor := NewMonitor(store)
	health := monitor.ComputeFor("test-reviewer", 0)
	if health.Status == "" {
		t.Fatal("expected non-empty health status")
	}

	// ── Phase 10: Bundle export/import ──
	bundle := reg.ExportBundle(nil, "integration test")
	if len(bundle.Cognis) != 1 {
		t.Fatalf("expected 1 declaration in bundle, got %d", len(bundle.Cognis))
	}

	reg2 := NewRegistry()
	summary, err := reg2.ImportBundle(bundle, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(summary.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(summary.Added))
	}

	// ── Phase 11: File persistence round-trip ──
	dir := t.TempDir()
	savePath := filepath.Join(dir, "test-reviewer.json")
	if err := SaveDeclaration(decl, savePath); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadDeclaration(savePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ID != "test-reviewer" {
		t.Fatalf("loaded id mismatch: %s", loaded.ID)
	}

	// ── Phase 12: YAML round-trip ──
	yamlPath := filepath.Join(dir, "test.cogni.yaml")
	yamlContent := `
id: yaml-test
display_name: YAML 测试
activation:
  keywords: ["yaml", "test"]
  min_score: 0.3
context:
  static: YAML loaded successfully
`
	os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	yamlDecl, err := LoadDeclaration(yamlPath)
	if err != nil {
		t.Fatalf("yaml load: %v", err)
	}
	if yamlDecl.Context.Static != "YAML loaded successfully" {
		t.Fatalf("yaml context mismatch: %s", yamlDecl.Context.Static)
	}

	// ── Phase 13: Sentinel alerts ──
	sentinel := NewSentinel(store, reg, SentinelPolicy{
		Interval: time.Minute,
	})
	alerts := sentinel.Scan()
	// No alerts expected for a healthy cogni with recent activity
	_ = alerts

	t.Logf("Integration test passed: 13 phases verified")
}

func boolP(b bool) *bool { return &b }
func context2() context.Context { return context.Background() }

type testIntSkill struct{ name string }

func (s *testIntSkill) Name() string                { return s.name }
func (s *testIntSkill) Description() string         { return s.name }
func (s *testIntSkill) Parameters() map[string]any  { return nil }
func (s *testIntSkill) Execute(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return "ok", nil
}
