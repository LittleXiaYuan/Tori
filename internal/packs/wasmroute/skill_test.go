package wasmroute

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/skills"
)

// TestWasmSkillDispatchesToSandbox proves the WASM tool line: a WasmSkill runs a
// sandboxed module and returns its output, so a downloaded WASM pack can give the
// agent a callable tool (Tier 0 microkernel keystone).
func TestWasmSkillDispatchesToSandbox(t *testing.T) {
	wasmBytes := wasmFixture(t)
	dir := t.TempDir()
	modPath := filepath.Join(dir, "m.wasm")
	if err := os.WriteFile(modPath, wasmBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	sb := sandbox.NewWasmSandbox(sandbox.DefaultWasmConfig())
	var sk skills.Skill = NewSkill("wasm_echo", "echo via wasm", nil, modPath, "", "", sb)

	if sk.Name() != "wasm_echo" {
		t.Fatalf("Name = %q", sk.Name())
	}
	if sk.Parameters() == nil {
		t.Fatalf("expected non-nil params schema")
	}

	out, err := sk.Execute(context.Background(), map[string]any{"hello": "world"}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, `"pong":true`) {
		t.Fatalf("expected wasm module output, got %q", out)
	}
	if !strings.Contains(out, "world") {
		t.Fatalf("expected echoed args in output, got %q", out)
	}
}
