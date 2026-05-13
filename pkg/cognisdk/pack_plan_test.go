package cognisdk

import (
	"strings"
	"testing"
)

func TestPlanPackBundleApplyReady(t *testing.T) {
	current, err := NewPackBundle("current", []PackManifest{XiaoyuCompanionPack()}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	candidate, err := NewPackBundle("candidate", BuiltinPacks(), []string{PackXiaoyuCompanion, PackYunqueWork})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}

	plan, err := PlanPackBundleApply(t.Context(), current, candidate)
	if err != nil {
		t.Fatalf("plan apply: %v", err)
	}
	if plan.Outcome != PackBundleReviewReady || plan.RequiresReview || plan.Blocked {
		t.Fatalf("unexpected apply plan gate: %#v", plan)
	}
	if !strings.HasPrefix(plan.FromDigest, "sha256:") || !strings.HasPrefix(plan.CandidateDigest, "sha256:") {
		t.Fatalf("plan missing digests: %#v", plan)
	}
	if len(plan.Diff.AddedPacks) != 1 || plan.Diff.AddedPacks[0].ID != PackYunqueWork {
		t.Fatalf("expected yunque work add action diff: %#v", plan.Diff)
	}
	joined := strings.Join(plan.RecommendedActions, "\n")
	for _, want := range []string{"keep rollback bundle", "verify current digest", "add pack \"" + PackYunqueWork + "\"", "write candidate bundle"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("apply actions missing %q: %#v", want, plan.RecommendedActions)
		}
	}
}

func TestPlanPackBundleApplyRequiresReviewForChangedPack(t *testing.T) {
	currentPack := XiaoyuCompanionPack()
	current, err := NewPackBundle("current", []PackManifest{currentPack}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	changed := currentPack
	changed.Version = "0.2.0"
	candidate, err := NewPackBundle("candidate", []PackManifest{changed}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}
	plan, err := PlanPackBundleApply(t.Context(), current, candidate)
	if err != nil {
		t.Fatalf("plan apply: %v", err)
	}
	if plan.Outcome != PackBundleReviewReview || !plan.RequiresReview || plan.Blocked {
		t.Fatalf("expected review gate: %#v", plan)
	}
	if !strings.Contains(strings.Join(plan.RecommendedActions, "\n"), "require a human or policy approval") {
		t.Fatalf("missing approval action: %#v", plan.RecommendedActions)
	}
}

func TestPlanPackBundleApplyBlocksGoldenFailure(t *testing.T) {
	badPack := PackManifest{
		ID:      "bad-pack",
		Version: "0.1.0",
		Type:    "cogni",
		GoldenTests: []GoldenTest{{
			Name:       "bad expectation",
			Input:      "hello",
			ExpectMode: "impossible_mode",
		}},
	}
	current, err := NewPackBundle("current", nil, nil)
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	candidate, err := NewPackBundle("candidate", []PackManifest{badPack}, []string{"bad-pack"})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}
	plan, err := PlanPackBundleApply(t.Context(), current, candidate)
	if err != nil {
		t.Fatalf("plan apply: %v", err)
	}
	if plan.Outcome != PackBundleReviewBlocked || !plan.Blocked {
		t.Fatalf("expected blocked plan: %#v", plan)
	}
	if !strings.Contains(strings.Join(plan.RecommendedActions, "\n"), "do not promote") {
		t.Fatalf("missing do-not-promote action: %#v", plan.RecommendedActions)
	}
}

func TestRenderPackBundleApplyPlanMarkdown(t *testing.T) {
	plan := PackBundleApplyPlan{
		FromID:             "current",
		CandidateID:        "candidate",
		FromDigest:         "sha256:current",
		CandidateDigest:    "sha256:candidate",
		Outcome:            PackBundleReviewReview,
		Reason:             "needs approval",
		RequiresReview:     true,
		RollbackBundleID:   "current",
		RecommendedActions: []string{"require a human or policy approval before writing the candidate bundle"},
		Diff:               PackBundleDiff{FromID: "current", ToID: "candidate"},
		GoldenTests:        GoldenTestSummary{Passed: 1},
	}
	markdown := RenderPackBundleApplyPlanMarkdown(plan)
	for _, want := range []string{"Cogni Pack Bundle Apply Plan", "requires_review: true", "current_digest: sha256:current", "Recommended Actions", "Cogni Pack Bundle Diff", "Cogni Pack Golden Tests"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("apply plan markdown missing %q:\n%s", want, markdown)
		}
	}
}

func hasApplyAction(actions []PackBundleApplyAction, kind PackBundleApplyActionKind) bool {
	for _, action := range actions {
		if action.Kind == kind {
			return true
		}
	}
	return false
}

func hasPackApplyAction(actions []PackBundleApplyAction, kind PackBundleApplyActionKind, packID string) bool {
	for _, action := range actions {
		if action.Kind == kind && action.PackID == packID {
			return true
		}
	}
	return false
}
