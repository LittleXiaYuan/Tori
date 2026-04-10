package approval

import (
	"testing"
	"time"
)

func testPolicy() Policy {
	return Policy{
		MinRiskLevel:     RiskHigh,
		TrustAutoApprove: 0.9,
		DefaultTimeout:   5 * time.Minute,
		AlwaysRequire:    []Category{CatFinancial},
	}
}

func TestApproveBasic(t *testing.T) {
	m := NewManager(testPolicy())

	// Create request in background (RequestApproval blocks)
	req := &Request{
		ID:        "test-1",
		TenantID:  "tenant-a",
		Category:  CatCodeExec,
		RiskLevel: RiskHigh,
		Summary:   "run dangerous command",
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		if err := m.Approve("test-1", "tenant-a"); err != nil {
			t.Errorf("approve failed: %v", err)
		}
	}()

	result := m.RequestApproval(req)
	if result.Status != StatusApproved {
		t.Fatalf("expected approved, got %s", result.Status)
	}
	if result.Approver != "tenant-a" {
		t.Fatalf("expected tenant-a, got %s", result.Approver)
	}
}

func TestDenyBasic(t *testing.T) {
	m := NewManager(testPolicy())

	req := &Request{
		ID:        "test-2",
		TenantID:  "tenant-a",
		Category:  CatDataMutation,
		RiskLevel: RiskHigh,
		Summary:   "delete database",
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		if err := m.Deny("test-2", "tenant-a", "too risky"); err != nil {
			t.Errorf("deny failed: %v", err)
		}
	}()

	result := m.RequestApproval(req)
	if result.Status != StatusDenied {
		t.Fatalf("expected denied, got %s", result.Status)
	}
	if result.Reason != "too risky" {
		t.Fatalf("expected reason 'too risky', got %s", result.Reason)
	}
}

func TestTenantMismatch(t *testing.T) {
	m := NewManager(testPolicy())

	req := &Request{
		ID:        "test-perm",
		TenantID:  "tenant-a",
		Category:  CatCodeExec,
		RiskLevel: RiskHigh,
		Summary:   "run commond",
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		// Different tenant tries to approve
		err := m.Approve("test-perm", "tenant-b")
		if err == nil {
			t.Error("expected permission denied error")
			return
		}
		// Now correct tenant approves
		if err := m.Approve("test-perm", "tenant-a"); err != nil {
			t.Errorf("same-tenant approve failed: %v", err)
		}
	}()

	result := m.RequestApproval(req)
	if result.Status != StatusApproved {
		t.Fatalf("expected approved, got %s", result.Status)
	}
}

func TestSystemApproverAlwaysAllowed(t *testing.T) {
	m := NewManager(testPolicy())

	req := &Request{
		ID:        "test-sys",
		TenantID:  "tenant-a",
		Category:  CatCodeExec,
		RiskLevel: RiskHigh,
		Summary:   "system action",
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		// Empty approver (system/internal) should always work
		if err := m.Approve("test-sys", ""); err != nil {
			t.Errorf("system approve failed: %v", err)
		}
	}()

	result := m.RequestApproval(req)
	if result.Status != StatusApproved {
		t.Fatalf("expected approved, got %s", result.Status)
	}
}

func TestAutoApprove_LowRisk(t *testing.T) {
	m := NewManager(testPolicy())

	req := &Request{
		ID:        "test-auto",
		TenantID:  "tenant-a",
		Category:  CatCodeExec,
		RiskLevel: RiskLow,
		Summary:   "read-only query",
	}

	result := m.RequestApproval(req)
	if result.Status != StatusAutoApproved {
		t.Fatalf("expected auto_approved, got %s", result.Status)
	}
}

func TestExpiry(t *testing.T) {
	policy := testPolicy()
	policy.DefaultTimeout = 100 * time.Millisecond
	m := NewManager(policy)

	req := &Request{
		ID:        "test-expire",
		TenantID:  "tenant-a",
		Category:  CatCodeExec,
		RiskLevel: RiskHigh,
		Summary:   "will expire",
	}

	go func() {
		// Wait for expiry + resolution margin
		time.Sleep(300 * time.Millisecond)
		// Try to approve after expiry
		err := m.Approve("test-expire", "tenant-a")
		if err == nil {
			t.Error("expected error approving expired request")
		}
	}()

	// The expiryLoop runs every 30s which is too slow for tests.
	// Manually trigger expiry by trying to resolve after timeout.
	go func() {
		time.Sleep(150 * time.Millisecond)
		// Manually expire by calling resolve (simulates expiryLoop)
		m.mu.Lock()
		r, ok := m.requests["test-expire"]
		if ok && r.Status == StatusPending && time.Now().After(r.ExpiresAt) {
			r.Status = StatusExpired
			now := time.Now()
			r.ResolvedAt = &now
			if ch, exists := m.waiters["test-expire"]; exists {
				close(ch)
				delete(m.waiters, "test-expire")
			}
		}
		m.mu.Unlock()
	}()

	result := m.RequestApproval(req)
	if result.Status != StatusExpired {
		t.Fatalf("expected expired, got %s", result.Status)
	}
}

func TestAlreadyResolved(t *testing.T) {
	m := NewManager(testPolicy())

	req := &Request{
		ID:        "test-double",
		TenantID:  "tenant-a",
		Category:  CatCodeExec,
		RiskLevel: RiskHigh,
		Summary:   "double approve test",
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		m.Approve("test-double", "tenant-a")
	}()

	m.RequestApproval(req)

	// Try to approve again
	err := m.Approve("test-double", "tenant-a")
	if err == nil {
		t.Fatal("expected error on double approval")
	}
}

func TestPendingAndHistory(t *testing.T) {
	m := NewManager(testPolicy())

	// Add a pending request (don't block on it)
	m.mu.Lock()
	m.requests["p1"] = &Request{ID: "p1", TenantID: "t1", Status: StatusPending}
	m.requests["p2"] = &Request{ID: "p2", TenantID: "t2", Status: StatusPending}
	m.requests["h1"] = &Request{ID: "h1", TenantID: "t1", Status: StatusApproved}
	m.waiters["p1"] = make(chan struct{})
	m.waiters["p2"] = make(chan struct{})
	m.mu.Unlock()

	pending := m.Pending("t1")
	if len(pending) != 1 || pending[0].ID != "p1" {
		t.Fatalf("expected 1 pending for t1, got %d", len(pending))
	}

	allPending := m.Pending("")
	if len(allPending) != 2 {
		t.Fatalf("expected 2 total pending, got %d", len(allPending))
	}

	history := m.History("t1", 10)
	if len(history) != 1 || history[0].ID != "h1" {
		t.Fatalf("expected 1 history for t1, got %d", len(history))
	}
}
