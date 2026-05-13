package cognisdk

import (
	"fmt"
	"strings"
)

// RenderFeedbackProposalMarkdown renders feedback proposals for review panels,
// CLI output, or plugin-side audit logs. It is presentation-only and does not
// imply that a proposal has been applied.
func RenderFeedbackProposalMarkdown(proposal FeedbackProposal) string {
	var b strings.Builder

	b.WriteString("## 反馈提案\n\n")
	fmt.Fprintf(&b, "- id: %s\n", emptyAs(proposal.ID, "unknown"))
	fmt.Fprintf(&b, "- outcome: %s\n", emptyAs(string(proposal.Outcome), string(FeedbackOutcomeNoAction)))
	if !proposal.Time.IsZero() {
		fmt.Fprintf(&b, "- time: %s\n", proposal.Time.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if strings.TrimSpace(proposal.Summary) != "" {
		fmt.Fprintf(&b, "- summary: %s\n", proposal.Summary)
	}

	if len(proposal.Proposals) > 0 {
		b.WriteString("\n### Belief Update Proposals\n")
		for _, item := range proposal.Proposals {
			fmt.Fprintf(&b, "- `%s` %s", emptyAs(item.ID, "proposal"), emptyAs(string(item.Action), string(BeliefUpdateReviewOnly)))
			if item.BeliefID != "" {
				fmt.Fprintf(&b, " -> `%s`", item.BeliefID)
			}
			if item.Kind != "" {
				fmt.Fprintf(&b, " (%s)", item.Kind)
			}
			b.WriteString("\n")
			if item.Statement != "" {
				fmt.Fprintf(&b, "  - statement: %s\n", oneLine(item.Statement))
			}
			if item.ConfidenceDelta != 0 {
				fmt.Fprintf(&b, "  - confidence_delta: %.2f\n", item.ConfidenceDelta)
			}
			if item.RequiresReview {
				b.WriteString("  - requires_review: true\n")
			}
			if item.ReadOnlyTarget {
				b.WriteString("  - read_only_target: true\n")
			}
			if item.Reason != "" {
				fmt.Fprintf(&b, "  - reason: %s\n", item.Reason)
			}
		}
	}

	if len(proposal.AuditEvents) > 0 {
		b.WriteString("\n### Audit Events\n")
		for _, event := range proposal.AuditEvents {
			fmt.Fprintf(&b, "- %s: %s\n", emptyAs(event.Type, "event"), event.Message)
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
