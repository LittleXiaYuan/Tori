package cogni

import (
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestInMemoryTraceStore_RingBufferEvictsOldest(t *testing.T) {
	s := NewInMemoryTraceStore(3)
	for i := 0; i < 5; i++ {
		s.Record(Trace{MessageHash: id(i)})
	}
	got := s.Recent(0)
	if len(got) != 3 {
		t.Fatalf("expected ring of 3, got %d", len(got))
	}
	// most recent first
	if got[0].MessageHash != id(4) || got[2].MessageHash != id(2) {
		t.Fatalf("unexpected ordering: %+v", got)
	}
}

func TestInMemoryTraceStore_StatsCountActivatedOnly(t *testing.T) {
	s := NewInMemoryTraceStore(10)
	s.Record(Trace{Activations: []TraceActivation{
		{ID: "a", Activated: true},
		{ID: "b", Activated: false},
	}})
	s.Record(Trace{Activations: []TraceActivation{
		{ID: "a", Activated: true},
	}})
	st := s.Stats()
	if st.TotalTurns != 2 {
		t.Fatalf("expected 2 turns, got %d", st.TotalTurns)
	}
	if st.PerCogni["a"] != 2 {
		t.Fatalf("a should appear in 2 traces, got %d", st.PerCogni["a"])
	}
	if _, ok := st.PerCogni["b"]; ok {
		t.Fatalf("non-activated cogni must not appear in stats: %+v", st.PerCogni)
	}
}

func TestInMemoryTraceStore_ByCogniFiltersAndLimits(t *testing.T) {
	s := NewInMemoryTraceStore(10)
	s.Record(Trace{MessageHash: "m1", Activations: []TraceActivation{{ID: "x"}}})
	s.Record(Trace{MessageHash: "m2", Activations: []TraceActivation{{ID: "y"}}})
	s.Record(Trace{MessageHash: "m3", Activations: []TraceActivation{{ID: "x"}}})

	got := s.ByCogni("x", 0)
	if len(got) != 2 {
		t.Fatalf("expected 2 traces for x, got %d", len(got))
	}
	if got[0].MessageHash != "m3" {
		t.Fatalf("ByCogni should return most recent first, got %v", got[0])
	}

	limited := s.ByCogni("x", 1)
	if len(limited) != 1 || limited[0].MessageHash != "m3" {
		t.Fatalf("limit=1 should return only the newest, got %+v", limited)
	}
}

func TestHook_TraceCaptured_OnFilterSkills(t *testing.T) {
	store := NewInMemoryTraceStore(8)
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "narrow",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{Only: []string{"a"}},
		Context:    ContextInjection{Static: "you are narrow"},
	}, "test")

	h := NewHook(r)
	h.SetTraceStore(store)

	// ContextRequest is shared between the two callbacks because they share
	// the same (msg, tenant, channel) fingerprint.
	req := ContextRequest{Message: "hello world from trace", TenantID: "t1", Channel: "webchat"}
	prompt := h.BuildContext(req)
	in := []skills.Skill{sk("a"), sk("b"), sk("c")}
	out := h.FilterSkills(req, in)

	if !strings.Contains(prompt, "you are narrow") {
		t.Fatalf("BuildContext should produce content; got %q", prompt)
	}
	if len(out) != 1 || out[0].Name() != "a" {
		t.Fatalf("FilterSkills should narrow to [a]; got %v", toolSet(out))
	}

	rec := store.Recent(0)
	if len(rec) != 1 {
		t.Fatalf("expected exactly one trace per turn, got %d (%+v)", len(rec), rec)
	}
	tr := rec[0]
	if len(tr.Activations) != 1 || tr.Activations[0].ID != "narrow" || !tr.Activations[0].Activated {
		t.Fatalf("trace activations malformed: %+v", tr.Activations)
	}
	if tr.Context.Bytes <= 0 || len(tr.Context.Sources) != 1 || tr.Context.Sources[0] != "narrow" {
		t.Fatalf("trace.Context not populated: %+v", tr.Context)
	}
	if tr.ToolFilter == nil {
		t.Fatalf("trace.ToolFilter must be populated when FilterSkills runs")
	}
	if tr.ToolFilter.Before != 3 || tr.ToolFilter.After != 1 {
		t.Fatalf("tool filter diff wrong: %+v", tr.ToolFilter)
	}
	if got := tr.ToolFilter.Removed; len(got) != 2 {
		t.Fatalf("expected 2 removed (b,c), got %v", got)
	}
}

