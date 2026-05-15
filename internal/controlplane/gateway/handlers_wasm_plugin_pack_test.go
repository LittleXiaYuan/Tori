package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	wasmpluginpack "yunque-agent/internal/packs/wasmplugin"
	"yunque-agent/pkg/packruntime"
)

func TestWASMPluginPackGateReturnsNotFoundWhenDisabled(t *testing.T) {
	gw, tm := newTestGatewayWithWASMPluginPack(t, packruntime.PackStatusDisabled)
	tenant := tm.Register("wasm-plugin-disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/wasm-plugin/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled WASM Plugin pack should gate status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestWASMPluginPackRoutesStatusWhenEnabled(t *testing.T) {
	gw, tm := newTestGatewayWithWASMPluginPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("wasm-plugin-enabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/wasm-plugin/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "yunque.pack.wasm-plugin") || !strings.Contains(w.Body.String(), "abi_plan_ready") || !strings.Contains(w.Body.String(), "wasm.host_abi.plan") || !strings.Contains(w.Body.String(), "host_abi_execution_gate_ready") || !strings.Contains(w.Body.String(), "host_abi_enforcement_ready") || !strings.Contains(w.Body.String(), "wasm.host_abi.execution_gate") || !strings.Contains(w.Body.String(), "module_integrity_gate_ready") || !strings.Contains(w.Body.String(), "wasm.module.integrity_gate") || !strings.Contains(w.Body.String(), "remote_install_plan_ready") || !strings.Contains(w.Body.String(), "wasm.remote_install.plan") || !strings.Contains(w.Body.String(), "approval_gate_plan_ready") || !strings.Contains(w.Body.String(), "wasm.remote_install.approval_plan") {
		t.Fatalf("enabled WASM Plugin pack should expose status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestWASMPluginPackRouteSpecsGateByMethod(t *testing.T) {
	gw, tm := newTestGatewayWithWASMPluginPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("wasm-plugin-method-gate")

	req := httptest.NewRequest(http.MethodGet, "/v1/wasm-plugin/execute", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/wasm-plugin/execute should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestWASMPluginPackCanInstallLoadAndDryRunExecute(t *testing.T) {
	gw, tm := newTestGatewayWithWASMPluginPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("wasm-plugin-flow")

	body := `{"slug":"calculator","name":"Calculator","module_path":"calculator.wasm","entrypoint":"plugin_exec","permissions":{"ledger_kv":true,"http_fetch":false},"capabilities":["math.add"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins", strings.NewReader(body))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "calculator") {
		t.Fatalf("install plugin status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins/load", strings.NewReader(`{"slug":"calculator"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), "loaded") {
		t.Fatalf("load plugin status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/execute", strings.NewReader(`{"slug":"calculator","input":"{}","dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "permission") || !strings.Contains(w.Body.String(), "plugin_exec") || !strings.Contains(w.Body.String(), "host_abi_plan") || !strings.Contains(w.Body.String(), "host_abi_gate") || !strings.Contains(w.Body.String(), "module_integrity_gate") || !strings.Contains(w.Body.String(), "integrity_gate_ready") || !strings.Contains(w.Body.String(), "execution_gate_ready") || !strings.Contains(w.Body.String(), "enforcement_ready") {
		t.Fatalf("dry-run execute status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/plan", strings.NewReader(`{"slug":"calculator-remote","name":"Calculator Remote","package_url":"https://packs.yunque.local/wasm/calculator-remote.tgz","module_path":"calculator-remote.wasm","sha256":"0123456789abcdef","signature":"sig","public_key_id":"root"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "remote_install_plan_ready") || !strings.Contains(w.Body.String(), "signature-verification.json") {
		t.Fatalf("remote install plan status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/approval/plan", strings.NewReader(`{"slug":"calculator-remote","name":"Calculator Remote","package_url":"https://packs.yunque.local/wasm/calculator-remote.tgz","module_path":"calculator-remote.wasm","sha256":"0123456789abcdef","signature":"sig","public_key_id":"root","requested_by":"operator","reason":"test approval","risk_tier":"high","approvers":["security"]}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval_gate_plan_ready") || !strings.Contains(w.Body.String(), "approval-gate-plan.json") || !strings.Contains(w.Body.String(), "requires_approval") {
		t.Fatalf("remote install approval plan status=%d body=%s", w.Code, w.Body.String())
	}
}

func newTestGatewayWithWASMPluginPack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           wasmpluginpack.PackID,
		Name:         "WASM Plugin Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{
				"/v1/wasm-plugin/status",
				"/v1/wasm-plugin/plugins",
				"/v1/wasm-plugin/plugins/",
				"/v1/wasm-plugin/plugins/load",
				"/v1/wasm-plugin/plugins/unload",
				"/v1/wasm-plugin/execute",
				"/v1/wasm-plugin/remote-install/plan",
				"/v1/wasm-plugin/remote-install/approval/plan",
				"/v1/wasm-plugin/evidence/",
			},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/wasm-plugin/status"},
				{Method: http.MethodGet, Path: "/v1/wasm-plugin/plugins"},
				{Method: http.MethodPost, Path: "/v1/wasm-plugin/plugins"},
				{Method: http.MethodGet, Path: "/v1/wasm-plugin/plugins/"},
				{Method: http.MethodPost, Path: "/v1/wasm-plugin/plugins/load"},
				{Method: http.MethodPost, Path: "/v1/wasm-plugin/plugins/unload"},
				{Method: http.MethodPost, Path: "/v1/wasm-plugin/execute"},
				{Method: http.MethodPost, Path: "/v1/wasm-plugin/remote-install/plan"},
				{Method: http.MethodPost, Path: "/v1/wasm-plugin/remote-install/approval/plan"},
				{Method: http.MethodGet, Path: "/v1/wasm-plugin/evidence/"},
			},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "wasm-plugin", Label: "WASM 插件", Path: "/packs/wasm-plugin"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/wasm-plugin"},
		Update:   packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusEnabled {
		if _, err := registry.Enable(wasmpluginpack.PackID); err != nil {
			t.Fatalf("Enable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.RegisterBackendPack(wasmpluginpack.New(wasmpluginpack.Config{PluginDir: t.TempDir(), DataDir: t.TempDir()}))
	return gw, tm
}
