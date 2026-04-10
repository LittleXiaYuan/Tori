package localbrain

import (
	"context"
	"testing"
)

func TestDefaultThresholds(t *testing.T) {
	th := DefaultThresholds()
	if th.ClassifyConfidence != 0.7 {
		t.Errorf("ClassifyConfidence = %v, want 0.7", th.ClassifyConfidence)
	}
	if th.MaxQueryLength != 500 {
		t.Errorf("MaxQueryLength = %v, want 500", th.MaxQueryLength)
	}
	if th.MaxToolSteps != 3 {
		t.Errorf("MaxToolSteps = %v, want 3", th.MaxToolSteps)
	}
}

func TestNewWithOptions(t *testing.T) {
	custom := Thresholds{ClassifyConfidence: 0.9, AnswerConfidence: 0.8, MaxQueryLength: 200, MaxToolSteps: 5}
	b := New(nil, nil, WithThresholds(custom))

	if b.thresholds.ClassifyConfidence != 0.9 {
		t.Errorf("ClassifyConfidence = %v, want 0.9", b.thresholds.ClassifyConfidence)
	}
	if b.thresholds.MaxQueryLength != 200 {
		t.Errorf("MaxQueryLength = %v, want 200", b.thresholds.MaxQueryLength)
	}
}

func TestClassifyLongQueryUpgrades(t *testing.T) {
	b := New(nil, nil)
	ctx := context.Background()

	longQuery := make([]byte, 600)
	for i := range longQuery {
		longQuery[i] = 'a'
	}

	decision, err := b.Classify(ctx, string(longQuery), "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Handler != "smart" {
		t.Errorf("handler = %s, want smart for long query", decision.Handler)
	}
	if decision.Intent.Complexity != "hard" {
		t.Errorf("complexity = %s, want hard", decision.Intent.Complexity)
	}
}

func TestClassifyNoClientUpgrades(t *testing.T) {
	b := New(nil, nil)
	ctx := context.Background()

	decision, err := b.Classify(ctx, "explain quantum computing", "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No local client → classifyWithLocal fails → upgrades to smart
	if decision.Handler != "smart" {
		t.Errorf("handler = %s, want smart (no local client)", decision.Handler)
	}
}

func TestSelectCloudTier(t *testing.T) {
	b := New(nil, nil)

	tests := []struct {
		intent Intent
		want   string
	}{
		{Intent{Complexity: "simple"}, "fast"},
		{Intent{Complexity: "medium"}, "smart"},
		{Intent{Complexity: "hard"}, "expert"},
	}
	for _, tt := range tests {
		got := b.selectCloudTier(&tt.intent)
		if got != tt.want {
			t.Errorf("selectCloudTier(%s) = %s, want %s", tt.intent.Complexity, got, tt.want)
		}
	}
}

func TestRecordFeedback(t *testing.T) {
	b := New(nil, nil)

	b.RecordFeedback("tenant1", "test query", Intent{Category: "chat", Complexity: "simple"}, "fast", false, true)

	pattern := b.getUserPattern("tenant1")
	if pattern == nil {
		t.Fatal("expected pattern for tenant1")
	}
	if len(pattern.QueryHistory) != 1 {
		t.Errorf("query history len = %d, want 1", len(pattern.QueryHistory))
	}
	if pattern.QueryHistory[0].Intent.Category != "chat" {
		t.Errorf("category = %s, want chat", pattern.QueryHistory[0].Intent.Category)
	}
}

func TestUserPatternsSlideWindow(t *testing.T) {
	b := New(nil, nil)

	// Add 250 records — should be capped at 200
	for i := 0; i < 250; i++ {
		b.RecordFeedback("tenant1", "query", Intent{Category: "chat"}, "fast", false, true)
	}

	pattern := b.getUserPattern("tenant1")
	if pattern == nil {
		t.Fatal("expected pattern")
	}
	if len(pattern.QueryHistory) > 200 {
		t.Errorf("query history = %d, want <= 200", len(pattern.QueryHistory))
	}
}

func TestBrainStats(t *testing.T) {
	b := New(nil, nil)
	ctx := context.Background()

	// Classify twice (both will upgrade since no local client)
	b.Classify(ctx, "hello", "t1")
	b.Classify(ctx, "hello again", "t1")

	stats := b.Stats()
	if stats.TotalClassify != 2 {
		t.Errorf("TotalClassify = %d, want 2", stats.TotalClassify)
	}
}

func TestExportTrainingData(t *testing.T) {
	b := New(nil, nil)
	b.RecordFeedback("t1", "hello", Intent{Category: "chat", Complexity: "simple"}, "fast", false, true)
	b.RecordFeedback("t1", "fix bug", Intent{Category: "code", Complexity: "hard"}, "expert", true, true)

	data := b.ExportTrainingData("t1")
	if len(data) != 2 {
		t.Errorf("training data len = %d, want 2", len(data))
	}

	// Only satisfied records should be exported
	b.RecordFeedback("t1", "failed", Intent{Category: "chat"}, "fast", false, false)
	data = b.ExportTrainingData("t1")
	if len(data) != 2 {
		t.Errorf("training data after unsatisfied = %d, want 2", len(data))
	}
}