func TestHook_TraceSnapshotReturnsCurrentTurnWithoutRawMessage(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:          "doc",
		DisplayName: "文档助手",
		Activation:  ActivationRules{AlwaysOn: true},
		Context:     ContextInjection{Static: "read files carefully"},
		Surface:     ToolSurface{Only: []string{"file_open"}},
	}, "test")
	h := NewHook(r)
	req := ContextRequest{Message: "请读取附件里的合同", TenantID: "t1", Channel: "web"}
	_ = h.BuildContext(req)
	_ = h.FilterSkills(req, []skills.Skill{sk("file_open"), sk("browser_search")})

	tr, ok := h.TraceSnapshot(req)
	if !ok {
		t.Fatal("expected trace snapshot")
	}
	if len(tr.Activations) != 1 || tr.Activations[0].DisplayName != "文档助手" || !tr.Activations[0].Activated {
		t.Fatalf("unexpected activations: %+v", tr.Activations)
	}
	if tr.MessageHash == "" || strings.Contains(tr.MessageHash, "合同") {
		t.Fatalf("snapshot should expose only hash, got %q", tr.MessageHash)
	}
	if tr.Context.Bytes == 0 || tr.ToolFilter == nil || tr.ToolFilter.Before != 2 || tr.ToolFilter.After != 1 {
		t.Fatalf("snapshot missing context/tool diff: %+v", tr)
	}
}

func TestHook_TraceRecordsSuppressed(t *testing.T) {
	store := NewInMemoryTraceStore(4)
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:        "lo",
		Exclusive: "g",
		Activation: ActivationRules{
			Keywords:      []string{"x"},
			KeywordWeight: 0.3,
			MinScore:      0.2,
		},
	}, "test")
	_ = r.Add(&Declaration{
		ID:        "hi",
		Exclusive: "g",
		Activation: ActivationRules{
			Keywords:      []string{"x"},
			KeywordWeight: 0.9,
			MinScore:      0.2,
		},
	}, "test")

	h := NewHook(r)
	h.SetTraceStore(store)
	// FilterSkills triggers the trace flush; supply at least one skill so the
	// path runs to completion.
	_ = h.FilterSkills(ContextRequest{Message: "x"}, []skills.Skill{sk("any")})

	rec := store.Recent(0)
	if len(rec) != 1 {
		t.Fatalf("expected one trace, got %d", len(rec))
	}

	var loEntry *TraceActivation
	for i := range rec[0].Activations {
		if rec[0].Activations[i].ID == "lo" {
			loEntry = &rec[0].Activations[i]
		}
	}
	if loEntry == nil {
		t.Fatalf("missing 'lo' entry in trace: %+v", rec[0].Activations)
	}
	if !loEntry.Suppressed {
		t.Fatalf("lo must be marked suppressed by exclusivity")
	}
	if loEntry.SuppressedByID != "hi" {
		t.Fatalf("lo must point at hi as the suppressor; got %q", loEntry.SuppressedByID)
	}
}

func TestHook_TraceMessageHashHidesContent(t *testing.T) {
	store := NewInMemoryTraceStore(4)
	r := NewRegistry()
	_ = r.Add(&Declaration{ID: "x", Activation: ActivationRules{AlwaysOn: true}}, "test")
	h := NewHook(r)
	h.SetTraceStore(store)
	_ = h.FilterSkills(ContextRequest{Message: "secret password 12345"}, []skills.Skill{sk("a")})

	rec := store.Recent(0)
	if rec[0].MessageHash == "" {
		t.Fatalf("hash must be populated")
	}
	if strings.Contains(rec[0].MessageHash, "secret") || strings.Contains(rec[0].MessageHash, "password") {
		t.Fatalf("trace must not retain raw message content; got hash=%q", rec[0].MessageHash)
	}
	if rec[0].MessageLen != 21 {
		t.Fatalf("MessageLen should equal rune count of input; got %d", rec[0].MessageLen)
	}
}

func TestTurnCache_CoalescesPaired_BuildContext_FilterSkills(t *testing.T) {
	store := NewInMemoryTraceStore(8)
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "x",
		Activation: ActivationRules{AlwaysOn: true},
		Context:    ContextInjection{Static: "x"},
		Surface:    ToolSurface{Exclude: []string{"b"}},
	}, "test")
	h := NewHook(r)
	h.SetTraceStore(store)

	// Same (msg,tenant,channel) → must produce one trace not two
	req := ContextRequest{Message: "shared", TenantID: "t", Channel: "c"}
	_ = h.BuildContext(req)
	_ = h.FilterSkills(req, []skills.Skill{sk("a"), sk("b")})

	if got := store.Recent(0); len(got) != 1 {
		t.Fatalf("expected exactly one trace per logical turn, got %d", len(got))
	}
}

// helper used by trace tests
func id(i int) string {
	return string(rune('a' + i))
}
