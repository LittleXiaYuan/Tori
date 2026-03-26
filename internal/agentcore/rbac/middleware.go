package rbac

import (
	"net/http"
)

// ──────────────────────────────────────────────
// RBAC Middleware — integrates with Gateway
//
// Usage (in route registration):
//
//   mw := rbac.NewMiddleware(enforcer, getSubject)
//   mux.HandleFunc("/v1/settings", mw.Require(ResSettings, ActionWrite, handler))
// ──────────────────────────────────────────────

// SubjectExtractor extracts the subject ID and tenant ID from an HTTP request.
// Typically reads from JWT claims or API key context.
type SubjectExtractor func(r *http.Request) (subjectID, tenantID string)

// Middleware provides HTTP middleware for RBAC checks.
type Middleware struct {
	enforcer  *Enforcer
	getSubject SubjectExtractor
}

// NewMiddleware creates RBAC middleware.
func NewMiddleware(enforcer *Enforcer, getSubject SubjectExtractor) *Middleware {
	return &Middleware{
		enforcer:   enforcer,
		getSubject: getSubject,
	}
}

// Require wraps an HTTP handler with a permission check.
// Returns 403 if the subject lacks the required permission.
func (m *Middleware) Require(resource Resource, action Action, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subjectID, tenantID := m.getSubject(r)
		if subjectID == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if err := m.enforcer.Require(subjectID, tenantID, resource, action); err != nil {
			http.Error(w, `{"error":"forbidden","detail":"`+err.Error()+`"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// RequireAny wraps a handler requiring ANY of the given permissions.
func (m *Middleware) RequireAny(perms []Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subjectID, tenantID := m.getSubject(r)
		if subjectID == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		for _, p := range perms {
			if m.enforcer.Check(subjectID, tenantID, p.Resource, p.Action) {
				next(w, r)
				return
			}
		}
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}
}
