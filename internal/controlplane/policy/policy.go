package policy

import (
	"fmt"
	"strings"
	"sync"
)

// Action represents an API action that can be controlled.
type Action string

const (
	ActionChat       Action = "chat"
	ActionMemoryRead Action = "memory.read"
	ActionMemoryWrite Action = "memory.write"
	ActionSkillExec  Action = "skill.execute"
	ActionBotManage  Action = "bot.manage"
	ActionPersona    Action = "persona"
	ActionInbox      Action = "inbox"
	ActionHeartbeat  Action = "heartbeat"
	ActionSearch     Action = "search"
	ActionUpload     Action = "upload"
	ActionAdmin      Action = "admin"
)

// Role defines a permission level.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleGuest  Role = "guest"
)

// Decision is the result of a policy check.
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// Rule defines what actions a role can perform.
type Rule struct {
	Role    Role     `json:"role"`
	Actions []Action `json:"actions"`
	Deny    []Action `json:"deny,omitempty"`
}

// Engine evaluates access control policies.
type Engine struct {
	mu    sync.RWMutex
	rules map[Role]*Rule
	guest bool // whether guest access is allowed
}

// NewEngine creates a policy engine with default rules.
func NewEngine() *Engine {
	e := &Engine{
		rules: make(map[Role]*Rule),
		guest: false,
	}
	e.loadDefaults()
	return e
}

func (e *Engine) loadDefaults() {
	e.rules[RoleOwner] = &Rule{
		Role: RoleOwner,
		Actions: []Action{
			ActionChat, ActionMemoryRead, ActionMemoryWrite,
			ActionSkillExec, ActionBotManage, ActionPersona,
			ActionInbox, ActionHeartbeat, ActionSearch, ActionUpload, ActionAdmin,
		},
	}
	e.rules[RoleAdmin] = &Rule{
		Role: RoleAdmin,
		Actions: []Action{
			ActionChat, ActionMemoryRead, ActionMemoryWrite,
			ActionSkillExec, ActionPersona, ActionInbox,
			ActionHeartbeat, ActionSearch, ActionUpload,
		},
	}
	e.rules[RoleMember] = &Rule{
		Role: RoleMember,
		Actions: []Action{
			ActionChat, ActionMemoryRead, ActionSkillExec, ActionSearch,
		},
	}
	e.rules[RoleGuest] = &Rule{
		Role:    RoleGuest,
		Actions: []Action{ActionChat},
	}
}

// Check evaluates whether a role can perform an action.
func (e *Engine) Check(role Role, action Action) Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if role == RoleGuest && !e.guest {
		return Decision{Allowed: false, Reason: "guest access disabled"}
	}

	rule, ok := e.rules[role]
	if !ok {
		return Decision{Allowed: false, Reason: fmt.Sprintf("unknown role: %s", role)}
	}

	// Check deny list first
	for _, d := range rule.Deny {
		if d == action {
			return Decision{Allowed: false, Reason: fmt.Sprintf("action %s denied for role %s", action, role)}
		}
	}

	// Check allow list
	for _, a := range rule.Actions {
		if a == action {
			return Decision{Allowed: true}
		}
	}

	return Decision{Allowed: false, Reason: fmt.Sprintf("action %s not permitted for role %s", action, role)}
}

// SetGuestAccess enables or disables guest access.
func (e *Engine) SetGuestAccess(allowed bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.guest = allowed
}

// GuestAccess returns whether guest access is enabled.
func (e *Engine) GuestAccess() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.guest
}

// Grant adds actions to a role.
func (e *Engine) Grant(role Role, actions ...Action) {
	e.mu.Lock()
	defer e.mu.Unlock()
	rule, ok := e.rules[role]
	if !ok {
		rule = &Rule{Role: role}
		e.rules[role] = rule
	}
	for _, a := range actions {
		if !containsAction(rule.Actions, a) {
			rule.Actions = append(rule.Actions, a)
		}
	}
}

// Revoke removes actions from a role.
func (e *Engine) Revoke(role Role, actions ...Action) {
	e.mu.Lock()
	defer e.mu.Unlock()
	rule, ok := e.rules[role]
	if !ok {
		return
	}
	for _, a := range actions {
		rule.Actions = removeAction(rule.Actions, a)
	}
}

// RoleFromString parses a role string.
func RoleFromString(s string) Role {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "owner":
		return RoleOwner
	case "admin":
		return RoleAdmin
	case "member":
		return RoleMember
	case "guest":
		return RoleGuest
	default:
		return RoleGuest
	}
}

func containsAction(list []Action, a Action) bool {
	for _, x := range list {
		if x == a {
			return true
		}
	}
	return false
}

func removeAction(list []Action, a Action) []Action {
	out := make([]Action, 0, len(list))
	for _, x := range list {
		if x != a {
			out = append(out, x)
		}
	}
	return out
}
