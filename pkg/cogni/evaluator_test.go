package cogni

import (
	"strings"
	"testing"
)

func TestEvaluator_AlwaysOnShortCircuits(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "always",
		Activation: ActivationRules{
			AlwaysOn: true,
			Channels: []string{"never-this-channel"},
		},
	}
	got := e.Evaluate([]*Declaration{d}, Session{Channel: "webchat"})
	if len(got) != 1 || !got[0].Activated || got[0].Score != 1.0 {
		t.Fatalf("always_on should activate with score 1.0, got %+v", got)
	}
	if !containsReason(got[0].Reasons, "always_on") {
		t.Fatalf("missing always_on reason: %v", got[0].Reasons)
	}
}

func TestEvaluator_KeywordsAccumulate(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "kw",
		Activation: ActivationRules{
			Keywords:      []string{"review", "PR"},
			KeywordWeight: 0.3,
			MinScore:      0.5,
		},
	}

	got := e.Evaluate([]*Declaration{d}, Session{Message: "please review this PR"})
	if len(got) != 1 || !got[0].Activated {
		t.Fatalf("two keyword hits should activate (0.6 >= 0.5); got %+v", got)
	}
	// Two keyword hits: 0.3 + 0.3 = 0.6
	if got[0].Score < 0.59 || got[0].Score > 0.61 {
		t.Fatalf("expected score ~0.6, got %v", got[0].Score)
	}
}

func TestEvaluator_KeywordsCaseInsensitive(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "case",
		Activation: ActivationRules{
			Keywords: []string{"REVIEW"},
			MinScore: 0.2,
		},
	}
	got := e.Evaluate([]*Declaration{d}, Session{Message: "let's review"})
	if !got[0].Activated {
		t.Fatalf("keyword match should be case-insensitive")
	}
}

func TestEvaluator_RegexMatching(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "regex",
		Activation: ActivationRules{
			Regex:       []string{`^review\s+#\d+`},
			RegexWeight: 0.6,
			MinScore:    0.5,
		},
	}
	got := e.Evaluate([]*Declaration{d}, Session{Message: "Review #42 please"})
	if !got[0].Activated {
		t.Fatalf("regex should activate (case-insensitive auto-injected)")
	}

	got = e.Evaluate([]*Declaration{d}, Session{Message: "no match"})
	if got[0].Activated {
		t.Fatalf("non-matching message should not activate")
	}
}

func TestEvaluator_InvalidRegexIsIgnored(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "bad-regex-noop",
		Activation: ActivationRules{
			Regex:    []string{"[unclosed"},
			Keywords: []string{"hello"},
			MinScore: 0.2,
		},
	}
	got := e.Evaluate([]*Declaration{d}, Session{Message: "hello world"})
	if !got[0].Activated {
		t.Fatalf("invalid regex must be silently ignored, keyword should still hit")
	}
}

func TestEvaluator_ChannelAndTenantGate(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "scoped",
		Activation: ActivationRules{
			Channels: []string{"telegram"},
			Tenants:  []string{"t1"},
			Keywords: []string{"hi"},
			MinScore: 0.2,
		},
	}

	if got := e.Evaluate([]*Declaration{d}, Session{Channel: "webchat", TenantID: "t1", Message: "hi"}); got[0].Activated {
		t.Fatalf("channel mismatch must block")
	}
	if got := e.Evaluate([]*Declaration{d}, Session{Channel: "telegram", TenantID: "t2", Message: "hi"}); got[0].Activated {
		t.Fatalf("tenant mismatch must block")
	}
	if got := e.Evaluate([]*Declaration{d}, Session{Channel: "telegram", TenantID: "t1", Message: "hi"}); !got[0].Activated {
		t.Fatalf("matching channel + tenant should activate")
	}
}

func TestEvaluator_HandoverContributesScore(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "handover",
		Activation: ActivationRules{
			HandoverOn: []string{"need-review"},
			MinScore:   0.5,
		},
	}
	got := e.Evaluate([]*Declaration{d}, Session{PriorHandover: []string{"need-review"}})
	if !got[0].Activated {
		t.Fatalf("handover tag should contribute 0.5; got score=%v", got[0].Score)
	}
}

func TestEvaluator_ScoreClampedToOne(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "overflow",
		Activation: ActivationRules{
			Keywords:      []string{"a", "a", "a", "a", "a"},
			KeywordWeight: 0.5,
			MinScore:      0.5,
		},
	}
	got := e.Evaluate([]*Declaration{d}, Session{Message: "a"})
	if got[0].Score > 1.0+1e-9 {
		t.Fatalf("score must be clamped to 1.0, got %v", got[0].Score)
	}
}

