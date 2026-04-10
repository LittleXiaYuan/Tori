package approval

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Manager — controls the approval lifecycle
//
// Workflow:
//  1. Agent calls manager.RequestApproval(...)
//  2. Manager creates a pending Request
//  3. Manager notifies all listeners (SSE push, webhook, etc.)
//  4. Human approves/denies via API
//  5. Manager resolves the request and unblocks the caller
// ──────────────────────────────────────────────

// Listener is called when a new approval request is created.
type Listener func(req *Request)

// Manager manages approval requests.
type Manager struct {
	mu        sync.RWMutex
	requests  map[string]*Request      // id → request
	waiters   map[string]chan struct{} // id → signal channel
	listeners []Listener
	policy    Policy
	rules     *RuleStore  // persistent allow/deny rules
}

// NewManager creates an approval manager with the given policy.
func NewManager(policy Policy) *Manager {
	m := &Manager{
		requests: make(map[string]*Request),
		waiters:  make(map[string]chan struct{}),
		policy:   policy,
		rules:    NewRuleStore("data"),
	}
	// Start expiry checker
	go m.expiryLoop()
	return m
}

// NewManagerWithRules creates a manager with a custom rule store.
func NewManagerWithRules(policy Policy, rules *RuleStore) *Manager {
	m := &Manager{
		requests: make(map[string]*Request),
		waiters:  make(map[string]chan struct{}),
		policy:   policy,
		rules:    rules,
	}
	go m.expiryLoop()
	return m
}

// Rules returns the rule store for external management.
func (m *Manager) Rules() *RuleStore { return m.rules }

// OnRequest registers a listener for new approval requests.
func (m *Manager) OnRequest(fn Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, fn)
}

