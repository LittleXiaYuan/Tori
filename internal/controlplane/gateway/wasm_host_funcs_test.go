package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/packruntime"
)

func hostFuncNames(funcs []sandbox.ModuleHostFunc) map[string]bool {
	out := make(map[string]bool, len(funcs))
	for _, f := range funcs {
		out[f.Name] = true
	}
	return out
}

func TestBuildWasmHostFuncsGating(t *testing.T) {
	g := &Gateway{}

	if got := g.buildWasmHostFuncs("p", nil); len(got) != 0 {
		t.Fatalf("no permissions should expose no host funcs, got %d", len(got))
	}
	if got := g.buildWasmHostFuncs("p", []string{"dlc:demo"}); len(got) != 0 {
		t.Fatalf("unrelated permission should not expose host funcs, got %d", len(got))
	}
	got := g.buildWasmHostFuncs("p", []string{PermNetFetch})
	if len(got) != 1 || got[0].Name != "http_fetch" {
		t.Fatalf("net:fetch should expose http_fetch, got %#v", got)
	}
	if got[0].Params != 4 || got[0].Results != 1 {
		t.Fatalf("http_fetch signature mismatch: %#v", got[0])
	}

	// memory:read only exports memory_search when an orchestrator is wired.
	if names := hostFuncNames(g.buildWasmHostFuncs("p", []string{PermMemoryRead})); names["memory_search"] {
		t.Fatal("memory_search must not export without an orchestrator provider")
	}

	// ledger perms only export when a KV store is wired.
	if names := hostFuncNames(g.buildWasmHostFuncs("p", []string{PermLedgerRead, PermLedgerWrite})); names["ledger_get"] || names["ledger_set"] {
		t.Fatal("ledger_* must not export without a KV provider")
	}
	g.wasmPackKV = &fakePackKV{store: map[string][]byte{}}
	names := hostFuncNames(g.buildWasmHostFuncs("p", []string{PermLedgerRead, PermLedgerWrite}))
	if !names["ledger_get"] || !names["ledger_set"] {
		t.Fatalf("ledger_* should export with a KV provider, got %v", names)
	}

	// llm:call (ABI v2) only exports llm_chat when an LLM provider is wired.
	if names := hostFuncNames(g.buildWasmHostFuncs("p", []string{PermLLMCall})); names["llm_chat"] {
		t.Fatal("llm_chat must not export without an llm provider")
	}
	g.llmCall = func(_ context.Context, _, _ string) (string, error) { return "ok", nil }
	if names := hostFuncNames(g.buildWasmHostFuncs("p", []string{PermLLMCall})); !names["llm_chat"] {
		t.Fatal("llm_chat should export with llm:call permission + provider")
	}
}

type fakePackKV struct {
	store map[string][]byte
}

func (f *fakePackKV) Put(_ context.Context, key string, value any) error {
	s, _ := value.(string)
	f.store[key] = []byte(s)
	return nil
}

func (f *fakePackKV) GetRaw(_ context.Context, key string) ([]byte, error) {
	if v, ok := f.store[key]; ok {
		return v, nil
	}
	return nil, nil
}

// fetchWasmSrc is a WASI module that imports env.http_fetch, calls it with the
// URL passed in the request body, and echoes the host response envelope back.
const fetchWasmSrc = `package main

import (
	"encoding/json"
	"io"
	"os"
	"unsafe"
)

//go:wasmimport env http_fetch
func http_fetch(reqPtr, reqLen, respPtr, respCap uint32) int32

var reqBuf [4096]byte
var respBuf [65536]byte

func main() {
	in, _ := io.ReadAll(os.Stdin)
	var q struct{ Method, Path, Body string }
	_ = json.Unmarshal(in, &q)
	reqJSON, _ := json.Marshal(map[string]string{"url": q.Body, "method": "GET"})
	n := copy(reqBuf[:], reqJSON)
	reqPtr := uint32(uintptr(unsafe.Pointer(&reqBuf[0])))
	respPtr := uint32(uintptr(unsafe.Pointer(&respBuf[0])))
	got := http_fetch(reqPtr, uint32(n), respPtr, uint32(len(respBuf)))
	if got < 0 {
		out, _ := json.Marshal(map[string]any{"status": 500, "body": "buffer too small"})
		os.Stdout.Write(out)
		return
	}
	out, _ := json.Marshal(map[string]any{"status": 200, "body": string(respBuf[:got])})
	os.Stdout.Write(out)
}
`

