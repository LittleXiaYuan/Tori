package policy

import (
	"testing"
)

func TestDefaultOwnerAllAccess(t *testing.T) {
	e := NewEngine()
	actions := []Action{ActionChat, ActionMemoryRead, ActionMemoryWrite, ActionSkillExec, ActionBotManage, ActionAdmin}
	for _, a := range actions {
		d := e.Check(RoleOwner, a)
		if !d.Allowed {
			t.Fatalf("owner should have %s", a)
		}
	}
}

func TestDefaultMemberLimited(t *testing.T) {
	e := NewEngine()
	d := e.Check(RoleMember, ActionChat)
	if !d.Allowed {
		t.Fatal("member should have chat")
	}
	d = e.Check(RoleMember, ActionAdmin)
	if d.Allowed {
		t.Fatal("member should not have admin")
	}
	d = e.Check(RoleMember, ActionBotManage)
	if d.Allowed {
		t.Fatal("member should not have bot.manage")
	}
}

func TestGuestDisabledByDefault(t *testing.T) {
	e := NewEngine()
	d := e.Check(RoleGuest, ActionChat)
	if d.Allowed {
		t.Fatal("guest should be denied when disabled")
	}
}

func TestGuestEnabled(t *testing.T) {
	e := NewEngine()
	e.SetGuestAccess(true)
	d := e.Check(RoleGuest, ActionChat)
	if !d.Allowed {
		t.Fatal("guest should be allowed when enabled")
	}
	d = e.Check(RoleGuest, ActionAdmin)
	if d.Allowed {
		t.Fatal("guest should not have admin")
	}
}

func TestGrant(t *testing.T) {
	e := NewEngine()
	e.Grant(RoleMember, ActionBotManage)
	d := e.Check(RoleMember, ActionBotManage)
	if !d.Allowed {
		t.Fatal("granted action should be allowed")
	}
}

func TestRevoke(t *testing.T) {
	e := NewEngine()
	e.Revoke(RoleAdmin, ActionPersona)
	d := e.Check(RoleAdmin, ActionPersona)
	if d.Allowed {
		t.Fatal("revoked action should be denied")
	}
}

func TestRoleFromString(t *testing.T) {
	cases := map[string]Role{
		"owner":  RoleOwner,
		"ADMIN":  RoleAdmin,
		"Member": RoleMember,
		"guest":  RoleGuest,
		"":       RoleGuest,
		"xyz":    RoleGuest,
	}
	for input, expected := range cases {
		if RoleFromString(input) != expected {
			t.Fatalf("RoleFromString(%q): got %s, want %s", input, RoleFromString(input), expected)
		}
	}
}

func TestUnknownRole(t *testing.T) {
	e := NewEngine()
	d := e.Check(Role("unknown"), ActionChat)
	if d.Allowed {
		t.Fatal("unknown role should be denied")
	}
}

func TestGuestAccessToggle(t *testing.T) {
	e := NewEngine()
	if e.GuestAccess() {
		t.Fatal("should be disabled by default")
	}
	e.SetGuestAccess(true)
	if !e.GuestAccess() {
		t.Fatal("should be enabled")
	}
}
