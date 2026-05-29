package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
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

// Full delivery slice: build a real .yqpack carrying a wasm module, serve it
// over HTTP, drive it through the actual /v1/packs/install endpoint with
// download+extract, then confirm the wasm-backed route goes live and disable
// gates it. This exercises packaging, download, extraction, and dynamic mount
// through the real HTTP handlers.
//
// The pack is unsigned: the install endpoint currently holds no trust root, so
// signed packs would fail closed at verification. Wiring a trust root into the
// gateway's install path is a tracked follow-up (see slice notes); this test
// proves the delivery mechanics independent of that gap.
func TestPackInstallEndpointDeliversWasmPack(t *testing.T) {
	wasmBytes := wasmFixture(t)
	moduleSHA := sha256.Sum256(wasmBytes)

	// Build a pack source dir: pack.json + module.wasm.
	packDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(packDir, "module.wasm"), wasmBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := packruntime.Manifest{
		ID:           "yunque.pack.hello-e2e",
		Name:         "Hello E2E",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"hello.ping"},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodPost, Path: "/v1/hello-e2e/ping", Entrypoint: "_start"},
			},
			Runtime: &packruntime.BackendRuntime{
				Type:   packruntime.RuntimeTypeWasm,
				Module: "module.wasm",
				SHA256: hex.EncodeToString(moduleSHA[:]),
			},
		},
	}
	if err := packruntime.SaveManifest(filepath.Join(packDir, "pack.json"), manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}

	// Pack into a deterministic .yqpack and compute its artifact sha.
	outDir := t.TempDir()
	yqpackPath := filepath.Join(outDir, "hello-e2e.yqpack")
	if _, err := packruntime.PackToYqpack(packDir, yqpackPath); err != nil {
		t.Fatalf("PackToYqpack: %v", err)
	}
	yqpackBytes, err := os.ReadFile(yqpackPath)
	if err != nil {
		t.Fatal(err)
	}
	artifactSHA := sha256.Sum256(yqpackBytes)

	// Serve manifest JSON and the .yqpack over HTTP.
	var manifestJSON []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/hello-e2e.yqpack", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(yqpackBytes)
	})
	mux.HandleFunc("/pack.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(manifestJSON)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Distribution points at the served artifact; publish the manifest JSON.
	manifest.Distribution = packruntime.DistributionManifest{
		PackageURL: srv.URL + "/hello-e2e.yqpack",
		SHA256:     hex.EncodeToString(artifactSHA[:]),
	}
	manifestJSON, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	// Wire a gateway with a fresh registry.
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{})
	apiKey := tm.Register("e2e").APIKey
	gw.SetPackRegistry(registry)

	// Install via the real HTTP endpoint, with download+extract.
	body, _ := json.Marshal(map[string]any{
		"manifest_url": srv.URL + "/pack.json",
		"download":     true,
	})
	w := doRequest(gw, http.MethodPost, "/v1/packs/install", apiKey, string(body))
	if w.Code != http.StatusOK {
		t.Fatalf("install failed: status=%d body=%s", w.Code, w.Body.String())
	}

	// Route should now be live and serve the wasm response.
	resp := doRequest(gw, http.MethodPost, "/v1/hello-e2e/ping", apiKey, "delivered")
	if resp.Code != http.StatusOK {
		t.Fatalf("wasm route not live after install: status=%d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "pong") || !strings.Contains(resp.Body.String(), "delivered") {
		t.Fatalf("unexpected wasm response: %s", resp.Body.String())
	}

	// Disable gates it to 404.
	if _, err := registry.Disable("yunque.pack.hello-e2e"); err != nil {
		t.Fatal(err)
	}
	if g := doRequest(gw, http.MethodPost, "/v1/hello-e2e/ping", apiKey, "x"); g.Code != http.StatusNotFound {
		t.Fatalf("disabled pack should gate to 404, got %d", g.Code)
	}
}