var (
	fetchWasmOnce  sync.Once
	fetchWasmBytes []byte
	fetchWasmErr   error
)

func fetchWasmFixture(t *testing.T) []byte {
	t.Helper()
	fetchWasmOnce.Do(func() {
		dir, err := os.MkdirTemp("", "fetchwasm")
		if err != nil {
			fetchWasmErr = err
			return
		}
		src := filepath.Join(dir, "main.go")
		if err := os.WriteFile(src, []byte(fetchWasmSrc), 0o644); err != nil {
			fetchWasmErr = err
			return
		}
		out := filepath.Join(dir, "m.wasm")
		cmd := exec.Command("go", "build", "-o", out, src)
		cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
		if b, err := cmd.CombinedOutput(); err != nil {
			fetchWasmErr = fmt.Errorf("%v: %s", err, b)
			return
		}
		fetchWasmBytes, fetchWasmErr = os.ReadFile(out)
	})
	if fetchWasmErr != nil {
		t.Skipf("fetch wasm fixture unavailable (need go wasip1 toolchain): %v", fetchWasmErr)
	}
	return fetchWasmBytes
}

const fetchPackID = "yunque.pack.fetch-demo"

func newFetchPackGateway(t *testing.T, permissions []string) (*Gateway, string) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	data := fetchWasmFixture(t)
	dir := registry.InstalledDir(fetchPackID, "0.1.0")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "module.wasm"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	manifest := packruntime.Manifest{
		ID:           fetchPackID,
		Name:         "Fetch Demo",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"fetch.demo"},
			Permissions:  permissions,
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodPost, Path: "/v1/fetch-demo/run", Entrypoint: "_start"},
			},
			Runtime: &packruntime.BackendRuntime{
				Type:   packruntime.RuntimeTypeWasm,
				Module: "module.wasm",
				SHA256: hex.EncodeToString(sum[:]),
			},
		},
	}
	if _, err := registry.Install(manifest, "test"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := registry.Enable(fetchPackID); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	apiKey := tm.Register("fetch-pack-test").APIKey
	pack, _ := registry.Get(fetchPackID)
	gw.mountWasmPack(pack, registry.InstalledDir(fetchPackID, "0.1.0"))
	return gw, apiKey
}

