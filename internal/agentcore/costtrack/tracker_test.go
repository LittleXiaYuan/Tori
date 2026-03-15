package costtrack

import (
	"testing"
	"time"
)

func TestRecordAndSummary(t *testing.T) {
	tr := New()
	cost, _ := tr.Record("gpt-4o-mini", "t1", "u1", "s1", 1000, 500, 200*time.Millisecond)
	if cost <= 0 {
		t.Fatalf("expected positive cost, got %f", cost)
	}

	s := tr.GetSummary()
	if s.TotalCalls != 1 {
		t.Fatalf("expected 1 call, got %d", s.TotalCalls)
	}
	if s.TotalCostUSD != cost {
		t.Fatalf("expected total cost %f, got %f", cost, s.TotalCostUSD)
	}
	if s.ByModel["gpt-4o-mini"] == nil {
		t.Fatal("expected model breakdown")
	}
	if s.ByUser["u1"] != cost {
		t.Fatalf("expected user cost %f, got %f", cost, s.ByUser["u1"])
	}
}

func TestMultiModelCost(t *testing.T) {
	tr := New()
	c1, _ := tr.Record("gpt-4o-mini", "t1", "u1", "s1", 1000, 500, 100*time.Millisecond)
	c2, _ := tr.Record("gpt-4o", "t1", "u1", "s2", 1000, 500, 300*time.Millisecond)

	// gpt-4o should be more expensive
	if c2 <= c1 {
		t.Fatalf("gpt-4o ($%f) should cost more than gpt-4o-mini ($%f)", c2, c1)
	}

	s := tr.GetSummary()
	if len(s.ByModel) != 2 {
		t.Fatalf("expected 2 models, got %d", len(s.ByModel))
	}
}

func TestBudgetDailyAlert(t *testing.T) {
	tr := New()
	tr.SetBudget(Budget{DailyLimitUSD: 0.001}) // very low limit

	// First call should trigger alert (80% threshold)
	_, alert := tr.Record("gpt-4o", "t1", "u1", "s1", 10000, 5000, 100*time.Millisecond)
	if alert == nil {
		t.Fatal("expected daily limit alert")
	}
	if alert.Type != "daily_limit" {
		t.Fatalf("expected daily_limit, got %s", alert.Type)
	}
}

func TestBudgetPerCallAlert(t *testing.T) {
	tr := New()
	tr.SetBudget(Budget{PerCallLimitUSD: 0.0001}) // very low

	_, alert := tr.Record("gpt-4o", "t1", "u1", "s1", 10000, 5000, 100*time.Millisecond)
	if alert == nil {
		t.Fatal("expected per-call alert")
	}
	if alert.Type != "per_call" {
		t.Fatalf("expected per_call, got %s", alert.Type)
	}
}

func TestWouldExceedBudget(t *testing.T) {
	tr := New()
	tr.SetBudget(Budget{DailyLimitUSD: 0.001})

	// Spend some
	tr.Record("gpt-4o", "t1", "u1", "s1", 10000, 5000, 100*time.Millisecond)

	// Check if next call would exceed
	exceed := tr.WouldExceedBudget("gpt-4o", 10000, 5000)
	if !exceed {
		t.Fatal("should exceed budget")
	}
}

func TestNoBudgetNoAlert(t *testing.T) {
	tr := New()
	// No budget set
	_, alert := tr.Record("gpt-4o", "t1", "u1", "s1", 100000, 50000, 100*time.Millisecond)
	if alert != nil {
		t.Fatalf("no alert expected without budget, got %s", alert.Type)
	}
}

func TestUnknownModelEstimate(t *testing.T) {
	tr := New()
	cost, _ := tr.Record("some-unknown-model", "t1", "u1", "s1", 1000, 500, 100*time.Millisecond)
	if cost <= 0 {
		t.Fatal("unknown model should still estimate cost")
	}
}

