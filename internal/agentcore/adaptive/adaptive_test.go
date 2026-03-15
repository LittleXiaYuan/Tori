package adaptive

import (
	"context"
	"testing"
	"time"
)

func TestRecordFeedback(t *testing.T) {
	l := NewLoop()
	l.RecordFeedback(Feedback{
		Type:        FeedbackCorrection,
		UserMessage: "不要用这么长的回复",
		Correction:  "concise",
		Dimension:   DimResponseLength,
	})
	if len(l.Feedbacks(10)) != 1 {
		t.Fatal("expected 1 feedback")
	}
}

func TestFeedbackAutoID(t *testing.T) {
	l := NewLoop()
	l.RecordFeedback(Feedback{Type: FeedbackPositive})
	fbs := l.Feedbacks(1)
	if fbs[0].ID == "" {
		t.Fatal("should auto-generate ID")
	}
	if fbs[0].CreatedAt.IsZero() {
		t.Fatal("should auto-set timestamp")
	}
}

func TestFeedbackLimit(t *testing.T) {
	l := NewLoop()
	l.maxFeedbacks = 5
	for i := 0; i < 10; i++ {
		l.RecordFeedback(Feedback{Type: FeedbackPositive})
	}
	if len(l.Feedbacks(0)) != 5 {
		t.Fatalf("expected 5 feedbacks, got %d", len(l.Feedbacks(0)))
	}
}

func TestCorrectionTracking(t *testing.T) {
	l := NewLoop()
	for i := 0; i < 3; i++ {
		l.RecordFeedback(Feedback{
			Type:        FeedbackCorrection,
			UserMessage: "太长了",
			Correction:  "简短回复",
			Dimension:   DimResponseLength,
		})
	}
	patterns := l.CorrectionPatterns()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0].Occurrences != 3 {
		t.Fatalf("expected 3 occurrences, got %d", patterns[0].Occurrences)
	}
}

func TestCorrectionExamplesLimit(t *testing.T) {
	l := NewLoop()
	for i := 0; i < 10; i++ {
		l.RecordFeedback(Feedback{
			Type:        FeedbackCorrection,
			UserMessage: "example",
			Correction:  "fix",
			Dimension:   DimFormality,
		})
	}
	patterns := l.CorrectionPatterns()
	if len(patterns[0].Examples) > 5 {
		t.Fatal("examples should be capped at 5")
	}
}

func TestHeuristicAdapt(t *testing.T) {
	l := NewLoop()
	l.SetAdaptThreshold(2)

	// Record corrections that should trigger adaptation
	for i := 0; i < 3; i++ {
		l.RecordFeedback(Feedback{
			Type:       FeedbackCorrection,
			Dimension:  DimResponseLength,
			Correction: "concise",
		})
	}

	ctx := context.Background()
	adapted, err := l.Adapt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if adapted == 0 {
		t.Fatal("expected adaptation")
	}

	val, ok := l.GetSetting(DimResponseLength)
	if !ok || val != "concise" {
		t.Fatalf("expected concise, got %q", val)
	}
}

func TestAdaptBelowThreshold(t *testing.T) {
	l := NewLoop()
	l.SetAdaptThreshold(10)
	l.RecordFeedback(Feedback{Type: FeedbackCorrection, Dimension: DimFormality, Correction: "formal"})

	ctx := context.Background()
	adapted, _ := l.Adapt(ctx)
	if adapted != 0 {
		t.Fatal("should not adapt below threshold")
	}
}

func TestAdaptNoSignal(t *testing.T) {
	l := NewLoop()
	l.SetAdaptThreshold(1)
	// Only positive feedback — no correction signal
	for i := 0; i < 5; i++ {
		l.RecordFeedback(Feedback{Type: FeedbackPositive, Dimension: DimResponseLength})
	}
	ctx := context.Background()
	adapted, _ := l.Adapt(ctx)
	if adapted != 0 {
		t.Fatal("should not adapt with only positive feedback")
	}
}

func TestAdaptWithCustomFunc(t *testing.T) {
	l := NewLoop()
	l.SetAdaptThreshold(1)
	l.SetAdaptFunc(func(_ context.Context, fbs []Feedback) ([]AdaptationRule, error) {
		return []AdaptationRule{
			{Dimension: DimLanguage, TargetVal: "zh", Confidence: 0.9, FeedbackCnt: len(fbs)},
		}, nil
	})
	l.RecordFeedback(Feedback{Type: FeedbackPreference, Dimension: DimLanguage})

	ctx := context.Background()
	adapted, err := l.Adapt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if adapted != 1 {
		t.Fatalf("expected 1 adaptation, got %d", adapted)
	}

	val, _ := l.GetSetting(DimLanguage)
	if val != "zh" {
		t.Fatalf("expected zh, got %s", val)
	}
}

