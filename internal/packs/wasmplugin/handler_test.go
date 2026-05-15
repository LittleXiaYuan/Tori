package wasmplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/execution/sandbox"
)

type fakeWasmExecutor struct {
	calls int
	stats map[string]any
}

func (f *fakeWasmExecutor) Execute(ctx context.Context, wasmBytes []byte, stdin string, entryPoint string) (*sandbox.WasmResult, error) {
	f.calls++
	return &sandbox.WasmResult{ExitCode: 0, Stdout: stdin, Duration: "1ms", MemUsed: 1024, Exports: []string{entryPoint}, KVWrites: map[string]string{"last_input": stdin}}, nil
}

func (f *fakeWasmExecutor) Stats() map[string]any {
	if f.stats != nil {
		return f.stats
	}
	return map[string]any{"memory_limit_pages": uint32(1024), "max_duration": "30s"}
}

func TestWASMPluginHandlerRoutesExposePackShellSurface(t *testing.T) {
	h := New(Config{PluginDir: t.TempDir(), DataDir: t.TempDir(), Sandbox: &fakeWasmExecutor{}})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	routes := h.Routes()
	if len(routes) != 7 {
		t.Fatalf("expected 7 WASM plugin routes, got %d", len(routes))
	}
	byPath := map[string][]string{}
	for _, route := range routes {
		methods := append([]string{}, route.Methods...)
		if route.Method != "" {
			methods = append([]string{route.Method}, methods...)
		}
		if route.Path == "" || route.Handler == nil || len(methods) == 0 {
			t.Fatalf("route must declare path, handler and method(s): %#v", route)
		}
		byPath[route.Path] = methods
	}
	expected := map[string][]string{
		"/v1/wasm-plugin/status":         {http.MethodGet},
		"/v1/wasm-plugin/plugins":        {http.MethodGet, http.MethodPost},
		"/v1/wasm-plugin/plugins/":       {http.MethodGet},
		"/v1/wasm-plugin/plugins/load":   {http.MethodPost},
		"/v1/wasm-plugin/plugins/unload": {http.MethodPost},
		"/v1/wasm-plugin/execute":        {http.MethodPost},
		"/v1/wasm-plugin/evidence/":      {http.MethodGet},
	}
	for path, methods := range expected {
		if got, want := strings.Join(byPath[path], ","), strings.Join(methods, ","); got != want {
			t.Fatalf("expected %s methods %s, got %s", path, want, got)
		}
	}
}

func TestWASMPluginInstallLoadDryRunExecuteAndEvidence(t *testing.T) {
	pluginDir := t.TempDir()
	wasmPath := filepath.Join(pluginDir, "calculator.wasm")
	if err := os.WriteFile(wasmPath, []byte("fake wasm bytes"), 0o644); err != nil {
		t.Fatalf("write wasm: %v", err)
	}
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	fake := &fakeWasmExecutor{}
	h := New(Config{PluginDir: pluginDir, DataDir: t.TempDir(), Sandbox: fake, Now: func() time.Time { return now }})

	body := `{"slug":"calculator","name":"Calculator","module_path":"calculator.wasm","entrypoint":"plugin_exec","permissions":{"ledger_kv":true,"http_fetch":false,"max_memory_mb":32,"timeout_seconds":5},"capabilities":["math.add"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins", strings.NewReader(body))
	h.Plugins(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "calculator") || !strings.Contains(w.Body.String(), "sha256") {
		t.Fatalf("install status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins/load", strings.NewReader(`{"slug":"calculator"}`))
	h.Load(w, req)
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), "loaded") {
		t.Fatalf("load status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/execute", strings.NewReader(`{"slug":"calculator","input":"{\"a\":1}","dry_run":true}`))
	h.Execute(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "permission") || fake.calls != 0 {
		t.Fatalf("dry-run execute status=%d calls=%d body=%s", w.Code, fake.calls, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/execute", strings.NewReader(`{"slug":"calculator","input":"hello"}`))
	h.Execute(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "last_input") || fake.calls != 1 {
		t.Fatalf("execute status=%d calls=%d body=%s", w.Code, fake.calls, w.Body.String())
	}
	var execResp struct {
		Result ExecuteResult `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&execResp); err != nil {
		t.Fatalf("decode execute: %v", err)
	}
	if !execResp.Result.Success || execResp.Result.Stdout != "hello" {
		t.Fatalf("unexpected execute result: %#v", execResp.Result)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/wasm-plugin/evidence/calculator", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-wasm-plugin-evidence") || !strings.Contains(w.Body.String(), "permission-plan.json") {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestWASMPluginRejectsAbsoluteModulePath(t *testing.T) {
	h := New(Config{PluginDir: t.TempDir(), DataDir: t.TempDir(), Sandbox: &fakeWasmExecutor{}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins", strings.NewReader(`{"slug":"bad","name":"Bad","module_path":"C:/Windows/System32/bad.wasm"}`))
	h.Plugins(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for absolute module path, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestWASMPluginRejectsTraversalModulePath(t *testing.T) {
	h := New(Config{PluginDir: t.TempDir(), DataDir: t.TempDir(), Sandbox: &fakeWasmExecutor{}})
	for _, modulePath := range []string{"../secret.wasm", "nested/../../secret.wasm", `nested\..\..\secret.wasm`} {
		t.Run(modulePath, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/plugins", strings.NewReader(`{"slug":"bad","name":"Bad","module_path":`+strconv.Quote(modulePath)+`}`))
			h.Plugins(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request for traversal module path, got %d body=%s", w.Code, w.Body.String())
			}
		})
	}
}