func TestTodayCost(t *testing.T) {
	tr := New()
	tr.Record("gpt-4o-mini", "t1", "u1", "s1", 1000, 500, 100*time.Millisecond)
	tr.Record("gpt-4o-mini", "t1", "u1", "s1", 2000, 1000, 100*time.Millisecond)

	today := tr.TodayCost()
	if today <= 0 {
		t.Fatal("today cost should be positive")
	}
	s := tr.GetSummary()
	if today != s.TotalCostUSD {
		t.Fatalf("today cost %f should equal total %f (all recorded today)", today, s.TotalCostUSD)
	}
}

func TestPrefixModelMatch(t *testing.T) {
	tr := New()
	// "gpt-4o-2024-08-06" should match "gpt-4o" pricing
	cost, _ := tr.Record("gpt-4o-2024-08-06", "t1", "u1", "s1", 1000, 500, 100*time.Millisecond)
	costDirect, _ := tr.Record("gpt-4o", "t1", "u1", "s1", 1000, 500, 100*time.Millisecond)
	if cost != costDirect {
		t.Fatalf("prefix match should use same pricing: %f vs %f", cost, costDirect)
	}
}

func TestRecordExtTaskTelemetry(t *testing.T) {
	tr := New()
	cost, _ := tr.RecordExt(RecordOpts{
		Model:      "gpt-4o-mini",
		TenantID:   "t1",
		TaskID:     "task-001",
		SkillName:  "web_search",
		Channel:    "telegram",
		RunnerType: "task",
		Tier:       "fast",
		TokensIn:   1000,
		TokensOut:  500,
		Latency:    100 * time.Millisecond,
	})
	if cost <= 0 {
		t.Fatal("expected positive cost")
	}

	// Check task cost
	tc := tr.GetTaskCost("task-001")
	if tc.TotalCost != cost {
		t.Fatalf("task cost %f != recorded cost %f", tc.TotalCost, cost)
	}
	if tc.Calls != 1 {
		t.Fatalf("expected 1 call, got %d", tc.Calls)
	}
	if tc.BySkill["web_search"] != cost {
		t.Fatalf("expected skill cost %f, got %f", cost, tc.BySkill["web_search"])
	}
}

func TestGetTaskCostMultiStep(t *testing.T) {
	tr := New()
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", TaskID: "task-002", SkillName: "code_exec",
		RunnerType: "task", TokensIn: 500, TokensOut: 200, Latency: 50 * time.Millisecond,
	})
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o", TaskID: "task-002", SkillName: "web_search",
		RunnerType: "task", TokensIn: 800, TokensOut: 400, Latency: 150 * time.Millisecond,
	})
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", TaskID: "task-other",
		RunnerType: "chat", TokensIn: 100, TokensOut: 50, Latency: 20 * time.Millisecond,
	})

	tc := tr.GetTaskCost("task-002")
	if tc.Calls != 2 {
		t.Fatalf("expected 2 calls for task-002, got %d", tc.Calls)
	}
	if len(tc.BySkill) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(tc.BySkill))
	}
	if len(tc.ByModel) != 2 {
		t.Fatalf("expected 2 models, got %d", len(tc.ByModel))
	}

	// Other task should have separate cost
	tcOther := tr.GetTaskCost("task-other")
	if tcOther.Calls != 1 {
		t.Fatalf("expected 1 call for task-other, got %d", tcOther.Calls)
	}
}

func TestGetCostByChannel(t *testing.T) {
	tr := New()
	tr.RecordExt(RecordOpts{Model: "gpt-4o-mini", Channel: "telegram", TokensIn: 1000, TokensOut: 500, Latency: 100 * time.Millisecond})
	tr.RecordExt(RecordOpts{Model: "gpt-4o-mini", Channel: "telegram", TokensIn: 500, TokensOut: 200, Latency: 80 * time.Millisecond})
	tr.RecordExt(RecordOpts{Model: "gpt-4o", Channel: "feishu", TokensIn: 1000, TokensOut: 500, Latency: 200 * time.Millisecond})

	channels := tr.GetCostByChannel()
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
	telegramFound := false
	for _, c := range channels {
		if c.Channel == "telegram" {
			telegramFound = true
			if c.Calls != 2 {
				t.Fatalf("expected 2 telegram calls, got %d", c.Calls)
			}
		}
	}
	if !telegramFound {
		t.Fatal("telegram channel not found")
	}
}

