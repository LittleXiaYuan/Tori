package cognisdk

import (
	"context"
	"fmt"
	"strings"
)

// PlanPackBundleApply returns a complete, non-mutating plan for moving from a
// current bundle snapshot to a candidate bundle. It does not write files, load
// packs into a host, or change a PackManager.
func PlanPackBundleApply(ctx context.Context, current, candidate PackBundle) (PackBundleApplyPlan, error) {
	review, err := ReviewPackBundleCandidate(ctx, current, candidate)
	if err != nil {
		return PackBundleApplyPlan{}, err
	}
	plan := PackBundleApplyPlan{
		FromID:             review.FromID,
		CandidateID:        review.CandidateID,
		FromDigest:         review.FromDigest,
		CandidateDigest:    review.CandidateDigest,
		Outcome:            review.Outcome,
		Reason:             review.Reason,
		RequiresReview:     review.Outcome == PackBundleReviewReview,
		Blocked:            review.Outcome == PackBundleReviewBlocked,
		RollbackBundleID:   review.RollbackBundleID,
		RecommendedActions: packBundleApplyActions(review),
		Diff:               review.Diff,
		GoldenTests:        review.GoldenTests,
	}
	return plan, nil
}

// RenderPackBundleApplyPlanMarkdown renders a dry-run apply plan for release
// notes, plugin previews, and CI logs. Rendering is non-mutating.
func RenderPackBundleApplyPlanMarkdown(plan PackBundleApplyPlan) string {
	var b strings.Builder
	b.WriteString("## Cogni Pack Bundle Apply Plan\n\n")
	fmt.Fprintf(&b, "- current: %s\n", emptyAs(plan.FromID, "unknown"))
	fmt.Fprintf(&b, "- candidate: %s\n", emptyAs(plan.CandidateID, "unknown"))
	if plan.FromDigest != "" {
		fmt.Fprintf(&b, "- current_digest: %s\n", plan.FromDigest)
	}
	if plan.CandidateDigest != "" {
		fmt.Fprintf(&b, "- candidate_digest: %s\n", plan.CandidateDigest)
	}
	fmt.Fprintf(&b, "- outcome: %s\n", emptyAs(string(plan.Outcome), string(PackBundleReviewReview)))
	fmt.Fprintf(&b, "- requires_review: %t\n", plan.RequiresReview)
	fmt.Fprintf(&b, "- blocked: %t\n", plan.Blocked)
	if plan.RollbackBundleID != "" {
		fmt.Fprintf(&b, "- rollback_bundle: %s\n", plan.RollbackBundleID)
	}
	if plan.Reason != "" {
		fmt.Fprintf(&b, "- reason: %s\n", plan.Reason)
	}
	writeList(&b, "Recommended Actions", plan.RecommendedActions)
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(RenderPackBundleDiffMarkdown(plan.Diff)))
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(RenderGoldenTestSummaryMarkdown(plan.GoldenTests)))
	b.WriteString("\n")
	return b.String()
}

func packBundleApplyActions(review PackBundleReview) []string {
	actions := []string{
		fmt.Sprintf("keep rollback bundle %q available until the candidate is verified", emptyAs(review.RollbackBundleID, review.FromID)),
		fmt.Sprintf("verify current digest %s before replacing the active bundle", emptyAs(review.FromDigest, "unknown")),
	}
	if review.Outcome == PackBundleReviewBlocked {
		return append(actions, "do not promote the candidate bundle because the review is blocked")
	}
	if review.Outcome == PackBundleReviewReview {
		actions = append(actions, "require a human or policy approval before writing the candidate bundle")
	}
	for _, pack := range review.Diff.AddedPacks {
		actions = append(actions, fmt.Sprintf("add pack %q", pack.ID))
	}
	for _, change := range review.Diff.ChangedPacks {
		actions = append(actions, fmt.Sprintf("replace pack %q (%s -> %s)", change.ID, emptyAs(change.FromVersion, "unknown"), emptyAs(change.ToVersion, "unknown")))
	}
	for _, pack := range review.Diff.RemovedPacks {
		actions = append(actions, fmt.Sprintf("remove pack %q", pack.ID))
	}
	for _, id := range review.Diff.EnabledPacks {
		actions = append(actions, fmt.Sprintf("enable pack %q", id))
	}
	for _, id := range review.Diff.DisabledPacks {
		actions = append(actions, fmt.Sprintf("disable pack %q", id))
	}
	if len(review.Diff.AddedPacks) == 0 && len(review.Diff.ChangedPacks) == 0 && len(review.Diff.RemovedPacks) == 0 && len(review.Diff.EnabledPacks) == 0 && len(review.Diff.DisabledPacks) == 0 {
		actions = append(actions, "no pack changes detected; keep the active bundle unchanged")
	} else {
		actions = append(actions, fmt.Sprintf("write candidate bundle %q only after the above gates pass", review.CandidateID))
	}
	return actions
}
