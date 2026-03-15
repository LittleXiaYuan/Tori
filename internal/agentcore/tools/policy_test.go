package tools

import (
	"testing"
)

func TestPolicyDefaultDeny(t *testing.T) {
	p := NewPolicy()
	if p.Check("exec") {
		t.Fatal("default should deny")
	}
}

func TestPolicyAllowAll(t *testing.T) {
	p := NewPolicyFromRules([]string{"*"}, nil)
	if !p.Check("exec") {
		t.Fatal("expected allow all")
	}
	if !p.Check("read") {
		t.Fatal("expected allow all")
	}
}

func TestPolicyDenyAll(t *testing.T) {
	p := NewPolicyFromRules(nil, []string{"*"})
	if p.Check("exec") {
		t.Fatal("expected deny all")
	}
}

func TestPolicyAllowThenDenySpecific(t *testing.T) {
	// Allow all, then deny exec specifically
	_ = NewPolicyFromRules([]string{"*"}, []string{"exec"})
	// deny rules added first, allow rules second -> allow * wins over deny exec
	// Need to add deny AFTER allow for it to take priority
	p2 := NewPolicy()
	p2.AddRule(RuleAllow, "*")
	p2.AddRule(RuleDeny, "exec")
	if p2.Check("exec") {
		t.Fatal("exec should be denied")
	}
	if !p2.Check("read") {
		t.Fatal("read should be allowed")
	}
}

func TestPolicyGroup(t *testing.T) {
	p := NewPolicyFromRules([]string{"group:fs"}, nil)
	if !p.Check("read") {
		t.Fatal("read should be allowed via group:fs")
	}
	if !p.Check("write") {
		t.Fatal("write should be allowed via group:fs")
	}
	if p.Check("exec") {
		t.Fatal("exec should not be in group:fs")
	}
}

func TestPolicyGroupDeny(t *testing.T) {
	p := NewPolicy()
	p.AddRule(RuleAllow, "*")
	p.AddRule(RuleDeny, "group:runtime")
	if p.Check("exec") {
		t.Fatal("exec should be denied via group:runtime")
	}
	if !p.Check("read") {
		t.Fatal("read should still be allowed")
	}
}

func TestPolicyProfileMinimal(t *testing.T) {
	p, err := NewPolicyFromProfile("minimal")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Check("read") {
		t.Fatal("read should be allowed in minimal")
	}
	if !p.Check("grep") {
		t.Fatal("grep should be allowed in minimal")
	}
	if p.Check("write") {
		t.Fatal("write should be denied in minimal")
	}
	if p.Check("exec") {
		t.Fatal("exec should be denied in minimal")
	}
}

func TestPolicyProfileCoding(t *testing.T) {
	p, err := NewPolicyFromProfile("coding")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Check("read") {
		t.Fatal("read should be allowed")
	}
	if !p.Check("exec") {
		t.Fatal("exec should be allowed")
	}
	if p.Check("gateway") {
		t.Fatal("gateway should be denied in coding")
	}
}

func TestPolicyProfileFull(t *testing.T) {
	p, err := NewPolicyFromProfile("full")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Check("exec") {
		t.Fatal("should allow everything")
	}
	if !p.Check("gateway") {
		t.Fatal("should allow everything")
	}
}

func TestPolicyProfileUnknown(t *testing.T) {
	_, err := NewPolicyFromProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPolicyFilter(t *testing.T) {
	p := NewPolicyFromRules([]string{"group:fs"}, nil)
	all := []string{"read", "write", "exec", "ls", "cron"}
	allowed := p.Filter(all)
	if len(allowed) != 3 { // read, write, ls
		t.Fatalf("expected 3 allowed, got %d: %v", len(allowed), allowed)
	}
}

func TestPolicyDeniedTools(t *testing.T) {
	p := NewPolicyFromRules([]string{"group:fs"}, nil)
	all := []string{"read", "write", "exec", "ls"}
	denied := p.DeniedTools(all)
	if len(denied) != 1 || denied[0] != "exec" {
		t.Fatalf("expected [exec], got %v", denied)
	}
}

func TestPolicyGlobPattern(t *testing.T) {
	p := NewPolicy()
	p.AddRule(RuleAllow, "session*")
	if !p.Check("sessions_list") {
		t.Fatal("should match glob")
	}
	if !p.Check("session_status") {
		t.Fatal("should match glob")
	}
	if p.Check("exec") {
		t.Fatal("should not match glob")
	}
}

func TestPolicyRules(t *testing.T) {
	p := NewPolicy()
	p.AddRule(RuleAllow, "*")
	p.AddRule(RuleDeny, "exec")
	rules := p.Rules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestExpandGroup(t *testing.T) {
	tools := ExpandGroup("group:fs")
	if len(tools) != 7 {
		t.Fatalf("expected 7 fs tools, got %d", len(tools))
	}
	single := ExpandGroup("exec")
	if len(single) != 1 || single[0] != "exec" {
		t.Fatalf("expected [exec], got %v", single)
	}
}

func TestListGroupsAndProfiles(t *testing.T) {
	groups := ListGroups()
	if len(groups) < 4 {
		t.Fatalf("expected at least 4 groups, got %d", len(groups))
	}
	profiles := ListProfiles()
	if len(profiles) != 4 {
		t.Fatalf("expected 4 profiles, got %d", len(profiles))
	}
}

func TestPolicyLastRuleWins(t *testing.T) {
	p := NewPolicy()
	p.AddRule(RuleDeny, "*")
	p.AddRule(RuleAllow, "read")
	p.AddRule(RuleDeny, "read")
	// Last rule for "read" is deny
	if p.Check("read") {
		t.Fatal("last deny should win")
	}
}
