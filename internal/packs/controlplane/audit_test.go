package controlplanepack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

type auditGateway struct {
	chain   *audit.Chain
	trail   *audit.Trail
	bridged int
}

func (g *auditGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *auditGateway) ApprovalManager() *approval.Manager { return nil }

func (g *auditGateway) AuditChain() *audit.Chain { return g.chain }

func (g *auditGateway) AuditTrail() *audit.Trail { return g.trail }

func (g *auditGateway) BotManager() *bots.Manager { return nil }

func (g *auditGateway) InboxStore() *inbox.Store { return nil }

func (g *auditGateway) ShellPolicy() *tools.ShellExecPolicy { return nil }

func (g *auditGateway) TenantManager() *tenant.Manager { return nil }

func (g *auditGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *auditGateway) ToolsManager() *tools.ProcessManager { return nil }

func (g *auditGateway) TrustTracker() *trust.Tracker { return nil }

func (g *auditGateway) RoleOf(ctx context.Context) string { return "user" }

func (g *auditGateway) ReviewGate() *review.Gate { return nil }

func (g *auditGateway) Distiller() *distill.Distiller { return nil }

func (g *auditGateway) SkillGrowDetector() *adapter.Detector { return nil }

func (g *auditGateway) IterateEngine() *iterate.Engine { return nil }

func (g *auditGateway) OutputDir() string { return "" }

func (g *auditGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *auditGateway) MetricsPrometheus() string { return "" }

func (g *auditGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *auditGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *auditGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func (g *auditGateway) ModelManager() *models.Manager { return nil }

func (g *auditGateway) ProviderModels() []models.ProviderModel { return nil }

func (g *auditGateway) DeleteProviderModel(id string) bool { return false }

func (g *auditGateway) UsageSnapshot(ctx context.Context) any { return nil }

func (g *auditGateway) SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64) {
}

func TestAuditRoutesAreNative(t *testing.T) {
	chain, err := audit.NewChain(audit.ChainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	chain.Append(audit.EventChat, "user1", "send", "hello")
	trail := audit.NewTrail(t.TempDir())
	trail.Record(audit.TrailEntry{Timestamp: time.Now(), Operation: "op1", Result: "ok", RiskLevel: "low"})

	gateway := &auditGateway{chain: chain, trail: trail}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/v1/audit/tail", "/v1/audit/verify", "/v1/audit/stats", "/api/audit/trail"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/v1/audit/tail"](rec, httptest.NewRequest(http.MethodGet, "/v1/audit/tail", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"count":1`) {
		t.Fatalf("audit tail status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("audit route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestAuditTrailQuery(t *testing.T) {
	trail := audit.NewTrail(t.TempDir())
	trail.Record(audit.TrailEntry{Timestamp: time.Now(), Operation: "op1", Result: "ok", RiskLevel: "low"})
	h := NewHandler(&auditGateway{trail: trail})
	rec := httptest.NewRecorder()
	h.handleAuditTrail(rec, httptest.NewRequest(http.MethodGet, "/api/audit/trail?type=op1", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"count":1`) {
		t.Fatalf("audit trail status=%d body=%s", rec.Code, rec.Body.String())
	}
}
