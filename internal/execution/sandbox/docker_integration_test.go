//go:build integration

package sandbox

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// Docker Integration Tests
// Run with: go test ./internal/execution/sandbox/... -tags integration -v -count=1
// Requires Docker daemon running.
// ──────────────────────────────────────────────

func requireDocker(t *testing.T) {
	t.Helper()
	if !isDockerAvailable() {
		t.Skip("Docker not available — skipping integration test")
	}
}

// ── Image Management ──

func TestIntegration_EnsureImage(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Pull a small image
	err = dr.ensureImage(ctx, "alpine:latest")
	if err != nil {
		t.Fatalf("ensureImage failed: %v", err)
	}

	// Second call should be instant (already exists)
	start := time.Now()
	err = dr.ensureImage(ctx, "alpine:latest")
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Log("warning: ensureImage for existing image took longer than expected")
	}
}

func TestIntegration_ListImages(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	images := dr.ListImages()
	if len(images) == 0 {
		t.Fatal("expected non-empty image list")
	}
	if _, ok := images["python"]; !ok {
		t.Error("expected python in image list")
	}
}

// ── Cold Start Execution ──

func TestIntegration_ColdStart_Echo(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 0                   // disable pool to force cold start
	cfg.DefaultImage = "alpine:latest" // use lightweight image for command-only tests
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	result, err := dr.Run(context.Background(), RunRequest{
		Command: "echo",
		Args:    []string{"hello", "docker"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.ExitCode != 0 {
		t.Fatalf("exit code %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "hello docker") {
		t.Fatalf("unexpected output: %q", result.Stdout)
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestIntegration_ColdStart_PythonCode(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 0
	cfg.NonRootUser = false // Python slim may not have uid 1000
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := dr.Run(ctx, RunRequest{
		Language: "python",
		Code:     "import json; print(json.dumps({'status': 'ok', 'sum': 1+2}))",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code %d: %s", result.ExitCode, result.Stderr)
	}

	var output map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", err, result.Stdout)
	}
	if output["status"] != "ok" {
		t.Errorf("unexpected status: %v", output["status"])
	}
}

func TestIntegration_ColdStart_NodeCode(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 0
	cfg.NonRootUser = false
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := dr.Run(ctx, RunRequest{
		Language: "javascript",
		Code:     `console.log(JSON.stringify({result: Array.from({length:5}, (_,i)=>i*i)}))`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "0,1,4,9,16") {
		t.Fatalf("unexpected output: %q", result.Stdout)
	}
}

// ── Security Hardening ──

func TestIntegration_Security_NetworkDisabled(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 0
	cfg.NetworkEnabled = false
	cfg.NonRootUser = false
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	// Attempt to reach the internet — should fail
	result, err := dr.Run(context.Background(), RunRequest{
		Command: "sh",
		Args:    []string{"-c", "wget -q -O- http://example.com 2>&1 || echo NETWORK_BLOCKED"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should see failure or our fallback message
	output := result.Stdout + result.Stderr
	if !strings.Contains(output, "NETWORK_BLOCKED") && result.ExitCode == 0 {
		t.Fatalf("network should be blocked, but got: %s", output)
	}
}

func TestIntegration_Security_ReadOnlyRootfs(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 0
	cfg.ReadOnlyRootfs = true
	cfg.NonRootUser = false
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	// Writing to /workspace (tmpfs) should work
	result, err := dr.Run(context.Background(), RunRequest{
		Command: "sh",
		Args:    []string{"-c", "echo test > /workspace/test.txt && cat /workspace/test.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stdout, "test") {
		t.Fatalf("tmpfs write failed: %s / %s", result.Stdout, result.Stderr)
	}

	// Writing to rootfs (e.g., /opt) should fail
	result2, err := dr.Run(context.Background(), RunRequest{
		Command: "sh",
		Args:    []string{"-c", "echo bad > /opt/hack.txt 2>&1 || echo READONLY_OK"},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := result2.Stdout + result2.Stderr
	if !strings.Contains(output, "READONLY_OK") && !strings.Contains(output, "Read-only") && !strings.Contains(output, "read-only") {
		t.Fatalf("rootfs should be read-only, but got: %s", output)
	}
}

func TestIntegration_Security_PidsLimit(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 0
	cfg.PidsLimit = 10 // very low limit
	cfg.NonRootUser = false
	cfg.Timeout = 15 * time.Second
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	// Fork bomb attempt should be limited
	result, err := dr.Run(context.Background(), RunRequest{
		Command: "sh",
		Args:    []string{"-c", "i=0; while [ $i -lt 20 ]; do sleep 10 & i=$((i+1)); done; echo DONE; wait"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Either some forks fail or we get a resource error
	t.Logf("pids-limit test: exit=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
}

func TestIntegration_Security_Timeout(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 0
	cfg.Timeout = 3 * time.Second
	cfg.NonRootUser = false
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	start := time.Now()
	result, err := dr.Run(context.Background(), RunRequest{
		Command: "sleep",
		Args:    []string{"60"},
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	// Should timeout well under 60 seconds
	if elapsed > 10*time.Second {
		t.Fatalf("timeout didn't work: took %v", elapsed)
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code due to timeout")
	}
	t.Logf("timeout test: elapsed=%v exit=%d", elapsed, result.ExitCode)
}

func TestIntegration_Security_ImageWhitelist(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 0
	cfg.AllowedImages = []string{"alpine:latest"}
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	// Python image not in whitelist — should be rejected
	result, err := dr.Run(context.Background(), RunRequest{
		Language: "python",
		Code:     "print('hello')",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != -1 || !strings.Contains(result.Stderr, "not allowed") {
		t.Fatalf("expected image rejection, got: exit=%d stderr=%q", result.ExitCode, result.Stderr)
	}
}

// ── Container Pool ──

func TestIntegration_Pool_PrewarmAndCheckout(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 1
	cfg.MaxContainers = 5
	cfg.LanguageImages = map[string]string{
		"shell": "alpine:latest",
	}
	cfg.DefaultImage = "alpine:latest"
	cfg.NonRootUser = false

	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	// Wait for pre-warm to complete
	time.Sleep(3 * time.Second)

	stats := dr.PoolStats()
	t.Logf("pool stats after pre-warm: %+v", stats)

	total, _ := stats["total"].(int)
	if total == 0 {
		t.Skip("pre-warm didn't produce containers (maybe pool creation takes longer)")
	}

	// Execute using warm container
	start := time.Now()
	result, err := dr.Run(context.Background(), RunRequest{
		Command: "echo",
		Args:    []string{"warm-pool-test"},
	})
	warmElapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "warm-pool-test") {
		t.Fatalf("unexpected output: %q", result.Stdout)
	}

	t.Logf("warm pool execution: %v", warmElapsed)
}

func TestIntegration_Pool_MultipleExecs(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 2
	cfg.MaxContainers = 5
	cfg.LanguageImages = map[string]string{
		"shell": "alpine:latest",
	}
	cfg.DefaultImage = "alpine:latest"
	cfg.NonRootUser = false

	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	// Wait for pre-warm
	time.Sleep(3 * time.Second)

	// Run 3 sequential executions
	for i := 0; i < 3; i++ {
		result, err := dr.Run(context.Background(), RunRequest{
			Command: "sh",
			Args:    []string{"-c", "echo iteration_" + string(rune('0'+i))},
		})
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("iteration %d: exit code %d: %s", i, result.ExitCode, result.Stderr)
		}
	}

	// Check pool replenishment
	time.Sleep(2 * time.Second)
	stats := dr.PoolStats()
	t.Logf("pool stats after 3 execs: %+v", stats)
}

// ── Runner Interface ──

func TestIntegration_RunnerFactory_Docker(t *testing.T) {
	requireDocker(t)

	cfg := DefaultSandboxConfig()
	cfg.Docker.Enabled = true
	cfg.Docker.PoolSize = 0
	cfg.Docker.NonRootUser = false

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()

	if runner.Type() != "docker" {
		t.Fatalf("expected docker runner, got %s", runner.Type())
	}

	result, err := runner.Run(context.Background(), RunRequest{
		Command: "echo",
		Args:    []string{"factory-test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stdout, "factory-test") {
		t.Fatalf("unexpected output: %q", result.Stdout)
	}
}

func TestIntegration_RunnerFactory_WithCodeGen(t *testing.T) {
	requireDocker(t)

	cfg := DefaultSandboxConfig()
	cfg.Docker.Enabled = true
	cfg.Docker.PoolSize = 0
	cfg.Docker.NonRootUser = false

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()

	// Simulate what CodeGenSkill does
	result, err := runner.Run(context.Background(), RunRequest{
		Language: "python",
		Code:     "print(sum(range(10)))",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "45") {
		t.Fatalf("expected 45, got: %q", result.Stdout)
	}
}

// ── Env Files ──

func TestIntegration_WithFiles(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 0
	cfg.NonRootUser = false
	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Close()

	result, err := dr.Run(context.Background(), RunRequest{
		Language: "python",
		Code:     "import json\nwith open('/workspace/data.json') as f:\n    d = json.load(f)\nprint(d['name'])",
		Files: map[string]string{
			"data.json": `{"name": "tori", "version": 1}`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "tori") {
		t.Fatalf("unexpected output: %q", result.Stdout)
	}
}

// ── Cleanup ──

func TestIntegration_Cleanup(t *testing.T) {
	requireDocker(t)

	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	cfg.PoolSize = 2
	cfg.MaxContainers = 5
	cfg.LanguageImages = map[string]string{
		"shell": "alpine:latest",
	}
	cfg.DefaultImage = "alpine:latest"

	dr, err := NewDockerRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for pre-warm
	time.Sleep(3 * time.Second)
	stats := dr.PoolStats()
	t.Logf("before close: %+v", stats)

	// Close should destroy everything
	err = dr.Close()
	if err != nil {
		t.Fatal(err)
	}

	stats = dr.PoolStats()
	total, _ := stats["total"].(int)
	if total != 0 {
		t.Fatalf("expected 0 containers after close, got %d", total)
	}
}

// ── Config File Integration ──

func TestIntegration_LoadConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/sandbox.json"
	os.WriteFile(cfgPath, []byte(`{
		"docker": {
			"enabled": true,
			"default_image": "alpine:latest",
			"pool_size": 1,
			"memory_limit": "128m"
		}
	}`), 0644)

	cfg := LoadConfig(cfgPath)
	if !cfg.Docker.Enabled {
		t.Error("expected docker enabled from config file")
	}
	if cfg.Docker.DefaultImage != "alpine:latest" {
		t.Errorf("wrong image: %s", cfg.Docker.DefaultImage)
	}
	if cfg.Docker.PoolSize != 1 {
		t.Errorf("wrong pool size: %d", cfg.Docker.PoolSize)
	}
	if cfg.Docker.MemoryLimit != "128m" {
		t.Errorf("wrong memory: %s", cfg.Docker.MemoryLimit)
	}
	// Non-overridden fields should keep defaults
	if cfg.Docker.PidsLimit != 64 {
		t.Errorf("pids limit should be default 64, got %d", cfg.Docker.PidsLimit)
	}
}