func TestEvaluator_EvaluateOrdersByScoreThenPriority(t *testing.T) {
	e := NewEvaluator()
	high := &Declaration{
		ID:       "high",
		Priority: 200,
		Activation: ActivationRules{
			Keywords:      []string{"x"},
			KeywordWeight: 0.6,
			MinScore:      0.5,
		},
	}
	medium := &Declaration{
		ID:       "medium",
		Priority: 50, // lower number = higher priority for tie-break
		Activation: ActivationRules{
			Keywords:      []string{"x"},
			KeywordWeight: 0.3,
			MinScore:      0.2,
		},
	}
	tieA := &Declaration{
		ID:       "tieA",
		Priority: 10,
		Activation: ActivationRules{
			Keywords:      []string{"x"},
			KeywordWeight: 0.6,
			MinScore:      0.5,
		},
	}

	got := e.Evaluate([]*Declaration{medium, high, tieA}, Session{Message: "x"})
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	// Highest score first; tieA and high tied at 0.6 → tieA wins (lower priority number)
	if got[0].Declaration.ID != "tieA" {
		t.Fatalf("tieA (priority=10, score=0.6) should rank first; got %s", got[0].Declaration.ID)
	}
	if got[1].Declaration.ID != "high" {
		t.Fatalf("high (priority=200, score=0.6) should rank second; got %s", got[1].Declaration.ID)
	}
	if got[2].Declaration.ID != "medium" {
		t.Fatalf("medium (score=0.3) should rank last; got %s", got[2].Declaration.ID)
	}
}

func TestEvaluator_NilDeclarationsSkipped(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{ID: "ok", Activation: ActivationRules{AlwaysOn: true}}
	got := e.Evaluate([]*Declaration{nil, d, nil}, Session{})
	if len(got) != 1 || got[0].Declaration.ID != "ok" {
		t.Fatalf("nil declarations must be skipped; got %+v", got)
	}
}

func TestEvaluator_RegexCachedAcrossCalls(t *testing.T) {
	e := NewEvaluator()
	d := &Declaration{
		ID: "cache",
		Activation: ActivationRules{
			Regex:    []string{`hello`},
			MinScore: 0.2,
		},
	}
	// Two evaluations should reuse the same compiled regex.
	_ = e.Evaluate([]*Declaration{d}, Session{Message: "hello"})
	_ = e.Evaluate([]*Declaration{d}, Session{Message: "hello again"})

	e.mu.RLock()
	defer e.mu.RUnlock()
	if _, ok := e.regexCache["hello"]; !ok {
		t.Fatalf("compiled regex must be cached")
	}
}

func TestApplyExclusivity_KeepsHighestPerGroup(t *testing.T) {
	a := Activation{Activated: true, Score: 0.6, Declaration: &Declaration{ID: "a", Exclusive: "g1"}}
	b := Activation{Activated: true, Score: 0.9, Declaration: &Declaration{ID: "b", Exclusive: "g1"}}
	c := Activation{Activated: true, Score: 0.4, Declaration: &Declaration{ID: "c", Exclusive: "g2"}}
	d := Activation{Activated: true, Score: 0.5, Declaration: &Declaration{ID: "d"}}
	inactive := Activation{Activated: false, Declaration: &Declaration{ID: "inactive", Exclusive: "g1"}}

	out := ApplyExclusivity([]Activation{a, b, c, d, inactive})

	// expected order (input order preserved):
	//   slot for g1 first sees a (0.6) → later replaced by b (0.9)
	//   c (g2)
	//   d (no group)
	//   inactive (passes through)
	if len(out) != 4 {
		t.Fatalf("expected 4 entries, got %d (%+v)", len(out), ids(out))
	}
	if out[0].Declaration.ID != "b" {
		t.Fatalf("g1 winner must be b (highest score); got %s", out[0].Declaration.ID)
	}
	if out[1].Declaration.ID != "c" {
		t.Fatalf("expected c at index 1; got %s", out[1].Declaration.ID)
	}
	if out[2].Declaration.ID != "d" {
		t.Fatalf("expected d at index 2; got %s", out[2].Declaration.ID)
	}
	if out[3].Declaration.ID != "inactive" {
		t.Fatalf("inactive must pass through unchanged; got %s", out[3].Declaration.ID)
	}
}

func TestApplyExclusivity_NilDeclarationSafe(t *testing.T) {
	a := Activation{Activated: true, Score: 0.6}
	out := ApplyExclusivity([]Activation{a})
	if len(out) != 1 {
		t.Fatalf("nil declaration entries must be passed through; got %+v", out)
	}
}

func TestFiltered_ReturnsOnlyActive(t *testing.T) {
	in := []Activation{
		{Activated: true, Declaration: &Declaration{ID: "1"}},
		{Activated: false, Declaration: &Declaration{ID: "2"}},
		{Activated: true, Declaration: &Declaration{ID: "3"}},
	}
	out := Filtered(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 active, got %d", len(out))
	}
	if out[0].Declaration.ID != "1" || out[1].Declaration.ID != "3" {
		t.Fatalf("Filtered must preserve order: %v", ids(out))
	}
}

// helpers

func containsReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if strings.Contains(r, want) {
			return true
		}
	}
	return false
}

func ids(acts []Activation) []string {
	out := make([]string, 0, len(acts))
	for _, a := range acts {
		if a.Declaration == nil {
			out = append(out, "<nil>")
			continue
		}
		out = append(out, a.Declaration.ID)
	}
	return out
}
