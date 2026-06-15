package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
)

// TestWasmPackTools_RegisterExecuteUnregister exercises the full WASM "tool line"
// end-to-end: a wasm pack declaring backend.toolSpecs gets a sandboxed WasmSkill
// registered into the skill registry on enable, the skill executes by dispatching
// into the sandbox, and the tool is removed on disable. This is the integration
// proof that a downloaded WASM pack can give the agent a callable tool.
func TestWasmPackTools_RegisterExecuteUnregister(t *testing.T) {
	wasm := wasmFixture(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.wasm"), wasm, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(wasm)

	gw, _ := newTestGateway()
	pack := packruntime.InstalledPack{
		Status: packruntime.PackStatusEnabled,
		Manifest: packruntime.Manifest{
			ID:      "yunque.pack.wasm-tool-e2e",
			Version: "1.0.0",
			Backend: packruntime.BackendManifest{
				Runtime: &packruntime.BackendRuntime{
					Type:       packruntime.RuntimeTypeWasm,
					Module:     "m.wasm",
					SHA256:     hex.EncodeToString(sum[:]),
					ABIVersion: packruntime.CurrentABIVersion,
				},
				ToolSpecs: []packruntime.BackendToolSpec{
					{Name: "wasm_echo_tool", Description: "echo via wasm"},
				},
			},
		},
	}

	// Enable → tool registered.
	gw.registerWasmPackTools(pack, dir)
	sk, ok := gw.registry.Get("wasm_echo_tool")
	if !ok {
		t.Fatalf("expected wasm_echo_tool registered after registerWasmPackTools")
	}

	// Execute → dispatches into the sandbox, returns the module's response body.
	out, err := sk.Execute(context.Background(), map[string]any{"hi": "there"}, nil)
	if err != nil {
		t.Fatalf("skill Execute: %v", err)
	}
	if !strings.Contains(out, `"pong":true`) {
		t.Fatalf("expected wasm module output, got %q", out)
	}

	// Disable → tool removed.
	gw.unregisterWasmPackTools(pack)
	if _, ok := gw.registry.Get("wasm_echo_tool"); ok {
		t.Fatalf("expected wasm_echo_tool removed after unregisterWasmPackTools")
	}
}
