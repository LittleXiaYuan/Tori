package gateway

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yunque-agent/pkg/packruntime"
)

const uiTestPackID = "yunque.pack.dlc-demo"

// stageUIPack writes a minimal iframe-bundle frontend into the registry's
// installed dir and installs+enables the pack. Returns the gateway.
func stageUIPack(t *testing.T, assetsType string) (*Gateway, *packruntime.Registry) {
	gw, registry, _ := stageUIPackWithKey(t, assetsType)
	return gw, registry
}

func stageUIPackWithKey(t *testing.T, assetsType string) (*Gateway, *packruntime.Registry, string) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	dir := filepath.Join(registry.InstalledDir(uiTestPackID, "0.1.0"), "frontend")
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>dlc</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log('dlc')"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := packruntime.Manifest{
		ID:           uiTestPackID,
		Name:         "DLC Demo Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Frontend: packruntime.FrontendManifest{
			Menus:  []packruntime.FrontendMenu{{Key: "dlc", Label: "DLC", Path: "/packs/dlc-demo"}},
			Routes: []packruntime.FrontendRoute{{Path: "/packs/dlc-demo", Component: "PackDlcHost"}},
			Assets: packruntime.FrontendAssets{Type: assetsType, Entry: "index.html"},
		},
	}
	if _, err := registry.Install(manifest, "test"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := registry.Enable(uiTestPackID); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	apiKey := tm.Register("pack-ui-test").APIKey
	return gw, registry, apiKey
}

func TestPackUIAssetServesBundleWithoutAuth(t *testing.T) {
	gw, _ := stageUIPack(t, packruntime.FrontendAssetsTypeIframeBundle)

	// No API key set: the bundle is public static.
	w := doRequest(gw, http.MethodGet, "/v1/packs/"+uiTestPackID+"/ui/", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "<title>dlc</title>") {
		t.Fatalf("entry not served, body = %q", w.Body.String())
	}
	// Global X-Frame-Options: DENY must be overridden so the shell can frame it.
	if got := w.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options should be removed for bundle, got %q", got)
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors") || !strings.Contains(csp, "connect-src 'none'") {
		t.Fatalf("unexpected bundle CSP: %q", csp)
	}
	if strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("bundle CSP must not forbid framing: %q", csp)
	}
}

func TestPackUIAssetServesNestedAsset(t *testing.T) {
	gw, _ := stageUIPack(t, packruntime.FrontendAssetsTypeIframeBundle)
	w := doRequest(gw, http.MethodGet, "/v1/packs/"+uiTestPackID+"/ui/assets/app.js", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "dlc") {
		t.Fatalf("asset not served: %q", w.Body.String())
	}
}

func TestPackUIAssetRejectsTraversal(t *testing.T) {
	gw, _ := stageUIPack(t, packruntime.FrontendAssetsTypeIframeBundle)
	// Attempt to escape the frontend/ dir up to installed.json.
	w := doRequest(gw, http.MethodGet, "/v1/packs/"+uiTestPackID+"/ui/..%2f..%2f..%2finstalled.json", "", "")
	if w.Code == http.StatusOK {
		t.Fatalf("path traversal must not succeed, got 200: %s", w.Body.String())
	}
}

func TestPackUIAssetDisabledPack404(t *testing.T) {
	gw, registry := stageUIPack(t, packruntime.FrontendAssetsTypeIframeBundle)
	if _, err := registry.Disable(uiTestPackID); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	w := doRequest(gw, http.MethodGet, "/v1/packs/"+uiTestPackID+"/ui/", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled pack should 404, got %d", w.Code)
	}
}

func TestPackUIAssetNonBundle404(t *testing.T) {
	gw, _ := stageUIPack(t, packruntime.FrontendAssetsTypeInline)
	w := doRequest(gw, http.MethodGet, "/v1/packs/"+uiTestPackID+"/ui/", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("inline pack should 404 on /ui, got %d", w.Code)
	}
}

// The dedicated isolation listener serves bundles from its own origin with a
// local-shell frame-ancestors policy, and /v1/packs/ui-origin reports it.
func TestPackUIIsolationListener(t *testing.T) {
	gw, _, apiKey := stageUIPackWithKey(t, packruntime.FrontendAssetsTypeIframeBundle)

	// Disabled by default.
	w := doRequest(gw, http.MethodGet, "/v1/packs/ui-origin", apiKey, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"origin":""`) {
		t.Fatalf("ui-origin (disabled) = %d %s", w.Code, w.Body.String())
	}

	origin, srv, err := gw.StartPackUIServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("StartPackUIServer: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	if !strings.HasPrefix(origin, "http://127.0.0.1:") {
		t.Fatalf("origin = %q", origin)
	}

	// The endpoint now reports the listener origin.
	w = doRequest(gw, http.MethodGet, "/v1/packs/ui-origin", apiKey, "")
	if !strings.Contains(w.Body.String(), origin) {
		t.Fatalf("ui-origin should report %q, got %s", origin, w.Body.String())
	}

	// Real HTTP request against the isolated listener.
	resp, err := http.Get(origin + "/v1/packs/" + uiTestPackID + "/ui/index.html")
	if err != nil {
		t.Fatalf("GET bundle via isolation listener: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "<title>dlc</title>") {
		t.Fatalf("isolated bundle = %d %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "" {
		t.Fatalf("isolated bundle must not carry X-Frame-Options, got %q", got)
	}
	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors http://localhost:*") {
		t.Fatalf("isolated bundle CSP should allow local shells to frame it: %q", csp)
	}
	if !strings.Contains(csp, "connect-src 'none'") {
		t.Fatalf("isolated bundle CSP must keep connect-src 'none': %q", csp)
	}

	// The isolated listener serves ONLY pack UI assets — core routes 404.
	other, err := http.Get(origin + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz via isolation listener: %v", err)
	}
	defer other.Body.Close()
	if other.StatusCode != http.StatusNotFound {
		t.Fatalf("isolation listener must not expose core routes, /healthz = %d", other.StatusCode)
	}
}
