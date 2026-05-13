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
	actions := packBundleApplyActions(review)
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
		RecommendedActions: packBundleApplyActionMessages(actions),
		Actions:            actions,
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

func packBundleApplyActions(review PackBundleReview) []PackBundleApplyAction {
	actions := []PackBundleApplyAction{
		{
			Kind:     PackBundleApplyActionKeepRollback,
			BundleID: emptyAs(review.RollbackBundleID, review.FromID),
			Message:  fmt.Sprintf("keep rollback bundle %q available until the candidate is verified", emptyAs(review.RollbackBundleID, review.FromID)),
		},
		{
			Kind:    PackBundleApplyActionVerifyDigest,
			Digest:  emptyAs(review.FromDigest, "unknown"),
			Message: fmt.Sprintf("verify current digest %s before replacing the active bundle", emptyAs(review.FromDigest, "unknown")),
		},
	}
	if review.Outcome == PackBundleReviewBlocked {
		return append(actions, PackBundleApplyAction{
			Kind:    PackBundleApplyActionStopBlocked,
			Message: "do not promote the candidate bundle because the review is blocked",
		})
	}
	if review.Outcome == PackBundleReviewReview {
		actions = append(actions, PackBundleApplyAction{
			Kind:    PackBundleApplyActionRequireReview,
			Message: "require a human or policy approval before writing the candidate bundle",
		})
	}
	for _, pack := range review.Diff.AddedPacks {
		actions = append(actions, PackBundleApplyAction{
			Kind:      PackBundleApplyActionAddPack,
			PackID:    pack.ID,
			ToVersion: pack.Version,
			Message:   fmt.Sprintf("add pack %q", pack.ID),
		})
	}
	for _, change := range review.Diff.ChangedPacks {
		actions = append(actions, PackBundleApplyAction{
			Kind:        PackBundleApplyActionReplacePack,
			PackID:      change.ID,
			FromVersion: change.FromVersion,
			ToVersion:   change.ToVersion,
			Message:     fmt.Sprintf("replace pack %q (%s -> %s)", change.ID, emptyAs(change.FromVersion, "unknown"), emptyAs(change.ToVersion, "unknown")),
		})
	}
	for _, pack := range review.Diff.RemovedPacks {
		actions = append(actions, PackBundleApplyAction{
			Kind:        PackBundleApplyActionRemovePack,
			PackID:      pack.ID,
			FromVersion: pack.Version,
			Message:     fmt.Sprintf("remove pack %q", pack.ID),
		})
	}
	for _, id := range review.Diff.EnabledPacks {
		actions = append(actions, PackBundleApplyAction{
			Kind:    PackBundleApplyActionEnablePack,
			PackID:  id,
			Message: fmt.Sprintf("enable pack %q", id),
		})
	}
	for _, id := range review.Diff.DisabledPacks {
		actions = append(actions, PackBundleApplyAction{
			Kind:    PackBundleApplyActionDisablePack,
			PackID:  id,
			Message: fmt.Sprintf("disable pack %q", id),
		})
	}
	if len(review.Diff.AddedPacks) == 0 && len(review.Diff.ChangedPacks) == 0 && len(review.Diff.RemovedPacks) == 0 && len(review.Diff.EnabledPacks) == 0 && len(review.Diff.DisabledPacks) == 0 {
		actions = append(actions, PackBundleApplyAction{
			Kind:    PackBundleApplyActionNoop,
			Message: "no pack changes detected; keep the active bundle unchanged",
		})
	} else {
		actions = append(actions, PackBundleApplyAction{
			Kind:     PackBundleApplyActionWriteCandidate,
			BundleID: review.CandidateID,
			Message:  fmt.Sprintf("write candidate bundle %q only after the above gates pass", review.CandidateID),
		})
	}
	return actions
}

func packBundleApplyActionMessages(actions []PackBundleApplyAction) []string {
	messages := make([]string, 0, len(actions))
	for _, action := range actions {
		if strings.TrimSpace(action.Message) != "" {
			messages = append(messages, action.Message)
		}
	}
	return messages
}

// PackBundleApplyActionKinds returns the stable public action-kind vocabulary
// emitted by PlanPackBundleApply. Callers may use it to populate plugin forms,
// validate frontend filters, or generate automation allowlists without copying
// constants by hand.
func PackBundleApplyActionKinds() []PackBundleApplyActionKind {
	return []PackBundleApplyActionKind{
		PackBundleApplyActionKeepRollback,
		PackBundleApplyActionVerifyDigest,
		PackBundleApplyActionRequireReview,
		PackBundleApplyActionStopBlocked,
		PackBundleApplyActionAddPack,
		PackBundleApplyActionReplacePack,
		PackBundleApplyActionRemovePack,
		PackBundleApplyActionEnablePack,
		PackBundleApplyActionDisablePack,
		PackBundleApplyActionNoop,
		PackBundleApplyActionWriteCandidate,
	}
}

