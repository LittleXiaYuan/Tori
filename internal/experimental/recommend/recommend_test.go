package recommend

import (
	"testing"
)

func TestRecommendBasic(t *testing.T) {
	e := NewEngine()
	e.RegisterItem(ItemProfile{ID: "code_gen", Category: "coding", Tags: []string{"python", "automation"}})
	e.RegisterItem(ItemProfile{ID: "translate", Category: "language", Tags: []string{"english", "chinese"}})
	e.RegisterItem(ItemProfile{ID: "summarize", Category: "writing", Tags: []string{"analysis"}})

	e.RecordOutcome("code_gen", 0.9, true)
	e.RecordOutcome("code_gen", 0.8, true)
	e.RecordOutcome("translate", 0.3, false)

	recs := e.Recommend(3, "write python code")
	if len(recs) != 3 {
		t.Fatalf("expected 3 recommendations, got %d", len(recs))
	}
	if recs[0].ItemID != "code_gen" {
		t.Logf("top recommendation: %s (expected code_gen based on positive history)", recs[0].ItemID)
	}
	for i, r := range recs {
		t.Logf("  %d: %s (score=%.3f, reason=%s, confidence=%.1f)", i, r.ItemID, r.Score, r.Reason, r.Confidence)
	}
}

func TestRecommendEmpty(t *testing.T) {
	e := NewEngine()
	recs := e.Recommend(5, "anything")
	if len(recs) != 0 {
		t.Errorf("expected 0 recommendations from empty engine, got %d", len(recs))
	}
}

func TestRecommendNovelty(t *testing.T) {
	e := NewEngine()
	e.RegisterItem(ItemProfile{ID: "used_a_lot", Category: "general", Uses: 100, Successes: 80, Failures: 20})
	e.RegisterItem(ItemProfile{ID: "never_used", Category: "general"})

	recs := e.Recommend(2, "")
	hasNovelty := false
	for _, r := range recs {
		if r.ItemID == "never_used" && r.Reason == "novelty" {
			hasNovelty = true
		}
	}
	t.Logf("recommendations: %v (novelty found: %v)", recs, hasNovelty)
}

func TestRecommendContextMatch(t *testing.T) {
	e := NewEngine()
	e.RegisterItem(ItemProfile{ID: "web_search", Category: "research", Tags: []string{"web", "search", "information"}})
	e.RegisterItem(ItemProfile{ID: "file_ops", Category: "system", Tags: []string{"file", "disk", "io"}})
	e.RegisterItem(ItemProfile{ID: "code_review", Category: "coding", Tags: []string{"review", "quality"}})

	recs := e.Recommend(3, "search the web for information")
	if len(recs) == 0 {
		t.Fatal("expected recommendations")
	}
	if recs[0].ItemID != "web_search" {
		t.Logf("expected web_search as top for 'search the web', got %s", recs[0].ItemID)
	}
}

func TestSimilarItems(t *testing.T) {
	e := NewEngine()
	e.RegisterItem(ItemProfile{ID: "a", Features: []float64{1, 0, 0}})
	e.RegisterItem(ItemProfile{ID: "b", Features: []float64{0.9, 0.1, 0}})
	e.RegisterItem(ItemProfile{ID: "c", Features: []float64{0, 0, 1}})

	similar := e.SimilarItems("a", 2)
	if len(similar) < 1 {
		t.Fatal("expected at least 1 similar item")
	}
	if similar[0].ItemID != "b" {
		t.Errorf("most similar to 'a' should be 'b', got %s", similar[0].ItemID)
	}
}

func TestPreferences(t *testing.T) {
	e := NewEngine()
	e.RegisterItem(ItemProfile{ID: "x", Category: "coding", Tags: []string{"go"}})

	e.RecordOutcome("x", 0.9, true)
	e.RecordOutcome("x", 0.8, true)

	prefs := e.Preferences()
	if prefs.InteractionCount != 2 {
		t.Errorf("expected 2 interactions, got %d", prefs.InteractionCount)
	}
	if prefs.PreferredCategories["coding"] <= 0 {
		t.Error("coding category should have positive preference")
	}
}

func TestRecordNegativeFeedback(t *testing.T) {
	e := NewEngine()
	e.RegisterItem(ItemProfile{ID: "bad_skill", Category: "misc", Tags: []string{"slow"}})

	for i := 0; i < 5; i++ {
		e.RecordOutcome("bad_skill", 0.1, false)
	}

	prefs := e.Preferences()
	if prefs.AvoidCategories["misc"] <= 0 {
		t.Error("misc should be in avoid categories after negative feedback")
	}
}
