package iterate

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TokenBudget != 5000 {
		t.Errorf("TokenBudget = %d, want 5000", cfg.TokenBudget)
	}
	if cfg.MaxRounds != 3 {
		t.Errorf("MaxRounds = %d, want 3", cfg.MaxRounds)
	}
	if cfg.Cooldown != 1*time.Hour {
		t.Errorf("Cooldown = %v, want 1h", cfg.Cooldown)
	}
}

func TestEngineNotEnabled(t *testing.T) {
	cfg := DefaultConfig()
	// Enabled is false by default
	e := NewEngine(cfg)
	_, err := e.RunCycle(context.Background())
	if err == nil {
		t.Error("expected error when not enabled")
	}
}

func TestEngineProposals(t *testing.T) {
	cfg := DefaultConfig()
	e := NewEngine(cfg)

	// No proposals initially
	if ps := e.Proposals(); len(ps) != 0 {
		t.Errorf("initial Proposals() = %d, want 0", len(ps))
	}
	if ps := e.AllProposals(); len(ps) != 0 {
		t.Errorf("initial AllProposals() = %d, want 0", len(ps))
	}
}

func TestEngineApproveReject(t *testing.T) {
	cfg := DefaultConfig()
	e := NewEngine(cfg)

	// Manually inject a proposal
	e.proposals = []Proposal{
		{ID: "p1", Status: StatusPending, Type: PropAddMemory, Title: "test"},
		{ID: "p2", Status: StatusPending, Type: PropFixBehavior, Title: "test2"},
	}

	if !e.ApproveProposal("p1") {
		t.Error("ApproveProposal should succeed for pending")
	}
	if e.ApproveProposal("p1") {
		t.Error("ApproveProposal on already-approved should fail")
	}

	if !e.RejectProposal("p2") {
		t.Error("RejectProposal should succeed for pending")
	}
	if e.RejectProposal("p2") {
		t.Error("RejectProposal on already-rejected should fail")
	}

	if e.ApproveProposal("nonexistent") {
		t.Error("ApproveProposal on nonexistent should fail")
	}

	// Filter pending
	pending := e.Proposals()
	if len(pending) != 0 {
		t.Errorf("Proposals() after approve/reject = %d, want 0", len(pending))
	}
	all := e.AllProposals()
	if len(all) != 2 {
		t.Errorf("AllProposals() after approve/reject = %d, want 2", len(all))
	}
}

func TestEngineEnabledMethod(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	e := NewEngine(cfg)
	if !e.Enabled() {
		t.Error("Enabled() should be true")
	}
}

func TestEngineCooldown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Cooldown = 1 * time.Hour
	e := NewEngine(cfg)

	// No LLM call, will fail but set lastRunAt
	e.SetLLMCall(func(ctx context.Context, system, user string) (string, int, error) {
		return "无需改进", 10, nil
	})
	e.RunCycle(context.Background())

	// Second run should hit cooldown
	_, err := e.RunCycle(context.Background())
	if err == nil {
		t.Error("expected cooldown error")
	}
}

func TestEngineRunCycleNoWeakness(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Cooldown = 0
	e := NewEngine(cfg)
	e.SetLLMCall(func(ctx context.Context, system, user string) (string, int, error) {
		return "无需改进", 10, nil
	})

	log, err := e.RunCycle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if log.StoppedBy != "complete" {
		t.Errorf("StoppedBy = %s, want complete", log.StoppedBy)
	}
}

func TestRiskLevelString(t *testing.T) {
	cases := []struct {
		r    RiskLevel
		want string
	}{
		{RiskLow, "low"},
		{RiskMed, "medium"},
		{RiskHigh, "high"},
		{RiskLevel(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.r.String(); got != tc.want {
			t.Errorf("RiskLevel(%d).String() = %q, want %q", tc.r, got, tc.want)
		}
	}
}

func TestDiscusserNew(t *testing.T) {
	d := NewDiscusser(nil)
	if d.maxRounds != 3 {
		t.Errorf("default maxRounds = %d, want 3", d.maxRounds)
	}

	d.SetMaxRounds(5)
	if d.maxRounds != 5 {
		t.Errorf("after SetMaxRounds(5) = %d", d.maxRounds)
	}

	// Invalid values ignored
	d.SetMaxRounds(0)
	if d.maxRounds != 5 {
		t.Error("SetMaxRounds(0) should be ignored")
	}
	d.SetMaxRounds(6)
	if d.maxRounds != 5 {
		t.Error("SetMaxRounds(6) should be ignored")
	}
}

func TestDiscusserNoLLM(t *testing.T) {
	d := NewDiscusser(nil)
	_, err := d.RunDiscussion(context.Background(), "topic", []string{"a"})
	if err == nil {
		t.Error("expected error without LLM")
	}
}

func TestDiscusserNoParticipants(t *testing.T) {
	d := NewDiscusser(func(ctx context.Context, s, u string) (string, int, error) {
		return "ok", 10, nil
	})
	_, err := d.RunDiscussion(context.Background(), "topic", nil)
	if err == nil {
		t.Error("expected error with no participants")
	}
}

func TestDiscusserRunDiscussion(t *testing.T) {
	callCount := 0
	d := NewDiscusser(func(ctx context.Context, system, user string) (string, int, error) {
		callCount++
		return "安全，同意", 50, nil
	})
	d.SetMaxRounds(1)

	conclusion, err := d.RunDiscussion(context.Background(), "测试提案", []string{"安全审查员", "用户体验师"})
	if err != nil {
		t.Fatal(err)
	}
	if !conclusion.Approved {
		t.Error("expected approved")
	}
	if len(conclusion.Messages) == 0 {
		t.Error("expected messages")
	}
	if conclusion.TokensUsed == 0 {
		t.Error("tokens should be > 0")
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("这个提案安全", "安全", "可行") {
		t.Error("should contain 安全")
	}
	if containsAny("随便说说", "安全", "可行") {
		t.Error("should not match")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abc", 5); got != "abc" {
		t.Errorf("truncate short = %q", got)
	}
	long := "这是一个很长的中文字符串测试"
	got := truncate(long, 5)
	if len([]rune(got)) > 8 { // 5 runes + "..."
		t.Errorf("truncate long result too long: %q", got)
	}
}
