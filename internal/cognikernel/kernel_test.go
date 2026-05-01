package cognikernel

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventBus_PublishSubscribe(t *testing.T) {
	bus := NewEventBus(16)

	var received int32
	bus.Subscribe(EventConversationEnded, func(ev Event) {
		atomic.AddInt32(&received, 1)
	})
	bus.Subscribe(EventConversationEnded, func(ev Event) {
		atomic.AddInt32(&received, 1)
	})

	bus.Publish(Event{Type: EventConversationEnded, Timestamp: time.Now()})

	if got := atomic.LoadInt32(&received); got != 2 {
		t.Errorf("expected 2 handlers called, got %d", got)
	}

	recent := bus.RecentEvents(10)
	if len(recent) != 1 {
		t.Errorf("expected 1 buffered event, got %d", len(recent))
	}
}

func TestEventBus_RingBuffer(t *testing.T) {
	bus := NewEventBus(4)

	for i := 0; i < 10; i++ {
		bus.Publish(Event{Type: EventConversationEnded, Timestamp: time.Now()})
	}

	recent := bus.RecentEvents(100)
	if len(recent) != 4 {
		t.Errorf("expected buffer capped at 4, got %d", len(recent))
	}
}

func TestReflectiveLoop_Run(t *testing.T) {
	rl := NewReflectiveLoop()

	var evalCalled, expCalled, distillCalled bool

	rl.SetReflectEval(func(ctx context.Context, intent, reply string, skills []string) (*ReflectEvalResult, error) {
		evalCalled = true
		return &ReflectEvalResult{
			Satisfied: true,
			Quality:   8,
		}, nil
	})

	rl.SetExperienceRecord(func(source, category, outcome, lesson, ctx string, tags []string) {
		expCalled = true
		if outcome != "success" {
			t.Errorf("expected 'success' outcome for quality=8, got %s", outcome)
		}
	})

	rl.SetDistill(func(ctx context.Context, question, expertReply string) {
		distillCalled = true
	})

	data := ConversationEndData{
		TenantID:   "test-tenant",
		UserIntent: "What is Go?",
		AgentReply: "Go is a programming language designed at Google. " + longString(300),
		ModelTier:  "expert",
		SkillsUsed: []string{"knowledge_search"},
	}

	result, err := rl.Run(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !evalCalled {
		t.Error("reflect eval was not called")
	}
	if !expCalled {
		t.Error("experience record was not called")
	}
	if !distillCalled {
		t.Error("distill was not called for expert-tier response")
	}
	if !result.Satisfied {
		t.Error("expected satisfied=true")
	}
	if result.Quality != 8 {
		t.Errorf("expected quality=8, got %d", result.Quality)
	}
	if result.ExperiencesAdded != 1 {
		t.Errorf("expected 1 experience, got %d", result.ExperiencesAdded)
	}
	if result.DistilledRules != 1 {
		t.Errorf("expected 1 distilled rule, got %d", result.DistilledRules)
	}
}

func TestReflectiveLoop_LowQuality_NoDistill(t *testing.T) {
	rl := NewReflectiveLoop()

	rl.SetReflectEval(func(ctx context.Context, intent, reply string, skills []string) (*ReflectEvalResult, error) {
		return &ReflectEvalResult{Quality: 3, Satisfied: false}, nil
	})

	var expOutcome string
	rl.SetExperienceRecord(func(source, category, outcome, lesson, ctx string, tags []string) {
		expOutcome = outcome
	})

	distillCalled := false
	rl.SetDistill(func(ctx context.Context, question, expertReply string) {
		distillCalled = true
	})

	data := ConversationEndData{
		UserIntent: "help",
		AgentReply: "ok",
		ModelTier:  "fast",
	}

	result, err := rl.Run(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if expOutcome != "failure" {
		t.Errorf("expected 'failure' for quality=3, got %s", expOutcome)
	}
	if distillCalled {
		t.Error("distill should not be called for non-expert tier")
	}
	if result.DistilledRules != 0 {
		t.Errorf("expected 0 distilled rules, got %d", result.DistilledRules)
	}
}

func TestDreamingLoop_Run(t *testing.T) {
	dl := NewDreamingLoop()

	dl.SetCuriosity(func(ctx context.Context, tenantID string) ([]string, int, error) {
		return []string{"discovered fact about Go generics"}, 1, nil
	})

	var reverieTriggers []string
	dl.SetReverie(func(ctx context.Context, trigger, data string) error {
		reverieTriggers = append(reverieTriggers, trigger)
		return nil
	})

	dl.SetSkillGrow(func(ctx context.Context, tenantID string) ([]string, error) {
		return []string{"data_analysis"}, nil
	})

	sinkCalled := false
	dl.SetFactSink(func(ctx context.Context, tenantID, fact, source string) error {
		sinkCalled = true
		return nil
	})

	result, err := dl.Run(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExplorationsRun != 1 {
		t.Errorf("expected 1 exploration, got %d", result.ExplorationsRun)
	}
	if result.FactsDiscovered != 1 {
		t.Errorf("expected 1 fact, got %d", result.FactsDiscovered)
	}
	if result.SkillsSuggested != 1 {
		t.Errorf("expected 1 skill suggestion, got %d", result.SkillsSuggested)
	}
	if !sinkCalled {
		t.Error("fact sink was not called")
	}
	// curiosity_discovery + skill_gap_detected = 2 reverie triggers
	if len(reverieTriggers) != 2 {
		t.Errorf("expected 2 reverie triggers, got %d: %v", len(reverieTriggers), reverieTriggers)
	}
}

func TestImmuneBridge_BeforeSkill(t *testing.T) {
	ib := NewImmuneBridge()

	ib.SetTrustCheck(func(skillName string) error {
		if skillName == "dangerous_tool" {
			return context.DeadlineExceeded
		}
		return nil
	})

	if err := ib.BeforeSkill(context.Background(), "safe_tool", nil); err != nil {
		t.Errorf("safe_tool should pass, got error: %v", err)
	}

	if err := ib.BeforeSkill(context.Background(), "dangerous_tool", nil); err == nil {
		t.Error("dangerous_tool should be blocked")
	}

	m := ib.Metrics()
	if m.TrustBlocks != 1 {
		t.Errorf("expected 1 trust block, got %d", m.TrustBlocks)
	}
	if m.TotalChecks != 2 {
		t.Errorf("expected 2 total checks, got %d", m.TotalChecks)
	}
}

func TestKernel_EndToEnd(t *testing.T) {
	cfg := DefaultKernelConfig()
	cfg.ReflectTimeout = 5 * time.Second
	k := New(cfg)

	rl := NewReflectiveLoop()
	var reflectRan int32
	rl.SetReflectEval(func(ctx context.Context, intent, reply string, skills []string) (*ReflectEvalResult, error) {
		atomic.AddInt32(&reflectRan, 1)
		return &ReflectEvalResult{Satisfied: true, Quality: 9}, nil
	})
	rl.SetExperienceRecord(func(source, category, outcome, lesson, ctx string, tags []string) {})

	k.SetReflectiveLoop(rl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	k.Start(ctx)

	// Simulate a conversation ending
	k.OnConversationEnd(ConversationEndData{
		TenantID:   "test",
		UserIntent: "hello",
		AgentReply: "hi there",
		ModelTier:  "fast",
	})

	// Wait for async reflection
	time.Sleep(200 * time.Millisecond)

	if got := atomic.LoadInt32(&reflectRan); got != 1 {
		t.Errorf("expected reflective loop to run once, got %d", got)
	}

	m := k.Metrics()
	if m.ActiveCycles != 1 {
		t.Errorf("expected 1 active cycle, got %d", m.ActiveCycles)
	}
	if m.ReflectCycles != 1 {
		t.Errorf("expected 1 reflect cycle, got %d", m.ReflectCycles)
	}
}

func TestKernel_DoubleStart_NoDuplicateSubscriptions(t *testing.T) {
	cfg := DefaultKernelConfig()
	cfg.ReflectTimeout = 5 * time.Second
	k := New(cfg)

	rl := NewReflectiveLoop()
	var reflectCount int32
	rl.SetReflectEval(func(ctx context.Context, intent, reply string, skills []string) (*ReflectEvalResult, error) {
		atomic.AddInt32(&reflectCount, 1)
		return &ReflectEvalResult{Satisfied: true, Quality: 9}, nil
	})
	rl.SetExperienceRecord(func(source, category, outcome, lesson, ctx string, tags []string) {})

	k.SetReflectiveLoop(rl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start twice — subscriptions should only register once
	k.Start(ctx)
	k.Stop()
	k.Start(ctx)

	k.OnConversationEnd(ConversationEndData{
		TenantID:   "test",
		UserIntent: "hello",
		AgentReply: "hi there",
		ModelTier:  "fast",
	})

	time.Sleep(200 * time.Millisecond)

	// Should only run once (no duplicate subscription)
	if got := atomic.LoadInt32(&reflectCount); got != 1 {
		t.Errorf("expected 1 reflection (no duplicate subscription), got %d", got)
	}
}

func TestKernel_ReflectSemaphore(t *testing.T) {
	cfg := DefaultKernelConfig()
	cfg.ReflectTimeout = 5 * time.Second
	k := New(cfg)

	rl := NewReflectiveLoop()
	var activeCount int32
	rl.SetReflectEval(func(ctx context.Context, intent, reply string, skills []string) (*ReflectEvalResult, error) {
		atomic.AddInt32(&activeCount, 1)
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&activeCount, -1)
		return &ReflectEvalResult{Satisfied: true, Quality: 7}, nil
	})
	rl.SetExperienceRecord(func(source, category, outcome, lesson, ctx string, tags []string) {})

	k.SetReflectiveLoop(rl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	k.Start(ctx)

	// Fire 5 events rapidly — semaphore limits to 2 concurrent
	for i := 0; i < 5; i++ {
		k.OnConversationEnd(ConversationEndData{
			TenantID:   "test",
			UserIntent: "hello",
			AgentReply: "hi",
			ModelTier:  "fast",
		})
	}

	time.Sleep(50 * time.Millisecond)
	peak := atomic.LoadInt32(&activeCount)
	if peak > 2 {
		t.Errorf("expected max 2 concurrent reflections, got %d", peak)
	}
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
