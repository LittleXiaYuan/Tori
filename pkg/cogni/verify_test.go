package cogni

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestVerifyDeclaration_NilAndEmpty(t *testing.T) {
	if got := VerifyDeclaration(nil, nil); got != nil {
		t.Fatalf("nil declaration must return nil, got %+v", got)
	}
	if got := VerifyDeclaration(&Declaration{ID: "x"}, nil); got != nil {
		t.Fatalf("declaration without checks must return nil, got %+v", got)
	}
}

func TestVerifyDeclaration_ActiveExpectation(t *testing.T) {
	d := &Declaration{
		ID: "reviewer",
		Activation: ActivationRules{
			Keywords: []string{"review"},
			MinScore: 0.2,
		},
		Checks: []ActivationCheck{
			{Name: "review matches", Message: "please review", ExpectActive: boolPtr(true)},
			{Name: "off-topic ignores", Message: "hello world", ExpectActive: boolPtr(false)},
		},
	}
	results := VerifyDeclaration(d, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %+v", results)
	}
	if !results[0].Passed {
		t.Fatalf("positive case should pass: %+v", results[0])
	}
	if !results[1].Passed {
		t.Fatalf("negative case should pass: %+v", results[1])
	}
}

func TestVerifyDeclaration_DetectsBrokenRule(t *testing.T) {
	d := &Declaration{
		ID: "review",
		Activation: ActivationRules{
			Keywords: []string{"never-matches"},
			MinScore: 0.2,
		},
		Checks: []ActivationCheck{
			{Name: "should activate", Message: "review please", ExpectActive: boolPtr(true)},
		},
	}
	results := VerifyDeclaration(d, nil)
	if results[0].Passed {
		t.Fatalf("check must catch the misconfiguration, got %+v", results[0])
	}
	if !strings.Contains(results[0].Reason, "expect_active=true got false") {
		t.Fatalf("failure reason must explain the mismatch: %s", results[0].Reason)
	}
}

func TestVerifyDeclaration_ScoreAtLeast(t *testing.T) {
	d := &Declaration{
		ID: "weak",
		Activation: ActivationRules{
			Keywords:      []string{"a"},
			KeywordWeight: 0.1,
			MinScore:      0.05,
		},
		Checks: []ActivationCheck{
			{Message: "a", ExpectScoreAtLeast: 0.5},
		},
	}
	results := VerifyDeclaration(d, nil)
	if results[0].Passed {
		t.Fatalf("0.1 score should fail >=0.5 assertion")
	}
	if !strings.Contains(results[0].Reason, "expect_score_at_least") {
		t.Fatalf("reason must mention score: %s", results[0].Reason)
	}
}

func TestVerifyDeclaration_ReasonContains(t *testing.T) {
	d := &Declaration{
		ID: "r",
		Activation: ActivationRules{
			Keywords: []string{"foo"},
			MinScore: 0.2,
		},
		Checks: []ActivationCheck{
			{Message: "foo bar", ExpectReasonContains: []string{"keyword: foo"}},
			{Message: "foo bar", ExpectReasonContains: []string{"keyword: missing"}},
		},
	}
	results := VerifyDeclaration(d, nil)
	if !results[0].Passed {
		t.Fatalf("first should pass (reason contains 'keyword: foo'): %+v", results[0])
	}
	if results[1].Passed {
		t.Fatalf("second should fail (reason does not contain 'keyword: missing')")
	}
}

func TestVerifyDeclaration_NoAssertionIsTolerated(t *testing.T) {
	d := &Declaration{
		ID:     "x",
		Activation: ActivationRules{AlwaysOn: true},
		Checks: []ActivationCheck{{Message: "ignored"}},
	}
	results := VerifyDeclaration(d, nil)
	if !results[0].Passed {
		t.Fatalf("check with no assertion must not be a failure, got %+v", results[0])
	}
}

func TestRegistry_VerifyAll_SkipsCogniWithoutChecks(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "has-checks",
		Activation: ActivationRules{Keywords: []string{"x"}, MinScore: 0.2},
		Checks: []ActivationCheck{
			{Message: "x", ExpectActive: boolPtr(true)},
		},
	}, "test")
	_ = r.Add(&Declaration{ID: "no-checks", Activation: ActivationRules{AlwaysOn: true}}, "test")

	got := r.VerifyAll()
	if len(got) != 1 {
		t.Fatalf("only cogni with checks should appear, got %+v", got)
	}
	if _, ok := got["has-checks"]; !ok {
		t.Fatalf("missing has-checks in results")
	}
}

func TestFailedChecks_FlattensAndSorts(t *testing.T) {
	m := map[string][]CheckResult{
		"b": {
			{CogniID: "b", CheckIndex: 0, Passed: false, Reason: "boom"},
			{CogniID: "b", CheckIndex: 1, Passed: true},
		},
		"a": {
			{CogniID: "a", CheckIndex: 0, Passed: false, Reason: "bad"},
		},
		"c": {
			{CogniID: "c", CheckIndex: 0, Passed: false, Reason: "no assertion configured (ignored)"},
		},
	}
	failed := FailedChecks(m)
	if len(failed) != 2 {
		t.Fatalf("expected 2 real failures, got %d (%+v)", len(failed), failed)
	}
	if failed[0].CogniID != "a" || failed[1].CogniID != "b" {
		t.Fatalf("expected sort by id, got %+v", failed)
	}
}

func TestRegistry_ReloadFromDir_StampsCheckFailures(t *testing.T) {
	dir := t.TempDir()
	body := `{
		"id": "broken",
		"activation": {"keywords": ["foo"], "min_score": 0.2},
		"checks": [
			{"message": "foo", "expect_active": true},
			{"message": "bar", "expect_active": true}
		]
	}`
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := NewRegistry()
	sum, err := r.ReloadFromDir(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if sum.Added != 1 {
		t.Fatalf("expected 1 added, got %+v", sum)
	}
	if len(sum.CheckFailures) != 1 {
		t.Fatalf("expected 1 check failure (bar should not activate), got %+v", sum.CheckFailures)
	}
	// Registry entry must record the failure summary so /v1/cognis shows it.
	entries := r.List()
	if len(entries) != 1 || entries[0].LoadError == "" {
		t.Fatalf("entry must stamp LoadError on failed checks, got %+v", entries)
	}
	if !strings.Contains(entries[0].LoadError, "checks failed") {
		t.Fatalf("LoadError must mention checks, got %q", entries[0].LoadError)
	}
}
