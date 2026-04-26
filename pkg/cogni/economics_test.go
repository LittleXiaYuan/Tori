package cogni

import (
	"testing"
	"time"
)

func TestCostTrackerBudget(t *testing.T) {
	ct := NewCostTracker()
	ct.SetConfig("c1", EconomicsConfig{
		BudgetPerRun: 0.05,
		DailyBudget:  1.0,
	})

	// Within budget
	if err := ct.CheckBudget("c1", 0.03); err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// Exceeds per-run
	if err := ct.CheckBudget("c1", 0.10); err == nil {
		t.Error("expected error for exceeding per-run budget")
	}

	// Record some cost
	ct.Record(CostEntry{CogniID: "c1", Cost: 0.90, Tokens: 5000, Operation: "chat"})

	// Would exceed daily budget
	if err := ct.CheckBudget("c1", 0.03); err != nil {
		t.Errorf("expected nil (still under), got %v", err)
	}
	if err := ct.CheckBudget("c1", 0.05); err != nil {
		t.Errorf("expected nil (at threshold), got %v", err)
	}

	ct.Record(CostEntry{CogniID: "c1", Cost: 0.09, Tokens: 500, Operation: "chat"})
	if err := ct.CheckBudget("c1", 0.05); err == nil {
		t.Error("expected error for exceeding daily budget")
	}
}

func TestCostTrackerNoConfig(t *testing.T) {
	ct := NewCostTracker()
	if err := ct.CheckBudget("unknown", 999); err != nil {
		t.Errorf("no config should pass any budget check, got %v", err)
	}
}

func TestDailySummary(t *testing.T) {
	ct := NewCostTracker()
	ct.SetConfig("c1", EconomicsConfig{DailyBudget: 2.0})

	ct.Record(CostEntry{CogniID: "c1", Cost: 0.5, Tokens: 1000, Operation: "chat"})
	ct.Record(CostEntry{CogniID: "c1", Cost: 0.3, Tokens: 800, Operation: "workflow"})

	summary := ct.DailySummary()
	s := summary["c1"]
	if s.Operations != 2 {
		t.Errorf("expected 2 operations, got %d", s.Operations)
	}
	if s.TotalTokens != 1800 {
		t.Errorf("expected 1800 tokens, got %d", s.TotalTokens)
	}
}

func TestPruneOld(t *testing.T) {
	ct := NewCostTracker()
	ct.Record(CostEntry{
		CogniID:   "c1",
		Cost:      0.1,
		Timestamp: time.Now().Add(-48 * time.Hour),
	})
	ct.Record(CostEntry{CogniID: "c1", Cost: 0.2})

	removed := ct.PruneOld(24 * time.Hour)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
}
