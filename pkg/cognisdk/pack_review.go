package cognisdk

import (
	"context"
	"fmt"
	"strings"
)

// ReviewPackBundleCandidate validates a candidate bundle, compares it with the
// current rollback snapshot, and runs candidate golden tests. It does not load
// the candidate into any host state.
func ReviewPackBundleCandidate(ctx context.Context, current, candidate PackBundle) (PackBundleReview, error) {
	diff, err := DiffPackBundles(current, candidate)
	if err != nil {
		return PackBundleReview{}, err
	}
	golden, err := RunPackBundleGoldenTests(ctx, candidate)
	if err != nil {
		return PackBundleReview{}, err
	}
	fromDigest, err := DigestPackBundle(current)
	if err != nil {
		return PackBundleReview{}, err
	}
	candidateDigest, err := DigestPackBundle(candidate)
	if err != nil {
		return PackBundleReview{}, err
	}
	review := PackBundleReview{
		FromID:           current.ID,
		CandidateID:      candidate.ID,
		FromDigest:       fromDigest,
		CandidateDigest:  candidateDigest,
		RollbackBundleID: current.ID,
		Diff:             diff,
		GoldenTests:      golden,
	}
	review.Outcome, review.Reason = classifyPackBundleReview(diff, golden)
	return review, nil
}

// RenderPackBundleReviewMarkdown renders the complete candidate review for a
// UI preview, plugin page, or CI log. Rendering is non-mutating.
func RenderPackBundleReviewMarkdown(review PackBundleReview) string {
	var b strings.Builder
	b.WriteString("## Cogni Pack Bundle Review\n\n")
	fmt.Fprintf(&b, "- current: %s\n", emptyAs(review.FromID, "unknown"))
	fmt.Fprintf(&b, "- candidate: %s\n", emptyAs(review.CandidateID, "unknown"))
	if review.FromDigest != "" {
		fmt.Fprintf(&b, "- current_digest: %s\n", review.FromDigest)
	}
	if review.CandidateDigest != "" {
		fmt.Fprintf(&b, "- candidate_digest: %s\n", review.CandidateDigest)
	}
	fmt.Fprintf(&b, "- outcome: %s\n", emptyAs(string(review.Outcome), string(PackBundleReviewReview)))
	if review.RollbackBundleID != "" {
		fmt.Fprintf(&b, "- rollback_bundle: %s\n", review.RollbackBundleID)
	}
	if review.Reason != "" {
		fmt.Fprintf(&b, "- reason: %s\n", review.Reason)
	}
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(RenderPackBundleDiffMarkdown(review.Diff)))
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(RenderGoldenTestSummaryMarkdown(review.GoldenTests)))
	b.WriteString("\n")
	return b.String()
}

func classifyPackBundleReview(diff PackBundleDiff, golden GoldenTestSummary) (PackBundleReviewOutcome, string) {
	if golden.Failed > 0 {
		return PackBundleReviewBlocked, "candidate bundle failed golden tests"
	}
	if len(diff.RemovedPacks) > 0 || len(diff.ChangedPacks) > 0 || len(diff.DisabledPacks) > 0 {
		return PackBundleReviewReview, "candidate changes existing pack surface and should be reviewed"
	}
	return PackBundleReviewReady, "candidate only adds or enables packs and passed golden tests"
}
