package cognisdk

import (
	"context"
	"testing"
)

func TestBuildFeedbackProposalAddsSoftPreferenceWithoutMutatingResult(t *testing.T) {
	engine := NewEngine(Config{Packs: []PackManifest{{
		ID:      "test-soft-pack",
		Version: "0.1.0",
		Type:    "cogni",
		BeliefSeeds: []BeliefNode{{
			ID:         "pref.rollback_checklist",
			Kind:       BeliefPreference,
			Statement:  "发布任务前先列可回滚清单",
			Confidence: 0.6,
		}},
	}}})
	result := engine.Evaluate(context.Background(), Input{Message: "请先帮我整理发布清单"})
	before := len(result.InnerState.ActiveBeliefs)

	proposal := BuildFeedbackProposal(result, AuditFeedback{
		ID:       "fb-pref-1",
		Kind:     FeedbackPreference,
		Message:  "以后做发布任务时，先给我一份可回滚清单。",
		Evidence: []string{"用户明确偏好发布前检查"},
	})

	if proposal.Outcome != FeedbackOutcomeProposed {
		t.Fatalf("outcome = %q, want %q: %#v", proposal.Outcome, FeedbackOutcomeProposed, proposal.Proposals)
	}
	if len(proposal.Proposals) == 0 {
		t.Fatal("expected at least one proposal")
	}
	for _, item := range proposal.Proposals {
		if item.RequiresReview || item.ReadOnlyTarget {
			t.Fatalf("soft work-pack preference should not require review: %#v", item)
		}
		if item.SourceFeedbackID != "fb-pref-1" {
			t.Fatalf("source feedback id not preserved: %#v", item)
		}
	}
	if got := len(result.InnerState.ActiveBeliefs); got != before {
		t.Fatalf("BuildFeedbackProposal mutated result beliefs: got %d, before %d", got, before)
	}
}

func TestFeedbackProposalRequiresReviewForBoundaryFeedback(t *testing.T) {
	engine := NewEngine(Config{})
	result := engine.Evaluate(context.Background(), Input{Message: "你会永远陪我吗？"})

	proposal := engine.ProposeUpdates(context.Background(), result, AuditFeedback{
		ID:              "fb-boundary-1",
		Kind:            FeedbackBoundaryViolation,
		Severity:        FeedbackSeverityHigh,
		Message:         "这里不能为了安慰而承诺永远在线。",
		TargetBeliefIDs: []string{"xy.boundary.no_forever_promise"},
	})

	if proposal.Outcome != FeedbackOutcomeReviewRequired {
		t.Fatalf("outcome = %q, want %q", proposal.Outcome, FeedbackOutcomeReviewRequired)
	}
	if len(proposal.Proposals) != 1 {
		t.Fatalf("expected one targeted proposal, got %#v", proposal.Proposals)
	}
	item := proposal.Proposals[0]
	if item.Action != BeliefUpdateReviewOnly {
		t.Fatalf("action = %q, want review_only", item.Action)
	}
	if !item.RequiresReview || !item.ReadOnlyTarget {
		t.Fatalf("boundary proposal must be read-only review: %#v", item)
	}
	if item.ConfidenceDelta != 0 {
		t.Fatalf("boundary review must not change confidence automatically: %#v", item)
	}
}

func TestFeedbackProposalDoesNotAutoWeakenRootValueBoundary(t *testing.T) {
	engine := NewEngine(Config{})
	result := engine.Evaluate(context.Background(), Input{Message: "我很不安"})

	proposal := BuildFeedbackProposal(result, AuditFeedback{
		ID:              "fb-reject-value",
		Kind:            FeedbackRejection,
		Message:         "这条价值表达不合适。",
		TargetBeliefIDs: []string{"xy.value.honest_comfort"},
	})

	if proposal.Outcome != FeedbackOutcomeReviewRequired {
		t.Fatalf("outcome = %q, want review_required", proposal.Outcome)
	}
	if len(proposal.Proposals) != 1 {
		t.Fatalf("expected one proposal, got %#v", proposal.Proposals)
	}
	if proposal.Proposals[0].Action != BeliefUpdateReviewOnly || proposal.Proposals[0].ConfidenceDelta != 0 {
		t.Fatalf("durable value should only produce review-only proposal: %#v", proposal.Proposals[0])
	}
}

func TestFeedbackProposalIgnoresEmptyFeedback(t *testing.T) {
	proposal := BuildFeedbackProposal(Result{}, AuditFeedback{ID: "empty", Kind: FeedbackPreference})
	if proposal.Outcome != FeedbackOutcomeNoAction {
		t.Fatalf("outcome = %q, want no_action", proposal.Outcome)
	}
	if len(proposal.Proposals) != 0 {
		t.Fatalf("empty feedback should not create proposals: %#v", proposal.Proposals)
	}
}
