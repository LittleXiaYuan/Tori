package cogni

import (
	"testing"
)

func TestEvaluatePerception_Semantic(t *testing.T) {
	rules := []PerceptionRule{
		{Type: "semantic", Threshold: 0.8, Weight: 0.5},
	}
	signal := &PerceptionSignal{SemanticSimilarity: 0.9}

	score, reasons := evaluatePerception(rules, Session{}, signal)
	if score <= 0 {
		t.Errorf("score = %f, want > 0", score)
	}
	if len(reasons) == 0 {
		t.Error("expected reasons")
	}
}

func TestEvaluatePerception_SemanticBelowThreshold(t *testing.T) {
	rules := []PerceptionRule{
		{Type: "semantic", Threshold: 0.8},
	}
	signal := &PerceptionSignal{SemanticSimilarity: 0.5}

	score, _ := evaluatePerception(rules, Session{}, signal)
	if score != 0 {
		t.Errorf("score = %f, want 0", score)
	}
}

func TestEvaluatePerception_ContextChain(t *testing.T) {
	rules := []PerceptionRule{
		{Type: "context_chain", Window: 3, Topics: []string{"code", "git"}, Weight: 0.5},
	}
	signal := &PerceptionSignal{
		RecentTopics: []string{"code", "deploy", "git"},
	}

	score, reasons := evaluatePerception(rules, Session{}, signal)
	if score <= 0 {
		t.Errorf("score = %f, want > 0", score)
	}
	if len(reasons) == 0 {
		t.Error("expected context_chain reason")
	}
}

func TestEvaluatePerception_FileWatcher(t *testing.T) {
	rules := []PerceptionRule{
		{Type: "file_watcher", Patterns: []string{"*.go"}, Events: []string{"modified"}, Weight: 0.6},
	}
	signal := &PerceptionSignal{
		FileEvent: &FileChangeEvent{Path: "main.go", Event: "modified"},
	}

	score, reasons := evaluatePerception(rules, Session{}, signal)
	if score <= 0 {
		t.Errorf("score = %f, want > 0", score)
	}
	if len(reasons) == 0 {
		t.Error("expected file_watcher reason")
	}
}

func TestEvaluatePerception_FileWatcher_NoMatch(t *testing.T) {
	rules := []PerceptionRule{
		{Type: "file_watcher", Patterns: []string{"*.go"}, Events: []string{"modified"}},
	}
	signal := &PerceptionSignal{
		FileEvent: &FileChangeEvent{Path: "readme.md", Event: "modified"},
	}

	score, _ := evaluatePerception(rules, Session{}, signal)
	if score != 0 {
		t.Errorf("score = %f, want 0", score)
	}
}

func TestEvaluatePerception_Schedule(t *testing.T) {
	rules := []PerceptionRule{
		{Type: "schedule", Cron: "0 2 * * *", Weight: 0.5},
	}
	signal := &PerceptionSignal{
		ScheduleTriggered: true,
		ScheduleCron:      "0 2 * * *",
	}

	score, _ := evaluatePerception(rules, Session{}, signal)
	if score <= 0 {
		t.Errorf("schedule should trigger, score = %f", score)
	}
}

func TestEvaluatePerception_Webhook(t *testing.T) {
	rules := []PerceptionRule{
		{Type: "webhook", Path: "/hooks/pr-opened", Weight: 0.5},
	}
	signal := &PerceptionSignal{
		WebhookTriggered: true,
		WebhookPath:      "/hooks/pr-opened",
	}

	score, _ := evaluatePerception(rules, Session{}, signal)
	if score <= 0 {
		t.Errorf("webhook should trigger, score = %f", score)
	}
}

func TestEvaluatePerception_NilSignal(t *testing.T) {
	rules := []PerceptionRule{{Type: "semantic"}}
	score, _ := evaluatePerception(rules, Session{}, nil)
	if score != 0 {
		t.Error("nil signal should return 0")
	}
}

func TestSimpleGlobMatch(t *testing.T) {
	tests := []struct {
		path, pattern string
		want          bool
	}{
		{"main.go", "*.go", true},
		{"main.py", "*.go", false},
		{"src/app.ts", "*.ts", true},
		{"anything", "*", true},
		{"src/file.go", "src/*", true},
	}
	for _, tt := range tests {
		if got := simpleGlobMatch(tt.path, tt.pattern); got != tt.want {
			t.Errorf("simpleGlobMatch(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
		}
	}
}

func TestEvaluator_WithPerception(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "test",
		Activation: ActivationRules{
			MinScore: 0.3,
			Perception: []PerceptionRule{
				{Type: "semantic", Threshold: 0.7, Weight: 0.5},
			},
		},
	}

	session := Session{
		Message: "hello",
		Perception: &PerceptionSignal{
			SemanticSimilarity: 0.9,
		},
	}

	results := e.Evaluate([]*Declaration{d}, session)
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if !results[0].Activated {
		t.Errorf("should be activated with semantic signal, score=%f", results[0].Score)
	}
}