// buildSignedWasmPackServer builds a signed .yqpack carrying the wasm fixture,
// serves it + its manifest over HTTP, and returns the manifest URL. The
// signature lives inside the .yqpack's pack.json (what InstallFromYqpack
// verifies); the served manifest.json adds Distribution for the download step.
func buildSignedWasmPackServer(t *testing.T, id, routePath, publisher, keyID string, priv ed25519.PrivateKey) (manifestURL string, cleanup func()) {
	t.Helper()
	wasmBytes := wasmFixture(t)
	moduleSHA := sha256.Sum256(wasmBytes)

	packDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(packDir, "module.wasm"), wasmBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := packruntime.Manifest{
		ID: id, Name: "Signed Pack", Version: "0.1.0", Optional: true, DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			RouteSpecs: []packruntime.BackendRouteSpec{{Method: http.MethodPost, Path: routePath, Entrypoint: "_start"}},
			Runtime:    &packruntime.BackendRuntime{Type: packruntime.RuntimeTypeWasm, Module: "module.wasm", SHA256: hex.EncodeToString(moduleSHA[:])},
		},
	}
	if err := packruntime.SignManifest(&manifest, priv, publisher, keyID); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := packruntime.SaveManifest(filepath.Join(packDir, "pack.json"), manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}

	yqpackPath := filepath.Join(t.TempDir(), "signed.yqpack")
	if _, err := packruntime.PackToYqpack(packDir, yqpackPath); err != nil {
		t.Fatalf("PackToYqpack: %v", err)
	}
	yqpackBytes, _ := os.ReadFile(yqpackPath)
	artifactSHA := sha256.Sum256(yqpackBytes)

	var manifestJSON []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/signed.yqpack", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(yqpackBytes) })
	mux.HandleFunc("/pack.json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(manifestJSON) })
	srv := httptest.NewServer(mux)

	distManifest := manifest
	distManifest.Distribution = packruntime.DistributionManifest{PackageURL: srv.URL + "/signed.yqpack", SHA256: hex.EncodeToString(artifactSHA[:])}
	manifestJSON, _ = json.Marshal(distManifest)
	return srv.URL + "/pack.json", srv.Close
}

// Trust-root wiring: a signed pack from a trusted publisher installs and its
// route goes live.
func TestPackInstallVerifiesTrustedSignedPack(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifestURL, cleanup := buildSignedWasmPackServer(t, "yunque.pack.signed-ok", "/v1/signed-ok/ping", "trusted-pub", "key-1", priv)
	defer cleanup()

	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	trust := packruntime.NewTrustRoot(t.TempDir())
	if err := trust.AddDiskKey("trusted-pub", "key-1", pub); err != nil {
		t.Fatal(err)
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{})
	apiKey := tm.Register("trust-ok").APIKey
	gw.SetPackRegistry(registry)
	gw.SetPackTrustRoot(trust)

	body, _ := json.Marshal(map[string]any{"manifest_url": manifestURL, "download": true})
	if w := doRequest(gw, http.MethodPost, "/v1/packs/install", apiKey, string(body)); w.Code != http.StatusOK {
		t.Fatalf("trusted signed pack install failed: status=%d body=%s", w.Code, w.Body.String())
	}
	if resp := doRequest(gw, http.MethodPost, "/v1/signed-ok/ping", apiKey, "ok"); resp.Code != http.StatusOK {
		t.Fatalf("signed pack route not live: status=%d body=%s", resp.Code, resp.Body.String())
	}
}

// A signed pack whose publisher key is NOT in the trust root must be rejected
// at install (fail closed), and its route must never go live.
func TestPackInstallRejectsUntrustedSignedPack(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifestURL, cleanup := buildSignedWasmPackServer(t, "yunque.pack.signed-bad", "/v1/signed-bad/ping", "unknown-pub", "key-9", priv)
	defer cleanup()

	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{})
	apiKey := tm.Register("trust-bad").APIKey
	gw.SetPackRegistry(registry)
	gw.SetPackTrustRoot(packruntime.NewTrustRoot(t.TempDir())) // empty: unknown-pub not trusted

	body, _ := json.Marshal(map[string]any{"manifest_url": manifestURL, "download": true})
	if w := doRequest(gw, http.MethodPost, "/v1/packs/install", apiKey, string(body)); w.Code == http.StatusOK {
		t.Fatalf("untrusted signed pack should be rejected, got 200: %s", w.Body.String())
	}
	// Route was never mounted (install failed): no wasm response. It falls
	// through to the SPA catch-all, so assert the wasm handler never ran
	// rather than a specific status.
	if resp := doRequest(gw, http.MethodPost, "/v1/signed-bad/ping", apiKey, "x"); strings.Contains(resp.Body.String(), "pong") {
		t.Fatalf("untrusted pack route must not run wasm, got: %s", resp.Body.String())
	}
}
