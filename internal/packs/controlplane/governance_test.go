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

type governanceGateway struct {
	reviewGate *review.Gate
	trust      *trust.Tracker
	distiller  *distill.Distiller
	skillGrow  *adapter.Detector
	bridged    int
}

func (g *governanceGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *governanceGateway) ApprovalManager() *approval.Manager { return nil }

func (g *governanceGateway) AuditChain() *audit.Chain { return nil }

func (g *governanceGateway) AuditTrail() *audit.Trail { return nil }

func (g *governanceGateway) BotManager() *bots.Manager { return nil }

func (g *governanceGateway) InboxStore() *inbox.Store { return nil }

func (g *governanceGateway) ShellPolicy() *tools.ShellExecPolicy { return nil }

func (g *governanceGateway) TenantManager() *tenant.Manager { return nil }

func (g *governanceGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *governanceGateway) ToolsManager() *tools.ProcessManager { return nil }

func (g *governanceGateway) TrustTracker() *trust.Tracker { return g.trust }

func (g *governanceGateway) RoleOf(ctx context.Context) string { return "user" }

func (g *governanceGateway) ReviewGate() *review.Gate { return g.reviewGate }

func (g *governanceGateway) Distiller() *distill.Distiller { return g.distiller }

func (g *governanceGateway) SkillGrowDetector() *adapter.Detector { return g.skillGrow }

func (g *governanceGateway) OutputDir() string { return "" }

func (g *governanceGateway) MetricsSnapshot() observe.MetricsSnapshot {
	return observe.MetricsSnapshot{}
}

func (g *governanceGateway) MetricsPrometheus() string { return "" }

func (g *governanceGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *governanceGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *governanceGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func (g *governanceGateway) ModelManager() *models.Manager { return nil }

func (g *governanceGateway) ProviderModels() []models.ProviderModel { return nil }

func (g *governanceGateway) DeleteProviderModel(id string) bool { return false }

func (g *governanceGateway) UsageSnapshot(ctx context.Context) any { return nil }

func (g *governanceGateway) SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64) {
}

func TestGovernanceStatusRoutesAreNative(t *testing.T) {
	gateway := &governanceGateway{reviewGate: review.NewGate(), trust: trust.NewTracker("")}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/api/review/status", "/api/skillgrow/patterns"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/api/review/status"](rec, httptest.NewRequest(http.MethodGet, "/api/review/status", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":true`) || !strings.Contains(rec.Body.String(), `"trust_enabled":true`) {
		t.Fatalf("review status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("governance status route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestSkillGrowPatternsEmptyWhenDetectorMissing(t *testing.T) {
	h := NewHandler(&governanceGateway{})
	rec := httptest.NewRecorder()
	h.handleSkillGrowPatterns(rec, httptest.NewRequest(http.MethodGet, "/api/skillgrow/patterns", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"count":0`) {
		t.Fatalf("skillgrow status=%d body=%s", rec.Code, rec.Body.String())
	}
}
