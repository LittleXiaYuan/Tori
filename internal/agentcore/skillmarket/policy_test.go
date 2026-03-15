package skillmarket

import (
	"path/filepath"
	"testing"
)

func TestPolicyDefaults(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	data := sp.Get()
	if data.MinScore != 60 {
		t.Errorf("expected min_score 60, got %d", data.MinScore)
	}
	if data.AutoApproveMin != 80 {
		t.Errorf("expected auto_approve_min 80, got %d", data.AutoApproveMin)
	}
}

func TestPolicyBlockedSlug(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	sp.Update(PolicyData{
		MinScore:       60,
		AutoApproveMin: 80,
		BlockedSlugs:   []string{"evil-tool"},
	})

	result := sp.Check("evil-tool", "author", nil, 100, true)
	if result.Allowed {
		t.Error("expected blocked slug to be denied")
	}
	if result.Reason == "" {
		t.Error("expected reason for denial")
	}
}

func TestPolicyBlockedAuthor(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	sp.Update(PolicyData{
		MinScore:       60,
		AutoApproveMin: 80,
		BlockedAuthors: []string{"bad-actor"},
	})

	result := sp.Check("some-skill", "bad-actor", nil, 100, true)
	if result.Allowed {
		t.Error("expected blocked author to be denied")
	}
}

func TestPolicyAllowedSlug(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	sp.Update(PolicyData{
		MinScore:       90,
		AutoApproveMin: 80,
		AllowedSlugs:   []string{"blessed-tool"},
	})

	// Should bypass the 90 min-score requirement
	result := sp.Check("blessed-tool", "anyone", nil, 50, true)
	if !result.Allowed {
		t.Error("expected allowed slug to be permitted")
	}
	if !result.AutoApprove {
		t.Error("expected auto approve for allowed slug")
	}
}

func TestPolicyMinScore(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	sp.Update(PolicyData{
		MinScore:       70,
		AutoApproveMin: 80,
	})

	result := sp.Check("some-skill", "author", nil, 50, true)
	if result.Allowed {
		t.Error("expected low score to be denied")
	}

	result = sp.Check("some-skill", "author", nil, 75, true)
	if !result.Allowed {
		t.Error("expected passing score to be allowed")
	}
	if result.AutoApprove {
		t.Error("expected no auto-approve at 75 (threshold 80)")
	}
}

func TestPolicyAutoApproveByScore(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	sp.Update(PolicyData{
		MinScore:       60,
		AutoApproveMin: 80,
	})

	result := sp.Check("good-skill", "author", nil, 85, true)
	if !result.Allowed {
		t.Error("expected allowed")
	}
	if !result.AutoApprove {
		t.Error("expected auto approve at score 85")
	}
}

func TestPolicyTrustedAuthor(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	sp.Update(PolicyData{
		MinScore:       60,
		AutoApproveMin: 80,
		TrustedAuthors: []string{"trusted-dev"},
	})

	result := sp.Check("any-skill", "trusted-dev", nil, 65, true)
	if !result.Allowed {
		t.Error("expected allowed for trusted author")
	}
	if !result.AutoApprove {
		t.Error("expected auto approve for trusted author")
	}
}

func TestPolicyMaxPermLevel(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	sp.Update(PolicyData{
		MinScore:       60,
		AutoApproveMin: 80,
		MaxPermLevel:   "write",
	})

	// Shell perm should be denied
	result := sp.Check("shell-skill", "author", []string{"shell"}, 80, true)
	if result.Allowed {
		t.Error("expected shell perm to exceed 'write' max level")
	}

	// Network perm should be denied
	result = sp.Check("net-skill", "author", []string{"network"}, 80, true)
	if result.Allowed {
		t.Error("expected network perm to exceed 'write' max level")
	}

	// Write perm should be allowed
	result = sp.Check("write-skill", "author", []string{"write"}, 80, true)
	if !result.Allowed {
		t.Error("expected write perm to be within 'write' max level")
	}
}

func TestPolicyRequireAudit(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	sp.Update(PolicyData{
		MinScore:       60,
		AutoApproveMin: 80,
		RequireAudit:   true,
	})

	result := sp.Check("unaudited-skill", "author", nil, 0, false)
	if result.Allowed {
		t.Error("expected denial when audit not available and required")
	}

	result = sp.Check("audited-skill", "author", nil, 70, true)
	if !result.Allowed {
		t.Error("expected allowed when audit available")
	}
}

func TestPolicyPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")

	sp := NewSecurityPolicy(path)
	sp.Update(PolicyData{
		MinScore:       75,
		AutoApproveMin: 90,
		TrustedAuthors: []string{"admin"},
		BlockedSlugs:   []string{"bad-tool"},
	})

	// Load from same path
	sp2 := NewSecurityPolicy(path)
	data := sp2.Get()
	if data.MinScore != 75 {
		t.Errorf("expected persisted min_score 75, got %d", data.MinScore)
	}
	if len(data.TrustedAuthors) != 1 || data.TrustedAuthors[0] != "admin" {
		t.Error("expected persisted trusted authors")
	}
	if len(data.BlockedSlugs) != 1 || data.BlockedSlugs[0] != "bad-tool" {
		t.Error("expected persisted blocked slugs")
	}
}

func TestPolicyBlockedTakesPrecedence(t *testing.T) {
	sp := NewSecurityPolicy(filepath.Join(t.TempDir(), "policy.json"))
	sp.Update(PolicyData{
		MinScore:       60,
		AutoApproveMin: 80,
		AllowedSlugs:   []string{"dual-tool"},
		BlockedSlugs:   []string{"dual-tool"}, // blocked should win
	})

	result := sp.Check("dual-tool", "author", nil, 100, true)
	if result.Allowed {
		t.Error("expected blocked to take precedence over allowed")
	}
}