// PackBundleApplyActionKindInfos returns UI/plugin-friendly labels and
// descriptions for the stable public apply action vocabulary. It intentionally
// mirrors PackBundleApplyActionKinds one-for-one so existing callers can keep
// using the compact string list while richer consumers avoid hard-coded copy.
func PackBundleApplyActionKindInfos() []PackBundleApplyActionKindInfo {
	return []PackBundleApplyActionKindInfo{
		{
			Kind:        PackBundleApplyActionKeepRollback,
			Label:       "Keep rollback bundle",
			Description: "Keep the current bundle available as a rollback source until the candidate is verified.",
		},
		{
			Kind:        PackBundleApplyActionVerifyDigest,
			Label:       "Verify digest",
			Description: "Check the expected bundle digest before replacing or writing bundle artifacts.",
		},
		{
			Kind:        PackBundleApplyActionRequireReview,
			Label:       "Require review",
			Description: "Pause promotion until a human reviewer or host policy approves the candidate.",
		},
		{
			Kind:        PackBundleApplyActionStopBlocked,
			Label:       "Stop blocked candidate",
			Description: "Stop promotion because the candidate failed a blocking review or golden-test gate.",
		},
		{
			Kind:        PackBundleApplyActionAddPack,
			Label:       "Add pack",
			Description: "Add a pack that exists in the candidate bundle but not in the current bundle.",
		},
		{
			Kind:        PackBundleApplyActionReplacePack,
			Label:       "Replace pack",
			Description: "Replace an existing pack with a changed candidate version.",
		},
		{
			Kind:        PackBundleApplyActionRemovePack,
			Label:       "Remove pack",
			Description: "Remove a pack that is no longer present in the candidate bundle.",
		},
		{
			Kind:        PackBundleApplyActionEnablePack,
			Label:       "Enable pack",
			Description: "Enable a pack that is present but newly activated by the candidate bundle.",
		},
		{
			Kind:        PackBundleApplyActionDisablePack,
			Label:       "Disable pack",
			Description: "Disable a pack that remains present but is no longer active in the candidate bundle.",
		},
		{
			Kind:        PackBundleApplyActionNoop,
			Label:       "No change",
			Description: "Report that no pack changes were detected and the active bundle can remain unchanged.",
		},
		{
			Kind:        PackBundleApplyActionWriteCandidate,
			Label:       "Write candidate",
			Description: "Write the candidate bundle only after digest, review, and rollback gates pass.",
		},
	}
}

// DescribePackBundleApplyActionKind returns UI/plugin metadata for one action
// kind. It is a convenience lookup for hosts that validate a CLI flag,
// populate a single settings row, or render contextual help next to a filtered
// action list without keeping their own copy of the vocabulary.
func DescribePackBundleApplyActionKind(kind PackBundleApplyActionKind) (PackBundleApplyActionKindInfo, bool) {
	for _, info := range PackBundleApplyActionKindInfos() {
		if info.Kind == kind {
			return info, true
		}
	}
	return PackBundleApplyActionKindInfo{}, false
}

// BuildPackBundleApplyChecklist converts a dry-run apply plan into rows that
// external installers, plugin UIs, and CI dashboards can render directly. The
// checklist is descriptive only: it never writes files or marks actions done.
func BuildPackBundleApplyChecklist(plan PackBundleApplyPlan) []PackBundleApplyChecklistItem {
	items := make([]PackBundleApplyChecklistItem, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		info, ok := DescribePackBundleApplyActionKind(action.Kind)
		if !ok {
			info = PackBundleApplyActionKindInfo{
				Kind:        action.Kind,
				Label:       string(action.Kind),
				Description: "Unknown apply action kind.",
			}
		}
		actionCopy := action
		items = append(items, PackBundleApplyChecklistItem{
			Kind:        action.Kind,
			Label:       info.Label,
			Description: info.Description,
			Required:    applyActionKindRequired(action.Kind),
			Done:        false,
			Blocked:     action.Kind == PackBundleApplyActionStopBlocked || plan.Blocked,
			Message:     action.Message,
			Action:      &actionCopy,
			Info:        info,
		})
	}
	return items
}

func applyActionKindRequired(kind PackBundleApplyActionKind) bool {
	switch kind {
	case PackBundleApplyActionKeepRollback,
		PackBundleApplyActionVerifyDigest,
		PackBundleApplyActionRequireReview,
		PackBundleApplyActionStopBlocked,
		PackBundleApplyActionWriteCandidate:
		return true
	default:
		return false
	}
}

// FilterPackBundleApplyActions returns only actions whose Kind matches one of
// the requested kinds. Passing no kinds returns the original action slice. The
// helper is intentionally non-mutating so external installers, plugin hooks,
// and automation scripts can safely derive their owned action view from a full
// apply plan.
func FilterPackBundleApplyActions(actions []PackBundleApplyAction, kinds ...PackBundleApplyActionKind) []PackBundleApplyAction {
	if len(kinds) == 0 {
		return actions
	}
	allowed := make(map[PackBundleApplyActionKind]bool, len(kinds))
	for _, kind := range kinds {
		allowed[kind] = true
	}
	filtered := make([]PackBundleApplyAction, 0, len(actions))
	for _, action := range actions {
		if allowed[action.Kind] {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

// KnownPackBundleApplyActionKind reports whether kind is part of the stable
// public action vocabulary emitted by PlanPackBundleApply.
func KnownPackBundleApplyActionKind(kind PackBundleApplyActionKind) bool {
	for _, known := range PackBundleApplyActionKinds() {
		if kind == known {
			return true
		}
	}
	return false
}
