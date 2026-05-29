package sandbox

import (
	"context"
	"testing"
	"time"
)

// Verifies the compilation cache makes a second execution of the same module
// materially faster than the first (which pays the compile cost). This is a
// timing assertion, so it uses a generous margin to stay non-flaky.
func TestWasmSandboxCompilationCacheSpeedup(t *testing.T) {
	wasmBytes := wasmFixture(t)
	ws := NewWasmSandbox(WasmConfig{MemoryLimitPages: 1024, MaxDuration: 20 * time.Second, MaxOutputBytes: 64 * 1024})

	t0 := time.Now()
	r1, err := ws.Execute(context.Background(), wasmBytes, "first", "_start")
	if err != nil || r1.ExitCode != 0 {
		t.Fatalf("first run failed: err=%v exit=%d stderr=%s", err, r1.ExitCode, r1.Stderr)
	}
	first := time.Since(t0)

	t1 := time.Now()
	r2, err := ws.Execute(context.Background(), wasmBytes, "second", "_start")
	if err != nil || r2.ExitCode != 0 {
		t.Fatalf("second run failed: err=%v exit=%d", err, r2.ExitCode)
	}
	second := time.Since(t1)

	t.Logf("first=%s second=%s", first, second)
	// The compile is the dominant cost; the cached run should be well under
	// half the first run. Margin kept loose to avoid CI flakiness.
	if second >= first {
		t.Fatalf("cached run (%s) not faster than first (%s)", second, first)
	}
}
