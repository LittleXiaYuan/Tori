package wasmroute

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
	"time"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/packruntime"
)

func newTestSandbox() *sandbox.WasmSandbox {
	return sandbox.NewWasmSandbox(sandbox.WasmConfig{
		MemoryLimitPages: 1024,
		MaxDuration:      20 * time.Second,
		MaxOutputBytes:   256 * 1024,
	})
}

// loads the testdata module and copies it into a temp "installed dir" with the
// given module filename, returning the dir and the module's real sha256.
func stageModule(t *testing.T, moduleName string) (string, string) {
	t.Helper()
	data := wasmFixture(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, moduleName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return dir, hex.EncodeToString(sum[:])
}

func TestBuildRouteHandlerEndToEnd(t *testing.T) {
	dir, sum := stageModule(t, "module.wasm")
	rt := packruntime.BackendRuntime{Type: packruntime.RuntimeTypeWasm, Module: "module.wasm", SHA256: sum}
	spec := packruntime.BackendRouteSpec{Method: "POST", Path: "/v1/hello/ping", Entrypoint: "_start"}

	h := BuildRouteHandler(dir, rt, spec, newTestSandbox())
	req := httptest.NewRequest(http.MethodPost, "/v1/hello/ping", strings.NewReader("payload-xyz"))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Pack"); got != "hello" {
		t.Errorf("X-Pack header = %q, want hello", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body not JSON: %v (raw=%s)", err, rec.Body.String())
	}
	if body["pong"] != true {
		t.Errorf("pong = %v, want true", body["pong"])
	}
	if body["method"] != "POST" {
		t.Errorf("method = %v, want POST", body["method"])
	}
	if body["echo"] != "payload-xyz" {
		t.Errorf("echo = %v, want payload-xyz", body["echo"])
	}
}

func TestBuildRouteHandlerIntegrityGate(t *testing.T) {
	dir, _ := stageModule(t, "module.wasm")
	rt := packruntime.BackendRuntime{Type: packruntime.RuntimeTypeWasm, Module: "module.wasm", SHA256: "deadbeef"}
	spec := packruntime.BackendRouteSpec{Method: "GET", Path: "/v1/hello/ping"}

	h := BuildRouteHandler(dir, rt, spec, newTestSandbox())
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/v1/hello/ping", nil))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 on sha mismatch, got %d", rec.Code)
	}
}

func TestBuildRouteHandlerMissingModule(t *testing.T) {
	rt := packruntime.BackendRuntime{Type: packruntime.RuntimeTypeWasm, Module: "absent.wasm"}
	spec := packruntime.BackendRouteSpec{Method: "GET", Path: "/v1/hello/ping"}
	h := BuildRouteHandler(t.TempDir(), rt, spec, newTestSandbox())
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/v1/hello/ping", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing module, got %d", rec.Code)
	}
}

func TestParseResponseDefaultsStatus(t *testing.T) {
	resp, err := parseResponse(`{"body":"hi"}`)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("status defaulted to %d, want 200", resp.Status)
	}
}

func TestParseResponseRejectsEmpty(t *testing.T) {
	if _, err := parseResponse("   "); err == nil {
		t.Fatal("expected error for empty stdout")
	}
}