func TestGetCostByTier(t *testing.T) {
	tr := New()
	tr.RecordExt(RecordOpts{Model: "gpt-4o-mini", Tier: "fast", TokensIn: 500, TokensOut: 200, Latency: 50 * time.Millisecond})
	tr.RecordExt(RecordOpts{Model: "gpt-4o", Tier: "smart", TokensIn: 1000, TokensOut: 500, Latency: 200 * time.Millisecond})
	tr.RecordExt(RecordOpts{Model: "gpt-4o", Tier: "smart", TokensIn: 800, TokensOut: 400, Latency: 180 * time.Millisecond})

	tiers := tr.GetCostByTier()
	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(tiers))
	}
	for _, tier := range tiers {
		if tier.Tier == "smart" && tier.Calls != 2 {
			t.Fatalf("expected 2 smart calls, got %d", tier.Calls)
		}
	}
}

func TestGetCostByRunnerType(t *testing.T) {
	tr := New()
	tr.RecordExt(RecordOpts{Model: "gpt-4o-mini", RunnerType: "chat", TokensIn: 500, TokensOut: 200, Latency: 50 * time.Millisecond})
	tr.RecordExt(RecordOpts{Model: "gpt-4o", RunnerType: "task", TokensIn: 1000, TokensOut: 500, Latency: 200 * time.Millisecond})
	tr.RecordExt(RecordOpts{Model: "gpt-4o-mini", RunnerType: "cron", TokensIn: 300, TokensOut: 100, Latency: 80 * time.Millisecond})

	runners := tr.GetCostByRunnerType()
	if len(runners) != 3 {
		t.Fatalf("expected 3 runner types, got %d", len(runners))
	}
}

func TestSummaryNewDimensions(t *testing.T) {
	tr := New()
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", TaskID: "t1", Channel: "telegram",
		Tier: "fast", RunnerType: "task", TokensIn: 500, TokensOut: 200, Latency: 50 * time.Millisecond,
	})
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o", TaskID: "t1", Channel: "feishu",
		Tier: "smart", RunnerType: "chat", TokensIn: 1000, TokensOut: 500, Latency: 200 * time.Millisecond,
	})

	s := tr.GetSummary()
	if len(s.ByChannel) != 2 {
		t.Fatalf("expected 2 channels in summary, got %d", len(s.ByChannel))
	}
	if len(s.ByTier) != 2 {
		t.Fatalf("expected 2 tiers in summary, got %d", len(s.ByTier))
	}
	if len(s.ByRunnerType) != 2 {
		t.Fatalf("expected 2 runner types in summary, got %d", len(s.ByRunnerType))
	}
	if len(s.ByTask) != 1 {
		t.Fatalf("expected 1 task in summary, got %d", len(s.ByTask))
	}
}

func TestRecordExtBackwardsCompatible(t *testing.T) {
	tr := New()
	// Record with original method should still work
	cost1, _ := tr.Record("gpt-4o-mini", "t1", "u1", "s1", 1000, 500, 100*time.Millisecond)

	// RecordExt with same params (no new fields)
	cost2, _ := tr.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", TenantID: "t1", UserID: "u1", SessionID: "s1",
		TokensIn: 1000, TokensOut: 500, Latency: 100 * time.Millisecond,
	})

	if cost1 != cost2 {
		t.Fatalf("Record and RecordExt should give same cost: %f vs %f", cost1, cost2)
	}

	s := tr.GetSummary()
	if s.TotalCalls != 2 {
		t.Fatalf("expected 2 calls, got %d", s.TotalCalls)
	}
}

// ── New tests for Task Cost Telemetry (#33) ──

