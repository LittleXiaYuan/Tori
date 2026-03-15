package guardrails

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"yunque-agent/internal/agentcore/audit"
)

// ──────────────────────────────────────────────
// EgressGuard — sanitises agent output before delivery
// Catches leaked secrets, internal prompts, PII echo-back, and other
// data that should never leave the agent boundary.
// ──────────────────────────────────────────────

// EgressGuardConfig defines what to check in outgoing text.
type EgressGuardConfig struct {
	RedactSecrets  bool     // redact patterns that look like API keys / tokens
	RedactPII      bool     // redact PII in output (reuses PIIGuard patterns)
	BlockPatterns  []string // extra literal substrings to block
	MaxOutputLen   int      // max output length (0 = unlimited)
}

// DefaultEgressGuardConfig returns a secure default.
func DefaultEgressGuardConfig() EgressGuardConfig {
	return EgressGuardConfig{
		RedactSecrets: true,
		RedactPII:     true,
		BlockPatterns: []string{
			"system prompt",      // leaking system prompt
			"你的系统提示词",       // Chinese variant
			"OPENAI_API_KEY",     // env var leak
			"LLM_API_KEY",
			"Bearer sk-",         // bearer token
		},
		MaxOutputLen: 0,
	}
}

// EgressGuard inspects and sanitises outgoing text.
type EgressGuard struct {
	config EgressGuardConfig
	audit  *audit.Chain
}

// NewEgressGuard creates an egress guard.
func NewEgressGuard(cfg EgressGuardConfig) *EgressGuard {
	return &EgressGuard{config: cfg}
}

// SetAudit attaches an audit chain for logging.
func (g *EgressGuard) SetAudit(chain *audit.Chain) { g.audit = chain }

func (g *EgressGuard) Name() string { return "egress_guard" }

// secret-like regex: long hex/base64 strings that look like API keys
var secretRegex = regexp.MustCompile(`(sk-[A-Za-z0-9]{20,}|ghp_[A-Za-z0-9]{36,}|[A-Za-z0-9]{32,64})`)

// Check sanitises outgoing text. Redacted output goes in CheckResult.Redacted.
func (g *EgressGuard) Check(_ context.Context, output string) CheckResult {
	result := CheckResult{Passed: true}
	text := output

	// 1. Block patterns (exact substring match, case-insensitive)
	lower := strings.ToLower(text)
	for _, bp := range g.config.BlockPatterns {
		if strings.Contains(lower, strings.ToLower(bp)) {
			result.Passed = false
			result.Blocked = true
			result.Rule = "egress_blocked_pattern"
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("output contains blocked pattern %q", bp))
			g.auditDeny("blocked_pattern", bp)
			return result
		}
	}

	// 2. Redact secrets
	if g.config.RedactSecrets {
		redacted := secretRegex.ReplaceAllStringFunc(text, func(match string) string {
			// Only redact if it's long enough to plausibly be a secret
			if len(match) >= 32 {
				result.Warnings = append(result.Warnings, "potential secret redacted")
				g.auditDeny("secret_redacted", match[:8]+"...")
				return "[REDACTED]"
			}
			return match
		})
		if redacted != text {
			text = redacted
		}
	}

	// 3. Redact PII in output
	if g.config.RedactPII {
		for _, re := range []*regexp.Regexp{emailRegex, phoneRegex, creditCardRegex} {
			if re.MatchString(text) {
				result.Warnings = append(result.Warnings, "PII redacted from output")
				text = re.ReplaceAllString(text, "[REDACTED]")
			}
		}
	}

	// 4. Length check
	if g.config.MaxOutputLen > 0 && len(text) > g.config.MaxOutputLen {
		text = text[:g.config.MaxOutputLen] + "\n...[truncated by egress guard]"
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("output truncated to %d chars", g.config.MaxOutputLen))
	}

	if text != output {
		result.Redacted = text
	}
	return result
}

func (g *EgressGuard) auditDeny(action, detail string) {
	if g.audit != nil {
		g.audit.Append(audit.EventSystem, "egress_guard", "deny:"+action, detail)
	}
}
