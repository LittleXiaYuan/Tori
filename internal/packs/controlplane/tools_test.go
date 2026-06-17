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
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/controlplane/models"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/observe"
)

type toolsGateway struct {
	manager     *tools.ProcessManager
	shellPolicy *tools.ShellExecPolicy
	outputDir   string
	bridged     int
}

func (g *toolsGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *toolsGateway) ApprovalManager() *approval.Manager { return nil }

func (g *toolsGateway) AuditChain() *audit.Chain { return nil }

func (g *toolsGateway) AuditTrail() *audit.Trail { return nil }

func (g *toolsGateway) BotManager() *bots.Manager { return nil }

func (g *toolsGateway) InboxStore() *inbox.Store { return nil }

func (g *toolsGateway) ShellPolicy() *tools.ShellExecPolicy { return g.shellPolicy }

func (g *toolsGateway) TenantManager() *tenant.Manager { return nil }

func (g *toolsGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *toolsGateway) ToolsManager() *tools.ProcessManager { return g.manager }

func (g *toolsGateway) TrustTracker() *trust.Tracker { return nil }

func (g *toolsGateway) RoleOf(ctx context.Context) string { return "user" }

func (g *toolsGateway) OutputDir() string { return g.outputDir }

func (g *toolsGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *toolsGateway) MetricsPrometheus() string { return "" }

func (g *toolsGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *toolsGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *toolsGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func (g *toolsGateway) ModelManager() *models.Manager { return nil }

func (g *toolsGateway) ProviderModels() []models.ProviderModel { return nil }

func (g *toolsGateway) DeleteProviderModel(id string) bool { return false }

func (g *toolsGateway) UsageSnapshot(ctx context.Context) any { return nil }

func (g *toolsGateway) SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64) {
}

func TestToolRoutesAreNative(t *testing.T) {
	gateway := &toolsGateway{manager: tools.NewProcessManager()}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/v1/tools/exec", "/v1/tools/list", "/v1/tools/poll", "/v1/tools/kill"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/v1/tools/list"](rec, httptest.NewRequest(http.MethodGet, "/v1/tools/list", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("tools route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestToolExecDisabledByDefault(t *testing.T) {
	t.Setenv("ENABLE_TOOLS_EXEC", "")
	h := NewHandler(&toolsGateway{manager: tools.NewProcessManager()})
	rec := httptest.NewRecorder()
	h.handleToolExec(rec, httptest.NewRequest(http.MethodPost, "/v1/tools/exec", bytes.NewBufferString(`{"command":"echo hi"}`)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Remote command execution is disabled") {
		t.Fatalf("expected disabled message, got %s", rec.Body.String())
	}
}

func TestToolExecRequiresPolicyUnlessUnrestricted(t *testing.T) {
	t.Setenv("ENABLE_TOOLS_EXEC", "true")
	t.Setenv("TOOLS_EXEC_ALLOW_UNRESTRICTED", "")
	h := NewHandler(&toolsGateway{manager: tools.NewProcessManager()})
	rec := httptest.NewRecorder()
	h.handleToolExec(rec, httptest.NewRequest(http.MethodPost, "/v1/tools/exec", bytes.NewBufferString(`{"command":"echo hi"}`)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Shell execution policy is not configured") {
		t.Fatalf("expected shell policy message, got %s", rec.Body.String())
	}
}

func TestToolPollRequiresSessionID(t *testing.T) {
	h := NewHandler(&toolsGateway{manager: tools.NewProcessManager()})
	rec := httptest.NewRecorder()
	h.handleToolPoll(rec, httptest.NewRequest(http.MethodGet, "/v1/tools/poll", nil))
	if !strings.Contains(rec.Body.String(), "session id required") {
		t.Fatalf("expected session id error, got %s", rec.Body.String())
	}
}