// With net:fetch permission, http_fetch is exported and runs; a loopback target
// is rejected by the SSRF guard, surfaced as an error envelope (not a crash).
func TestWasmHostFetchPermittedSSRFGuard(t *testing.T) {
	gw, apiKey := newFetchPackGateway(t, []string{PermNetFetch})
	w := doRequest(gw, http.MethodPost, "/v1/fetch-demo/run", apiKey, `http://127.0.0.1:1/secret`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	// The module echoes the host response envelope inside body; the SSRF guard
	// should have refused the loopback dial.
	if !strings.Contains(w.Body.String(), "private/loopback") {
		t.Fatalf("expected SSRF guard to block loopback, got: %s", w.Body.String())
	}
}

// Without the permission, http_fetch is never exported, so the module (which
// imports it) fails to instantiate — the capability is truly absent.
func TestWasmHostFetchDeniedWithoutPermission(t *testing.T) {
	gw, apiKey := newFetchPackGateway(t, nil)
	w := doRequest(gw, http.MethodPost, "/v1/fetch-demo/run", apiKey, `http://example.com/x`)
	if w.Code == http.StatusOK {
		t.Fatalf("module importing http_fetch must fail without net:fetch permission, got 200: %s", w.Body.String())
	}
}

// ledgerWasmSrc imports ledger_set + ledger_get and round-trips a value through
// the host's pack-scoped KV.
const ledgerWasmSrc = `package main

import (
	"encoding/json"
	"os"
	"unsafe"
)

//go:wasmimport env ledger_set
func ledger_set(reqPtr, reqLen, respPtr, respCap uint32) int32

//go:wasmimport env ledger_get
func ledger_get(reqPtr, reqLen, respPtr, respCap uint32) int32

var b1 [4096]byte
var b2 [4096]byte

func ptr(p *byte) uint32 { return uint32(uintptr(unsafe.Pointer(p))) }

func main() {
	setReq, _ := json.Marshal(map[string]string{"key": "k", "value": "hello-ledger"})
	n := copy(b1[:], setReq)
	ledger_set(ptr(&b1[0]), uint32(n), ptr(&b2[0]), uint32(len(b2)))

	getReq, _ := json.Marshal(map[string]string{"key": "k"})
	n2 := copy(b1[:], getReq)
	got := ledger_get(ptr(&b1[0]), uint32(n2), ptr(&b2[0]), uint32(len(b2)))
	body := ""
	if got >= 0 {
		body = string(b2[:got])
	}
	out, _ := json.Marshal(map[string]any{"status": 200, "body": body})
	os.Stdout.Write(out)
}
`

var (
	ledgerWasmOnce  sync.Once
	ledgerWasmBytes []byte
	ledgerWasmErr   error
)

func ledgerWasmFixture(t *testing.T) []byte {
	t.Helper()
	ledgerWasmOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ledgerwasm")
		if err != nil {
			ledgerWasmErr = err
			return
		}
		src := filepath.Join(dir, "main.go")
		if err := os.WriteFile(src, []byte(ledgerWasmSrc), 0o644); err != nil {
			ledgerWasmErr = err
			return
		}
		out := filepath.Join(dir, "m.wasm")
		cmd := exec.Command("go", "build", "-o", out, src)
		cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
		if b, err := cmd.CombinedOutput(); err != nil {
			ledgerWasmErr = fmt.Errorf("%v: %s", err, b)
			return
		}
		ledgerWasmBytes, ledgerWasmErr = os.ReadFile(out)
	})
	if ledgerWasmErr != nil {
		t.Skipf("ledger wasm fixture unavailable (need go wasip1 toolchain): %v", ledgerWasmErr)
	}
	return ledgerWasmBytes
}

// A pack with ledger:read+write and a wired KV provider can persist and read
// back its own data through the bridge end to end.
func TestWasmHostLedgerRoundTrip(t *testing.T) {
	const packID = "yunque.pack.ledger-demo"
	data := ledgerWasmFixture(t)

	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	dir := registry.InstalledDir(packID, "0.1.0")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "module.wasm"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	manifest := packruntime.Manifest{
		ID:           packID,
		Name:         "Ledger Demo",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"ledger.demo"},
			Permissions:  []string{PermLedgerRead, PermLedgerWrite},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodPost, Path: "/v1/ledger-demo/run", Entrypoint: "_start"},
			},
			Runtime: &packruntime.BackendRuntime{
				Type:   packruntime.RuntimeTypeWasm,
				Module: "module.wasm",
				SHA256: hex.EncodeToString(sum[:]),
			},
		},
	}
	if _, err := registry.Install(manifest, "test"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := registry.Enable(packID); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	apiKey := tm.Register("ledger-pack-test").APIKey
	kv := &fakePackKV{store: map[string][]byte{}}
	gw.wasmPackKV = kv
	pack, _ := registry.Get(packID)
	gw.mountWasmPack(pack, registry.InstalledDir(packID, "0.1.0"))

	w := doRequest(gw, http.MethodPost, "/v1/ledger-demo/run", apiKey, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello-ledger") {
		t.Fatalf("ledger round-trip failed, body = %s", w.Body.String())
	}
	// The value must be persisted under the pack+tenant namespace, not the raw key.
	foundNS := false
	for k := range kv.store {
		if strings.HasPrefix(k, packID+":") && strings.HasSuffix(k, ":k") {
			foundNS = true
		}
	}
	if !foundNS {
		t.Fatalf("expected key namespaced as %s:<tenant>:k, store keys = %v", packID, kv.store)
	}
}
