package wasmroute

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/packruntime"
)

// TestWasmSkillFromToolSpec proves the manifest tool-spec → WasmSkill → execute
// path: a backend.toolSpecs entry's fields build a sandboxed agent tool that runs
// the module and returns its output (the WASM "tool line", Tier 0 microkernel).
func TestWasmSkillFromToolSpec(t *testing.T) {
	wasmBytes := wasmFixture(t)
	dir := t.TempDir()
	modPath := filepath.Join(dir, "m.wasm")
	if err := os.WriteFile(modPath, wasmBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	// A manifest tool spec as it would appear in pack.json backend.toolSpecs[].
	spec := packruntime.BackendToolSpec{
		Name:        "echo_tool",
		Description: "echo via wasm",
		Entrypoint:  "",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}

	sb := sandbox.NewWasmSandbox(sandbox.DefaultWasmConfig())
	sk := NewSkill(spec.Name, spec.Description, spec.Parameters, modPath, "", spec.Entrypoint, sb)

	if sk.Name() != "echo_tool" {
		t.Fatalf("Name = %q", sk.Name())
	}
	if sk.Description() != "echo via wasm" {
		t.Fatalf("Description = %q", sk.Description())
	}

	out, err := sk.Execute(context.Background(), map[string]any{"k": "v"}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, `"pong":true`) {
		t.Fatalf("expected module output, got %q", out)
	}
}
