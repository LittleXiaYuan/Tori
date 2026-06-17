package controlplanepack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

type trustGateway struct {
	tracker *trust.Tracker
	role    string
	bridged int
}

func (g *trustGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *trustGateway) ApprovalManager() *approval.Manager { return nil }

func (g *trustGateway) AuditChain() *audit.Chain { return nil }

func (g *trustGateway) AuditTrail() *audit.Trail { return nil }

func (g *trustGateway) BotManager() *bots.Manager { return nil }

func (g *trustGateway) InboxStore() *inbox.Store { return nil }

func (g *trustGateway) ShellPolicy() *tools.ShellExecPolicy { return nil }

func (g *trustGateway) TenantManager() *tenant.Manager { return nil }

func (g *trustGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *trustGateway) ToolsManager() *tools.ProcessManager { return nil }

func (g *trustGateway) TrustTracker() *trust.Tracker { return g.tracker }

func (g *trustGateway) RoleOf(ctx context.Context) string { return g.role }

func (g *trustGateway) OutputDir() string { return "" }

func (g *trustGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *trustGateway) MetricsPrometheus() string { return "" }

func (g *trustGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *trustGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *trustGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func (g *trustGateway) ModelManager() *models.Manager { return nil }

func (g *trustGateway) ProviderModels() []models.ProviderModel { return nil }

func (g *trustGateway) DeleteProviderModel(id string) bool { return false }

func (g *trustGateway) UsageSnapshot(ctx context.Context) any { return nil }

func (g *trustGateway) SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64) {
}

func TestTrustRoutesAreNative(t *testing.T) {
	tracker := trust.NewTracker("")
	tracker.Seed("skill-a", 42)
	gateway := &trustGateway{tracker: tracker, role: "admin"}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/api/trust/scores", "/api/trust/reset", "/api/trust/grant"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/api/trust/scores"](rec, httptest.NewRequest(http.MethodGet, "/api/trust/scores", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"count":1`) {
		t.Fatalf("trust scores status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("trust route should not call bridge, calls=%d", gateway.bridged)
	}
}
