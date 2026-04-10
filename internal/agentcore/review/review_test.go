package review

import (
	"context"
	"testing"
)

func TestClassifyRisk(t *testing.T) {
	tests := []struct {
		operation string
		want      Risk
	}{
		{"query_knowledge", RiskLow},
		{"translate_text", RiskLow},
		{"chat_response", RiskLow},
		{"write_file to /tmp", RiskMed},
		{"create_file new.txt", RiskMed},
		{"add_memory fact", RiskMed},
		{"use_skill calculator", RiskMed},
		{"shell rm -rf /", RiskHigh},
		{"exec command", RiskHigh},
		{"install_skill hacker_tool", RiskHigh},
		{"modify_persona evil", RiskHigh},
		{"delete_file important.db", RiskHigh},
	}

	for _, tt := range tests {
		got := ClassifyRisk(tt.operation)
		if got != tt.want {
			t.Errorf("ClassifyRisk(%q) = %s, want %s", tt.operation, got, tt.want)
		}
	}
}

func TestGateReviewLowRisk(t *testing.T) {
	g := NewGate()
	v := g.Review(context.Background(), "chat", "hello world")
	if !v.Allowed {
		t.Error("low risk should be allowed")
	}
	if v.Risk != RiskLow {
		t.Errorf("risk = %s, want low", v.Risk)
	}
}

func TestGateReviewMedRiskNoLLM(t *testing.T) {
	g := NewGate()
	v := g.Review(context.Background(), "write_file", "writing to /tmp/test.txt")
	// No LLM set → should pass through
	if !v.Allowed {
		t.Error("med risk without LLM should pass")
	}
}

func TestGateReviewMedRiskWithLLM(t *testing.T) {
	g := NewGate()
	g.SetLLMReview(func(ctx context.Context, operation string) (bool, error) {
		return false, nil // deny
	})
	v := g.Review(context.Background(), "write_file", "writing malicious content")
	if v.Allowed {
		t.Error("LLM denied, should not be allowed")
	}
}

func TestGateReviewHighRisk(t *testing.T) {
	g := NewGate()
	v := g.Review(context.Background(), "shell rm -rf /", "destructive command")
	if v.Allowed {
		t.Error("high risk should not be auto-approved")
	}
	if v.Risk != RiskHigh {
		t.Errorf("risk = %s, want high", v.Risk)
	}
}

// ── IntelligentGate tests ──

func TestIntelligentGateReviewLowRisk(t *testing.T) {
	ig := NewIntelligentGate()
	v := ig.ReviewDetailed(context.Background(), "chat", "hello", "tenant1")
	if !v.Allowed {
		t.Error("low risk should be allowed")
	}
	if v.ReviewedBy != "auto" {
		t.Errorf("ReviewedBy = %s, want auto", v.ReviewedBy)
	}
}

func TestIntelligentGateHighRiskBlocked(t *testing.T) {
	ig := NewIntelligentGate()
	v := ig.ReviewDetailed(context.Background(), "shell rm -rf /", "dangerous", "tenant1")
	if v.Allowed {
		t.Error("high risk without LLM should be blocked")
	}
	if v.ReviewedBy != "auto" {
		t.Errorf("ReviewedBy = %s, want auto (no llmDetail)", v.ReviewedBy)
	}
}

func TestIntelligentGateAuditLog(t *testing.T) {
	ig := NewIntelligentGate()

	ig.ReviewDetailed(context.Background(), "chat", "hello", "t1")
	ig.ReviewDetailed(context.Background(), "shell exec", "rm", "t1")
	ig.ReviewDetailed(context.Background(), "write_file", "data", "t1")

	log := ig.AuditLog(0)
	if len(log) != 3 {
		t.Errorf("audit log len = %d, want 3", len(log))
	}

	log2 := ig.AuditLog(2)
	if len(log2) != 2 {
		t.Errorf("audit log limited len = %d, want 2", len(log2))
	}
}

func TestIntelligentGateBlockedStats(t *testing.T) {
	ig := NewIntelligentGate()

	ig.ReviewDetailed(context.Background(), "shell exec", "cmd1", "t1")
	ig.ReviewDetailed(context.Background(), "shell exec", "cmd2", "t1")
	ig.ReviewDetailed(context.Background(), "delete_file", "secret.db", "t1")

	stats := ig.BlockedStats()
	if stats["shell exec"] != 2 {
		t.Errorf("shell exec blocked count = %d, want 2", stats["shell exec"])
	}
	if stats["delete_file"] != 1 {
		t.Errorf("delete_file blocked count = %d, want 1", stats["delete_file"])
	}
}

func TestIntelligentGateDeepReviewWithLLM(t *testing.T) {
	ig := NewIntelligentGate()
	ig.SetLLMDetail(func(ctx context.Context, system, prompt string) (string, error) {
		return `{"allowed":true,"reason":"safe operation","concerns":["none"],"mitigations":["verified"]}`, nil
	})

	v := ig.ReviewDetailed(context.Background(), "shell echo hello", "echo hello", "t1")
	if !v.Allowed {
		t.Error("LLM approved, should be allowed")
	}
	if v.ReviewedBy != "llm" {
		t.Errorf("ReviewedBy = %s, want llm", v.ReviewedBy)
	}
	if len(v.Concerns) == 0 {
		t.Error("expected parsed concerns")
	}
}

func TestRiskString(t *testing.T) {
	if RiskLow.String() != "low" {
		t.Errorf("RiskLow = %s, want low", RiskLow.String())
	}
	if RiskHigh.String() != "high" {
		t.Errorf("RiskHigh = %s, want high", RiskHigh.String())
	}
}
