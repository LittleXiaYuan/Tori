package guardrails

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"yunque-agent/internal/agentcore/audit"
)

// ──────────────────────────────────────────────
// ToolGuard — validates tool/skill call parameters before execution
// Prevents path traversal, command injection, and unauthorized API access.
// ──────────────────────────────────────────────

// ToolGuardConfig defines whitelists for tool calls.
type ToolGuardConfig struct {
	AllowedCommands []string // e.g. ["ls", "cat", "python3"]
	AllowedPaths    []string // e.g. ["/data", "/tmp"] (prefix match)
	BlockedParams   []string // patterns that must never appear in any param value
}

// DefaultToolGuardConfig returns a safe default config.
func DefaultToolGuardConfig() ToolGuardConfig {
	return ToolGuardConfig{
		AllowedCommands: []string{
			"echo", "cat", "head", "tail", "wc", "sort", "grep",
			"find", "ls", "dir", "type", "python3", "python", "node",
		},
		AllowedPaths: []string{"/data", "/tmp", "data/"},
		BlockedParams: []string{
			"../",       // path traversal
			"..\\",      // path traversal (windows)
			"; rm ",     // command chaining
			"| rm ",     // pipe to rm
			"$(", "`",   // command substitution
			"&&", "||",  // shell operators
		},
	}
}

// ToolGuard validates tool call parameters against whitelists.
type ToolGuard struct {
	config ToolGuardConfig
	audit  *audit.Chain
}

// NewToolGuard creates a tool guard with the given config.
func NewToolGuard(cfg ToolGuardConfig) *ToolGuard {
	return &ToolGuard{config: cfg}
}

// SetAudit attaches an audit chain for logging denials.
func (g *ToolGuard) SetAudit(chain *audit.Chain) { g.audit = chain }

func (g *ToolGuard) Name() string { return "tool_guard" }

// ToolCallInput represents a tool call to be validated.
type ToolCallInput struct {
	SkillName string
	Command   string            // for command-execution skills
	Params    map[string]string // flattened key=value of all parameters
}

// CheckToolCall validates a tool call against the guard's whitelist config.
func (g *ToolGuard) CheckToolCall(_ context.Context, input ToolCallInput) CheckResult {
	result := CheckResult{Passed: true}

	// 1. Command whitelist
	if input.Command != "" && len(g.config.AllowedCommands) > 0 {
		baseCmd := baseCommand(input.Command)
		allowed := false
		for _, a := range g.config.AllowedCommands {
			if strings.EqualFold(baseCmd, a) {
				allowed = true
				break
			}
		}
		if !allowed {
			result.Passed = false
			result.Blocked = true
			result.Rule = "tool_command_blocked"
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("command %q not in allowlist", baseCmd))
			g.auditDeny("command_blocked", input.SkillName, input.Command)
			return result
		}
	}

	// 2. Parameter value checks
	for key, val := range input.Params {
		// Block dangerous patterns
		valLower := strings.ToLower(val)
		for _, blocked := range g.config.BlockedParams {
			if strings.Contains(valLower, strings.ToLower(blocked)) {
				result.Passed = false
				result.Blocked = true
				result.Rule = "tool_param_blocked"
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("param %q contains blocked pattern %q", key, blocked))
				g.auditDeny("param_blocked", input.SkillName,
					fmt.Sprintf("%s=%s (pattern: %s)", key, val, blocked))
				return result
			}
		}

		// Path validation for path-like parameters
		if isPathParam(key) {
			if !g.isPathAllowed(val) {
				result.Passed = false
				result.Blocked = true
				result.Rule = "tool_path_blocked"
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("path %q not in allowed paths", val))
				g.auditDeny("path_blocked", input.SkillName, val)
				return result
			}
		}
	}

	return result
}

// Check implements Guard interface for pipeline compatibility (checks raw text).
func (g *ToolGuard) Check(_ context.Context, _ string) CheckResult {
	return CheckResult{Passed: true} // text-level check is a no-op; use CheckToolCall
}

func (g *ToolGuard) isPathAllowed(path string) bool {
	if len(g.config.AllowedPaths) == 0 {
		return true // no restriction
	}
	cleaned := filepath.Clean(path)
	for _, allowed := range g.config.AllowedPaths {
		if strings.HasPrefix(strings.ToLower(cleaned), strings.ToLower(allowed)) {
			return true
		}
	}
	return false
}

func (g *ToolGuard) auditDeny(action, skill, detail string) {
	if g.audit != nil {
		g.audit.Append(audit.EventToolCall, "tool_guard",
			fmt.Sprintf("deny:%s:%s", action, skill), detail)
	}
}

// baseCommand extracts the base command name from a full command string.
func baseCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	return filepath.Base(parts[0])
}

// isPathParam heuristically determines if a parameter key represents a path.
func isPathParam(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "path") ||
		strings.Contains(lower, "file") ||
		strings.Contains(lower, "dir") ||
		strings.Contains(lower, "folder")
}
