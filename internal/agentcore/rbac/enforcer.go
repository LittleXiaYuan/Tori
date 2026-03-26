package rbac

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Enforcer — RBAC permission check engine
//
// Central authority for authorization decisions.
// Maintains an in-memory role→permissions cache for fast lookups.
// ──────────────────────────────────────────────

// Enforcer checks permissions against roles and policies.
type Enforcer struct {
	mu       sync.RWMutex
	roles    map[string]*Role   // roleID → Role
	policies []Policy           // all active policies
}

// NewEnforcer creates a permission enforcer with default roles.
func NewEnforcer() *Enforcer {
	e := &Enforcer{
		roles: make(map[string]*Role),
	}
	for _, r := range DefaultRoles() {
		role := r
		e.roles[r.ID] = &role
	}
	return e
}

// AddRole registers a custom role.
func (e *Enforcer) AddRole(role Role) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if role.CreatedAt.IsZero() {
		role.CreatedAt = time.Now()
	}
	e.roles[role.ID] = &role
}

// RemoveRole removes a non-builtin role.
func (e *Enforcer) RemoveRole(roleID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.roles[roleID]
	if !ok {
		return fmt.Errorf("role %s not found", roleID)
	}
	if r.IsBuiltIn {
		return fmt.Errorf("cannot remove built-in role %s", roleID)
	}
	delete(e.roles, roleID)
	return nil
}

// GetRole returns a role by ID.
func (e *Enforcer) GetRole(roleID string) (*Role, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r, ok := e.roles[roleID]
	return r, ok
}

// ListRoles returns all registered roles.
func (e *Enforcer) ListRoles() []Role {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Role, 0, len(e.roles))
	for _, r := range e.roles {
		out = append(out, *r)
	}
	return out
}

// AssignRole creates a policy binding a subject to a role.
func (e *Enforcer) AssignRole(subjectID, roleID, tenantID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.roles[roleID]; !ok {
		return fmt.Errorf("role %s not found", roleID)
	}
	// Check for duplicate
	for _, p := range e.policies {
		if p.SubjectID == subjectID && p.RoleID == roleID && p.TenantID == tenantID {
			return nil // already assigned
		}
	}
	e.policies = append(e.policies, Policy{
		ID:        fmt.Sprintf("%s:%s:%s", subjectID, roleID, tenantID),
		SubjectID: subjectID,
		RoleID:    roleID,
		TenantID:  tenantID,
		CreatedAt: time.Now(),
	})
	return nil
}

// RevokeRole removes a policy binding.
func (e *Enforcer) RevokeRole(subjectID, roleID, tenantID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	filtered := e.policies[:0]
	for _, p := range e.policies {
		if !(p.SubjectID == subjectID && p.RoleID == roleID && p.TenantID == tenantID) {
			filtered = append(filtered, p)
		}
	}
	e.policies = filtered
}

// Check returns true if the subject has the specified permission.
func (e *Enforcer) Check(subjectID, tenantID string, resource Resource, action Action) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, policy := range e.policies {
		// Match subject
		if policy.SubjectID != subjectID {
			continue
		}
		// Match tenant scope (empty policy tenant = global)
		if policy.TenantID != "" && policy.TenantID != tenantID {
			continue
		}
		// Check expiry
		if policy.ExpiresAt != nil && time.Now().After(*policy.ExpiresAt) {
			continue
		}
		// Check role permissions
		role, ok := e.roles[policy.RoleID]
		if !ok {
			continue
		}
		for _, perm := range role.Permissions {
			if perm.Resource == resource && perm.Action == action {
				return true
			}
		}
	}
	return false
}

// Require checks permission and returns an error if denied.
func (e *Enforcer) Require(subjectID, tenantID string, resource Resource, action Action) error {
	if e.Check(subjectID, tenantID, resource, action) {
		return nil
	}
	slog.Warn("rbac: permission denied",
		"subject", subjectID,
		"tenant", tenantID,
		"resource", resource,
		"action", action,
	)
	return fmt.Errorf("permission denied: %s cannot %s on %s", subjectID, action, resource)
}

// SubjectRoles returns all roles assigned to a subject.
func (e *Enforcer) SubjectRoles(subjectID, tenantID string) []Role {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var roles []Role
	seen := make(map[string]bool)
	for _, p := range e.policies {
		if p.SubjectID != subjectID {
			continue
		}
		if p.TenantID != "" && p.TenantID != tenantID {
			continue
		}
		if seen[p.RoleID] {
			continue
		}
		if role, ok := e.roles[p.RoleID]; ok {
			roles = append(roles, *role)
			seen[p.RoleID] = true
		}
	}
	return roles
}
