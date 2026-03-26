package rbac

import (
	"time"
)

// ──────────────────────────────────────────────
// RBAC — Role-Based Access Control
//
// Provides fine-grained permission control for the Agent platform.
// Supports roles, permissions, and policy rules with resource-level
// granularity and action-based authorization.
// ──────────────────────────────────────────────

// Action represents an operation that can be performed on a resource.
type Action string

const (
	ActionRead    Action = "read"
	ActionWrite   Action = "write"
	ActionExecute Action = "execute"
	ActionDelete  Action = "delete"
	ActionAdmin   Action = "admin"
)

// Resource represents a protected entity in the system.
type Resource string

const (
	ResChat       Resource = "chat"
	ResMemory     Resource = "memory"
	ResKnowledge  Resource = "knowledge"
	ResTasks      Resource = "tasks"
	ResWorkflows  Resource = "workflows"
	ResPlugins    Resource = "plugins"
	ResSettings   Resource = "settings"
	ResAudit      Resource = "audit"
	ResTrust      Resource = "trust"
	ResProviders  Resource = "providers"
	ResUsers      Resource = "users"
	ResBilling    Resource = "billing"
)

// Permission is a single action-on-resource grant.
type Permission struct {
	Resource   Resource `json:"resource"`
	Action     Action   `json:"action"`
	Conditions []string `json:"conditions,omitempty"` // optional: "own", "tenant:X"
}

// Role groups permissions under a named identity.
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	IsBuiltIn   bool         `json:"is_built_in"` // cannot be deleted
	CreatedAt   time.Time    `json:"created_at"`
}

// Policy is a rule that associates a subject (user/tenant) with a role.
type Policy struct {
	ID        string    `json:"id"`
	SubjectID string    `json:"subject_id"` // user or tenant ID
	RoleID    string    `json:"role_id"`
	TenantID  string    `json:"tenant_id,omitempty"` // scope to tenant
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ── Built-in roles ──

// DefaultRoles returns the built-in role set.
func DefaultRoles() []Role {
	return []Role{
		{
			ID: "owner", Name: "Owner", Description: "Full access to everything", IsBuiltIn: true,
			Permissions: allPermissions(),
		},
		{
			ID: "admin", Name: "Admin", Description: "Manage settings, users, and plugins", IsBuiltIn: true,
			Permissions: []Permission{
				{ResChat, ActionRead, nil}, {ResChat, ActionWrite, nil}, {ResChat, ActionExecute, nil},
				{ResMemory, ActionRead, nil}, {ResMemory, ActionWrite, nil},
				{ResKnowledge, ActionRead, nil}, {ResKnowledge, ActionWrite, nil},
				{ResTasks, ActionRead, nil}, {ResTasks, ActionWrite, nil}, {ResTasks, ActionExecute, nil},
				{ResWorkflows, ActionRead, nil}, {ResWorkflows, ActionWrite, nil}, {ResWorkflows, ActionExecute, nil},
				{ResPlugins, ActionRead, nil}, {ResPlugins, ActionWrite, nil},
				{ResSettings, ActionRead, nil}, {ResSettings, ActionWrite, nil},
				{ResAudit, ActionRead, nil},
				{ResProviders, ActionRead, nil}, {ResProviders, ActionWrite, nil},
				{ResUsers, ActionRead, nil},
			},
		},
		{
			ID: "operator", Name: "Operator", Description: "Use the agent and manage tasks", IsBuiltIn: true,
			Permissions: []Permission{
				{ResChat, ActionRead, nil}, {ResChat, ActionWrite, nil}, {ResChat, ActionExecute, nil},
				{ResMemory, ActionRead, nil},
				{ResKnowledge, ActionRead, nil},
				{ResTasks, ActionRead, nil}, {ResTasks, ActionWrite, nil}, {ResTasks, ActionExecute, nil},
				{ResWorkflows, ActionRead, nil}, {ResWorkflows, ActionExecute, nil},
				{ResPlugins, ActionRead, nil},
			},
		},
		{
			ID: "viewer", Name: "Viewer", Description: "Read-only access", IsBuiltIn: true,
			Permissions: []Permission{
				{ResChat, ActionRead, nil},
				{ResMemory, ActionRead, nil},
				{ResKnowledge, ActionRead, nil},
				{ResTasks, ActionRead, nil},
				{ResWorkflows, ActionRead, nil},
				{ResPlugins, ActionRead, nil},
				{ResAudit, ActionRead, nil},
			},
		},
	}
}

func allPermissions() []Permission {
	resources := []Resource{
		ResChat, ResMemory, ResKnowledge, ResTasks, ResWorkflows,
		ResPlugins, ResSettings, ResAudit, ResTrust, ResProviders,
		ResUsers, ResBilling,
	}
	actions := []Action{ActionRead, ActionWrite, ActionExecute, ActionDelete, ActionAdmin}
	var perms []Permission
	for _, r := range resources {
		for _, a := range actions {
			perms = append(perms, Permission{Resource: r, Action: a})
		}
	}
	return perms
}
