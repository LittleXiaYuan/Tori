package rbac

import (
	"testing"
)

func TestNewEnforcerDefaultRoles(t *testing.T) {
	e := NewEnforcer()
	roles := e.ListRoles()
	if len(roles) < 3 {
		t.Errorf("default roles = %d, want >= 3 (owner/admin/user)", len(roles))
	}

	// Owner should exist
	_, found := e.GetRole("owner")
	if !found {
		t.Error("owner role should exist")
	}
}

func TestAssignAndCheckPermission(t *testing.T) {
	e := NewEnforcer()

	err := e.AssignRole("user1", "owner", "tenant1")
	if err != nil {
		t.Fatalf("AssignRole: %v", err)
	}

	if !e.Check("user1", "tenant1", ResChat, ActionRead) {
		t.Error("owner should have read access to chat")
	}
	if !e.Check("user1", "tenant1", ResSettings, ActionAdmin) {
		t.Error("owner should have admin access to settings")
	}
}

func TestCheckDenied(t *testing.T) {
	e := NewEnforcer()

	// No role assigned
	if e.Check("user2", "tenant1", ResSettings, ActionAdmin) {
		t.Error("unassigned user should be denied")
	}
}

func TestAssignUnknownRole(t *testing.T) {
	e := NewEnforcer()
	err := e.AssignRole("user1", "nonexistent_role", "tenant1")
	if err == nil {
		t.Error("expected error for unknown role")
	}
}

func TestRevokeRole(t *testing.T) {
	e := NewEnforcer()
	e.AssignRole("user1", "owner", "tenant1")
	e.RevokeRole("user1", "owner", "tenant1")

	if e.Check("user1", "tenant1", ResChat, ActionRead) {
		t.Error("revoked role should deny access")
	}
}

func TestRequirePermission(t *testing.T) {
	e := NewEnforcer()
	e.AssignRole("user1", "owner", "tenant1")

	err := e.Require("user1", "tenant1", ResChat, ActionRead)
	if err != nil {
		t.Errorf("require: %v", err)
	}

	err = e.Require("user2", "tenant1", ResSettings, ActionAdmin)
	if err == nil {
		t.Error("unassigned user should get error")
	}
}

func TestDuplicateAssign(t *testing.T) {
	e := NewEnforcer()
	e.AssignRole("user1", "owner", "tenant1")
	e.AssignRole("user1", "owner", "tenant1") // duplicate

	roles := e.SubjectRoles("user1", "tenant1")
	if len(roles) != 1 {
		t.Errorf("roles = %d, want 1 (no duplicates)", len(roles))
	}
}

func TestRemoveBuiltInRole(t *testing.T) {
	e := NewEnforcer()
	err := e.RemoveRole("owner")
	if err == nil {
		t.Error("should not allow removing built-in role")
	}
}

func TestAddCustomRole(t *testing.T) {
	e := NewEnforcer()
	e.AddRole(Role{
		ID:   "custom",
		Name: "Custom Role",
		Permissions: []Permission{
			{Resource: ResChat, Action: ActionRead},
		},
	})

	e.AssignRole("user1", "custom", "t1")
	if !e.Check("user1", "t1", ResChat, ActionRead) {
		t.Error("custom role should grant read chat")
	}
	if e.Check("user1", "t1", ResSettings, ActionAdmin) {
		t.Error("custom role should not grant admin settings")
	}
}

func TestRemoveCustomRole(t *testing.T) {
	e := NewEnforcer()
	e.AddRole(Role{ID: "temporary", Name: "Temp"})

	err := e.RemoveRole("temporary")
	if err != nil {
		t.Errorf("RemoveRole: %v", err)
	}

	_, found := e.GetRole("temporary")
	if found {
		t.Error("removed role should not be found")
	}
}

func TestSubjectRoles(t *testing.T) {
	e := NewEnforcer()
	e.AssignRole("user1", "owner", "t1")

	roles := e.SubjectRoles("user1", "t1")
	if len(roles) != 1 {
		t.Errorf("roles = %d, want 1", len(roles))
	}
	if roles[0].ID != "owner" {
		t.Errorf("role = %s, want owner", roles[0].ID)
	}
}
