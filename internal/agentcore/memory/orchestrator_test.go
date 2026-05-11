package memory

import (
	"context"
	"testing"
	"time"
)

func newTestOrchestrator() *Orchestrator {
	short := NewShortTerm(30 * time.Minute)
	mid := NewMidTerm()
	long := NewLongTerm()
	mgr := NewManager(short, mid, long)
	graph := NewGraph()
	em := NewEditableMemory()
	cfg := DefaultOrchestratorConfig()
	return NewOrchestrator(cfg, mgr, graph, em)
}

func TestOrchestratorIngestLow(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	err := o.Ingest(ctx, "t1", "hello", "chat", "user")
	if err != nil {
		t.Fatal(err)
	}
	// Low importance → short-term only
	if o.manager.Short.Count("t1") != 1 {
		t.Fatalf("expected 1 short item, got %d", o.manager.Short.Count("t1"))
	}
	if o.manager.Mid.Count("t1") != 0 {
		t.Fatal("should not be in mid")
	}
}

func TestOrchestratorIngestMedium(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	err := o.Ingest(ctx, "t1", "I prefer dark mode for all my editors", "preference", "user")
	if err != nil {
		t.Fatal(err)
	}
	// Medium → mid-term
	if o.manager.Mid.Count("t1") != 1 {
		t.Fatalf("expected 1 mid item, got %d", o.manager.Mid.Count("t1"))
	}
}

func TestOrchestratorIngestHigh(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	err := o.Ingest(ctx, "t1", "This is very important: remember my API key pattern", "fact", "user")
	if err != nil {
		t.Fatal(err)
	}
	// High → both mid and long
	if o.manager.Mid.Count("t1") != 1 {
		t.Fatalf("expected 1 mid, got %d", o.manager.Mid.Count("t1"))
	}
	if o.manager.Long.Count("t1") != 1 {
		t.Fatalf("expected 1 long, got %d", o.manager.Long.Count("t1"))
	}
}

func TestOrchestratorIngestCustomImportance(t *testing.T) {
	o := newTestOrchestrator()
	o.SetImportanceFunc(func(_ context.Context, _ string) Importance {
		return ImportanceHigh
	})
	ctx := context.Background()
	_ = o.Ingest(ctx, "t1", "trivial", "", "")
	if o.manager.Long.Count("t1") != 1 {
		t.Fatal("custom importance should route to long")
	}
}

func TestOrchestratorRecallShort(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	_ = o.manager.Short.Put(ctx, "t1", Item{Value: "the weather is sunny today", Key: "weather"})
	results := o.Recall(ctx, "t1", "weather", 5)
	if len(results) == 0 {
		t.Fatal("expected recall results")
	}
	if results[0].Source != "short" {
		t.Fatalf("expected source=short, got %s", results[0].Source)
	}
}

func TestOrchestratorRecallMid(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	_ = o.manager.AddMid(ctx, "t1", Item{Value: "user prefers dark theme", Category: "preference"})
	results := o.Recall(ctx, "t1", "theme", 5)
	if len(results) == 0 {
		t.Fatal("expected recall results from mid")
	}
	if results[0].Source != "mid" {
		t.Fatalf("expected source=mid, got %s", results[0].Source)
	}
}

func TestOrchestratorRecallGraph(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	o.graph.PutEntity(Entity{ID: "e1", Name: "Alice", Type: "person"})
	results := o.Recall(ctx, "t1", "Alice", 5)
	found := false
	for _, r := range results {
		if r.Source == "graph" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected graph results")
	}
}

func TestOrchestratorRecallEditable(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	o.editable.AddBlock("persona", "I am a helpful assistant", 0)
	results := o.Recall(ctx, "t1", "assistant", 5)
	found := false
	for _, r := range results {
		if r.Source == "editable" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected editable results")
	}
}

func TestOrchestratorRecallMultiLayer(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	_ = o.manager.Short.Put(ctx, "t1", Item{Value: "golang is great", Key: "k1"})
	_ = o.manager.AddMid(ctx, "t1", Item{Value: "golang best practices include testing"})
	o.graph.PutEntity(Entity{ID: "go1", Name: "golang", Type: "skill"})
	results := o.Recall(ctx, "t1", "golang", 10)
	sources := map[string]bool{}
	for _, r := range results {
		sources[r.Source] = true
	}
	if len(sources) < 2 {
		t.Fatalf("expected results from multiple layers, got %v", sources)
	}
}

func TestOrchestratorPromoteShortToMid(t *testing.T) {
	o := newTestOrchestrator()
	o.config.ShortToMidAccessCount = 2
	ctx := context.Background()
	item := Item{Value: "frequently accessed fact", Key: "faq", AccessCnt: 3}
	_ = o.manager.Short.Put(ctx, "t1", item)
	promoted := o.Promote(ctx, "t1")
	if promoted == 0 {
		t.Fatal("expected at least 1 promotion")
	}
	if o.manager.Mid.Count("t1") == 0 {
		t.Fatal("expected item in mid after promotion")
	}
}

