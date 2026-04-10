package approval

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// kvStore abstracts Ledger KV to avoid import cycles with internal/ledger.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// ──────────────────────────────────────────────
// Rules Engine — allowlist / denylist / permanent decisions
//
// Supports three decision modes:
//   - AllowOnce:       skip approval this time only
//   - AllowAlways:     add to allowlist, never ask again
//   - DenyAlways:      add to denylist, always block
//
// Rules are persisted to JSON and survive restarts.
// ──────────────────────────────────────────────

// Decision represents a human's decision on a past approval.
type Decision string

const (
	DecisionAllowOnce   Decision = "allow_once"   // one-time pass
	DecisionAllowAlways Decision = "allow_always" // persist to allowlist
	DecisionDenyAlways  Decision = "deny_always"  // persist to denylist
)

// Rule is a persisted allow/deny entry.
type Rule struct {
	ID        string   `json:"id"`
	Pattern   string   `json:"pattern"`             // glob or exact skill name
	Action    Decision `json:"action"`              // allow_always | deny_always
	Category  Category `json:"category,omitempty"`  // optional: limit to category
	Scope     Scope    `json:"scope"`               // user | session | global
	UserID    string   `json:"user_id,omitempty"`   // for user-scope rules
	TenantID  string   `json:"tenant_id,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	CreatedBy string   `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// Scope determines rule granularity.
type Scope string

const (
	ScopeGlobal  Scope = "global"  // applies to all users/sessions
	ScopeUser    Scope = "user"    // applies to one user
	ScopeSession Scope = "session" // only current session (not persisted)
)

// RuleStore manages persistent allow/deny rules.
type RuleStore struct {
	mu      sync.RWMutex
	rules   []Rule
	path string  // legacy JSON persistence path
	kvs  kvStore // Ledger KV (preferred when set)
	session []Rule                 // session-scoped (not persisted)
}

// NewRuleStore creates a rule store with JSON persistence.
func NewRuleStore(dataDir string) *RuleStore {
	rs := &RuleStore{
		path: filepath.Join(dataDir, "approval_rules.json"),
	}
	rs.load()
	return rs
}

// SetKVStore enables Ledger KV-backed persistence, replacing file I/O.
func (rs *RuleStore) SetKVStore(kvs kvStore) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.kvs = kvs
	rs.loadFromKV()
}

// Add persists a new rule.
func (rs *RuleStore) Add(rule Rule) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}
	rule.CreatedAt = time.Now()

	if rule.Scope == ScopeSession {
		rs.session = append(rs.session, rule)
		return
	}

	// Remove conflicting rules for same pattern+scope+user
	rs.rules = filterRules(rs.rules, func(r Rule) bool {
		return !(r.Pattern == rule.Pattern && r.Scope == rule.Scope && r.UserID == rule.UserID)
	})
	rs.rules = append(rs.rules, rule)
	rs.save()
}

// Remove deletes a rule by ID.
func (rs *RuleStore) Remove(id string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	before := len(rs.rules)
	rs.rules = filterRules(rs.rules, func(r Rule) bool { return r.ID != id })
	rs.session = filterRules(rs.session, func(r Rule) bool { return r.ID != id })

	if len(rs.rules) < before {
		rs.save()
		return true
	}
	return len(rs.rules)+len(rs.session) < before+len(rs.session)
}

