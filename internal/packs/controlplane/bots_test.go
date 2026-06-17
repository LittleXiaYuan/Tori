package controlplanepack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

type botGateway struct {
	manager *bots.Manager
	bridged int
}

func (g *botGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *botGateway) ApprovalManager() *approval.Manager { return nil }

func (g *botGateway) AuditChain() *audit.Chain { return nil }

func (g *botGateway) AuditTrail() *audit.Trail { return nil }

func (g *botGateway) BotManager() *bots.Manager { return g.manager }

func (g *botGateway) InboxStore() *inbox.Store { return nil }

func (g *botGateway) ShellPolicy() *tools.ShellExecPolicy { return nil }

func (g *botGateway) TenantManager() *tenant.Manager { return nil }

func (g *botGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *botGateway) ToolsManager() *tools.ProcessManager { return nil }

func (g *botGateway) TrustTracker() *trust.Tracker { return nil }

func (g *botGateway) RoleOf(ctx context.Context) string { return "user" }

func (g *botGateway) OutputDir() string { return "" }

func (g *botGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *botGateway) MetricsPrometheus() string { return "" }

func (g *botGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *botGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *botGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func (g *botGateway) ModelManager() *models.Manager { return nil }

func (g *botGateway) ProviderModels() []models.ProviderModel { return nil }

func (g *botGateway) DeleteProviderModel(id string) bool { return false }

func (g *botGateway) UsageSnapshot(ctx context.Context) any { return nil }

func (g *botGateway) SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64) {
}

func TestBotRoutesAreNative(t *testing.T) {
	gateway := &botGateway{manager: bots.NewManager()}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/v1/bots", "/v1/bots/detail"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/v1/bots"](rec, httptest.NewRequest(http.MethodGet, "/v1/bots", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("bot route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestBotCRUD(t *testing.T) {
	h := NewHandler(&botGateway{manager: bots.NewManager()})
	create := httptest.NewRecorder()
	h.handleBots(create, httptest.NewRequest(http.MethodPost, "/v1/bots", bytes.NewBufferString(`{"name":"ops","description":"Ops bot","config":{"max_steps":3}}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", create.Code, create.Body.String())
	}
	var created bots.Bot
	if err := json.NewDecoder(create.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID == "" || created.Name != "ops" || !created.IsActive {
		t.Fatalf("created = %+v, want active ops bot", created)
	}

	update := httptest.NewRecorder()
	h.handleBotDetail(update, httptest.NewRequest(http.MethodPut, "/v1/bots/detail?id="+created.ID, bytes.NewBufferString(`{"active":false}`)))
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", update.Code, update.Body.String())
	}
	var updated bots.Bot
	if err := json.NewDecoder(update.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update: %v", err)
	}
	if updated.IsActive {
		t.Fatalf("updated = %+v, want inactive bot", updated)
	}

	list := httptest.NewRecorder()
	h.handleBots(list, httptest.NewRequest(http.MethodGet, "/v1/bots", nil))
	if !bytes.Contains(list.Body.Bytes(), []byte(`"total":1`)) || !bytes.Contains(list.Body.Bytes(), []byte(`"active":0`)) {
		t.Fatalf("expected total=1 active=0, got %s", list.Body.String())
	}

	del := httptest.NewRecorder()
	h.handleBotDetail(del, httptest.NewRequest(http.MethodDelete, "/v1/bots/detail?id="+created.ID, nil))
	if del.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200: %s", del.Code, del.Body.String())
	}
}
