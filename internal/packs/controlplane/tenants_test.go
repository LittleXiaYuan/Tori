package controlplanepack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/observe"
)

type tenantGateway struct {
	tenants *tenant.Manager
	bridged int
}

func (g *tenantGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *tenantGateway) ApprovalManager() *approval.Manager { return nil }

func (g *tenantGateway) InboxStore() *inbox.Store { return nil }

func (g *tenantGateway) TenantManager() *tenant.Manager { return g.tenants }

func (g *tenantGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *tenantGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *tenantGateway) MetricsPrometheus() string { return "" }

func (g *tenantGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *tenantGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *tenantGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func TestTenantsRouteIsNative(t *testing.T) {
	gateway := &tenantGateway{tenants: tenant.NewManager()}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	if byPath["/v1/tenants"] == nil {
		t.Fatal("tenants route not mounted")
	}
	rec := httptest.NewRecorder()
	byPath["/v1/tenants"](rec, httptest.NewRequest(http.MethodGet, "/v1/tenants", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("tenants route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestTenantsCreateAndList(t *testing.T) {
	h := NewHandler(&tenantGateway{tenants: tenant.NewManager()})
	create := httptest.NewRecorder()
	h.handleTenants(create, httptest.NewRequest(http.MethodPost, "/v1/tenants", bytes.NewBufferString(`{"name":"beta"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", create.Code, create.Body.String())
	}
	var created tenant.Tenant
	if err := json.NewDecoder(create.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Name != "beta" || created.APIKey == "" {
		t.Fatalf("created = %+v, want named tenant with api key", created)
	}

	list := httptest.NewRecorder()
	h.handleTenants(list, httptest.NewRequest(http.MethodGet, "/v1/tenants", nil))
	var out struct {
		Tenants []*tenant.Tenant `json:"tenants"`
		Count   int              `json:"count"`
	}
	if err := json.NewDecoder(list.Body).Decode(&out); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if out.Count != 1 || len(out.Tenants) != 1 || out.Tenants[0].ID != created.ID {
		t.Fatalf("list = %+v, want created tenant", out)
	}
}