// RequestApproval creates a pending request and blocks until resolved.
// Returns the resolved request (approved/denied/expired).
func (m *Manager) RequestApproval(req *Request) *Request {
	if req.ID == "" {
		req.ID = uuid.New().String()[:8]
	}
	req.Status = StatusPending
	req.CreatedAt = time.Now()
	if req.ExpiresAt.IsZero() {
		req.ExpiresAt = req.CreatedAt.Add(m.policy.DefaultTimeout)
	}

	// ── Phase 0: check persistent rules (allowlist/denylist) ──
	if m.rules != nil {
		skillName, _ := req.Details["skill_name"].(string)
		decision := m.rules.Evaluate(skillName, req.Category, req.Requester, req.TenantID)
		switch decision {
		case DecisionDenyAlways:
			req.Status = StatusDenied
			now := time.Now()
			req.ResolvedAt = &now
			req.Approver = "system:denylist"
			req.Reason = "blocked by persistent deny rule"
			slog.Info("approval: denied by rule",
				"id", req.ID, "skill", skillName)
			return req
		case DecisionAllowAlways:
			req.Status = StatusAutoApproved
			now := time.Now()
			req.ResolvedAt = &now
			req.Approver = "system:allowlist"
			slog.Info("approval: allowed by rule",
				"id", req.ID, "skill", skillName)
			return req
		}
	}

	// ── Phase 0b: shell syntax guard ──
	if cmd, ok := req.Details["command"].(string); ok && cmd != "" {
		shellResult := AnalyzeShellCommand(cmd)
		if shellResult.Risk == ShellCritical {
			req.Status = StatusDenied
			now := time.Now()
			req.ResolvedAt = &now
			req.Approver = "system:shell_guard"
			req.Reason = fmt.Sprintf("critical shell risk: %s", strings.Join(shellResult.Patterns, "; "))
			slog.Warn("approval: shell critical block",
				"id", req.ID, "patterns", shellResult.Patterns)
			return req
		}
		if shellResult.Risk == ShellDanger {
			// Force high risk level for dangerous shell patterns
			req.RiskLevel = RiskHigh
			req.Details["shell_patterns"] = shellResult.Patterns
		}
	}

	// ── Phase 1: check if auto-approve is possible ──
	if m.shouldAutoApprove(req) {
		req.Status = StatusAutoApproved
		now := time.Now()
		req.ResolvedAt = &now
		req.Approver = "system:auto"
		slog.Info("approval: auto-approved",
			"id", req.ID, "risk", req.RiskLevel, "category", req.Category)
		return req
	}

	// Create wait channel
	ch := make(chan struct{})
	m.mu.Lock()
	m.requests[req.ID] = req
	m.waiters[req.ID] = ch
	listeners := make([]Listener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.Unlock()

	slog.Info("approval: request created",
		"id", req.ID, "risk", req.RiskLevel,
		"category", req.Category, "summary", req.Summary)

	// Notify listeners (async)
	for _, fn := range listeners {
		go fn(req)
	}

	// Block until resolved or expired
	<-ch

	m.mu.RLock()
	result := m.requests[req.ID]
	m.mu.RUnlock()

	return result
}

// Approve resolves a request as approved.
// approverTenantID is the tenant of the person approving.
func (m *Manager) Approve(id, approverTenantID string) error {
	return m.resolve(id, StatusApproved, approverTenantID, "")
}

// ApproveWithDecision resolves and optionally persists a rule.
// decision: allow_once (just this time), allow_always (add to allowlist), deny_always (add to denylist).
func (m *Manager) ApproveWithDecision(id, approverTenantID string, decision Decision) error {
	m.mu.RLock()
	req, ok := m.requests[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("approval %s not found", id)
	}

	switch decision {
	case DecisionAllowOnce:
		return m.resolve(id, StatusApproved, approverTenantID, "")

	case DecisionAllowAlways:
		if err := m.resolve(id, StatusApproved, approverTenantID, ""); err != nil {
			return err
		}
		if m.rules != nil {
			skillName, _ := req.Details["skill_name"].(string)
			if skillName != "" {
				m.rules.Add(Rule{
					Pattern:   skillName,
					Action:    DecisionAllowAlways,
					Category:  req.Category,
					Scope:     ScopeGlobal,
					TenantID:  req.TenantID,
					CreatedBy: approverTenantID,
					Reason:    "approved permanently via approval flow",
				})
				slog.Info("approval: added to allowlist",
					"skill", skillName, "by", approverTenantID)
			}
		}
		return nil

	case DecisionDenyAlways:
		if err := m.resolve(id, StatusDenied, approverTenantID, "permanently denied"); err != nil {
			return err
		}
		if m.rules != nil {
			skillName, _ := req.Details["skill_name"].(string)
			if skillName != "" {
				m.rules.Add(Rule{
					Pattern:   skillName,
					Action:    DecisionDenyAlways,
					Category:  req.Category,
					Scope:     ScopeGlobal,
					TenantID:  req.TenantID,
					CreatedBy: approverTenantID,
					Reason:    "denied permanently via approval flow",
				})
				slog.Info("approval: added to denylist",
					"skill", skillName, "by", approverTenantID)
			}
		}
		return nil

	default:
		return fmt.Errorf("unknown decision: %s", decision)
	}
}

// Deny resolves a request as denied.
func (m *Manager) Deny(id, approverTenantID, reason string) error {
	return m.resolve(id, StatusDenied, approverTenantID, reason)
}

// Get returns a request by ID.
func (m *Manager) Get(id string) (*Request, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.requests[id]
	return r, ok
}

// Pending returns all pending approval requests.
func (m *Manager) Pending(tenantID string) []*Request {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*Request
	for _, r := range m.requests {
		if r.Status == StatusPending {
			if tenantID == "" || r.TenantID == tenantID {
				out = append(out, r)
			}
		}
	}
	return out
}

// History returns recent resolved requests.
func (m *Manager) History(tenantID string, limit int) []*Request {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*Request
	for _, r := range m.requests {
		if r.Status != StatusPending {
			if tenantID == "" || r.TenantID == tenantID {
				out = append(out, r)
			}
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// resolve sets the final status and unblocks the waiter.
func (m *Manager) resolve(id string, status Status, approver, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok {
		return fmt.Errorf("approval %s not found", id)
	}
	if req.Status != StatusPending {
		return fmt.Errorf("approval %s already resolved (%s)", id, req.Status)
	}

	// Permission check: approver must belong to the same tenant
	// (empty approver = system/internal, always allowed)
	if approver != "" && req.TenantID != "" && approver != req.TenantID {
		return fmt.Errorf("approval %s: permission denied (tenant mismatch)", id)
	}

	// Expiry check: reject if already expired
	if !req.ExpiresAt.IsZero() && time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		now := time.Now()
		req.ResolvedAt = &now
		if ch, ok := m.waiters[id]; ok {
			close(ch)
			delete(m.waiters, id)
		}
		return fmt.Errorf("approval %s has expired", id)
	}

	now := time.Now()
	req.Status = status
	req.Approver = approver
	req.Reason = reason
	req.ResolvedAt = &now

	slog.Info("approval: resolved",
		"id", id, "status", status,
		"approver", approver, "reason", reason)

	// Unblock waiter
	if ch, ok := m.waiters[id]; ok {
		close(ch)
		delete(m.waiters, id)
	}

	return nil
}

// shouldAutoApprove checks if a request can be auto-approved.
func (m *Manager) shouldAutoApprove(req *Request) bool {
	// Critical risk: never auto-approve
	if req.RiskLevel == RiskCritical {
		return false
	}
	// Always-require categories
	for _, cat := range m.policy.AlwaysRequire {
		if req.Category == cat {
			return false
		}
	}
	// Low risk: always auto-approve
	if req.RiskLevel == RiskLow {
		return true
	}
	// Below policy threshold
	if req.RiskLevel < m.policy.MinRiskLevel {
		return true
	}
	return false
}

// expiryLoop periodically checks for expired requests.
func (m *Manager) expiryLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for id, req := range m.requests {
			if req.Status == StatusPending && now.After(req.ExpiresAt) {
				req.Status = StatusExpired
				req.ResolvedAt = &now
				slog.Warn("approval: expired", "id", id)
				if ch, ok := m.waiters[id]; ok {
					close(ch)
					delete(m.waiters, id)
				}
			}
		}
		m.mu.Unlock()
	}
}
