package cogni

import (
	"strings"
	"testing"
)

func TestSentinel_NilStoreIsNoOp(t *testing.T) {
	s := NewSentinel(nil, nil, SentinelPolicy{})
	if got := s.Scan(); got != nil {
		t.Fatalf("nil store should produce no alerts, got %+v", got)
	}
	if got := s.Alerts(); len(got) != 0 {
		t.Fatalf("expected empty alert set, got %+v", got)
	}
}

func TestSentinel_SuppressesOnThinData(t *testing.T) {
	store := NewInMemoryTraceStore(32)
	// Only 3 evaluations — below default MinEvaluations=5
	for i := 0; i < 3; i++ {
		store.Record(Trace{Activations: []TraceActivation{
			{ID: "x", Activated: true},
		}, Context: TraceContext{Bytes: 100, Sources: []string{"x"}, TemplateFallbacks: 1}})
	}
	s := NewSentinel(store, nil, SentinelPolicy{})
	alerts := s.Scan()
	for _, a := range alerts {
		if a.CogniID == "x" {
			t.Fatalf("thin data must not fire alerts, got %+v", a)
		}
	}
}

func TestSentinel_FiresOnPersistentTemplateErrors(t *testing.T) {
	store := NewInMemoryTraceStore(16)
	for i := 0; i < 10; i++ {
		store.Record(Trace{Activations: []TraceActivation{{ID: "x", Activated: true}},
			Context: TraceContext{Bytes: 100, Sources: []string{"x"}, TemplateFallbacks: 1}})
	}
	alerts := NewSentinel(store, nil, SentinelPolicy{}).Scan()

	var hit *Alert
	for i := range alerts {
		if alerts[i].CogniID == "x" && alerts[i].Kind == AlertTemplateErrors {
			hit = &alerts[i]
		}
	}
	if hit == nil {
		t.Fatalf("expected template-error alert, got %+v", alerts)
	}
	if hit.Severity != "critical" {
		t.Fatalf("100%% fallback should be critical, got %s", hit.Severity)
	}
}