func TestOrchestratorPromoteMidToLong(t *testing.T) {
	o := newTestOrchestrator()
	o.config.MidToLongAccessCount = 3
	ctx := context.Background()
	_ = o.manager.Mid.Put(ctx, "t1", Item{Value: "very important fact for long term", Key: "lt1"})
	// Bump access count by calling Get multiple times (Put sets AccessCnt=1, each Get increments)
	for i := 0; i < 3; i++ {
		o.manager.Mid.Get(ctx, "t1", "lt1")
	}
	promoted := o.Promote(ctx, "t1")
	if promoted == 0 {
		t.Fatal("expected promotion")
	}
	if o.manager.Long.Count("t1") == 0 {
		t.Fatal("expected item in long after promotion")
	}
}

func TestOrchestratorNoPromotionBelowThreshold(t *testing.T) {
	o := newTestOrchestrator()
	o.config.ShortToMidAccessCount = 10
	ctx := context.Background()
	_ = o.manager.Short.Put(ctx, "t1", Item{Value: "casual mention", Key: "c1", AccessCnt: 1})
	promoted := o.Promote(ctx, "t1")
	if promoted != 0 {
		t.Fatalf("expected 0 promotions, got %d", promoted)
	}
}

func TestOrchestratorDecayFactor(t *testing.T) {
	o := newTestOrchestrator()
	o.config.DecayHalfLife = 24 * time.Hour

	// Fresh item: no decay
	f := o.decayFactor(0)
	if f < 0.99 {
		t.Fatalf("fresh item decay should be ~1.0, got %f", f)
	}

	// After 1 stability period: FSRS uses R(t)=e^(-t/S), so e^(-1) ≈ 0.368
	f = o.decayFactor(24 * time.Hour)
	if f < 0.35 || f > 0.39 {
		t.Fatalf("1 stability-period decay (FSRS) should be ~0.368, got %f", f)
	}

	// After 2 stability periods: FSRS e^(-2) ≈ 0.135
	f = o.decayFactor(48 * time.Hour)
	if f < 0.12 || f > 0.15 {
		t.Fatalf("2 stability-periods decay (FSRS) should be ~0.135, got %f", f)
	}
}

func TestOrchestratorDecayDisabled(t *testing.T) {
	o := newTestOrchestrator()
	o.config.DecayHalfLife = 0
	f := o.decayFactor(999 * time.Hour)
	if f != 1.0 {
		t.Fatalf("disabled decay should return 1.0, got %f", f)
	}
}

func TestCategoryDifficulty(t *testing.T) {
	tests := []struct {
		category string
		want     float64
	}{
		{"identity", 1.0},
		{"name", 1.0},
		{"persona", 1.0},
		{"preference", 3.0},
		{"knowledge", 5.0},
		{"fact", 5.0},
		{"chat", 8.0},
		{"", 5.0},
		{"unknown_category", 5.0},
	}
	for _, tt := range tests {
		got := categoryDifficulty(tt.category)
		if got != tt.want {
			t.Errorf("categoryDifficulty(%q) = %f, want %f", tt.category, got, tt.want)
		}
	}
}

func TestCategoryAdaptiveDecay_IdentityDecaysSlower(t *testing.T) {
	o := newTestOrchestrator()
	o.config.DecayHalfLife = 24 * time.Hour
	age := 7 * 24 * time.Hour // 1 week

	identityDecay := o.decayFactorForCategory(age, 3, "identity")
	chatDecay := o.decayFactorForCategory(age, 3, "chat")
	knowledgeDecay := o.decayFactorForCategory(age, 3, "knowledge")

	if identityDecay <= knowledgeDecay {
		t.Errorf("identity (D=1.0) should decay slower than knowledge (D=5.0): identity=%f, knowledge=%f",
			identityDecay, knowledgeDecay)
	}
	if knowledgeDecay <= chatDecay {
		t.Errorf("knowledge (D=5.0) should decay slower than chat (D=8.0): knowledge=%f, chat=%f",
			knowledgeDecay, chatDecay)
	}
	if identityDecay <= chatDecay {
		t.Errorf("identity should decay much slower than chat: identity=%f, chat=%f",
			identityDecay, chatDecay)
	}
}

func TestCategoryAdaptiveDecay_EmptyCategoryUsesDefault(t *testing.T) {
	o := newTestOrchestrator()
	o.config.DecayHalfLife = 24 * time.Hour
	age := 48 * time.Hour

	defaultDecay := o.decayFactorForCategory(age, 2, "")
	knowledgeDecay := o.decayFactorForCategory(age, 2, "knowledge")

	// Empty category should match knowledge (both D=5.0)
	diff := defaultDecay - knowledgeDecay
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.001 {
		t.Errorf("empty category should equal knowledge decay: default=%f, knowledge=%f", defaultDecay, knowledgeDecay)
	}
}

