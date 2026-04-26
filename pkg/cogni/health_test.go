package cogni

import (
	"testing"
	"time"
)

func TestMonitor_NilStoreReturnsIdle(t *testing.T) {
	m := NewMonitor(nil)
	got := m.ComputeFor("x", 0)
	if got.Status != "idle" || got.Score != 0 {
		t.Fatalf("nil store should yield idle/0, got %+v", got)
	}
}

func TestMonitor_NoTracesIsIdleWithNeutralScore(t *testing.T) {
	store := NewInMemoryTraceStore(8)
	m := NewMonitor(store)
	got := m.ComputeFor("anything", 0)
	if got.Status != "idle" {
		t.Fatalf("expected idle, got %+v", got)
	}
	if got.Score != 50 {
		t.Fatalf("expected neutral 50 score on empty data, got %d", got.Score)
	}
}

func TestMonitor_ComputeFor_ActivationRateAndDuration(t *testing.T) {
	store := NewInMemoryTraceStore(16)
	now := time.Now()
	for i := 0; i < 10; i++ {
		store.Record(Trace{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			DurationMs: int64(100 + 10*i),
			Activations: []TraceActivation{
				{ID: "x", Activated: i < 5},
				{ID: "y", Activated: true},
			},
		})
	}
	got := NewMonitor(store).ComputeFor("x", 0)
	if got.Evaluations != 10 || got.Activations != 5 {
		t.Fatalf("expected 10/5, got %+v", got)
	}
	if got.ActivationRate != 0.5 {
		t.Fatalf("expected ActivationRate 0.5, got %v", got.ActivationRate)
	}
	if got.AvgDurationMs == 0 {
		t.Fatalf("expected positive avg duration, got %d", got.AvgDurationMs)
	}
	if got.Status != "healthy" {
		t.Fatalf("active cogni with no errors should be healthy; got %s (%d)", got.Status, got.Score)
	}
}

func TestMonitor_TemplateFallbackHurtsScore(t *testing.T) {
	store := NewInMemoryTraceStore(8)
	for i := 0; i < 10; i++ {
		store.Record(Trace{
			Activations: []TraceActivation{{ID: "x", Activated: true}},
			Context:     TraceContext{Bytes: 80, Sources: []string{"x"}, TemplateFallbacks: 1},
		})
	}
	got := NewMonitor(store).ComputeFor("x", 0)
	if got.TemplateFallbackRate != 1.0 {
		t.Fatalf("expected 100%% fallback, got %v", got.TemplateFallbackRate)
	}
	if got.Score >= 70 {
		t.Fatalf("100%% template fallback should drop score below healthy; got %d", got.Score)
	}
}

func TestMonitor_SurfaceFallbackHurtsScore(t *testing.T) {
	store := NewInMemoryTraceStore(8)
	for i := 0; i < 5; i++ {
		store.Record(Trace{
			Activations: []TraceActivation{{ID: "x", Activated: true}},
			ToolFilter: &TraceToolFilter{
				Before:          10,
				After:           10,
				AppliedByCognis: []string{"x"},
				FellBackToInput: true,
			},
		})
	}
	got := NewMonitor(store).ComputeFor("x", 0)
	if got.Score >= 70 {
		t.Fatalf("100%% surface fallback should drop below healthy; got %d", got.Score)
	}
}

func TestMonitor_ToolFilterRatioBoosts(t *testing.T) {
	store := NewInMemoryTraceStore(8)
	for i := 0; i < 5; i++ {
		store.Record(Trace{
			Activations: []TraceActivation{{ID: "x", Activated: true}},
			ToolFilter: &TraceToolFilter{
				Before:          10,
				After:           3, // narrows to 30%
				Removed:         []string{"a", "b", "c", "d", "e", "f", "g"},
				AppliedByCognis: []string{"x"},
			},
		})
	}
	got := NewMonitor(store).ComputeFor("x", 0)
	if got.ToolFilterRatio == 0 {
		t.Fatalf("expected ratio populated, got 0")
	}
	if got.ToolFilterRatio >= 0.5 {
		t.Fatalf("ratio should be <0.5 (active narrowing), got %v", got.ToolFilterRatio)
	}
	if got.Status != "healthy" {
		t.Fatalf("active narrower should remain healthy; got %s (%d)", got.Status, got.Score)
	}
}

func TestMonitor_ChronicSuppressionPenalizesIdle(t *testing.T) {
	store := NewInMemoryTraceStore(8)
	for i := 0; i < 10; i++ {
		store.Record(Trace{
			Activations: []TraceActivation{
				{ID: "loser", Activated: false, Suppressed: true, SuppressedByID: "winner"},
			},
		})
	}
	got := NewMonitor(store).ComputeFor("loser", 0)
	// 0 activations + 100% suppression → likely misconfigured
	if got.Score >= 50 {
		t.Fatalf("chronically suppressed cogni should get a low score; got %d", got.Score)
	}
}

func TestMonitor_ComputeAll_ListsEveryCogniSeen(t *testing.T) {
	store := NewInMemoryTraceStore(8)
	store.Record(Trace{Activations: []TraceActivation{
		{ID: "a", Activated: true},
		{ID: "b", Activated: false},
	}})
	store.Record(Trace{Activations: []TraceActivation{
		{ID: "c", Activated: true},
	}})

	all := NewMonitor(store).ComputeAll(0)
	if len(all) != 3 {
		t.Fatalf("expected 3 cognis (a,b,c), got %d (%+v)", len(all), all)
	}
	want := []string{"a", "b", "c"}
	for i, m := range all {
		if m.ID != want[i] {
			t.Fatalf("ComputeAll must be sorted by ID; got %v", all)
		}
	}
}

func TestComputeMetrics_LastSeenAtPopulated(t *testing.T) {
	store := NewInMemoryTraceStore(4)
	now := time.Now().Round(time.Second)
	store.Record(Trace{
		Timestamp:   now,
		Activations: []TraceActivation{{ID: "x", Activated: true}},
	})
	got := NewMonitor(store).ComputeFor("x", 0)
	if got.LastSeenAt == "" {
		t.Fatalf("LastSeenAt should be populated when cogni activates")
	}
}
