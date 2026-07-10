package controlplanepack

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/models"
	"yunque-agent/internal/agentcore/router"
	"yunque-agent/internal/tori"
)

type providerTestGateway struct {
	toolsGateway
	reg          *llm.ProviderRegistry
	toriStore    *tori.TokenStore
	smartRouter  *router.Router
	execProvider string
}

func (g *providerTestGateway) ProviderRegistry() *llm.ProviderRegistry { return g.reg }

func (g *providerTestGateway) ToriTokenStore() *tori.TokenStore { return g.toriStore }

func (g *providerTestGateway) SmartRouter() *router.Router { return g.smartRouter }

func (g *providerTestGateway) ExecProvider() string { return g.execProvider }

func (g *providerTestGateway) SetExecProvider(id string) { g.execProvider = id }

func TestProviderRoutesAreNative(t *testing.T) {
	reg := llm.NewProviderRegistry(nil)
	if err := reg.Register(llm.ProviderConfig{
		ID:      "direct",
		Type:    llm.ProviderTypeChat,
		BaseURL: "https://example.invalid/v1",
		APIKeys: []string{"sk-test"},
		Model:   "test-model",
		Enabled: true,
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	gateway := &providerTestGateway{reg: reg}
	h := NewHandler(gateway)

	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/api/providers", "/api/providers/test", "/api/providers/enable", "/api/providers/disable", "/api/providers/switch-model", "/api/providers/session", "/api/providers/local/discover", "/api/providers/local/register", "/api/providers/delete", "/api/providers/tori/discover", "/v1/router/stats", "/api/breaker/reset", "/api/providers/exec", "/api/providers/image-gen"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/api/providers"](rec, httptest.NewRequest(http.MethodGet, "/api/providers", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "direct") {
		t.Fatalf("providers status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("provider route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestProviderSessionRejectsUnavailableProvider(t *testing.T) {
	reg := llm.NewProviderRegistry(nil)
	h := NewHandler(&providerTestGateway{reg: reg})

	rec := httptest.NewRecorder()
	h.handleProviderSessionOverride(rec, httptest.NewRequest(http.MethodPost, "/api/providers/session", strings.NewReader(`{"session_id":"s1","provider_id":"missing"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := reg.GetForSession("s1"); got != nil {
		t.Fatalf("session provider should not change, got %#v", got)
	}
}

func TestProviderSessionModeRoundTrip(t *testing.T) {
	reg := llm.NewProviderRegistry(nil)
	h := NewHandler(&providerTestGateway{reg: reg})

	rec := httptest.NewRecorder()
	h.handleProviderSessionOverride(rec, httptest.NewRequest(http.MethodPost, "/api/providers/session", strings.NewReader(`{"session_id":"s1","provider_id":"","mode":"api"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"mode":"api"`) {
		t.Fatalf("expected response to echo mode=api, got %s", rec.Body.String())
	}
	if got := reg.SessionMode("s1"); got != "api" {
		t.Fatalf("expected session mode api, got %q", got)
	}

	rec = httptest.NewRecorder()
	h.handleProviderSessionOverride(rec, httptest.NewRequest(http.MethodPost, "/api/providers/session", strings.NewReader(`{"session_id":"s1","provider_id":"","mode":"xiaoyu"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := reg.SessionMode("s1"); got != "xiaoyu" {
		t.Fatalf("expected session mode xiaoyu after switching back, got %q", got)
	}

	// An unrecognized mode value is ignored, not stored — keeps the field
	// closed to exactly the two known modes.
	rec = httptest.NewRecorder()
	h.handleProviderSessionOverride(rec, httptest.NewRequest(http.MethodPost, "/api/providers/session", strings.NewReader(`{"session_id":"s2","provider_id":"","mode":"bogus"}`)))
	if got := reg.SessionMode("s2"); got != "" {
		t.Fatalf("expected unrecognized mode to be ignored, got %q", got)
	}
}

func TestProviderSessionModeOnlySwitchDoesNotClearProvider(t *testing.T) {
	reg := llm.NewProviderRegistry(nil)
	if err := reg.Register(llm.ProviderConfig{ID: "direct", Type: llm.ProviderTypeChat, BaseURL: "https://example.invalid/v1", APIKeys: []string{"sk"}, Model: "m", Enabled: true}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	h := NewHandler(&providerTestGateway{reg: reg})

	rec := httptest.NewRecorder()
	h.handleProviderSessionOverride(rec, httptest.NewRequest(http.MethodPost, "/api/providers/session", strings.NewReader(`{"session_id":"s1","provider_id":"direct"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := reg.GetForSession("s1"); got == nil || got.Config.ID != "direct" {
		t.Fatalf("expected session provider direct, got %#v", got)
	}

	// A mode-only toggle (empty provider_id alongside a mode) must not wipe
	// the provider override that was just set.
	rec = httptest.NewRecorder()
	h.handleProviderSessionOverride(rec, httptest.NewRequest(http.MethodPost, "/api/providers/session", strings.NewReader(`{"session_id":"s1","provider_id":"","mode":"api"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := reg.GetForSession("s1"); got == nil || got.Config.ID != "direct" {
		t.Fatalf("mode-only switch must preserve the existing provider override, got %#v", got)
	}
	if got := reg.SessionMode("s1"); got != "api" {
		t.Fatalf("expected session mode api, got %q", got)
	}
}

func TestExecProviderRejectsUnavailableProvider(t *testing.T) {
	gateway := &providerTestGateway{reg: llm.NewProviderRegistry(nil)}
	h := NewHandler(gateway)

	rec := httptest.NewRecorder()
	h.handleExecProvider(rec, httptest.NewRequest(http.MethodPost, "/api/providers/exec", strings.NewReader(`{"provider_id":"missing"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gateway.execProvider != "" {
		t.Fatalf("exec provider should not change, got %q", gateway.execProvider)
	}
}

func TestImageGenProviderRoundTrip(t *testing.T) {
	reg := llm.NewProviderRegistry(nil)
	if err := reg.Register(llm.ProviderConfig{
		ID: "gemini-img", Type: llm.ProviderTypeChat, BaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Model: "gemini-2.5-flash-image", Enabled: true, Capabilities: []llm.Capability{llm.CapImageGen},
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	h := NewHandler(&providerTestGateway{reg: reg})

	rec := httptest.NewRecorder()
	h.handleImageGenProvider(rec, httptest.NewRequest(http.MethodPost, "/api/providers/image-gen", strings.NewReader(`{"provider_id":"gemini-img"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := reg.ImageGenProvider(); got != "gemini-img" {
		t.Fatalf("expected pinned gemini-img, got %q", got)
	}

	rec = httptest.NewRecorder()
	h.handleImageGenProvider(rec, httptest.NewRequest(http.MethodGet, "/api/providers/image-gen", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "gemini-img") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestImageGenProviderRejectsUnavailableProvider(t *testing.T) {
	h := NewHandler(&providerTestGateway{reg: llm.NewProviderRegistry(nil)})

	rec := httptest.NewRecorder()
	h.handleImageGenProvider(rec, httptest.NewRequest(http.MethodPost, "/api/providers/image-gen", strings.NewReader(`{"provider_id":"missing"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRouterStatsUsesNativeGatewayAccessor(t *testing.T) {
	modelReg := models.NewRegistry()
	_ = modelReg.Register(models.Model{ModelID: "fast-model", Name: "Fast"})
	smart := router.New(modelReg)
	smart.SetSlot(router.TierFast, "fast-model")
	h := NewHandler(&providerTestGateway{smartRouter: smart})

	rec := httptest.NewRecorder()
	h.handleRouterStats(rec, httptest.NewRequest(http.MethodGet, "/v1/router/stats", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "fast-model") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBreakerResetHandlesMissingRegistry(t *testing.T) {
	h := NewHandler(&providerTestGateway{})
	rec := httptest.NewRecorder()
	h.handleBreakerReset(rec, httptest.NewRequest(http.MethodPost, "/api/breaker/reset", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"reset_count":0`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