func TestOrchestratorLinkEntity(t *testing.T) {
	o := newTestOrchestrator()
	o.graph.PutEntity(Entity{ID: "e1", Name: "Project X", Type: "project"})
	o.LinkEntityToMemory("e1", "mem_key_1", "related_to")
	rels := o.graph.GetRelations("e1")
	if len(rels) == 0 {
		t.Fatal("expected relation after linking")
	}
}

func TestOrchestratorRecallForEntity(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	o.graph.PutEntity(Entity{ID: "e1", Name: "Bob", Type: "person"})
	_ = o.manager.AddMid(ctx, "t1", Item{Value: "Bob likes coffee"})
	results := o.RecallForEntity(ctx, "t1", "Bob", 5)
	if len(results) == 0 {
		t.Fatal("expected results for entity")
	}
}

func TestOrchestratorRecallForEntityNotFound(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	_ = o.manager.Short.Put(ctx, "t1", Item{Value: "something about Charlie", Key: "ch"})
	results := o.RecallForEntity(ctx, "t1", "Charlie", 5)
	// Falls back to query-based recall
	if len(results) == 0 {
		t.Fatal("expected fallback recall")
	}
}

func TestOrchestratorCompileContext(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	o.editable.AddBlock("bio", "I am Tori", 0)
	_ = o.manager.Short.Put(ctx, "t1", Item{Value: "user asked about golang", Key: "q1"})
	compiled := o.CompileContext(ctx, "t1", "golang")
	if compiled == "" {
		t.Fatal("expected non-empty context")
	}
}

func TestOrchestratorStats(t *testing.T) {
	o := newTestOrchestrator()
	ctx := context.Background()
	_ = o.manager.Short.Put(ctx, "t1", Item{Value: "a", Key: "k1"})
	_ = o.manager.AddMid(ctx, "t1", Item{Value: "b"})
	o.graph.PutEntity(Entity{ID: "e1", Name: "X", Type: "t"})
	o.editable.AddBlock("l", "c", 0)

	s := o.Stats("t1")
	if s.ShortCount != 1 || s.MidCount != 1 || s.GraphEntities != 1 || s.EditableBlocks != 1 {
		t.Fatalf("unexpected stats: %+v", s)
	}
}

func TestOrchestratorPromotionLog(t *testing.T) {
	o := newTestOrchestrator()
	o.config.ShortToMidAccessCount = 1
	ctx := context.Background()
	_ = o.manager.Short.Put(ctx, "t1", Item{Value: "log test item", Key: "lt1", AccessCnt: 2})
	o.Promote(ctx, "t1")
	log := o.PromotionLog(10)
	if len(log) == 0 {
		t.Fatal("expected promotion log entries")
	}
	if log[0].From != "short" || log[0].To != "mid" {
		t.Fatalf("unexpected log entry: %+v", log[0])
	}
}

func TestHeuristicImportance(t *testing.T) {
	tests := []struct {
		input string
		want  Importance
	}{
		{"hi", ImportanceLow},
		{"I prefer vim over emacs", ImportanceMedium},
		{"This is very important: remember this always", ImportanceHigh},
		{"这是非常重要的信息，请记住", ImportanceHigh},
		{"我喜欢用深色主题", ImportanceMedium},
	}
	for _, tt := range tests {
		got := heuristicImportance(tt.input)
		if got != tt.want {
			t.Errorf("heuristicImportance(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestOrchestratorDefaultConfig(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	if cfg.MaxRecallResults != 20 {
		t.Fatalf("expected 20 max recall, got %d", cfg.MaxRecallResults)
	}
	if cfg.DecayHalfLife != 7*24*time.Hour {
		t.Fatalf("unexpected decay half-life: %v", cfg.DecayHalfLife)
	}
}

func TestOrchestratorNilGraph(t *testing.T) {
	short := NewShortTerm(30 * time.Minute)
	mid := NewMidTerm()
	long := NewLongTerm()
	mgr := NewManager(short, mid, long)
	cfg := DefaultOrchestratorConfig()
	o := NewOrchestrator(cfg, mgr, nil, nil)
	ctx := context.Background()
	_ = o.manager.Short.Put(ctx, "t1", Item{Value: "test", Key: "k1"})
	results := o.Recall(ctx, "t1", "test", 5)
	if len(results) == 0 {
		t.Fatal("recall should work without graph")
	}
}

func TestOrchestratorLayerWeight(t *testing.T) {
	o := newTestOrchestrator()
	if o.layerWeight("short") != 0.5 {
		t.Fatal("wrong short weight")
	}
	if o.layerWeight("long") != 1.0 {
		t.Fatal("wrong long weight")
	}
	if o.layerWeight("unknown") != 0.5 {
		t.Fatal("unknown should default to 0.5")
	}
}
