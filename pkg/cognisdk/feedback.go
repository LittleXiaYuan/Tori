package cognisdk

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ProposeUpdates converts post-turn audit feedback into non-mutating belief
// update proposals. Hosts remain responsible for persistence, approval, and
// applying any proposal to Memory/Ledger/Pack state.
func (e *Engine) ProposeUpdates(ctx context.Context, result Result, feedback AuditFeedback) FeedbackProposal {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		now := time.Now().UTC()
		return FeedbackProposal{
			ID:      feedbackProposalID(feedback, "cancelled"),
			Time:    now,
			Outcome: FeedbackOutcomeNoAction,
			Summary: ctx.Err().Error(),
			AuditEvents: []AuditEvent{{
				Time:    now,
				Type:    "belief.feedback.cancelled",
				Message: ctx.Err().Error(),
			}},
		}
	default:
	}
	return BuildFeedbackProposal(result, feedback)
}

// BuildFeedbackProposal is a pure helper suitable for SDK callers that do not
// need an Engine instance. It never mutates Result, PackManager, or host state.
func BuildFeedbackProposal(result Result, feedback AuditFeedback) FeedbackProposal {
	now := feedback.Time
	if now.IsZero() {
		now = time.Now().UTC()
	}
	feedback = normalizeFeedback(feedback)
	proposal := FeedbackProposal{
		ID:      feedbackProposalID(feedback, result.InnerState.Intent),
		Time:    now,
		Outcome: FeedbackOutcomeNoAction,
		Summary: "no durable belief update proposed",
	}

	message := strings.TrimSpace(feedback.Message)
	if message == "" && len(feedback.Evidence) == 0 {
		proposal.AuditEvents = []AuditEvent{feedbackAuditEvent(now, "belief.feedback.ignored", "empty feedback ignored", feedback, proposal.Outcome)}
		return proposal
	}

	beliefs := selectFeedbackTargets(result.InnerState.ActiveBeliefs, feedback.TargetBeliefIDs)
	switch feedback.Kind {
	case FeedbackBoundaryViolation:
		proposal.Proposals = proposeReviewForTargets(feedback, beliefs, "boundary feedback may affect durable constraints")
		if len(proposal.Proposals) == 0 {
			proposal.Proposals = []BeliefUpdateProposal{newBeliefProposal(feedback, BeliefUpdateReviewOnly, "", BeliefBoundary, feedbackStatement(feedback), 0, "boundary feedback requires host review", true, false)}
		}
	case FeedbackPreference, FeedbackCorrection:
		proposal.Proposals = proposePreferenceOrCorrection(feedback, beliefs)
	case FeedbackPraise:
		proposal.Proposals = proposeConfidenceShift(feedback, beliefs, BeliefUpdateReinforce, 0.05, "positive feedback reinforces a soft belief")
	case FeedbackRejection:
		proposal.Proposals = proposeConfidenceShift(feedback, beliefs, BeliefUpdateWeaken, -0.05, "negative feedback weakens a soft belief")
	default:
		proposal.Proposals = []BeliefUpdateProposal{newBeliefProposal(feedback, BeliefUpdateReviewOnly, "", BeliefPreference, feedbackStatement(feedback), 0, "unknown feedback kind requires host review", true, false)}
	}

	proposal.Outcome = feedbackOutcome(proposal.Proposals)
	proposal.Summary = feedbackSummary(proposal.Outcome, len(proposal.Proposals))
	proposal.AuditEvents = []AuditEvent{feedbackAuditEvent(now, "belief.update_proposed", proposal.Summary, feedback, proposal.Outcome)}
	return proposal
}

func normalizeFeedback(feedback AuditFeedback) AuditFeedback {
	if feedback.Kind == "" {
		feedback.Kind = FeedbackCorrection
	}
	if feedback.Severity == "" {
		feedback.Severity = FeedbackSeverityLow
	}
	return feedback
}

func selectFeedbackTargets(active []BeliefNode, ids []string) []BeliefNode {
	if len(active) == 0 {
		return nil
	}
	if len(ids) == 0 {
		out := make([]BeliefNode, 0, len(active))
		for _, belief := range active {
			if !isDurableBoundaryKind(belief.Kind) {
				out = append(out, belief)
			}
		}
		if len(out) > 0 {
			return out
		}
		return append([]BeliefNode(nil), active...)
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[strings.TrimSpace(id)] = true
	}
	out := make([]BeliefNode, 0, len(ids))
	for _, belief := range active {
		if want[belief.ID] {
			out = append(out, belief)
		}
	}
	return out
}

func proposeReviewForTargets(feedback AuditFeedback, beliefs []BeliefNode, reason string) []BeliefUpdateProposal {
	out := make([]BeliefUpdateProposal, 0, len(beliefs))
	for _, belief := range beliefs {
		out = append(out, newBeliefProposal(feedback, BeliefUpdateReviewOnly, belief.ID, belief.Kind, belief.Statement, 0, reason, true, isReadOnlyBelief(belief)))
	}
	return out
}

