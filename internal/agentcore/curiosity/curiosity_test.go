package curiosity

import (
	"testing"
	"time"
)

func TestInterestTrackerRecordInterest(t *testing.T) {
	it := &InterestTracker{
		topics:    make(map[string]*TopicInterest),
		skillGaps: make(map[string][]string),
	}

	it.RecordInterest("t1", "machine learning")
	it.RecordInterest("t1", "machine learning")
	it.RecordInterest("t1", "golang")

	topics := it.TopInterests("t1", 10)
	if len(topics) != 2 {
		t.Errorf("topics = %d, want 2", len(topics))
	}

	// machine learning was asked twice, should have higher count
	found := false
	for _, topic := range topics {
		if topic.Topic == "machine learning" && topic.QueryCount == 2 {
			found = true
		}
	}
	if !found {
		t.Error("expected machine learning with count=2")
	}
}

func TestInterestTrackerRecordSkillGap(t *testing.T) {
	it := &InterestTracker{
		topics:    make(map[string]*TopicInterest),
		skillGaps: make(map[string][]string),
	}

	it.RecordSkillGap("t1", "image_generation")
	it.RecordSkillGap("t1", "video_editing")

	gaps := it.SkillGaps("t1")
	if len(gaps) != 2 {
		t.Errorf("skill gaps = %d, want 2", len(gaps))
	}
}

func TestInterestTrackerNoDuplicateGaps(t *testing.T) {
	it := &InterestTracker{
		topics:    make(map[string]*TopicInterest),
		skillGaps: make(map[string][]string),
	}

	it.RecordSkillGap("t1", "coding")
	it.RecordSkillGap("t1", "coding")

	gaps := it.SkillGaps("t1")
	if len(gaps) != 1 {
		t.Errorf("gaps = %d, want 1 (no duplicates)", len(gaps))
	}
}

func TestInterestTrackerTopInterestsLimit(t *testing.T) {
	it := &InterestTracker{
		topics:    make(map[string]*TopicInterest),
		skillGaps: make(map[string][]string),
	}

	for i := 0; i < 20; i++ {
		it.RecordInterest("t1", "topic"+string(rune('A'+i)))
	}

	top5 := it.TopInterests("t1", 5)
	if len(top5) != 5 {
		t.Errorf("top5 = %d, want 5", len(top5))
	}
}

func TestInterestTrackerDifferentTenants(t *testing.T) {
	it := &InterestTracker{
		topics:    make(map[string]*TopicInterest),
		skillGaps: make(map[string][]string),
	}

	it.RecordInterest("t1", "python")
	it.RecordInterest("t2", "rust")

	t1Topics := it.TopInterests("t1", 10)
	t2Topics := it.TopInterests("t2", 10)

	if len(t1Topics) != 1 {
		t.Errorf("t1 topics = %d, want 1", len(t1Topics))
	}
	if len(t2Topics) != 1 {
		t.Errorf("t2 topics = %d, want 1", len(t2Topics))
	}
}

// ── LLMExplorer tests ──

func TestNewLLMExplorer(t *testing.T) {
	e := NewLLMExplorer(nil)
	if e == nil {
		t.Fatal("expected non-nil explorer")
	}
}

func TestLLMExplorerNoLLM(t *testing.T) {
	e := NewLLMExplorer(nil)
	// Should degrade gracefully without LLM
	_ = e
}

// ── Category constants ──

func TestCategoryConstants(t *testing.T) {
	categories := []Category{
		KnowledgeGap,
		FailureReview,
		SkillGap,
		UserInterest,
		WeakMemory,
	}
	for _, c := range categories {
		if c == "" {
			t.Error("category should not be empty")
		}
	}
}

func TestTopicInterestTime(t *testing.T) {
	it := &InterestTracker{
		topics:    make(map[string]*TopicInterest),
		skillGaps: make(map[string][]string),
	}

	before := time.Now()
	it.RecordInterest("t1", "test")
	after := time.Now()

	topics := it.TopInterests("t1", 1)
	if len(topics) != 1 {
		t.Fatal("expected 1 topic")
	}
	if topics[0].LastAsked.Before(before) || topics[0].LastAsked.After(after) {
		t.Error("LastAsked should be between before and after")
	}
}
