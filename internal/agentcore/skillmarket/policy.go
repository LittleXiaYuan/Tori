package skillmarket

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// SecurityPolicy defines marketplace security enforcement rules.
type SecurityPolicy struct {
	mu   sync.RWMutex
	path string
	data PolicyData
}

// PolicyData is the persisted security policy configuration.
type PolicyData struct {
	MinScore       int      `json:"min_score"`        // Minimum audit score required (0-100)
	TrustedAuthors []string `json:"trusted_authors"`  // Authors whose skills auto-approve
	BlockedAuthors []string `json:"blocked_authors"`  // Authors permanently blocked
	AllowedSlugs   []string `json:"allowed_slugs"`    // Explicitly allowed skills (bypass score)
	BlockedSlugs   []string `json:"blocked_slugs"`    // Explicitly blocked skills
	MaxPermLevel   string   `json:"max_perm_level"`   // Maximum allowed perm: "read-only", "write", "network", "shell"
	RequireAudit   bool     `json:"require_audit"`    // Block install if audit not available
	AutoApproveMin int      `json:"auto_approve_min"` // Score threshold for auto-approve (default 80)
}

// NewSecurityPolicy creates a policy backed by a JSON file.
func NewSecurityPolicy(path string) *SecurityPolicy {
	sp := &SecurityPolicy{path: path}
	sp.load()
	// Set sensible defaults if empty
	if sp.data.MinScore == 0 {
		sp.data.MinScore = 60
	}
	if sp.data.AutoApproveMin == 0 {
		sp.data.AutoApproveMin = 80
	}
	return sp
}

// Get returns a copy of the current policy data.
func (sp *SecurityPolicy) Get() PolicyData {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	d := sp.data
	if d.TrustedAuthors == nil {
		d.TrustedAuthors = []string{}
	}
	if d.BlockedAuthors == nil {
		d.BlockedAuthors = []string{}
	}
	if d.AllowedSlugs == nil {
		d.AllowedSlugs = []string{}
	}
	if d.BlockedSlugs == nil {
		d.BlockedSlugs = []string{}
	}
	return d
}

// Update replaces the entire policy.
func (sp *SecurityPolicy) Update(data PolicyData) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.data = data
	sp.save()
}

// PolicyCheckResult describes the outcome of a policy check.
type PolicyCheckResult struct {
	Allowed     bool   `json:"allowed"`
	Reason      string `json:"reason,omitempty"`
	AutoApprove bool   `json:"auto_approve,omitempty"`
}

// Check evaluates whether a skill should be allowed to install.
func (sp *SecurityPolicy) Check(slug, author string, permissions []string, auditScore int, auditAvailable bool) PolicyCheckResult {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	// Explicit block
	for _, b := range sp.data.BlockedSlugs {
		if b == slug {
			return PolicyCheckResult{Allowed: false, Reason: fmt.Sprintf("技能 %q 在黑名单中", slug)}
		}
	}
	for _, b := range sp.data.BlockedAuthors {
		if b == author {
			return PolicyCheckResult{Allowed: false, Reason: fmt.Sprintf("作者 %q 在黑名单中", author)}
		}
	}

	// Explicit allow (bypass score check)
	for _, a := range sp.data.AllowedSlugs {
		if a == slug {
			return PolicyCheckResult{Allowed: true, AutoApprove: true}
		}
	}

	// Require audit
	if sp.data.RequireAudit && !auditAvailable {
		return PolicyCheckResult{Allowed: false, Reason: "需要安全审计但审计报告不可用"}
	}

	// Score check
	if auditAvailable && auditScore < sp.data.MinScore {
		return PolicyCheckResult{Allowed: false, Reason: fmt.Sprintf("安全评分 %d 低于最低要求 %d", auditScore, sp.data.MinScore)}
	}

	// Permission level check
	if sp.data.MaxPermLevel != "" {
		maxLevel := parsePermLevel(sp.data.MaxPermLevel)
		for _, p := range permissions {
			if classifyPermLevel(p) > maxLevel {
				return PolicyCheckResult{Allowed: false, Reason: fmt.Sprintf("权限 %q 超出允许的最大级别 %q", p, sp.data.MaxPermLevel)}
			}
		}
	}

	// Check trusted author for auto-approve
	for _, ta := range sp.data.TrustedAuthors {
		if ta == author {
			return PolicyCheckResult{Allowed: true, AutoApprove: true}
		}
	}

	// Auto-approve by score
	autoApprove := auditAvailable && auditScore >= sp.data.AutoApproveMin
	return PolicyCheckResult{Allowed: true, AutoApprove: autoApprove}
}

func parsePermLevel(s string) PermLevel {
	switch s {
	case "read-only":
		return PermReadOnly
	case "write":
		return PermWrite
	case "network":
		return PermNetwork
	case "shell":
		return PermShell
	default:
		return PermShell // most permissive by default
	}
}

func classifyPermLevel(perm string) PermLevel {
	p := strings.ToLower(perm)
	if strings.Contains(p, "shell") || strings.Contains(p, "exec") {
		return PermShell
	}
	if strings.Contains(p, "network") || strings.Contains(p, "http") {
		return PermNetwork
	}
	if strings.Contains(p, "write") {
		return PermWrite
	}
	return PermReadOnly
}

func (sp *SecurityPolicy) load() {
	data, err := os.ReadFile(sp.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &sp.data); err != nil {
		slog.Warn("security-policy: load failed", "err", err)
	}
}

func (sp *SecurityPolicy) save() {
	data, _ := json.MarshalIndent(sp.data, "", "  ")
	if err := os.WriteFile(sp.path, data, 0644); err != nil {
		slog.Warn("security-policy: save failed", "err", err)
	}
}