func TestSentinel_FiresOnChronicSuppression(t *testing.T) {
	store := NewInMemoryTraceStore(16)
	for i := 0; i < 10; i++ {
		store.Record(Trace{Activations: []TraceActivation{
			{ID: "loser", Activated: false, Suppressed: true, SuppressedByID: "winner"},
		}})
	}
	alerts := NewSentinel(store, nil, SentinelPolicy{}).Scan()
	found := false
	for _, a := range alerts {
		if a.CogniID == "loser" && a.Kind == AlertChronicSuppression {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected chronic-suppression alert, got %+v", alerts)
	}
}

func TestSentinel_AutoDisableWhenEnabled(t *testing.T) {
	store := NewInMemoryTraceStore(16)
	for i := 0; i < 10; i++ {
		store.Record(Trace{Activations: []TraceActivation{{ID: "x", Activated: true}},
			Context: TraceContext{Bytes: 100, Sources: []string{"x"}, TemplateFallbacks: 1}})
	}
	reg := NewRegistry()
	_ = reg.Add(&Declaration{ID: "x", Activation: ActivationRules{AlwaysOn: true}}, "test")

	s := NewSentinel(store, reg, SentinelPolicy{AutoDisableOnCritical: true})
	_ = s.Scan()
	if reg.IsEnabled("x") {
		t.Fatalf("critical alert should have auto-disabled x")
	}
	// Alert must record the action so UI doesn't offer it again.
	found := false
	for _, a := range s.Alerts() {
		if a.CogniID == "x" && a.AutoActionTaken == "disabled" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected AutoActionTaken=disabled in alerts, got %+v", s.Alerts())
	}
}

func TestSentinel_AutoDisableOffByDefault(t *testing.T) {
	store := NewInMemoryTraceStore(16)
	for i := 0; i < 10; i++ {
		store.Record(Trace{Activations: []TraceActivation{{ID: "x", Activated: true}},
			Context: TraceContext{Bytes: 100, Sources: []string{"x"}, TemplateFallbacks: 1}})
	}
	reg := NewRegistry()
	_ = reg.Add(&Declaration{ID: "x", Activation: ActivationRules{AlwaysOn: true}}, "test")

	_ = NewSentinel(store, reg, SentinelPolicy{}).Scan()
	if !reg.IsEnabled("x") {
		t.Fatalf("default policy must NOT auto-disable; x should still be enabled")
	}
}

func TestSentinel_AlertRecovery_ClearsOnNextScan(t *testing.T) {
	store := NewInMemoryTraceStore(16)
	for i := 0; i < 10; i++ {
		store.Record(Trace{Activations: []TraceActivation{{ID: "x", Activated: true}},
			Context: TraceContext{Bytes: 100, Sources: []string{"x"}, TemplateFallbacks: 1}})
	}
	s := NewSentinel(store, nil, SentinelPolicy{})
	if alerts := s.Scan(); len(alerts) == 0 {
		t.Fatalf("expected alerts on first scan, got none")
	}

	// Clean traces: a brand-new store simulating "cogni fixed its template"
	clean := NewInMemoryTraceStore(16)
	for i := 0; i < 10; i++ {
		clean.Record(Trace{Activations: []TraceActivation{{ID: "x", Activated: true}},
			Context: TraceContext{Bytes: 100, Sources: []string{"x"}}})
	}
	s2 := NewSentinel(clean, nil, SentinelPolicy{})
	s2.alerts = s.alerts // reuse prior alert state to exercise GC path
	got := s2.Scan()
	for _, a := range got {
		if a.CogniID == "x" && a.Kind == AlertTemplateErrors {
			t.Fatalf("recovered cogni must clear its alert, still got %+v", a)
		}
	}
}

func TestSentinel_FiresOnDeclarationCheckFailures(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Add(&Declaration{
		ID:         "broken",
		Activation: ActivationRules{Keywords: []string{"never-matches"}, MinScore: 0.2},
		Checks: []ActivationCheck{
			{Name: "positive case", Message: "should match", ExpectActive: boolPtr(true)},
			{Name: "another one", Message: "also should match", ExpectActive: boolPtr(true)},
		},
	}, "test")

	store := NewInMemoryTraceStore(8)
	s := NewSentinel(store, reg, SentinelPolicy{})
	alerts := s.Scan()

	var hit *Alert
	for i := range alerts {
		if alerts[i].CogniID == "broken" && alerts[i].Kind == AlertDeclarationChecksFail {
			hit = &alerts[i]
		}
	}
	if hit == nil {
		t.Fatalf("expected declaration_checks_failed alert, got %+v", alerts)
	}
	if hit.Severity != "critical" {
		t.Fatalf("check failures must be critical, got %s", hit.Severity)
	}
	if !strings.Contains(hit.Message, "2 declarative check") {
		t.Fatalf("message must report count: %q", hit.Message)
	}
	if !strings.Contains(hit.Message, "positive case") {
		t.Fatalf("message must list check labels: %q", hit.Message)
	}
}

func TestSentinel_CheckFailureRecoveryClearsAlert(t *testing.T) {
	reg := NewRegistry()
	decl := &Declaration{
		ID:         "c",
		Activation: ActivationRules{Keywords: []string{"hit"}, MinScore: 0.2},
		Checks: []ActivationCheck{
			{Message: "miss", ExpectActive: boolPtr(true)}, // fails
		},
	}
	_ = reg.Add(decl, "test")

	store := NewInMemoryTraceStore(8)
	s := NewSentinel(store, reg, SentinelPolicy{})
	if got := s.Scan(); len(got) == 0 {
		t.Fatalf("expected alerts from bad check")
	}

	// Author "fixes" the declaration: replace with a passing check
	decl.Checks = []ActivationCheck{
		{Message: "hit please", ExpectActive: boolPtr(true)},
	}
	_ = reg.Add(decl, "test")

	got := s.Scan()
	for _, a := range got {
		if a.Kind == AlertDeclarationChecksFail {
			t.Fatalf("alert should have cleared after check passes, still got %+v", a)
		}
	}
}

func TestSentinel_AlertsSortedBySeverity(t *testing.T) {
	store := NewInMemoryTraceStore(16)
	for i := 0; i < 10; i++ {
		store.Record(Trace{Activations: []TraceActivation{
			{ID: "crit", Activated: true},
			{ID: "warn-only", Activated: false, Suppressed: true, SuppressedByID: "other"},
		}, Context: TraceContext{Bytes: 100, Sources: []string{"crit"}, TemplateFallbacks: 1}})
	}
	s := NewSentinel(store, nil, SentinelPolicy{})
	alerts := s.Scan()
	if len(alerts) < 2 {
		t.Fatalf("expected ≥2 alerts, got %+v", alerts)
	}
	if alerts[0].Severity != "critical" {
		t.Fatalf("critical must sort first, got %s", alerts[0].Severity)
	}
}
