package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
)

const wasmTestPackID = "yunque.pack.hello"

// stages the hello.wasm fixture into the registry's installed dir for the pack
// and returns its sha256.
func stageWasmPack(t *testing.T, registry *packruntime.Registry, version string) string {
	t.Helper()
	data := wasmFixture(t)
	dir := registry.InstalledDir(wasmTestPackID, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "module.wasm"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func wasmPackManifest(version, sha string) packruntime.Manifest {
	return packruntime.Manifest{
		ID:           wasmTestPackID,
		Name:         "Hello WASM Pack",
		Version:      version,
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"hello.ping"},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodPost, Path: "/v1/hello/ping", Entrypoint: "_start"},
			},
			Runtime: &packruntime.BackendRuntime{
				Type:   packruntime.RuntimeTypeWasm,
				Module: "module.wasm",
				SHA256: sha,
			},
		},
	}
}

func newWasmPackGateway(t *testing.T, version string) (*Gateway, *packruntime.Registry, string) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	sha := stageWasmPack(t, registry, version)
	if _, err := registry.Install(wasmPackManifest(version, sha), "test"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	apiKey := tm.Register("wasm-pack-test").APIKey
	return gw, registry, apiKey
}

func doRequest(gw *Gateway, method, path, apiKey, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("X-API-Key", apiKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, r)
	return w
}

func TestDynamicWasmRouteEndToEnd(t *testing.T) {
	gw, registry, apiKey := newWasmPackGateway(t, "0.1.0")
	pack, _ := registry.Get(wasmTestPackID)
	gw.mountWasmPack(pack, registry.InstalledDir(wasmTestPackID, "0.1.0"))

	w := doRequest(gw, http.MethodPost, "/v1/hello/ping", apiKey, "ping-body")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (raw=%s)", err, w.Body.String())
	}
	if body["pong"] != true || body["echo"] != "ping-body" {
		t.Errorf("unexpected wasm response: %v", body)
	}
}

func TestDynamicWasmRouteRequiresAuth(t *testing.T) {
	gw, registry, _ := newWasmPackGateway(t, "0.1.0")
	pack, _ := registry.Get(wasmTestPackID)
	gw.mountWasmPack(pack, registry.InstalledDir(wasmTestPackID, "0.1.0"))

	r := httptest.NewRequest(http.MethodPost, "/v1/hello/ping", strings.NewReader("x"))
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, r) // no API key
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}
}

func TestDynamicWasmRouteDisabledGate(t *testing.T) {
	gw, registry, apiKey := newWasmPackGateway(t, "0.1.0")
	pack, _ := registry.Get(wasmTestPackID)
	gw.mountWasmPack(pack, registry.InstalledDir(wasmTestPackID, "0.1.0"))

	if _, err := registry.Disable(wasmTestPackID); err != nil {
		t.Fatal(err)
	}
	// Route is still mounted, but the enabled-gate must turn it into a 404.
	w := doRequest(gw, http.MethodPost, "/v1/hello/ping", apiKey, "x")
	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled wasm pack should gate to 404, got %d", w.Code)
	}
}

func TestDynamicWasmRouteUnmount(t *testing.T) {
	gw, registry, apiKey := newWasmPackGateway(t, "0.1.0")
	pack, _ := registry.Get(wasmTestPackID)
	gw.mountWasmPack(pack, registry.InstalledDir(wasmTestPackID, "0.1.0"))
	gw.unmountPack(wasmTestPackID)

	// After unmount the dynamic table misses and falls through to the mux,
	// which has no such route -> 404 from the SPA/catch-all is acceptable;
	// the key assertion is the wasm handler no longer runs (no pong body).
	w := doRequest(gw, http.MethodPost, "/v1/hello/ping", apiKey, "x")
	if strings.Contains(w.Body.String(), "pong") {
		t.Fatalf("unmounted route still served wasm response: %s", w.Body.String())
	}
}

func TestDynamicDispatchFallthrough(t *testing.T) {
	gw, _, apiKey := newWasmPackGateway(t, "0.1.0")
	// No mount: a known static route must still work through dynamicDispatch.
	w := doRequest(gw, http.MethodGet, "/v1/packs", apiKey, "")
	if w.Code == http.StatusNotFound {
		t.Fatalf("static /v1/packs route should pass through dynamicDispatch, got 404")
	}
}

// Exercises the full OnChange wiring: SetPackRegistry subscribes, and an
// enable event auto-mounts the wasm route without any manual mountWasmPack.
func TestWireWasmPacksAutoMountsOnEnable(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	sha := stageWasmPack(t, registry, "0.1.0")
	m := wasmPackManifest("0.1.0", sha)
	m.DefaultState = "disabled" // install disabled; no route yet
	if _, err := registry.Install(m, "test"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	gw, tm := newTestGatewayWithConfig(GatewayConfig{})
	apiKey := tm.Register("wire-test").APIKey
	gw.SetPackRegistry(registry) // subscribes + scans (nothing enabled yet)

	// Disabled → gated 404.
	if w := doRequest(gw, http.MethodPost, "/v1/hello/ping", apiKey, "x"); w.Code != http.StatusNotFound {
		t.Fatalf("disabled pack should not route, got %d", w.Code)
	}

	// Enable fires OnChange → auto-mount.
	if _, err := registry.Enable(wasmTestPackID); err != nil {
		t.Fatal(err)
	}
	w := doRequest(gw, http.MethodPost, "/v1/hello/ping", apiKey, "wired")
	if w.Code != http.StatusOK {
		t.Fatalf("enabled pack should route via OnChange wiring, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "pong") {
		t.Errorf("expected wasm pong response, got %s", w.Body.String())
	}

	// Disable fires OnChange → unmount (gated 404 again).
	if _, err := registry.Disable(wasmTestPackID); err != nil {
		t.Fatal(err)
	}
	if w := doRequest(gw, http.MethodPost, "/v1/hello/ping", apiKey, "x"); w.Code != http.StatusNotFound {
		t.Fatalf("disabled-again pack should not route, got %d", w.Code)
	}
}