func TestGetTaskTimeline(t *testing.T) {
	tr := New()
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", TaskID: "t1", StepID: "step-1", SkillName: "search",
		RunnerType: "task", TokensIn: 500, TokensOut: 200, Latency: 50 * time.Millisecond,
	})
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o", TaskID: "t1", StepID: "step-2", SkillName: "code_exec",
		RunnerType: "task", TokensIn: 800, TokensOut: 400, Latency: 150 * time.Millisecond,
	})
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", TaskID: "t2", StepID: "step-1",
		RunnerType: "task", TokensIn: 100, TokensOut: 50, Latency: 20 * time.Millisecond,
	})

	timeline := tr.GetTaskTimeline("t1")
	if len(timeline) != 2 {
		t.Fatalf("expected 2 events for t1, got %d", len(timeline))
	}
	if timeline[0].StepID != "step-1" || timeline[1].StepID != "step-2" {
		t.Fatal("timeline should be ordered by timestamp")
	}

	// t2 should have separate timeline
	tl2 := tr.GetTaskTimeline("t2")
	if len(tl2) != 1 {
		t.Fatalf("expected 1 event for t2, got %d", len(tl2))
	}
}

func TestStepIDAndProviderID(t *testing.T) {
	tr := New()
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", TaskID: "t1", StepID: "step-3",
		SkillName: "translate", ProviderID: "provider-openai",
		RunnerType: "task", TokensIn: 500, TokensOut: 200, Latency: 50 * time.Millisecond,
	})

	timeline := tr.GetTaskTimeline("t1")
	if len(timeline) != 1 {
		t.Fatal("expected 1 event")
	}
	if timeline[0].StepID != "step-3" {
		t.Fatalf("expected step-3, got %s", timeline[0].StepID)
	}
	if timeline[0].ProviderID != "provider-openai" {
		t.Fatalf("expected provider-openai, got %s", timeline[0].ProviderID)
	}
}

func TestGetUsageHistory(t *testing.T) {
	tr := New()
	for i := 0; i < 10; i++ {
		model := "gpt-4o-mini"
		ch := "telegram"
		if i%2 == 0 {
			model = "gpt-4o"
			ch = "feishu"
		}
		tr.RecordExt(RecordOpts{
			Model: model, Channel: ch, RunnerType: "chat",
			TokensIn: 100 * (i + 1), TokensOut: 50 * (i + 1), Latency: 20 * time.Millisecond,
		})
	}

	// No filter, page 1, limit 5
	page := tr.GetUsageHistory(UsageFilter{Page: 1, Limit: 5})
	if page.Total != 10 {
		t.Fatalf("expected total 10, got %d", page.Total)
	}
	if len(page.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(page.Items))
	}
	if page.Page != 1 || page.Limit != 5 {
		t.Fatal("page metadata mismatch")
	}

	// Filter by channel
	page2 := tr.GetUsageHistory(UsageFilter{Channel: "telegram", Page: 1, Limit: 50})
	if page2.Total != 5 {
		t.Fatalf("expected 5 telegram, got %d", page2.Total)
	}

	// Filter by model
	page3 := tr.GetUsageHistory(UsageFilter{Model: "gpt-4o", Page: 1, Limit: 50})
	if page3.Total != 5 {
		t.Fatalf("expected 5 gpt-4o, got %d", page3.Total)
	}

	// Page 3 (out of range)
	page4 := tr.GetUsageHistory(UsageFilter{Page: 3, Limit: 5})
	if len(page4.Items) != 0 {
		t.Fatal("expected empty page 3")
	}
}

func TestMonthCost(t *testing.T) {
	tr := New()
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", TokensIn: 1000, TokensOut: 500, Latency: 50 * time.Millisecond,
	})
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o", TokensIn: 2000, TokensOut: 1000, Latency: 100 * time.Millisecond,
	})

	mc := tr.MonthCost()
	if mc <= 0 {
		t.Fatal("month cost should be positive")
	}
	// All records are from today, so month cost should equal today cost
	tc := tr.TodayCost()
	if mc != tc {
		t.Fatalf("month cost %f should equal today cost %f (all recorded now)", mc, tc)
	}
}

