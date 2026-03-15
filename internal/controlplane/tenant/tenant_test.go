package tenant

import "testing"

func TestRegisterAndLookup(t *testing.T) {
	m := NewManager()
	tenant := m.Register("test-org")
	if tenant.Name != "test-org" || tenant.APIKey == "" {
		t.Fatalf("invalid tenant: %+v", tenant)
	}

	found := m.ByAPIKey(tenant.APIKey)
	if found == nil || found.ID != tenant.ID {
		t.Fatal("ByAPIKey lookup failed")
	}

	found2 := m.ByID(tenant.ID)
	if found2 == nil || found2.Name != "test-org" {
		t.Fatal("ByID lookup failed")
	}
}

func TestInvalidAPIKey(t *testing.T) {
	m := NewManager()
	m.Register("org1")
	if m.ByAPIKey("invalid_key") != nil {
		t.Fatal("expected nil for invalid key")
	}
}

func TestList(t *testing.T) {
	m := NewManager()
	m.Register("org1")
	m.Register("org2")
	list := m.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(list))
	}
}
