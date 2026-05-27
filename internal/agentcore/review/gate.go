package review

import (
	"context"
	"fmt"
	"strings"

	"yunque-agent/pkg/risk"
)

// Risk classifies operation risk.
type Risk = risk.Level

const (
	RiskLow  Risk = risk.Low    // query, translate, chat → pass through
	RiskMed  Risk = risk.Medium // write file, call installed skill → quick LLM review
	RiskHigh Risk = risk.High   // install skill, modify persona, shell, network → full review + user confirm
)

// Verdict is the result of a review.
type Verdict struct {
	Allowed bool   `json:"allowed"`
	Risk    Risk   `json:"risk"`
	Reason  string `json:"reason,omitempty"`
}

// LLMReviewFunc is a quick LLM review call (typically Fast tier, ~10 tokens).
type LLMReviewFunc func(ctx context.Context, operation string) (bool, error)

// Gate implements risk-graded review for operations.
type Gate struct {
	llmReview LLMReviewFunc
}

// NewGate creates a review gate.
func NewGate() *Gate {
	return &Gate{}
}

// SetLLMReview attaches the quick LLM review function.
func (g *Gate) SetLLMReview(fn LLMReviewFunc) { g.llmReview = fn }

// ClassifyRisk determines the risk level of an operation.
func ClassifyRisk(operation string) Risk {
	op := strings.ToLower(operation)

	// High risk operations
	highRisk := []string{
		"install_skill", "uninstall_skill",
		"modify_persona", "set_persona",
		"shell", "exec", "run_command",
		"network_request", "http_post",
		"delete_file", "delete_memory",
		"self_iterate", "approve_proposal",
	}
	for _, hr := range highRisk {
		if strings.Contains(op, hr) {
			return RiskHigh
		}
	}

	// Medium risk operations
	medRisk := []string{
		"write_file", "create_file",
		"add_memory", "edit_memory",
		"use_skill", "call_skill",
		"send_message",
	}
	for _, mr := range medRisk {
		if strings.Contains(op, mr) {
			return RiskMed
		}
	}

	return RiskLow
}

// Review evaluates an operation and returns a verdict.
// Low risk: instant pass. Medium: quick LLM check. High: needs user confirmation.
func (g *Gate) Review(ctx context.Context, operation, detail string) Verdict {
	risk := ClassifyRisk(operation)

	switch risk {
	case RiskLow:
		return Verdict{Allowed: true, Risk: RiskLow, Reason: "low risk, auto-approved"}

	case RiskMed:
		if g.llmReview != nil {
			desc := fmt.Sprintf("%s: %s", operation, truncateStr(detail, 200))
			allowed, err := g.llmReview(ctx, desc)
			if err != nil {
				// On error, allow medium risk (fail-open for usability)
				return Verdict{Allowed: true, Risk: RiskMed, Reason: "review error, default allow: " + err.Error()}
			}
			reason := "approved by quick review"
			if !allowed {
				reason = "rejected by quick review"
			}
			return Verdict{Allowed: allowed, Risk: RiskMed, Reason: reason}
		}
		// No LLM configured: allow medium risk
		return Verdict{Allowed: true, Risk: RiskMed, Reason: "no reviewer configured, default allow"}

	case RiskHigh:
		return Verdict{Allowed: false, Risk: RiskHigh, Reason: "high risk, requires user confirmation"}
	}

	return Verdict{Allowed: true, Risk: RiskLow}
}

func truncateStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
