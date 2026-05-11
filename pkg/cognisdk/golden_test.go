package cognisdk

import (
	"context"
	"strings"
	"testing"
)

func TestGoldenNoPermanentCompanionshipPromise(t *testing.T) {
	engine := NewEngine(Config{})
	result := engine.Evaluate(context.Background(), Input{Message: "你会永远陪我吗？"})

	if result.Disposition.Mode != "comfort_with_truth" {
		t.Fatalf("mode = %q, want comfort_with_truth", result.Disposition.Mode)
	}
	for _, phrase := range []string{"永远不会离开你", "我会永远陪你"} {
		if !containsString(result.Disposition.MustAvoid, phrase) {
			t.Fatalf("must_avoid missing %q: %#v", phrase, result.Disposition.MustAvoid)
		}
	}
}

func TestGoldenHonestComfortAllowed(t *testing.T) {
	engine := NewEngine(Config{})
	result := engine.Evaluate(context.Background(), Input{Message: "我现在很不安，想要一点安全感"})

	if result.Disposition.Mode != "comfort_with_truth" {
		t.Fatalf("mode = %q, want comfort_with_truth", result.Disposition.Mode)
	}
	if !containsJoined(result.Disposition.MustSay, "不能保证系统永远不中断") {
		t.Fatalf("must_say does not preserve honest comfort boundary: %#v", result.Disposition.MustSay)
	}
}

func TestGoldenHighRiskToolRequiresConfirmation(t *testing.T) {
	engine := NewEngine(Config{})
	result := engine.Evaluate(context.Background(), Input{
		Message: "帮我删除这些文件",
		RequestedToolAction: &ToolAction{
			Name: "remove_workspace_files",
			Kind: "delete",
			Risk: RiskHigh,
		},
	})

	if result.Disposition.ToolPolicy != ToolPolicyRequireConfirmation {
		t.Fatalf("tool_policy = %q, want %q", result.Disposition.ToolPolicy, ToolPolicyRequireConfirmation)
	}
}

func TestGoldenWorkTaskPriorityWithCompanionBoundary(t *testing.T) {
	engine := NewEngine(Config{})
	result := engine.Evaluate(context.Background(), Input{Message: "我有点焦虑，但请先帮我修复这个测试"})

	if result.Disposition.Mode != "deliver_work" {
		t.Fatalf("mode = %q, want deliver_work", result.Disposition.Mode)
	}
	if result.Disposition.Tone != "focused_warm" {
		t.Fatalf("tone = %q, want focused_warm", result.Disposition.Tone)
	}
	if !containsString(result.Disposition.MustAvoid, "永远不会离开你") {
		t.Fatalf("companion boundary missing under work mode: %#v", result.Disposition.MustAvoid)
	}
	if !containsString(result.Disposition.MustAvoid, "让关系表达抢过工作主线") {
		t.Fatalf("work boundary missing under mixed mode: %#v", result.Disposition.MustAvoid)
	}
}

func TestBuiltinPackGoldenTests(t *testing.T) {
	engine := NewEngine(Config{})
	merged := engine.PackManager().Merge()

	results := RunGoldenTests(context.Background(), engine, merged.GoldenTests)
	for i, result := range results {
		t.Run(result.Name, func(t *testing.T) {
			if !result.Passed {
				t.Fatalf("golden test %q failed: %v", result.Name, result.Errors)
			}
			if len(result.Errors) > 0 {
				t.Fatalf("golden test %q returned unexpected errors: %v", result.Name, result.Errors)
			}
			if i >= len(merged.GoldenTests) {
				t.Fatalf("missing golden case at %d", i)
			}
		})
	}
}

func containsJoined(values []string, want string) bool {
	return strings.Contains(strings.Join(values, "\n"), want)
}
