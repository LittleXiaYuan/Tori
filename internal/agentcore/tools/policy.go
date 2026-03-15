package tools

import (
	"fmt"
	"strings"
	"sync"
)

// ──────────────────────────────────────────────
// Tool groups
// ──────────────────────────────────────────────

// Predefined tool groups matching OpenClaw's grouping.
var BuiltinGroups = map[string][]string{
	"group:fs":       {"read", "write", "edit", "apply_patch", "grep", "find", "ls"},
	"group:runtime":  {"exec", "process"},
	"group:web":      {"web_search", "web_fetch", "browser"},
	"group:session":  {"agents_list", "sessions_list", "sessions_history", "sessions_send", "sessions_spawn", "subagents", "session_status"},
	"group:messaging": {"message", "cron", "gateway"},
	"group:media":    {"image", "canvas", "nodes"},
}

// ──────────────────────────────────────────────
// Profile presets
// ──────────────────────────────────────────────

// Profile is a named preset of allowed/denied tools.
type Profile struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Allow       []string `json:"allow"`       // tool names or group:xxx
	Deny        []string `json:"deny"`        // tool names or group:xxx
}

// BuiltinProfiles provides common security presets.
var BuiltinProfiles = map[string]*Profile{
	"minimal": {
		Name:        "minimal",
		Description: "Read-only access, no execution or writing",
		Allow:       []string{"read", "grep", "find", "ls", "web_search", "image"},
		Deny:        []string{"*"},
	},
	"coding": {
		Name:        "coding",
		Description: "Full file system and runtime access for coding tasks",
		Allow:       []string{"group:fs", "group:runtime", "group:web"},
		Deny:        []string{"group:messaging", "gateway"},
	},
	"messaging": {
		Name:        "messaging",
		Description: "Messaging and session management, limited fs",
		Allow:       []string{"read", "ls", "grep", "group:session", "group:messaging", "web_search"},
		Deny:        []string{"exec", "write", "edit"},
	},
	"full": {
		Name:        "full",
		Description: "All tools allowed",
		Allow:       []string{"*"},
		Deny:        []string{},
	},
}

// ──────────────────────────────────────────────
// Rule — a single allow/deny entry
// ──────────────────────────────────────────────

// RuleAction is allow or deny.
type RuleAction string

const (
	RuleAllow RuleAction = "allow"
	RuleDeny  RuleAction = "deny"
)

// Rule represents a single policy rule.
type Rule struct {
	Action  RuleAction `json:"action"`
	Pattern string     `json:"pattern"` // tool name, group:xxx, or *
}

// ──────────────────────────────────────────────
// Policy engine
// ──────────────────────────────────────────────

// Policy evaluates whether a tool invocation is permitted.
type Policy struct {
	mu    sync.RWMutex
	rules []Rule
}

// NewPolicy creates an empty policy (default-deny).
func NewPolicy() *Policy {
	return &Policy{}
}

// NewPolicyFromProfile creates a policy from a builtin profile name.
func NewPolicyFromProfile(name string) (*Policy, error) {
	prof, ok := BuiltinProfiles[name]
	if !ok {
		return nil, fmt.Errorf("policy: unknown profile %q", name)
	}
	return NewPolicyFromRules(prof.Allow, prof.Deny), nil
}

// NewPolicyFromRules creates a policy from allow/deny lists.
func NewPolicyFromRules(allow, deny []string) *Policy {
	p := &Policy{}
	// Deny rules first (lower priority)
	for _, d := range deny {
		p.rules = append(p.rules, Rule{Action: RuleDeny, Pattern: d})
	}
	// Allow rules second (higher priority)
	for _, a := range allow {
		p.rules = append(p.rules, Rule{Action: RuleAllow, Pattern: a})
	}
	return p
}

// AddRule appends a rule. Later rules take priority.
func (p *Policy) AddRule(action RuleAction, pattern string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = append(p.rules, Rule{Action: action, Pattern: pattern})
}

// Check returns true if the tool is allowed.
// Evaluation: last matching rule wins. If no rule matches, default is deny.
func (p *Policy) Check(toolName string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := false // default deny
	for _, rule := range p.rules {
		if matchesRule(rule.Pattern, toolName) {
			result = (rule.Action == RuleAllow)
		}
	}
	return result
}

// Filter returns only the allowed tools from a list.
func (p *Policy) Filter(tools []string) []string {
	var allowed []string
	for _, t := range tools {
		if p.Check(t) {
			allowed = append(allowed, t)
		}
	}
	return allowed
}

// AllowedTools returns all tools that would pass the policy from a given full set.
func (p *Policy) AllowedTools(allTools []string) []string {
	return p.Filter(allTools)
}

// DeniedTools returns all tools that would be denied.
func (p *Policy) DeniedTools(allTools []string) []string {
	var denied []string
	for _, t := range allTools {
		if !p.Check(t) {
			denied = append(denied, t)
		}
	}
	return denied
}

// Rules returns a copy of current rules.
func (p *Policy) Rules() []Rule {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cp := make([]Rule, len(p.rules))
	copy(cp, p.rules)
	return cp
}

// ──────────────────────────────────────────────
// Rule matching
// ──────────────────────────────────────────────

// matchesRule checks if a tool name matches a rule pattern.
func matchesRule(pattern, toolName string) bool {
	// Wildcard
	if pattern == "*" {
		return true
	}

	// Group reference
	if strings.HasPrefix(pattern, "group:") {
		tools, ok := BuiltinGroups[pattern]
		if !ok {
			return false
		}
		for _, t := range tools {
			if t == toolName {
				return true
			}
		}
		return false
	}

	// Glob-like: "exec*" matches "exec", "exec_elevated"
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(toolName, prefix)
	}

	// Exact match
	return pattern == toolName
}

// ──────────────────────────────────────────────
// Expand helpers
// ──────────────────────────────────────────────

// ExpandGroup returns the tool names in a group, or the pattern itself if not a group.
func ExpandGroup(pattern string) []string {
	if tools, ok := BuiltinGroups[pattern]; ok {
		return tools
	}
	return []string{pattern}
}

// ListGroups returns all defined group names.
func ListGroups() []string {
	groups := make([]string, 0, len(BuiltinGroups))
	for g := range BuiltinGroups {
		groups = append(groups, g)
	}
	return groups
}

// ListProfiles returns all builtin profile names.
func ListProfiles() []string {
	names := make([]string, 0, len(BuiltinProfiles))
	for n := range BuiltinProfiles {
		names = append(names, n)
	}
	return names
}
