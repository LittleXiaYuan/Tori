package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// wasmFixtureSrc is a tiny WASI command module: it reads a JSON request
// envelope from stdin and writes a JSON response envelope to stdout. Built at
// test time (Go native wasip1) so no multi-MB binary is committed.
const wasmFixtureSrc = `package main
import ("encoding/json";"io";"os")
func main() {
	in, _ := io.ReadAll(os.Stdin)
	var q struct{ Method, Path, Body string }
	_ = json.Unmarshal(in, &q)
	body, _ := json.Marshal(map[string]any{"pong": true, "method": q.Method, "echo": q.Body})
	resp, _ := json.Marshal(map[string]any{"status": 200, "headers": map[string][]string{"X-Pack": {"hello"}}, "body": string(body)})
	os.Stdout.Write(resp)
}
`

var (
	wasmFixtureOnce  sync.Once
	wasmFixtureBytes []byte
	wasmFixtureErr   error
)

// wasmFixture builds (once) and returns the test WASM module bytes, skipping
// the test if the wasip1 toolchain isn't available.
func wasmFixture(t *testing.T) []byte {
	t.Helper()
	wasmFixtureOnce.Do(func() {
		dir, err := os.MkdirTemp("", "wasmfix")
		if err != nil {
			wasmFixtureErr = err
			return
		}
		src := filepath.Join(dir, "main.go")
		if err := os.WriteFile(src, []byte(wasmFixtureSrc), 0o644); err != nil {
			wasmFixtureErr = err
			return
		}
		out := filepath.Join(dir, "m.wasm")
		cmd := exec.Command("go", "build", "-o", out, src)
		cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
		if b, err := cmd.CombinedOutput(); err != nil {
			wasmFixtureErr = fmt.Errorf("%v: %s", err, b)
			return
		}
		wasmFixtureBytes, wasmFixtureErr = os.ReadFile(out)
	})
	if wasmFixtureErr != nil {
		t.Skipf("wasm fixture unavailable (need go wasip1 toolchain): %v", wasmFixtureErr)
	}
	return wasmFixtureBytes
}
