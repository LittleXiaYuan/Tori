package guardrails

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/internal/agentcore/audit"

	"gopkg.in/yaml.v3"
)

// ToolGuardAction defines what to do when a rule matches.
type ToolGuardAction string

const (
	ActionAllow           ToolGuardAction = "ALLOW"
	ActionBlock           ToolGuardAction = "BLOCK"
	ActionRequireApproval ToolGuardAction = "REQUIRE_APPROVAL"
)

// ToolGuardRule is a declarative rule loaded from YAML.
type ToolGuardRule struct {
	Tool    string          `yaml:"tool" json:"tool"`
	Pattern string          `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Action  ToolGuardAction `yaml:"action" json:"action"`
	Reason  string          `yaml:"reason,omitempty" json:"reason,omitempty"`
}

// ToolGuardConfig supports both legacy fields and declarative rules.
type ToolGuardConfig struct {
	AllowedCommands []string        `yaml:"allowed_commands,omitempty" json:"allowed_commands,omitempty"`
	AllowedPaths    []string        `yaml:"allowed_paths,omitempty" json:"allowed_paths,omitempty"`
	BlockedParams   []string        `yaml:"blocked_params,omitempty" json:"blocked_params,omitempty"`
	Rules           []ToolGuardRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

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
		Rules: []ToolGuardRule{
			{Tool: "code_exec", Action: ActionRequireApproval, Reason: "code execution requires approval"},
		},
	}
}

// LoadToolGuardConfig loads config from a YAML file, falling back to defaults.
func LoadToolGuardConfig(path string) ToolGuardConfig {
	if path == "" {
		path = "data/tool-guard.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultToolGuardConfig()
	}
	var cfg ToolGuardConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		slog.Warn("tool_guard: failed to parse YAML config, using defaults", "err", err)
		return DefaultToolGuardConfig()
	}
	if len(cfg.AllowedCommands) == 0 && len(cfg.Rules) == 0 {
		def := DefaultToolGuardConfig()
		cfg.AllowedCommands = def.AllowedCommands
		cfg.AllowedPaths = def.AllowedPaths
		cfg.BlockedParams = def.BlockedParams
	}
	slog.Info("tool_guard: loaded config from YAML", "path", path, "rules", len(cfg.Rules))
	return cfg
}

type ToolGuard struct {
	config ToolGuardConfig
	audit  *audit.Chain
}

func NewToolGuard(cfg ToolGuardConfig) *ToolGuard {
	return &ToolGuard{config: cfg}
}

func (g *ToolGuard) SetAudit(chain *audit.Chain) { g.audit = chain }

func (g *ToolGuard) Name() string { return "tool_guard" }

type ToolCallInput struct {
	SkillName string
	Command   string            // for command-execution skills
	Params    map[string]string // flattened key=value of all parameters
}

// CheckToolCall validates params against the allowlist and declarative rules.
func (g *ToolGuard) CheckToolCall(_ context.Context, input ToolCallInput) CheckResult {
	result := CheckResult{Passed: true}

	// 0. Declarative rules (highest priority)
	for _, rule := range g.config.Rules {
		if !matchToolName(rule.Tool, input.SkillName) {
			continue
		}
		if rule.Pattern != "" {
			matched := false
			for _, val := range input.Params {
				if matchPattern(rule.Pattern, val) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		switch rule.Action {
		case ActionBlock:
			result.Passed = false
			result.Blocked = true
			result.Rule = "declarative_block"
			reason := rule.Reason
			if reason == "" {
				reason = fmt.Sprintf("tool %q blocked by rule", input.SkillName)
			}
			result.Warnings = append(result.Warnings, reason)
			g.auditDeny("rule_block", input.SkillName, reason)
			return result
		case ActionRequireApproval:
			result.NeedsApproval = true
			reason := rule.Reason
			if reason == "" {
				reason = fmt.Sprintf("tool %q requires approval", input.SkillName)
			}
			result.Warnings = append(result.Warnings, reason)
		case ActionAllow:
			return result
		}
	}

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

// Check is a no-op for raw text; use CheckToolCall for structured validation.
func (g *ToolGuard) Check(_ context.Context, _ string) CheckResult {
	return CheckResult{Passed: true} // text-level check is a no-op; use CheckToolCall
}

func (g *ToolGuard) isPathAllowed(path string) bool {
	if len(g.config.AllowedPaths) == 0 {
		return true // no restriction
	}
	cleaned := filepath.Clean(path)
	cleanedLower := strings.ToLower(cleaned)
	for _, allowed := range g.config.AllowedPaths {
		allowedClean := strings.ToLower(filepath.Clean(allowed))
		if cleanedLower == allowedClean ||
			strings.HasPrefix(cleanedLower, allowedClean+string(filepath.Separator)) ||
			strings.HasPrefix(cleanedLower, allowedClean+"/") {
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

// baseCommand extracts the binary name from a command string (e.g. "/usr/bin/python3 foo" -> "python3").
func baseCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	return filepath.Base(parts[0])
}

// isPathParam guesses whether a parameter key refers to a file path.
func isPathParam(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "path") ||
		strings.Contains(lower, "file") ||
		strings.Contains(lower, "dir") ||
		strings.Contains(lower, "folder")
}

// matchToolName checks if a rule's tool pattern matches a skill name.
// Supports * prefix/suffix wildcards (e.g. "*DeleteTool" matches "FileDeleteTool").
func matchToolName(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	patLower := strings.ToLower(pattern)
	nameLower := strings.ToLower(name)
	if strings.HasPrefix(patLower, "*") && strings.HasSuffix(patLower, "*") {
		return strings.Contains(nameLower, strings.Trim(patLower, "*"))
	}
	if strings.HasPrefix(patLower, "*") {
		return strings.HasSuffix(nameLower, strings.TrimPrefix(patLower, "*"))
	}
	if strings.HasSuffix(patLower, "*") {
		return strings.HasPrefix(nameLower, strings.TrimSuffix(patLower, "*"))
	}
	return strings.EqualFold(pattern, name)
}

// matchPattern checks if a value matches a glob-like pattern (e.g. "*.env").
func matchPattern(pattern, value string) bool {
	patLower := strings.ToLower(pattern)
	valLower := strings.ToLower(value)
	if strings.HasPrefix(patLower, "*") {
		return strings.HasSuffix(valLower, strings.TrimPrefix(patLower, "*"))
	}
	if strings.HasSuffix(patLower, "*") {
		return strings.HasPrefix(valLower, strings.TrimSuffix(patLower, "*"))
	}
	return strings.Contains(valLower, patLower)
}
