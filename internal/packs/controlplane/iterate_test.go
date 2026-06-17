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
	"yunque-agent/internal/agentcore/selfheal/iterate"
	"yunque-agent/internal/agentcore/skillgrowth/adapter"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/controlplane/models"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/observe"
)

type iterateGateway struct {
	engine  *iterate.Engine
	bridged int
}

func (g *iterateGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *iterateGateway) ApprovalManager() *approval.Manager { return nil }

func (g *iterateGateway) AuditChain() *audit.Chain { return nil }

func (g *iterateGateway) AuditTrail() *audit.Trail { return nil }

func (g *iterateGateway) BotManager() *bots.Manager { return nil }

func (g *iterateGateway) InboxStore() *inbox.Store { return nil }

func (g *iterateGateway) ShellPolicy() *tools.ShellExecPolicy { return nil }

func (g *iterateGateway) TenantManager() *tenant.Manager { return nil }

func (g *iterateGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *iterateGateway) ToolsManager() *tools.ProcessManager { return nil }

func (g *iterateGateway) TrustTracker() *trust.Tracker { return nil }

func (g *iterateGateway) RoleOf(ctx context.Context) string { return "user" }

func (g *iterateGateway) ReviewGate() *review.Gate { return nil }

func (g *iterateGateway) Distiller() *distill.Distiller { return nil }

func (g *iterateGateway) SkillGrowDetector() *adapter.Detector { return nil }

func (g *iterateGateway) IterateEngine() *iterate.Engine { return g.engine }

func (g *iterateGateway) OutputDir() string { return "" }

func (g *iterateGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *iterateGateway) MetricsPrometheus() string { return "" }

func (g *iterateGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *iterateGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *iterateGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func (g *iterateGateway) ModelManager() *models.Manager { return nil }

func (g *iterateGateway) ProviderModels() []models.ProviderModel { return nil }

func (g *iterateGateway) DeleteProviderModel(id string) bool { return false }

func (g *iterateGateway) UsageSnapshot(ctx context.Context) any { return nil }

func (g *iterateGateway) SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64) {
}

func TestIterateRoutesAreNative(t *testing.T) {
	gateway := &iterateGateway{engine: iterate.NewEngine(iterate.Config{Enabled: true, DataDir: t.TempDir()})}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/api/iterate/proposals", "/api/iterate/approve", "/api/iterate/reject", "/api/iterate/trigger", "/api/iterate/status"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/api/iterate/status"](rec, httptest.NewRequest(http.MethodGet, "/api/iterate/status", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Fatalf("iterate status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("iterate route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestIterateApproveRequiresID(t *testing.T) {
	h := NewHandler(&iterateGateway{engine: iterate.NewEngine(iterate.Config{Enabled: true, DataDir: t.TempDir()})})
	rec := httptest.NewRecorder()
	h.handleIterateApprove(rec, httptest.NewRequest(http.MethodPost, "/api/iterate/approve", bytes.NewBufferString(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("approve status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