func proposePreferenceOrCorrection(feedback AuditFeedback, beliefs []BeliefNode) []BeliefUpdateProposal {
	if len(beliefs) == 0 {
		return []BeliefUpdateProposal{newBeliefProposal(feedback, BeliefUpdateAddPreference, "", BeliefPreference, feedbackStatement(feedback), 0.3, "feedback can be stored as a new soft preference", feedback.Severity == FeedbackSeverityHigh, false)}
	}
	out := make([]BeliefUpdateProposal, 0, len(beliefs))
	for _, belief := range beliefs {
		if isReadOnlyBelief(belief) || feedback.Severity == FeedbackSeverityHigh {
			out = append(out, newBeliefProposal(feedback, BeliefUpdateReviewOnly, belief.ID, belief.Kind, belief.Statement, 0, "feedback targets a durable belief and requires review", true, isReadOnlyBelief(belief)))
			continue
		}
		action := BeliefUpdateReinforce
		delta := 0.05
		reason := "feedback reinforces an existing soft belief"
		if feedback.Kind == FeedbackCorrection {
			action = BeliefUpdateWeaken
			delta = -0.05
			reason = "correction weakens an existing soft belief before host review"
		}
		out = append(out, newBeliefProposal(feedback, action, belief.ID, belief.Kind, belief.Statement, delta, reason, false, false))
	}
	return out
}

func proposeConfidenceShift(feedback AuditFeedback, beliefs []BeliefNode, action BeliefUpdateAction, delta float64, reason string) []BeliefUpdateProposal {
	out := make([]BeliefUpdateProposal, 0, len(beliefs))
	for _, belief := range beliefs {
		if isReadOnlyBelief(belief) {
			out = append(out, newBeliefProposal(feedback, BeliefUpdateReviewOnly, belief.ID, belief.Kind, belief.Statement, 0, "feedback targets a durable belief and requires review", true, true))
			continue
		}
		out = append(out, newBeliefProposal(feedback, action, belief.ID, belief.Kind, belief.Statement, delta, reason, false, false))
	}
	return out
}

func newBeliefProposal(feedback AuditFeedback, action BeliefUpdateAction, beliefID string, kind BeliefKind, statement string, delta float64, reason string, requiresReview, readOnly bool) BeliefUpdateProposal {
	if strings.TrimSpace(statement) == "" {
		statement = feedbackStatement(feedback)
	}
	p := BeliefUpdateProposal{
		Action:           action,
		BeliefID:         beliefID,
		Kind:             kind,
		Statement:        statement,
		ConfidenceDelta:  delta,
		Reason:           reason,
		RequiresReview:   requiresReview,
		ReadOnlyTarget:   readOnly,
		SourceFeedbackID: feedback.ID,
		Evidence:         append([]string(nil), feedback.Evidence...),
	}
	p.ID = beliefProposalID(feedback, p)
	return p
}

func feedbackStatement(feedback AuditFeedback) string {
	parts := make([]string, 0, 1+len(feedback.Evidence))
	if strings.TrimSpace(feedback.Message) != "" {
		parts = append(parts, strings.TrimSpace(feedback.Message))
	}
	for _, evidence := range feedback.Evidence {
		if strings.TrimSpace(evidence) != "" {
			parts = append(parts, strings.TrimSpace(evidence))
		}
	}
	return strings.Join(parts, "\n")
}

func feedbackOutcome(proposals []BeliefUpdateProposal) FeedbackOutcome {
	if len(proposals) == 0 {
		return FeedbackOutcomeNoAction
	}
	for _, proposal := range proposals {
		if proposal.RequiresReview || proposal.Action == BeliefUpdateReviewOnly {
			return FeedbackOutcomeReviewRequired
		}
	}
	return FeedbackOutcomeProposed
}

func feedbackSummary(outcome FeedbackOutcome, count int) string {
	switch outcome {
	case FeedbackOutcomeReviewRequired:
		return fmt.Sprintf("%d belief update proposal(s) require host review", count)
	case FeedbackOutcomeProposed:
		return fmt.Sprintf("%d belief update proposal(s) ready for host policy", count)
	default:
		return "no durable belief update proposed"
	}
}

func feedbackAuditEvent(now time.Time, typ, message string, feedback AuditFeedback, outcome FeedbackOutcome) AuditEvent {
	return AuditEvent{
		Time:    now,
		Type:    typ,
		Message: message,
		Metadata: map[string]string{
			"feedback_id": feedback.ID,
			"kind":        string(feedback.Kind),
			"severity":    string(feedback.Severity),
			"outcome":     string(outcome),
		},
	}
}

func isReadOnlyBelief(belief BeliefNode) bool {
	return belief.ReadOnly || isDurableBoundaryKind(belief.Kind)
}

func isDurableBoundaryKind(kind BeliefKind) bool {
	return kind == BeliefRoot || kind == BeliefValue || kind == BeliefBoundary
}

func feedbackProposalID(feedback AuditFeedback, salt string) string {
	return "fbp_" + stableShortHash(string(feedback.Kind)+"|"+feedback.ID+"|"+feedback.Message+"|"+salt)
}

func beliefProposalID(feedback AuditFeedback, proposal BeliefUpdateProposal) string {
	return "bup_" + stableShortHash(string(feedback.Kind)+"|"+feedback.ID+"|"+proposal.BeliefID+"|"+string(proposal.Action)+"|"+proposal.Statement)
}

func stableShortHash(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:])[:12]
}
