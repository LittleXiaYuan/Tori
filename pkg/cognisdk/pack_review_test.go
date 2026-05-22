package cognisdk

import (
	"strings"
	"testing"
)

func TestReviewPackBundleCandidateReady(t *testing.T) {
	current, err := NewPackBundle("current", []PackManifest{PersonalCompanionPack()}, []string{PackPersonalCompanion})
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	candidate, err := NewPackBundle("candidate", BuiltinPacks(), []string{PackPersonalCompanion, PackYunqueWork})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}

	review, err := ReviewPackBundleCandidate(t.Context(), current, candidate)
	if err != nil {
		t.Fatalf("review candidate: %v", err)
	}
	if review.Outcome != PackBundleReviewReady {
		t.Fatalf("outcome = %q, want ready: %#v", review.Outcome, review)
	}
	if review.RollbackBundleID != current.ID {
		t.Fatalf("rollback bundle id = %q, want %q", review.RollbackBundleID, current.ID)
	}
	if !strings.HasPrefix(review.FromDigest, "sha256:") || !strings.HasPrefix(review.CandidateDigest, "sha256:") {
		t.Fatalf("review missing bundle digests: %#v", review)
	}
	if review.GoldenTests.Failed != 0 {
		t.Fatalf("expected golden tests to pass: %#v", review.GoldenTests)
	}
}

func TestReviewPackBundleCandidateRequiresReviewForChangedPack(t *testing.T) {
	currentPack := PersonalCompanionPack()
	current, err := NewPackBundle("current", []PackManifest{currentPack}, []string{PackPersonalCompanion})
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	changed := currentPack
	changed.Version = "0.2.0"
	candidate, err := NewPackBundle("candidate", []PackManifest{changed}, []string{PackPersonalCompanion})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}
	review, err := ReviewPackBundleCandidate(t.Context(), current, candidate)
	if err != nil {
		t.Fatalf("review candidate: %v", err)
	}
	if review.Outcome != PackBundleReviewReview {
		t.Fatalf("outcome = %q, want review", review.Outcome)
	}
	if len(review.Diff.ChangedPacks) != 1 {
		t.Fatalf("expected changed pack diff: %#v", review.Diff)
	}
}

func TestReviewPackBundleCandidateBlocksGoldenFailure(t *testing.T) {
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
	review, err := ReviewPackBundleCandidate(t.Context(), current, candidate)
	if err != nil {
		t.Fatalf("review candidate: %v", err)
	}
	if review.Outcome != PackBundleReviewBlocked {
		t.Fatalf("outcome = %q, want blocked", review.Outcome)
	}
	if review.GoldenTests.Failed != 1 {
		t.Fatalf("expected one failed golden test: %#v", review.GoldenTests)
	}
}

func TestRenderPackBundleReviewMarkdown(t *testing.T) {
	review := PackBundleReview{
		FromID:           "current",
		CandidateID:      "candidate",
		FromDigest:       "sha256:current",
		CandidateDigest:  "sha256:candidate",
		Outcome:          PackBundleReviewReady,
		Reason:           "ok",
		RollbackBundleID: "current",
		Diff:             PackBundleDiff{FromID: "current", ToID: "candidate"},
		GoldenTests:      GoldenTestSummary{Passed: 1},
	}
	markdown := RenderPackBundleReviewMarkdown(review)
	for _, want := range []string{"Cogni Pack Bundle Review", "outcome: ready", "current_digest: sha256:current", "candidate_digest: sha256:candidate", "rollback_bundle: current", "Cogni Pack Bundle Diff", "Cogni Pack Golden Tests"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("review markdown missing %q:\n%s", want, markdown)
		}
	}
}