func TestGetCostByProvider(t *testing.T) {
	tr := New()
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", ProviderID: "openai-1",
		TokensIn: 500, TokensOut: 200, Latency: 50 * time.Millisecond,
	})
	tr.RecordExt(RecordOpts{
		Model: "gpt-4o", ProviderID: "openai-1",
		TokensIn: 1000, TokensOut: 500, Latency: 200 * time.Millisecond,
	})
	tr.RecordExt(RecordOpts{
		Model: "deepseek-chat", ProviderID: "deepseek-1",
		TokensIn: 2000, TokensOut: 800, Latency: 100 * time.Millisecond,
	})

	providers := tr.GetCostByProvider()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	for _, p := range providers {
		if p.ProviderID == "openai-1" && p.Calls != 2 {
			t.Fatalf("expected 2 openai-1 calls, got %d", p.Calls)
		}
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	// Create tracker and record data
	tr1 := NewWithPersistence(dir)
	tr1.RecordExt(RecordOpts{
		Model: "gpt-4o-mini", TaskID: "t1", StepID: "step-1",
		RunnerType: "task", TokensIn: 500, TokensOut: 200, Latency: 50 * time.Millisecond,
	})
	tr1.RecordExt(RecordOpts{
		Model: "gpt-4o", TaskID: "t1", StepID: "step-2", ProviderID: "p1",
		RunnerType: "task", TokensIn: 1000, TokensOut: 500, Latency: 150 * time.Millisecond,
	})

	// Writes are synchronous, so data is immediately available on disk

	// Create new tracker from same dir — should load existing records
	tr2 := NewWithPersistence(dir)
	if len(tr2.usages) != 2 {
		t.Fatalf("expected 2 loaded usages, got %d", len(tr2.usages))
	}
	tc := tr2.GetTaskCost("t1")
	if tc.Calls != 2 {
		t.Fatalf("expected 2 calls for t1 after reload, got %d", tc.Calls)
	}

	// Verify StepID and ProviderID survived serialization
	timeline := tr2.GetTaskTimeline("t1")
	if timeline[0].StepID != "step-1" {
		t.Fatalf("expected step-1 after reload, got %s", timeline[0].StepID)
	}
	if timeline[1].ProviderID != "p1" {
		t.Fatalf("expected provider p1 after reload, got %s", timeline[1].ProviderID)
	}
}

// ──────────────────────────────────────────────
// #37 Runtime Hardening: Diagnostic Power Verification
// ──────────────────────────────────────────────

