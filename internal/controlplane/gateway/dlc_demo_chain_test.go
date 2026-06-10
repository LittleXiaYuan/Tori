package gateway

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"yunque-agent/pkg/packruntime"
)

// TestDlcDemoPackInstallEnableServeChain exercises the real DLC chain end to
// end at the HTTP level: build a genuine .yqpack (compiled wasip1 module + the
// committed demo frontend bundle), install it via the registry's yqpack path,
// enable it, mount its routes, then verify both halves the postMessage bridge
// connects — the UI bundle serving (M1) and the backend.call target route.
//
// The only hop not covered here is the in-browser postMessage round trip, which
// is unit-tested in apps/web/src/lib/__tests__/pack-bridge.test.ts.
func TestDlcDemoPackInstallEnableServeChain(t *testing.T) {
	const packID = "yunque.pack.dlc-demo"
	const version = "0.1.0"
	const route = "/v1/dlc-demo/ping"

	wasmBytes := wasmFixture(t) // compiles a wasip1 module; skips if no toolchain
	sum := sha256.Sum256(wasmBytes)
	wasmSHA := hex.EncodeToString(sum[:])

	// Use the committed demo frontend so this guards the real artifact.
	demoIndex := filepath.Join("..", "..", "..", "packs", "official", "dlc-demo-pack", "frontend", "index.html")
	indexHTML, err := os.ReadFile(demoIndex)
	if err != nil {
		t.Fatalf("read demo frontend: %v", err)
	}

	manifest := packruntime.Manifest{
		ID:           packID,
		Name:         "DLC Demo Pack",
		Version:      version,
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"dlc.demo.ping"},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodPost, Path: route, Entrypoint: "_start"},
			},
			Runtime: &packruntime.BackendRuntime{
				Type:   packruntime.RuntimeTypeWasm,
				Module: "module.wasm",
				SHA256: wasmSHA,
			},
		},
		Frontend: packruntime.FrontendManifest{
			Menus:  []packruntime.FrontendMenu{{Key: "dlc-demo", Label: "DLC", Path: "/packs/dlc-demo"}},
			Routes: []packruntime.FrontendRoute{{Path: "/packs/dlc-demo", Component: "PackDlcHost"}},
			Assets: packruntime.FrontendAssets{Type: packruntime.FrontendAssetsTypeIframeBundle, Entry: "index.html"},
		},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}

	yqpackPath := buildYqpack(t, manifest, wasmBytes, indexHTML)

	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if _, err := registry.InstallFromYqpack(yqpackPath, packruntime.InstallOptions{AllowUnsigned: true, Source: "test"}); err != nil {
		t.Fatalf("InstallFromYqpack: %v", err)
	}
	if _, err := registry.Enable(packID); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	apiKey := tm.Register("dlc-demo-test").APIKey
	pack, _ := registry.Get(packID)
	gw.mountWasmPack(pack, registry.InstalledDir(packID, version))

	// Half 1: the UI bundle is served (public static, no auth).
	uiResp := doRequest(gw, http.MethodGet, "/v1/packs/"+packID+"/ui/index.html", "", "")
	if uiResp.Code != http.StatusOK {
		t.Fatalf("GET ui/index.html = %d, body=%s", uiResp.Code, uiResp.Body.String())
	}
	if !bytes.Contains(uiResp.Body.Bytes(), []byte("backend.call")) {
		t.Fatalf("served bundle is not the demo bridge client")
	}
	if uiResp.Header().Get("X-Frame-Options") != "" {
		t.Fatalf("bundle must not carry X-Frame-Options: DENY")
	}

	// Half 2: the backend.call target route runs the wasm module and returns pong.
	pingResp := doRequest(gw, http.MethodPost, route, apiKey, `{"hello":"dlc"}`)
	if pingResp.Code != http.StatusOK {
		t.Fatalf("POST %s = %d, body=%s", route, pingResp.Code, pingResp.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(pingResp.Body.Bytes(), &body); err != nil {
		t.Fatalf("ping body not JSON: %v (raw=%s)", err, pingResp.Body.String())
	}
	if body["pong"] != true {
		t.Fatalf("unexpected wasm response: %v", body)
	}
}

// buildYqpack writes a real .yqpack (zip) with pack.json + the wasm module +
// frontend/index.html and returns its path.
func buildYqpack(t *testing.T, manifest packruntime.Manifest, wasmBytes, indexHTML []byte) string {
	t.Helper()
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeEntry := func(name string, data []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	writeEntry("pack.json", manifestJSON)
	writeEntry("module.wasm", wasmBytes)
	writeEntry("frontend/index.html", indexHTML)
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	path := filepath.Join(t.TempDir(), "dlc-demo.yqpack")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write yqpack: %v", err)
	}
	return path
}
