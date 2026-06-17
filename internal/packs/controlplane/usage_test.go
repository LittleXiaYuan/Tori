package controlplanepack

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/llm/distill"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/review"
	"yunque-agent/internal/agentcore/skillgrowth/adapter"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/controlplane/models"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/observe"
)

type usageGateway struct {
	snapshot        any
	quotaTenant     string
	maxChatCalls    int64
	maxTokensPerDay int64
	bridged         int
}

func (g *usageGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *usageGateway) ApprovalManager() *approval.Manager { return nil }

func (g *usageGateway) AuditChain() *audit.Chain { return nil }

func (g *usageGateway) AuditTrail() *audit.Trail { return nil }

func (g *usageGateway) BotManager() *bots.Manager { return nil }

func (g *usageGateway) InboxStore() *inbox.Store { return nil }

func (g *usageGateway) ShellPolicy() *tools.ShellExecPolicy { return nil }

func (g *usageGateway) TenantManager() *tenant.Manager { return nil }

func (g *usageGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *usageGateway) ToolsManager() *tools.ProcessManager { return nil }

func (g *usageGateway) TrustTracker() *trust.Tracker { return nil }

func (g *usageGateway) RoleOf(ctx context.Context) string { return "user" }

func (g *usageGateway) ReviewGate() *review.Gate { return nil }

func (g *usageGateway) Distiller() *distill.Distiller { return nil }

func (g *usageGateway) SkillGrowDetector() *adapter.Detector { return nil }

func (g *usageGateway) OutputDir() string { return "" }

func (g *usageGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *usageGateway) MetricsPrometheus() string { return "" }

func (g *usageGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *usageGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *usageGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func (g *usageGateway) ModelManager() *models.Manager { return nil }

func (g *usageGateway) ProviderModels() []models.ProviderModel { return nil }

func (g *usageGateway) DeleteProviderModel(id string) bool { return false }

func (g *usageGateway) UsageSnapshot(ctx context.Context) any { return g.snapshot }

func (g *usageGateway) SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64) {
	g.quotaTenant = tenantID
	g.maxChatCalls = maxChatCalls
	g.maxTokensPerDay = maxTokensPerDay
}

func TestUsageRoutesAreNative(t *testing.T) {
	gateway := &usageGateway{snapshot: map[string]any{"tenant_id": "tenant-a", "chat_calls": 2}}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/v1/usage", "/v1/quota"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/v1/usage"](rec, httptest.NewRequest(http.MethodGet, "/v1/usage", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"chat_calls":2`) {
		t.Fatalf("usage response status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("usage route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestQuotaPost(t *testing.T) {
	gateway := &usageGateway{}
	h := NewHandler(gateway)
	rec := httptest.NewRecorder()
	h.handleQuota(rec, httptest.NewRequest(http.MethodPost, "/v1/quota", bytes.NewBufferString(`{"tenant_id":"tenant-b","quota":{"max_chat_calls":10,"max_tokens_per_day":2000}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("quota status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if gateway.quotaTenant != "tenant-b" || gateway.maxChatCalls != 10 || gateway.maxTokensPerDay != 2000 {
		t.Fatalf("quota call = tenant=%q chat=%d tokens=%d", gateway.quotaTenant, gateway.maxChatCalls, gateway.maxTokensPerDay)
	}
}
