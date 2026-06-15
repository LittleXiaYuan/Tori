package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	controlplanepack "yunque-agent/internal/packs/controlplane"
	"yunque-agent/pkg/packruntime"
)

// newTestGatewayWithControlPlanePack builds a gateway hosting only the
// control-plane pack at the requested status, so its governance route gating can
// be verified in isolation.
func newTestGatewayWithControlPlanePack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if _, err := registry.Install(packruntime.Manifest{
		ID:           controlplanepack.PackID,
		Name:         "Control Plane",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Routes: controlplanepack.Paths},
	}, "test"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusDisabled {
		if _, err := registry.Disable(controlplanepack.PackID); err != nil {
			t.Fatalf("Disable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.RegisterBackendPack(controlplanepack.NewHandler(gw))
	return gw, tm
}

// TestControlPlanePackRouteGating verifies the migrated governance surface is
// owned by the control-plane pack: the auth gate fires before the enable gate
// (enabled + no auth → 401), and disabling the pack removes the surface
// (disabled + authed → 404). Both checks are gate-level and never invoke the
// real governance handler.
func TestControlPlanePackRouteGating(t *testing.T) {
	// Enabled but unauthenticated → 401.
	gw, _ := newTestGatewayWithControlPlanePack(t, packruntime.PackStatusEnabled)
	req := httptest.NewRequest(http.MethodGet, "/v1/audit/tail", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("enabled+no-auth: expected 401, got %d", w.Code)
	}

	// Disabled but authenticated → 404.
	gwD, tmD := newTestGatewayWithControlPlanePack(t, packruntime.PackStatusDisabled)
	key := tmD.Register("cp-org").APIKey
	reqD := httptest.NewRequest(http.MethodGet, "/v1/audit/tail", nil)
	reqD.Header.Set("X-API-Key", key)
	wD := httptest.NewRecorder()
	gwD.ServeHTTP(wD, reqD)
	if wD.Code != http.StatusNotFound {
		t.Fatalf("disabled+authed: expected 404, got %d", wD.Code)
	}
}
