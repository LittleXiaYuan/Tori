package controlplanepack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/controlplane/models"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/observe"
)

type approvalGateway struct {
	manager *approval.Manager
	tenant  string
	bridged int
}

func (g *approvalGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *approvalGateway) ApprovalManager() *approval.Manager { return g.manager }

func (g *approvalGateway) AuditChain() *audit.Chain { return nil }

func (g *approvalGateway) AuditTrail() *audit.Trail { return nil }

func (g *approvalGateway) BotManager() *bots.Manager { return nil }

func (g *approvalGateway) InboxStore() *inbox.Store { return nil }

func (g *approvalGateway) ShellPolicy() *tools.ShellExecPolicy { return nil }

func (g *approvalGateway) TenantManager() *tenant.Manager { return nil }

func (g *approvalGateway) TenantOf(ctx context.Context) string { return g.tenant }

func (g *approvalGateway) ToolsManager() *tools.ProcessManager { return nil }

func (g *approvalGateway) TrustTracker() *trust.Tracker { return nil }

func (g *approvalGateway) RoleOf(ctx context.Context) string { return "user" }

func (g *approvalGateway) OutputDir() string { return "" }

func (g *approvalGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *approvalGateway) MetricsPrometheus() string { return "" }

func (g *approvalGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *approvalGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *approvalGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func (g *approvalGateway) ModelManager() *models.Manager { return nil }

func (g *approvalGateway) ProviderModels() []models.ProviderModel { return nil }

func (g *approvalGateway) DeleteProviderModel(id string) bool { return false }

func (g *approvalGateway) UsageSnapshot(ctx context.Context) any { return nil }

func (g *approvalGateway) SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64) {
}

func newApprovalHandler(t *testing.T) (*Handler, *approval.Manager, *approvalGateway) {
	t.Helper()
	manager := approval.NewManagerWithRules(approval.DefaultPolicy(), approval.NewRuleStore(t.TempDir()))
	gateway := &approvalGateway{manager: manager, tenant: "tenant-a"}
	return NewHandler(gateway), manager, gateway
}

func createPendingApproval(t *testing.T, manager *approval.Manager, id string) {
	t.Helper()
	done := make(chan *approval.Request, 1)
	go func() {
		done <- manager.RequestApproval(&approval.Request{
			ID:        id,
			TenantID:  "tenant-a",
			Requester: "tenant-a",
			Category:  approval.CatCodeExec,
			RiskLevel: approval.RiskHigh,
			Summary:   "run command",
			Details:   map[string]any{"skill_name": "shell_exec"},
			ExpiresAt: time.Now().Add(time.Hour),
		})
	}()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(manager.Pending("tenant-a")) == 1 {
			return
		}
		select {
		case result := <-done:
			t.Fatalf("approval resolved unexpectedly: %+v", result)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatal("approval did not become pending")
}

func TestApprovalRoutesAreNative(t *testing.T) {
	h, _, gateway := newApprovalHandler(t)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/v1/approvals", "/v1/approvals/approve", "/v1/approvals/deny", "/v1/approvals/decide", "/v1/approvals/rules"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/v1/approvals"](rec, httptest.NewRequest(http.MethodGet, "/v1/approvals", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("approval route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestApprovalApproveAndList(t *testing.T) {
	h, manager, _ := newApprovalHandler(t)
	createPendingApproval(t, manager, "approval-1")

	list := httptest.NewRecorder()
	h.handleApprovalList(list, httptest.NewRequest(http.MethodGet, "/v1/approvals", nil))
	var before struct {
		Approvals []*approval.Request `json:"approvals"`
		Total     int                 `json:"total"`
	}
	if err := json.NewDecoder(list.Body).Decode(&before); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if before.Total != 1 || before.Approvals[0].ID != "approval-1" {
		t.Fatalf("before = %+v, want pending approval-1", before)
	}

	rec := httptest.NewRecorder()
	h.handleApprovalApprove(rec, httptest.NewRequest(http.MethodPost, "/v1/approvals/approve", bytes.NewBufferString(`{"id":"approval-1"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	history := manager.History("tenant-a", 10)
	if len(history) != 1 || history[0].Status != approval.StatusApproved {
		t.Fatalf("history = %+v, want approved", history)
	}
}

func TestApprovalRulesCRUD(t *testing.T) {
	h, _, _ := newApprovalHandler(t)
	rec := httptest.NewRecorder()
	h.handleApprovalRules(rec, httptest.NewRequest(http.MethodPost, "/v1/approvals/rules", bytes.NewBufferString(`{"pattern":"shell_*","action":"allow_always","scope":"global"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	list := httptest.NewRecorder()
	h.handleApprovalRules(list, httptest.NewRequest(http.MethodGet, "/v1/approvals/rules", nil))
	var listed struct {
		Rules []approval.Rule `json:"rules"`
		Total int             `json:"total"`
	}
	if err := json.NewDecoder(list.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listed.Total != 1 || len(listed.Rules) != 1 || listed.Rules[0].ID == "" {
		t.Fatalf("expected one persisted rule with id, got %+v", listed)
	}

	del := httptest.NewRecorder()
	h.handleApprovalRules(del, httptest.NewRequest(http.MethodDelete, "/v1/approvals/rules?id="+listed.Rules[0].ID, nil))
	if !bytes.Contains(del.Body.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected deleted=true, got %s", del.Body.String())
	}
}