// TestDiagnosticPower simulates a realistic production workload and verifies
// the telemetry system can answer these diagnostic questions:
// Q1: 哪个 skill 最烧 token?
// Q2: 哪个模型最贵?
// Q3: 哪个任务最长?
func TestDiagnosticPower(t *testing.T) {
	tr := New()

	// Simulate mixed workload across 3 tasks, 4 skills, 3 models
	workload := []RecordOpts{
		// Task A: document analysis — heavy on tokens
		{Model: "gpt-4o", TaskID: "task-a", StepID: "step-1", SkillName: "doc_parse", RunnerType: "task", TokensIn: 8000, TokensOut: 2000, Latency: 3 * time.Second},
		{Model: "gpt-4o", TaskID: "task-a", StepID: "step-2", SkillName: "summarize", RunnerType: "task", TokensIn: 4000, TokensOut: 1000, Latency: 2 * time.Second},
		{Model: "gpt-4o-mini", TaskID: "task-a", StepID: "step-3", SkillName: "translate", RunnerType: "task", TokensIn: 2000, TokensOut: 2000, Latency: 1 * time.Second},
		// Task B: code generation — uses expert model
		{Model: "claude-opus", TaskID: "task-b", StepID: "step-1", SkillName: "code_gen", RunnerType: "task", TokensIn: 3000, TokensOut: 5000, Latency: 8 * time.Second},
		{Model: "claude-opus", TaskID: "task-b", StepID: "step-2", SkillName: "code_gen", RunnerType: "task", TokensIn: 2000, TokensOut: 3000, Latency: 6 * time.Second},
		// Task C: simple Q&A
		{Model: "gpt-4o-mini", TaskID: "task-c", StepID: "step-1", SkillName: "web_search", RunnerType: "task", TokensIn: 500, TokensOut: 200, Latency: 500 * time.Millisecond},
		// Direct chat (no task)
		{Model: "gpt-4o-mini", RunnerType: "chat", Channel: "telegram", TokensIn: 300, TokensOut: 200, Latency: 400 * time.Millisecond},
		{Model: "gpt-4o-mini", RunnerType: "chat", Channel: "web", TokensIn: 500, TokensOut: 300, Latency: 600 * time.Millisecond},
	}

	for _, w := range workload {
		tr.RecordExt(w)
	}

	// Q1: 哪个 skill 最烧 token?
	t.Run("most_expensive_skill", func(t *testing.T) {
		summary := tr.GetSummary()
		maxSkill := ""
		maxCost := 0.0
		// Need per-skill aggregation — use task costs
		skillCosts := make(map[string]float64)
		for _, taskID := range []string{"task-a", "task-b", "task-c"} {
			tc := tr.GetTaskCost(taskID)
			for skill, cost := range tc.BySkill {
				skillCosts[skill] += cost
			}
		}
		for skill, cost := range skillCosts {
			if cost > maxCost {
				maxCost = cost
				maxSkill = skill
			}
		}

		t.Logf("Q1 结果: 最贵的 skill = %s ($%.4f)", maxSkill, maxCost)
		if maxSkill != "code_gen" {
			t.Errorf("expected code_gen (uses claude-opus), got %s", maxSkill)
		}
		// Verify total summary matches
		if summary.TotalCalls != int64(len(workload)) {
			t.Errorf("expected %d calls, got %d", len(workload), summary.TotalCalls)
		}
	})

	// Q2: 哪个模型最贵?
	t.Run("most_expensive_model", func(t *testing.T) {
		summary := tr.GetSummary()
		maxModel := ""
		maxCost := 0.0
		for model, mc := range summary.ByModel {
			if mc.CostUSD > maxCost {
				maxCost = mc.CostUSD
				maxModel = model
			}
		}
		t.Logf("Q2 结果: 最贵的模型 = %s ($%.4f, %d calls)", maxModel, maxCost, summary.ByModel[maxModel].Calls)
		if maxModel != "claude-opus" {
			t.Errorf("expected claude-opus, got %s", maxModel)
		}
	})

	// Q3: 哪个任务最长(延迟)?
	t.Run("longest_task", func(t *testing.T) {
		longestTask := ""
		maxLatency := int64(0)
		for _, taskID := range []string{"task-a", "task-b", "task-c"} {
			tc := tr.GetTaskCost(taskID)
			if tc.AvgLatency*tc.Calls > maxLatency {
				maxLatency = tc.AvgLatency * tc.Calls
				longestTask = taskID
			}
		}
		t.Logf("Q3 结果: 最长的任务 = %s (total latency ~%dms)", longestTask, maxLatency)
		if longestTask != "task-b" {
			t.Errorf("expected task-b (14s total), got %s", longestTask)
		}
	})

	// Q4: Provider 成本分布 (验证 by_provider 能力)
	t.Run("provider_cost", func(t *testing.T) {
		providers := tr.GetCostByProvider()
		// All entries have empty ProviderID, should show "unknown"
		if len(providers) == 0 {
			t.Error("expected at least 1 provider entry")
		}
		t.Logf("Q4 结果: %d provider entries", len(providers))
	})

	// Q5: Timeline 查询
	t.Run("task_timeline", func(t *testing.T) {
		timeline := tr.GetTaskTimeline("task-a")
		if len(timeline) != 3 {
			t.Fatalf("expected 3 entries for task-a, got %d", len(timeline))
		}
		// Should be ordered by time
		for i, u := range timeline {
			t.Logf("  step=%s skill=%s model=%s cost=$%.4f latency=%v",
				u.StepID, u.SkillName, u.Model, u.CostUSD, u.Latency)
			if u.TaskID != "task-a" {
				t.Errorf("entry %d: wrong taskID %s", i, u.TaskID)
			}
		}
	})

	// Q6: History 分页查询
	t.Run("usage_history_filtered", func(t *testing.T) {
		page := tr.GetUsageHistory(UsageFilter{
			Model: "claude-opus",
			Limit: 10,
			Page:  1,
		})
		if page.Total != 2 {
			t.Fatalf("expected 2 claude-opus entries, got %d", page.Total)
		}
		t.Logf("Q6 结果: claude-opus usage count=%d total_cost=$%.4f", page.Total, page.Items[0].CostUSD+page.Items[1].CostUSD)
	})
}