func TestObserveInteraction(t *testing.T) {
	l := NewLoop()
	l.SetExtractFunc(func(_ context.Context, userMsg, agentAction string) (*Feedback, error) {
		return &Feedback{
			Type:        FeedbackCorrection,
			UserMessage: userMsg,
			AgentAction: agentAction,
			Correction:  "shorter",
			Dimension:   DimResponseLength,
		}, nil
	})

	ctx := context.Background()
	fb, err := l.ObserveInteraction(ctx, "太啰嗦了", "这是一段很长的回复...")
	if err != nil {
		t.Fatal(err)
	}
	if fb == nil {
		t.Fatal("expected feedback")
	}
	if fb.Dimension != DimResponseLength {
		t.Fatalf("expected response_length, got %s", fb.Dimension)
	}
}

func TestObserveNoExtractFunc(t *testing.T) {
	l := NewLoop()
	ctx := context.Background()
	fb, err := l.ObserveInteraction(ctx, "hello", "hi")
	if err != nil || fb != nil {
		t.Fatal("should return nil without extract func")
	}
}

func TestProfileCompile(t *testing.T) {
	l := NewLoop()
	l.SetSetting(DimResponseLength, "concise")
	l.SetSetting(DimFormality, "casual")

	p := l.Profile()
	compiled := p.Compile()
	if compiled == "" {
		t.Fatal("expected non-empty compiled profile")
	}
	if !containsStr(compiled, "concise") || !containsStr(compiled, "casual") {
		t.Fatalf("missing settings in compiled: %s", compiled)
	}
}

func TestProfileVersion(t *testing.T) {
	l := NewLoop()
	l.SetSetting("a", "1")
	l.SetSetting("b", "2")
	p := l.Profile()
	if p.Version != 2 {
		t.Fatalf("expected version 2, got %d", p.Version)
	}
}

func TestProfileEmptyCompile(t *testing.T) {
	bp := BehaviorProfile{Settings: make(map[string]string)}
	if bp.Compile() != "" {
		t.Fatal("empty profile should compile to empty string")
	}
}

func TestRules(t *testing.T) {
	l := NewLoop()
	l.SetAdaptThreshold(1)
	l.RecordFeedback(Feedback{Type: FeedbackCorrection, Dimension: DimEmoji, Correction: "no_emoji"})
	l.RecordFeedback(Feedback{Type: FeedbackCorrection, Dimension: DimEmoji, Correction: "no_emoji"})

	ctx := context.Background()
	l.Adapt(ctx)
	rules := l.Rules()
	if len(rules) == 0 {
		t.Fatal("expected rules after adaptation")
	}
	if !rules[0].Active {
		t.Fatal("rule should be active")
	}
}

func TestStats(t *testing.T) {
	l := NewLoop()
	l.RecordFeedback(Feedback{Type: FeedbackPositive})
	l.RecordFeedback(Feedback{Type: FeedbackCorrection, Dimension: "x", Correction: "y"})
	l.RecordFeedback(Feedback{Type: FeedbackNegative})

	s := l.Stats()
	if s.TotalFeedbacks != 3 {
		t.Fatalf("expected 3, got %d", s.TotalFeedbacks)
	}
	if s.FeedbackByType["positive"] != 1 {
		t.Fatal("wrong positive count")
	}
	if s.CorrectionCount != 1 {
		t.Fatalf("expected 1 correction, got %d", s.CorrectionCount)
	}
}

func TestReset(t *testing.T) {
	l := NewLoop()
	l.RecordFeedback(Feedback{Type: FeedbackPositive})
	l.SetSetting("a", "b")
	l.Reset()
	if len(l.Feedbacks(0)) != 0 {
		t.Fatal("feedbacks should be cleared")
	}
	if l.Profile().Version != 0 {
		t.Fatal("version should be 0")
	}
}

func TestFeedbackTypes(t *testing.T) {
	types := []FeedbackType{
		FeedbackExplicit, FeedbackCorrection, FeedbackPreference,
		FeedbackPositive, FeedbackNegative, FeedbackIgnore,
	}
	for _, ft := range types {
		if ft == "" {
			t.Fatal("feedback type should not be empty")
		}
	}
}

func TestDimensionConstants(t *testing.T) {
	dims := []string{
		DimResponseLength, DimFormality, DimProactivity,
		DimCodeStyle, DimExplanationDepth, DimLanguage,
		DimEmoji, DimTechnicalLevel,
	}
	seen := map[string]bool{}
	for _, d := range dims {
		if seen[d] {
			t.Fatalf("duplicate dimension: %s", d)
		}
		seen[d] = true
	}
}

func TestAdaptTimestamp(t *testing.T) {
	l := NewLoop()
	l.SetAdaptThreshold(1)
	l.RecordFeedback(Feedback{Type: FeedbackCorrection, Dimension: "x", Correction: "y"})
	l.RecordFeedback(Feedback{Type: FeedbackCorrection, Dimension: "x", Correction: "y"})

	before := time.Now()
	l.Adapt(context.Background())
	p := l.Profile()
	if p.UpdatedAt.Before(before) {
		t.Fatal("profile should be updated after adapt")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
