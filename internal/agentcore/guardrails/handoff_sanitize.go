package guardrails

import (
	"context"
	"fmt"
	"log/slog"
)

// SanitizeHandoffInput runs handoff input through the unified sanitizer before
// it reaches the subagent planner (#39).
//
// handoff input comes from LLM tool-call args (handoffInputFromArgs), which is
// attacker-controllable via prompt injection — sanitize it as untrusted
// tool-return content (full SQL/XSS/cmd-injection/path/null-byte + length
// checks). Returns:
//   - (cleanInput, nil) when sanitizer is nil (defense-in-depth, not a hard
//     gate — fall back to prior behavior), or when the sanitizer passed and
//     produced no redacted form.
//   - (redactedInput, nil) when the sanitizer passed but redacted null bytes
//     or other cleanable content — the redacted form replaces the input.
//   - ("", err) when the sanitizer blocked the input — handoff is aborted.
//
// agentName is logged on block for audit triage but does not affect the
// sanitization result.
func SanitizeHandoffInput(ctx context.Context, sanitizer *Sanitizer, agentName, input string) (string, error) {
	if sanitizer == nil {
		return input, nil
	}
	sr := sanitizer.Sanitize(ctx, SanitizeRequest{
		Input:  input,
		Source: SourceToolReturn,
	})
	if sr.Blocked {
		slog.Warn("handoff: input blocked by sanitizer",
			"agent", agentName, "rule", sr.Rule, "threat", sr.ThreatType)
		return "", fmt.Errorf("handoff input rejected by sanitizer: %s (%s)", sr.Rule, sr.ThreatType)
	}
	if sr.Sanitized != "" {
		return sr.Sanitized, nil
	}
	return input, nil
}
