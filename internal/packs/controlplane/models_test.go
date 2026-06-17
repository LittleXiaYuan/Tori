package controlplanepack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/controlplane/models"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/observe"
)

type modelsGateway struct {
	manager         *models.Manager
	providers       []models.ProviderModel
	deletedProvider string
	bridged         int
}

func (g *modelsGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	g.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (g *modelsGateway) ApprovalManager() *approval.Manager { return nil }

func (g *modelsGateway) BotManager() *bots.Manager { return nil }

func (g *modelsGateway) InboxStore() *inbox.Store { return nil }

func (g *modelsGateway) ShellPolicy() *tools.ShellExecPolicy { return nil }

func (g *modelsGateway) TenantManager() *tenant.Manager { return nil }

func (g *modelsGateway) TenantOf(ctx context.Context) string { return "tenant-a" }

func (g *modelsGateway) ToolsManager() *tools.ProcessManager { return nil }

func (g *modelsGateway) OutputDir() string { return "" }

func (g *modelsGateway) MetricsSnapshot() observe.MetricsSnapshot { return observe.MetricsSnapshot{} }

func (g *modelsGateway) MetricsPrometheus() string { return "" }

func (g *modelsGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return planner.ModelRuntimeHealth{}
}

func (g *modelsGateway) LLMResponseCacheStats() map[string]any { return nil }

func (g *modelsGateway) SystemStats(ctx context.Context) map[string]any { return map[string]any{} }

func (g *modelsGateway) ModelManager() *models.Manager { return g.manager }

func (g *modelsGateway) ProviderModels() []models.ProviderModel { return g.providers }

func (g *modelsGateway) DeleteProviderModel(id string) bool {
	for _, provider := range g.providers {
		if provider.ID == id || provider.ID+"-"+provider.Model == id {
			g.deletedProvider = provider.ID
			return true
		}
	}
	return false
}

func (g *modelsGateway) UsageSnapshot(ctx context.Context) any { return nil }

func (g *modelsGateway) SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64) {
}

func TestModelRouteIsNative(t *testing.T) {
	gateway := &modelsGateway{manager: models.NewManager()}
	h := NewHandler(gateway)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	if byPath["/v1/models"] == nil {
		t.Fatal("models route not mounted")
	}
	rec := httptest.NewRecorder()
	byPath["/v1/models"](rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("models route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestModelCRUDAndProviderDerivedModels(t *testing.T) {
	manager := models.NewManager()
	gateway := &modelsGateway{
		manager: manager,
		providers: []models.ProviderModel{{
			ID:      "openai",
			Model:   "gpt-4o",
			Type:    "openai",
			BaseURL: "https://api.openai.com/v1",
		}},
	}
	h := NewHandler(gateway)

	list := httptest.NewRecorder()
	h.handleModelsGet(list, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	var listed struct {
		Models []models.Entry `json:"models"`
	}
	if err := json.NewDecoder(list.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Models) != 1 || listed.Models[0].ID != "openai-gpt-4o" {
		t.Fatalf("expected provider-derived model, got %+v", listed)
	}

	create := httptest.NewRecorder()
	h.handleModelsPost(create, httptest.NewRequest(http.MethodPost, "/v1/models", bytes.NewBufferString(`{"id":"custom","model_id":"custom-1","name":"Custom","type":"openai","client_type":"openai"}`)))
	if create.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200: %s", create.Code, create.Body.String())
	}

	delExplicit := httptest.NewRecorder()
	h.handleModelsDelete(delExplicit, httptest.NewRequest(http.MethodDelete, "/v1/models?id=custom", nil))
	if delExplicit.Code != http.StatusOK || gateway.deletedProvider != "" {
		t.Fatalf("explicit delete should not delete provider, status=%d provider=%q", delExplicit.Code, gateway.deletedProvider)
	}

	delProvider := httptest.NewRecorder()
	h.handleModelsDelete(delProvider, httptest.NewRequest(http.MethodDelete, "/v1/models?id=openai-gpt-4o", nil))
	if gateway.deletedProvider != "openai" {
		t.Fatalf("provider-derived delete should delete provider openai, got %q", gateway.deletedProvider)
	}
}

func TestModelDeleteHidesUnknownSyntheticID(t *testing.T) {
	manager := models.NewManager()
	gateway := &modelsGateway{manager: manager}
	h := NewHandler(gateway)

	rec := httptest.NewRecorder()
	h.handleModelsDelete(rec, httptest.NewRequest(http.MethodDelete, "/v1/models?id=local-llama3", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	gateway.providers = []models.ProviderModel{{
		ID:    "local",
		Model: "llama3",
		Type:  "ollama",
	}}
	list := httptest.NewRecorder()
	h.handleModelsGet(list, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	var listed struct {
		Models []models.Entry `json:"models"`
	}
	if err := json.NewDecoder(list.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Models) != 0 {
		t.Fatalf("hidden synthetic model should not be listed, got %+v", listed.Models)
	}
}
