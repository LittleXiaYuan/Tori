package planner

import "testing"

func TestBuildPlannerFailureSummaryTargetsMissingSkills(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "research_skill", Status: StepFailed, Error: "unknown skill: research_skill"},
		{ID: 2, Skill: "research_skill", Status: StepFailed, Error: "missing skill research_skill"},
	})
	if !ok {
		t.Fatal("expected repeated failure summary")
	}
	if summary.PrimaryTarget == nil {
		t.Fatalf("expected skill recovery target, got nil summary=%#v", summary)
	}
	if summary.PrimaryTarget.Category != "skill" || summary.PrimaryTarget.Href != "/skills" {
		t.Fatalf("expected skill target, got %#v", summary.PrimaryTarget)
	}
	if summary.FailurePattern != "所需技能不可用" {
		t.Fatalf("unexpected failure pattern: %q", summary.FailurePattern)
	}
}

func TestBuildPlannerFailureSummaryKeepsUnknownToolsOnToolSurface(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "browser_extract", Status: StepFailed, Error: "unknown tool: browser_extract"},
		{ID: 2, Skill: "browser_extract", Status: StepFailed, Error: "not in allowed tool surface"},
	})
	if !ok {
		t.Fatal("expected repeated failure summary")
	}
	if summary.PrimaryTarget == nil {
		t.Fatalf("expected tool recovery target, got nil summary=%#v", summary)
	}
	if summary.PrimaryTarget.Category != "tool" || summary.PrimaryTarget.Href != "/tools" {
		t.Fatalf("expected tool target, got %#v", summary.PrimaryTarget)
	}
}

func TestBuildPlannerFailureSummaryTargetsConnectorRecovery(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "github", Status: StepFailed, Error: "connector github token expired"},
		{ID: 2, Skill: "github", Status: StepFailed, Error: "github rate limit 429"},
	})
	if !ok {
		t.Fatal("expected repeated failure summary")
	}
	if summary.PrimaryTarget == nil {
		t.Fatalf("expected connector recovery target, got nil summary=%#v", summary)
	}
	if summary.PrimaryTarget.Category != "connector" || summary.PrimaryTarget.Href != "/settings/connectors?focus=github" {
		t.Fatalf("expected focused connector target, got %#v", summary.PrimaryTarget)
	}
	if summary.FailurePattern != "连接器不可用" {
		t.Fatalf("unexpected failure pattern: %q", summary.FailurePattern)
	}
}

func TestBuildPlannerFailureSummaryTargetsProviderRecovery(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "llm", Status: StepFailed, Error: "chat API status 402: insufficient balance for openai provider"},
		{ID: 2, Skill: "model", Status: StepFailed, Error: "model provider rate limit 429 too many requests"},
	})
	if !ok {
		t.Fatal("expected repeated failure summary")
	}
	if summary.PrimaryTarget == nil {
		t.Fatalf("expected provider recovery target, got nil summary=%#v", summary)
	}
	if summary.PrimaryTarget.Category != "provider" || summary.PrimaryTarget.Href != "/settings/providers?tab=providers" {
		t.Fatalf("expected provider settings target, got %#v", summary.PrimaryTarget)
	}
	if summary.FailurePattern != "模型供应商不可用" {
		t.Fatalf("unexpected failure pattern: %q", summary.FailurePattern)
	}
	if summary.NextStep == "" || summary.Recommendation == "" {
		t.Fatalf("expected actionable provider recommendation, got %#v", summary)
	}
}

func TestBuildPlannerFailureSummaryKeepsConnectorRateLimitOnConnectorRecovery(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "github", Status: StepFailed, Error: "connector github rate limit 429"},
		{ID: 2, Skill: "github", Status: StepFailed, Error: "github connector upstream failed"},
	})
	if !ok {
		t.Fatal("expected repeated failure summary")
	}
	if summary.PrimaryTarget == nil {
		t.Fatalf("expected connector recovery target, got nil summary=%#v", summary)
	}
	if summary.PrimaryTarget.Category != "connector" || summary.PrimaryTarget.Href != "/settings/connectors?focus=github" {
		t.Fatalf("expected focused connector target, got %#v", summary.PrimaryTarget)
	}
}

func TestBuildPlannerFailureSummaryDoesNotTreatAuthTypeAsConnectorFailure(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "github", Status: StepFailed, Error: "github connector config auth_type=token"},
		{ID: 2, Skill: "github", Status: StepFailed, Error: "github connector config auth_type=token"},
	})
	if !ok {
		t.Fatal("expected repeated failure summary")
	}
	if summary.PrimaryTarget != nil && summary.PrimaryTarget.Category == "connector" {
		t.Fatalf("auth_type config should not become connector recovery, got %#v", summary.PrimaryTarget)
	}
}

func TestBuildPlannerFailureSummaryTargetsBrowserPackRecovery(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "browser", Status: StepFailed, Error: "browser extension pairing lost"},
		{ID: 2, Skill: "browser", Status: StepFailed, Error: "browser not paired"},
	})
	if !ok {
		t.Fatal("expected repeated failure summary")
	}
	if summary.PrimaryTarget == nil {
		t.Fatalf("expected browser recovery target, got nil summary=%#v", summary)
	}
	if summary.PrimaryTarget.Category != "browser" || summary.PrimaryTarget.Href != "/packs/browser" {
		t.Fatalf("expected browser pack target, got %#v", summary.PrimaryTarget)
	}
	if summary.FailurePattern != "浏览器连接不可用" {
		t.Fatalf("unexpected failure pattern: %q", summary.FailurePattern)
	}
}

func TestBuildPlannerFailureSummaryTargetsDependencyInspectionWithoutFakeHref(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Action: "等待前置步骤", Status: StepFailed, Error: "dependency step 2 尚未完成"},
		{ID: 2, Action: "继续后续步骤", Status: StepFailed, Error: "no ready steps: dependency blocked"},
	})
	if !ok {
		t.Fatal("expected repeated failure summary")
	}
	if summary.PrimaryTarget == nil {
		t.Fatalf("expected dependency recovery target, got nil summary=%#v", summary)
	}
	if summary.PrimaryTarget.Category != "dependency" || summary.PrimaryTarget.Action != "inspect_dependencies" {
		t.Fatalf("expected dependency inspection target, got %#v", summary.PrimaryTarget)
	}
	if summary.PrimaryTarget.Href != "" {
		t.Fatalf("dependency target should not invent a href without plan_id, got %#v", summary.PrimaryTarget)
	}
}