// Evaluate checks if a skill call matches an existing rule.
// Returns: Decision (allow/deny) or empty string if no rule matched.
// Priority: session > user > global; deny > allow.
func (rs *RuleStore) Evaluate(skillName string, category Category, userID, tenantID string) Decision {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	allRules := append(rs.session, rs.rules...)

	// Phase 1: check deny rules (deny always wins)
	for _, r := range allRules {
		if r.Action != DecisionDenyAlways {
			continue
		}
		if !rs.ruleMatches(r, skillName, category, userID, tenantID) {
			continue
		}
		slog.Info("approval_rules: deny match",
			"rule_id", r.ID, "pattern", r.Pattern, "skill", skillName)
		return DecisionDenyAlways
	}

	// Phase 2: check allow rules (session > user > global)
	for _, scope := range []Scope{ScopeSession, ScopeUser, ScopeGlobal} {
		for _, r := range allRules {
			if r.Action != DecisionAllowAlways || r.Scope != scope {
				continue
			}
			if !rs.ruleMatches(r, skillName, category, userID, tenantID) {
				continue
			}
			slog.Debug("approval_rules: allow match",
				"rule_id", r.ID, "pattern", r.Pattern, "scope", scope)
			return DecisionAllowAlways
		}
	}

	return "" // no rule matched
}

// List returns all rules, optionally filtered by tenant.
func (rs *RuleStore) List(tenantID string) []Rule {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	var out []Rule
	for _, r := range append(rs.rules, rs.session...) {
		if tenantID == "" || r.TenantID == tenantID {
			out = append(out, r)
		}
	}
	return out
}

// ClearSession removes all session-scoped rules.
func (rs *RuleStore) ClearSession() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.session = nil
}

// ── internal helpers ──

func (rs *RuleStore) ruleMatches(r Rule, skillName string, cat Category, userID, tenantID string) bool {
	// Tenant check
	if r.TenantID != "" && r.TenantID != tenantID {
		return false
	}
	// User scope check
	if r.Scope == ScopeUser && r.UserID != userID {
		return false
	}
	// Category check (empty = match all)
	if r.Category != "" && r.Category != cat {
		return false
	}
	// Pattern matching (support * glob)
	return matchGlob(r.Pattern, skillName)
}

func matchGlob(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		// Convert glob to regex
		re := "^" + regexp.QuoteMeta(pattern) + "$"
		re = strings.ReplaceAll(re, `\*`, `.*`)
		matched, _ := regexp.MatchString(re, s)
		return matched
	}
	return strings.EqualFold(pattern, s)
}

func filterRules(rules []Rule, keep func(Rule) bool) []Rule {
	out := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

func generateID() string {
	b := make([]byte, 4)
	// Simple time-based ID (good enough for local persistence)
	t := time.Now().UnixNano()
	for i := 0; i < 4; i++ {
		b[i] = byte(t >> (i * 8))
	}
	return strings.ReplaceAll(
		strings.ReplaceAll(
			time.Now().Format("0102")+string(rune('a'+b[0]%26))+string(rune('a'+b[1]%26))+string(rune('a'+b[2]%26))+string(rune('a'+b[3]%26)),
			" ", ""),
		"\n", "")
}

func (rs *RuleStore) load() {
	if rs.path == "" {
		return
	}
	data, err := os.ReadFile(rs.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &rs.rules); err != nil {
		slog.Warn("approval_rules: load failed", "err", err)
	}
}

func (rs *RuleStore) loadFromKV() {
	if rs.kvs == nil {
		return
	}
	var rules []Rule
	found, err := rs.kvs.Get(context.Background(), "rules", &rules)
	if err != nil {
		slog.Warn("approval_rules: kv load failed", "err", err)
		return
	}
	if found && len(rules) > 0 {
		rs.rules = rules
		slog.Info("approval_rules: loaded from Ledger KV", "count", len(rules))
	}
}

func (rs *RuleStore) save() {
	if rs.kvs != nil {
		if err := rs.kvs.Put(context.Background(), "rules", rs.rules); err != nil {
			slog.Warn("approval_rules: kv save failed, falling back to file", "err", err)
		} else {
			return
		}
	}
	if rs.path == "" {
		return
	}
	dir := filepath.Dir(rs.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("approval_rules: mkdir failed", "err", err)
		return
	}
	data, _ := json.MarshalIndent(rs.rules, "", "  ")
	if err := os.WriteFile(rs.path, data, 0644); err != nil {
		slog.Warn("approval_rules: save failed", "err", err)
	}
}
