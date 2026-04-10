package review

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// LLMReviewDetailFunc does a detailed LLM review with structured output.
type LLMReviewDetailFunc func(ctx context.Context, system, prompt string) (string, error)

// DetailedVerdict has richer information than basic Verdict.
type DetailedVerdict struct {
	Allowed       bool     `json:"allowed"`
	Risk          Risk     `json:"risk"`
	Reason        string   `json:"reason"`
	Concerns      []string `json:"concerns,omitempty"`
	Mitigations   []string `json:"mitigations,omitempty"`
	ReviewedBy    string   `json:"reviewed_by"` // "auto" | "llm" | "user"
	ReviewLatency time.Duration `json:"-"`
}

// IntelligentGate extends Gate with LLM-powered contextual review.
type IntelligentGate struct {
	gate           *Gate
	llmDetail      LLMReviewDetailFunc
	mu             sync.Mutex
	auditLog       []AuditEntry
	maxAuditLog    int
	blockedPatterns map[string]int // operation → blocked count
}

// AuditEntry records a review decision for auditing.
type AuditEntry struct {
	Operation string          `json:"operation"`
	Detail    string          `json:"detail"`
	Verdict   DetailedVerdict `json:"verdict"`
	Timestamp time.Time       `json:"timestamp"`
	TenantID  string          `json:"tenant_id"`
}

// NewIntelligentGate creates an intelligent review gate.
func NewIntelligentGate() *IntelligentGate {
	return &IntelligentGate{
		gate:           NewGate(),
		maxAuditLog:    1000,
		blockedPatterns: make(map[string]int),
	}
}

// SetLLMDetail attaches the detailed LLM review function.
func (ig *IntelligentGate) SetLLMDetail(fn LLMReviewDetailFunc) {
	ig.llmDetail = fn
}

// SetLLMReview delegates to the basic gate.
func (ig *IntelligentGate) SetLLMReview(fn LLMReviewFunc) {
	ig.gate.SetLLMReview(fn)
}

// ReviewDetailed performs a context-aware review with LLM analysis.
func (ig *IntelligentGate) ReviewDetailed(ctx context.Context, operation, detail, tenantID string) DetailedVerdict {
	start := time.Now()
	risk := ClassifyRisk(operation)

	var verdict DetailedVerdict
	switch risk {
	case RiskLow:
		verdict = DetailedVerdict{
			Allowed: true, Risk: RiskLow,
			Reason: "low risk, auto-approved", ReviewedBy: "auto",
		}

	case RiskMed:
		// 中风险：快速 LLM 审查
		basic := ig.gate.Review(ctx, operation, detail)
		verdict = DetailedVerdict{
			Allowed: basic.Allowed, Risk: basic.Risk,
			Reason: basic.Reason, ReviewedBy: "llm",
		}

	case RiskHigh:
		// 高风险：详细 LLM 分析
		if ig.llmDetail != nil {
			verdict = ig.deepReview(ctx, operation, detail)
		} else {
			verdict = DetailedVerdict{
				Allowed: false, Risk: RiskHigh,
				Reason: "high risk, requires user confirmation",
				ReviewedBy: "auto",
			}
		}
	}

	verdict.ReviewLatency = time.Since(start)

	// 记录审计日志
	ig.mu.Lock()
	ig.auditLog = append(ig.auditLog, AuditEntry{
		Operation: operation, Detail: truncateStr(detail, 200),
		Verdict: verdict, Timestamp: time.Now(), TenantID: tenantID,
	})
	if len(ig.auditLog) > ig.maxAuditLog {
		ig.auditLog = ig.auditLog[len(ig.auditLog)-ig.maxAuditLog:]
	}
	if !verdict.Allowed {
		ig.blockedPatterns[operation]++
	}
	ig.mu.Unlock()

	return verdict
}

// deepReview uses LLM for thorough risk analysis of high-risk operations.
func (ig *IntelligentGate) deepReview(ctx context.Context, operation, detail string) DetailedVerdict {
	system := `You are a security review agent. Analyze the operation for risks.
Output JSON:
{"allowed":true|false,"reason":"...","concerns":["concern1"],"mitigations":["mitigation1"]}`

	prompt := fmt.Sprintf("Operation: %s\nDetails: %s\n\nAnalyze: Is this safe? Should it be allowed?",
		operation, truncateStr(detail, 500))

	reply, err := ig.llmDetail(ctx, system, prompt)
	if err != nil {
		slog.Warn("review: deep review failed", "err", err)
		return DetailedVerdict{
			Allowed: false, Risk: RiskHigh,
			Reason:  "review failed, blocking for safety: " + err.Error(),
			ReviewedBy: "auto",
		}
	}

	var parsed struct {
		Allowed     bool     `json:"allowed"`
		Reason      string   `json:"reason"`
		Concerns    []string `json:"concerns"`
		Mitigations []string `json:"mitigations"`
	}
	if err := json.Unmarshal([]byte(extractJSONStr(reply)), &parsed); err != nil {
		return DetailedVerdict{
			Allowed: false, Risk: RiskHigh,
			Reason:  "review parse failed, blocking",
			ReviewedBy: "auto",
		}
	}

	return DetailedVerdict{
		Allowed:     parsed.Allowed,
		Risk:        RiskHigh,
		Reason:      parsed.Reason,
		Concerns:    parsed.Concerns,
		Mitigations: parsed.Mitigations,
		ReviewedBy:  "llm",
	}
}

// AuditLog returns recent review decisions.
func (ig *IntelligentGate) AuditLog(limit int) []AuditEntry {
	ig.mu.Lock()
	defer ig.mu.Unlock()

	if limit <= 0 || limit > len(ig.auditLog) {
		limit = len(ig.auditLog)
	}
	start := len(ig.auditLog) - limit
	result := make([]AuditEntry, limit)
	copy(result, ig.auditLog[start:])
	return result
}

// BlockedStats returns operation → blocked count mapping.
func (ig *IntelligentGate) BlockedStats() map[string]int {
	ig.mu.Lock()
	defer ig.mu.Unlock()
	cp := make(map[string]int, len(ig.blockedPatterns))
	for k, v := range ig.blockedPatterns {
		cp[k] = v
	}
	return cp
}

func extractJSONStr(s string) string {
	start := -1
	for i, c := range s {
		if c == '{' {
			start = i
			break
		}
	}
	if start < 0 {
		return s
	}
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}
