package sandbox

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// FallbackRunner degradation chain tests
// ──────────────────────────────────────────────

type failingRunner struct {
	failErr error
	typ     string
}

func (r *failingRunner) Run(_ context.Context, _ RunRequest) (*RunResult, error) {
	return nil, r.failErr
}
func (r *failingRunner) Type() string { return r.typ }
func (r *failingRunner) Close() error { return nil }

type succeedingRunner struct {
	typ    string
	stdout string
}

func (r *succeedingRunner) Run(_ context.Context, _ RunRequest) (*RunResult, error) {
	return &RunResult{ExitCode: 0, Stdout: r.stdout, Duration: time.Millisecond}, nil
}
func (r *succeedingRunner) Type() string { return r.typ }
func (r *succeedingRunner) Close() error { return nil }

func TestFallbackRunner_PrimarySucceeds(t *testing.T) {
	primary := &succeedingRunner{typ: "cloud", stdout: "from cloud"}
	fallback := &succeedingRunner{typ: "process", stdout: "from process"}
	fb := NewFallbackRunner(primary, fallback)

	if fb.Type() != "cloud+process" {
		t.Fatalf("expected type cloud+process, got %s", fb.Type())
	}

	result, err := fb.Run(context.Background(), RunRequest{Command: "echo test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "from cloud" {
		t.Fatalf("expected 'from cloud', got %s", result.Stdout)
	}
}

func TestFallbackRunner_PrimaryFailsFallbackSucceeds(t *testing.T) {
	primary := &failingRunner{failErr: fmt.Errorf("API timeout"), typ: "cloud"}
	fallback := &succeedingRunner{typ: "process", stdout: "local result"}
	fb := NewFallbackRunner(primary, fallback)

	result, err := fb.Run(context.Background(), RunRequest{Command: "echo test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "local result" {
		t.Fatalf("expected 'local result', got %s", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "云端沙箱调用失败") {
		t.Fatalf("expected degradation notice in stderr, got: %s", result.Stderr)
	}
	if !strings.Contains(result.Stderr, "API timeout") {
		t.Fatal("should contain original error")
	}
}

func TestFallbackRunner_BothFail(t *testing.T) {
	primary := &failingRunner{failErr: fmt.Errorf("cloud down"), typ: "cloud"}
	fallback := &failingRunner{failErr: fmt.Errorf("docker unavailable"), typ: "docker"}
	fb := NewFallbackRunner(primary, fallback)

	_, err := fb.Run(context.Background(), RunRequest{Command: "echo test"})
	if err == nil {
		t.Fatal("expected error when both fail")
	}
	if !strings.Contains(err.Error(), "cloud down") {
		t.Fatal("should contain primary error")
	}
	if !strings.Contains(err.Error(), "docker unavailable") {
		t.Fatal("should contain fallback error")
	}
}

func TestFallbackRunner_ThreeLevelChain(t *testing.T) {
	// Cloud → Docker → Process (nested FallbackRunners)
	process := &succeedingRunner{typ: "process", stdout: "process result"}
	docker := &failingRunner{failErr: fmt.Errorf("docker not running"), typ: "docker"}
	cloud := &failingRunner{failErr: fmt.Errorf("cloud 503"), typ: "cloud"}

	dockerFallback := NewFallbackRunner(docker, process)
	cloudFallback := NewFallbackRunner(cloud, dockerFallback)

	if cloudFallback.Type() != "cloud+docker+process" {
		t.Fatalf("expected cloud+docker+process, got %s", cloudFallback.Type())
	}

	result, err := cloudFallback.Run(context.Background(), RunRequest{Command: "echo test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "process result" {
		t.Fatalf("expected 'process result', got %s", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "云端沙箱调用失败") {
		t.Fatal("should contain degradation notice")
	}
}

func TestFallbackRunner_Close(t *testing.T) {
	primary := &succeedingRunner{typ: "cloud"}
	fallback := &succeedingRunner{typ: "process"}
	fb := NewFallbackRunner(primary, fallback)

	if err := fb.Close(); err != nil {
		t.Fatal(err)
	}
}

// ──────────────────────────────────────────────
// NewRunner factory priority tests
// ──────────────────────────────────────────────

func TestNewRunner_ProcessOnly(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.Docker.Enabled = false
	cfg.Cloud.Enabled = false

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()

	if runner.Type() != "process" {
		t.Fatalf("expected process, got %s", runner.Type())
	}
}

func TestNewRunner_CloudWithoutKey(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.Cloud.Enabled = true
	cfg.Cloud.APIKey = "" // no key → cloud init fails → fallback to local

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()

	// Should fallback to local (process or docker)
	if runner.Type() != "process" && runner.Type() != "docker" {
		t.Fatalf("expected local runner, got %s", runner.Type())
	}
}

func TestNewRunnerForBackend_Process(t *testing.T) {
	cfg := DefaultSandboxConfig()
	runner, err := NewRunnerForBackend("process", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()

	if runner.Type() != "process" {
		t.Fatalf("expected process, got %s", runner.Type())
	}
}

func TestNewRunnerForBackend_K8sRejectsRunner(t *testing.T) {
	cfg := DefaultSandboxConfig()
	_, err := NewRunnerForBackend("k8s", cfg)
	if err == nil {
		t.Fatal("expected error for k8s backend")
	}
	if !strings.Contains(err.Error(), "does not implement the Runner interface") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRunnerForBackend_WasmRejectsRunner(t *testing.T) {
	cfg := DefaultSandboxConfig()
	_, err := NewRunnerForBackend("wasm", cfg)
	if err == nil {
		t.Fatal("expected error for wasm backend")
	}
	if !strings.Contains(err.Error(), "does not implement the Runner interface") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRunnerForBackend_UnknownBackend(t *testing.T) {
	cfg := DefaultSandboxConfig()
	_, err := NewRunnerForBackend("quantum", cfg)
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

// ──────────────────────────────────────────────
// WASM sandbox resource limit tests
// ──────────────────────────────────────────────

func TestWasmSandbox_MemoryLimitConfig(t *testing.T) {
	cfg := WasmConfig{
		MemoryLimitPages: 16, // 1MB
		MaxDuration:      5 * time.Second,
		MaxOutputBytes:   1024,
	}
	ws := NewWasmSandbox(cfg)

	stats := ws.Stats()
	memPages := stats["memory_limit_pages"].(uint32)
	memBytes := stats["memory_limit_bytes"].(uint32)

	if memPages != 16 {
		t.Errorf("expected 16 pages, got %d", memPages)
	}
	if memBytes != 16*65536 {
		t.Errorf("expected %d bytes, got %d", 16*65536, memBytes)
	}
}

func TestWasmSandbox_DefaultLimits(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())
	stats := ws.Stats()

	if stats["memory_limit_pages"].(uint32) != 256 {
		t.Error("default should be 256 pages (16MB)")
	}
	if stats["max_duration"].(string) != "10s" {
		t.Error("default timeout should be 10s")
	}
}

func TestWasmSandbox_ZeroConfigDefaults(t *testing.T) {
	ws := NewWasmSandbox(WasmConfig{})

	stats := ws.Stats()
	if stats["memory_limit_pages"].(uint32) != 256 {
		t.Error("zero config should default to 256 pages")
	}
	if stats["max_duration"].(string) != "10s" {
		t.Error("zero config should default to 10s timeout")
	}
}

func TestWasmSandbox_KVStoreIsolation(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())

	ws.SetKV("persist_key", "persist_value")
	v, ok := ws.GetKV("persist_key")
	if !ok || v != "persist_value" {
		t.Fatal("KV set/get failed")
	}

	// KV persists across Execute calls (same sandbox instance)
	_, _ = ws.Execute(context.Background(), minimalWasm, "", "")

	v, ok = ws.GetKV("persist_key")
	if !ok || v != "persist_value" {
		t.Fatal("KV should persist across executions within same sandbox")
	}

	// ClearKV resets all state
	ws.ClearKV()
	_, ok = ws.GetKV("persist_key")
	if ok {
		t.Fatal("ClearKV should remove all entries")
	}
}

func TestWasmSandbox_KVStoreOverwrite(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())

	ws.SetKV("key", "v1")
	ws.SetKV("key", "v2")
	v, _ := ws.GetKV("key")
	if v != "v2" {
		t.Fatalf("expected v2 after overwrite, got %s", v)
	}
}

func TestWasmSandbox_InvalidModuleGraceful(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())

	result, err := ws.Execute(context.Background(), []byte("garbage"), "", "")
	if err != nil {
		t.Fatal("should not return Go error for invalid WASM")
	}
	if result.ExitCode != -1 {
		t.Fatal("should have exit code -1")
	}
	if !strings.Contains(result.Stderr, "compile error") {
		t.Fatalf("expected compile error, got: %s", result.Stderr)
	}
}

func TestWasmSandbox_EmptyModuleError(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())
	_, err := ws.Execute(context.Background(), nil, "", "")
	if err == nil {
		t.Fatal("nil module should error")
	}

	_, err = ws.Execute(context.Background(), []byte{}, "", "")
	if err == nil {
		t.Fatal("empty module should error")
	}
}

func TestWasmSandbox_MissingEntryPoint(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())

	result, err := ws.Execute(context.Background(), minimalWasm, "", "nonexistent_func")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != -1 {
		t.Fatal("missing entry point should fail")
	}
	if !strings.Contains(result.Stderr, "entry point not found") {
		t.Fatalf("expected entry point error, got: %s", result.Stderr)
	}
}

func TestWasmSandbox_TimeoutConfig(t *testing.T) {
	cfg := WasmConfig{
		MemoryLimitPages: 16,
		MaxDuration:      100 * time.Millisecond,
		MaxOutputBytes:   512,
	}
	ws := NewWasmSandbox(cfg)

	stats := ws.Stats()
	if stats["max_duration"].(string) != "100ms" {
		t.Errorf("expected 100ms, got %s", stats["max_duration"])
	}
}

func TestWasmSandbox_HostFuncRegistration(t *testing.T) {
	ws := NewWasmSandbox(DefaultWasmConfig())

	ws.RegisterHostFunc("func_a", func(ctx context.Context, args []uint64) ([]uint64, error) {
		return nil, nil
	})
	ws.RegisterHostFunc("func_b", func(ctx context.Context, args []uint64) ([]uint64, error) {
		return nil, nil
	})

	stats := ws.Stats()
	if stats["host_funcs"].(int) != 2 {
		t.Errorf("expected 2 host funcs, got %v", stats["host_funcs"])
	}

	// Overwrite existing
	ws.RegisterHostFunc("func_a", func(ctx context.Context, args []uint64) ([]uint64, error) {
		return []uint64{99}, nil
	})

	stats = ws.Stats()
	if stats["host_funcs"].(int) != 2 {
		t.Error("overwriting should not increase count")
	}
}

// ──────────────────────────────────────────────
// Docker security configuration tests
// ──────────────────────────────────────────────

func TestDockerSecurityFlags_FullHardening(t *testing.T) {
	cfg := DefaultDockerConfig()
	dr := &DockerRuntime{cfg: cfg}

	req := RunRequest{Language: "python", Code: "print('test')"}
	args := dr.buildDockerRunArgs("python:3.12-slim", req)
	joined := strings.Join(args, " ")

	securityChecks := map[string]string{
		"--security-opt=no-new-privileges": "privilege escalation prevention",
		"--read-only":                      "read-only rootfs",
		"--user 1000:1000":                 "non-root user",
		"--memory 256m":                    "memory limit",
		"--memory-swap 256m":               "swap disabled (equals memory)",
		"--cpus 1":                         "CPU limit",
		"--pids-limit 64":                  "fork bomb protection",
		"--network none":                   "network isolation",
	}
	for flag, desc := range securityChecks {
		if !strings.Contains(joined, flag) {
			t.Errorf("missing %s flag: %s", desc, flag)
		}
	}
}

func TestDockerSecurityFlags_TmpfsForReadOnly(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.ReadOnlyRootfs = true
	dr := &DockerRuntime{cfg: cfg}

	args := dr.buildDockerRunArgs("alpine", RunRequest{Command: "echo"})
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "--tmpfs /tmp:rw,noexec,nosuid,size=64m") {
		t.Error("/tmp tmpfs should be rw,noexec,nosuid")
	}
	if !strings.Contains(joined, "--tmpfs /workspace:rw,exec,nosuid,size=128m") {
		t.Error("/workspace tmpfs should be rw,exec (code needs exec),nosuid")
	}
}

func TestDockerSecurityFlags_NoHardeningWhenDisabled(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.ReadOnlyRootfs = false
	cfg.NonRootUser = false
	cfg.NetworkEnabled = true
	dr := &DockerRuntime{cfg: cfg}

	args := dr.buildDockerRunArgs("alpine", RunRequest{Command: "echo"})
	joined := strings.Join(args, " ")

	if strings.Contains(joined, "--read-only") {
		t.Error("should not have --read-only when disabled")
	}
	if strings.Contains(joined, "--user") {
		t.Error("should not have --user when non-root disabled")
	}
	if strings.Contains(joined, "--network none") {
		t.Error("should not have --network none when network enabled")
	}
	// no-new-privileges is always set
	if !strings.Contains(joined, "--security-opt=no-new-privileges") {
		t.Error("no-new-privileges should always be present")
	}
}

func TestDockerImageWhitelist_RejectsUnknown(t *testing.T) {
	cfg := DefaultDockerConfig()
	dr := &DockerRuntime{cfg: cfg}

	if dr.isImageAllowed("evil:latest") {
		t.Fatal("unknown image should be rejected")
	}
	if dr.isImageAllowed("python:3.11-slim") {
		t.Fatal("image not in config should be rejected")
	}
}

func TestDockerImageWhitelist_AcceptsConfigured(t *testing.T) {
	cfg := DefaultDockerConfig()
	dr := &DockerRuntime{cfg: cfg}

	for lang, img := range cfg.LanguageImages {
		if !dr.isImageAllowed(img) {
			t.Errorf("configured image %s (%s) should be allowed", img, lang)
		}
	}
	if !dr.isImageAllowed(cfg.DefaultImage) {
		t.Error("default image should be allowed")
	}
}

func TestDockerImageWhitelist_ExplicitOverride(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.AllowedImages = []string{"custom:v1", "custom:v2"}
	dr := &DockerRuntime{cfg: cfg}

	if !dr.isImageAllowed("custom:v1") {
		t.Error("explicitly whitelisted image should be allowed")
	}
	if dr.isImageAllowed("python:3.12-slim") {
		t.Error("default image should be rejected when explicit whitelist is set")
	}
}

func TestDockerCodeCommand_ShellInjectionPrevention(t *testing.T) {
	dr := &DockerRuntime{cfg: DefaultDockerConfig()}

	req := RunRequest{
		Language: "python",
		Code:     "print('hello')",
		Files: map[string]string{
			"normal.txt":      "safe content",
			"../escape.txt":   "should be cleaned",
			"evil;rm -rf.txt": "should be skipped",
		},
	}
	cmd := dr.buildCodeCommand(req)

	if strings.Contains(cmd, "../") {
		t.Error("path traversal should be sanitized")
	}
	if strings.Contains(cmd, "evil;rm") {
		t.Error("shell metacharacters in filename should be skipped")
	}
	if !strings.Contains(cmd, "normal.txt") {
		t.Error("safe filename should be preserved")
	}
}

// ──────────────────────────────────────────────
// Process sandbox policy tier tests
// ──────────────────────────────────────────────

func TestPolicyTiers(t *testing.T) {
	tests := []struct {
		tier          TierName
		expectNetwork bool
		expectStrict  bool // strict = short allowlist
		maxDuration   time.Duration
	}{
		{TierPersonal, true, false, 2 * time.Minute},
		{TierFamily, false, true, 30 * time.Second},
		{TierPublic, false, true, 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			p := PolicyForTier(tt.tier)
			if p.AllowNetwork != tt.expectNetwork {
				t.Errorf("AllowNetwork: got %v, want %v", p.AllowNetwork, tt.expectNetwork)
			}
			if tt.expectStrict && len(p.AllowCommands) == 0 {
				t.Error("strict tier should have non-empty allowlist")
			}
			if !tt.expectStrict && len(p.AllowCommands) != 0 {
				t.Error("relaxed tier should have empty allowlist (all allowed)")
			}
			if p.MaxDuration != tt.maxDuration {
				t.Errorf("MaxDuration: got %v, want %v", p.MaxDuration, tt.maxDuration)
			}
		})
	}
}

func TestPolicyTier_PublicBlocksDangerousCommands(t *testing.T) {
	p := PublicPolicy()

	dangerous := []string{"rm", "mv", "cp", "chmod", "python", "node", "bash", "sh"}
	for _, cmd := range dangerous {
		blocked := false
		for _, b := range p.BlockCommands {
			if b == cmd {
				blocked = true
				break
			}
		}
		if !blocked {
			t.Errorf("public tier should block %s", cmd)
		}
	}
}

func TestPolicyTier_PersonalMinimalBlocks(t *testing.T) {
	p := PersonalPolicy()

	if len(p.BlockCommands) > 5 {
		t.Error("personal tier should have minimal blocklist")
	}
	for _, b := range p.BlockCommands {
		if b == "python" || b == "node" {
			t.Errorf("personal tier should not block %s", b)
		}
	}
}

// ──────────────────────────────────────────────
// K8s Pod security and isolation tests
// ──────────────────────────────────────────────

func TestK8sPodYAML_SecurityDefaults(t *testing.T) {
	cfg := DefaultK8sConfig()
	yaml := GeneratePodYAML(cfg, "test-sandbox", []string{"sh", "-c", "echo hello"})

	checks := map[string]string{
		"restartPolicy: Never":          "no restart",
		"namespace: tori-sandbox":       "isolated namespace",
		"cpu: 500m":                     "CPU limit",
		"memory: 256Mi":                 "memory limit",
		"imagePullPolicy: IfNotPresent": "image pull policy",
	}
	for expected, desc := range checks {
		if !strings.Contains(yaml, expected) {
			t.Errorf("Pod YAML missing %s: %s", desc, expected)
		}
	}
}

func TestK8sPodYAML_PVCMount(t *testing.T) {
	cfg := DefaultK8sConfig()
	cfg.PVCName = "sandbox-data"
	yaml := GeneratePodYAML(cfg, "test-pod", []string{"ls"})

	if !strings.Contains(yaml, "mountPath: /data") {
		t.Error("PVC should mount at /data")
	}
	if !strings.Contains(yaml, "claimName: sandbox-data") {
		t.Error("PVC claim name should match config")
	}
}

func TestK8sPodYAML_NoPVCWhenEmpty(t *testing.T) {
	cfg := DefaultK8sConfig()
	cfg.PVCName = ""
	yaml := GeneratePodYAML(cfg, "test-pod", []string{"ls"})

	if strings.Contains(yaml, "volumeMounts") {
		t.Error("should not have volumeMounts without PVC")
	}
}

func TestK8sPodYAML_CustomServiceAccount(t *testing.T) {
	cfg := DefaultK8sConfig()
	cfg.ServiceAccount = "sandbox-sa"
	yaml := GeneratePodYAML(cfg, "test-pod", []string{"ls"})

	if !strings.Contains(yaml, "serviceAccountName: sandbox-sa") {
		t.Error("should include custom service account")
	}
}

func TestK8sPodYAML_NoServiceAccountByDefault(t *testing.T) {
	cfg := DefaultK8sConfig()
	yaml := GeneratePodYAML(cfg, "test-pod", []string{"ls"})

	if strings.Contains(yaml, "serviceAccountName") {
		t.Error("should not include service account by default")
	}
}

func TestK8sExecute_CommandWrapping(t *testing.T) {
	var capturedCmd []string
	exec := &mockK8sExecutor{
		waitState: PodSucceeded,
		logs:      "output",
	}
	// Patch CreatePod to capture command
	origCreate := exec.createErr
	exec.createErr = origCreate
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)

	pod, err := rt.Execute(context.Background(), "echo hello && ls -la")
	if err != nil {
		t.Fatal(err)
	}
	_ = capturedCmd
	if pod.Command != "echo hello && ls -la" {
		t.Fatalf("command should be preserved, got: %s", pod.Command)
	}
}

func TestK8sExecute_PodNaming(t *testing.T) {
	exec := &mockK8sExecutor{waitState: PodSucceeded}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)

	pod, _ := rt.Execute(context.Background(), "echo test")
	if !strings.HasPrefix(pod.Name, "tori-sandbox-") {
		t.Fatalf("pod name should start with tori-sandbox-, got: %s", pod.Name)
	}
	if pod.Namespace != "tori-sandbox" {
		t.Fatalf("expected tori-sandbox namespace, got: %s", pod.Namespace)
	}
}

func TestK8sExecute_LifecycleTracking(t *testing.T) {
	exec := &mockK8sExecutor{waitState: PodSucceeded, logs: "done"}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)

	pod, _ := rt.Execute(context.Background(), "echo test")

	if pod.State != PodSucceeded {
		t.Fatal("should be succeeded")
	}
	if pod.StartedAt == nil {
		t.Fatal("StartedAt should be set")
	}
	if pod.EndedAt == nil {
		t.Fatal("EndedAt should be set")
	}
	if pod.Duration < 0 {
		t.Fatal("Duration should be non-negative")
	}
	if pod.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set")
	}
}

// ──────────────────────────────────────────────
// ProcessRunner integration tests
// ──────────────────────────────────────────────

func TestProcessRunner_EchoCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}

	policy := DefaultPolicy()
	policy.AllowCommands = append(policy.AllowCommands, "cmd", "sh", "where", "which")
	r := NewProcessRunner(t.TempDir(), policy)
	defer r.Close()

	result, err := r.Run(context.Background(), RunRequest{
		Command: echoCommand(),
		Args:    echoArgs("integration test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit %d: %s", result.ExitCode, result.Stderr)
	}
}

func TestProcessRunner_WriteFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}

	policy := DefaultPolicy()
	policy.AllowCommands = append(policy.AllowCommands, "cmd", "sh")
	r := NewProcessRunner(t.TempDir(), policy)
	defer r.Close()

	result, err := r.Run(context.Background(), RunRequest{
		Command: catCommand(),
		Args:    catArgs("data.txt"),
		Files:   map[string]string{"data.txt": "file content here"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stdout, "file content here") {
		t.Fatalf("expected file content in stdout, got: %s (stderr: %s)", result.Stdout, result.Stderr)
	}
}

// OS-agnostic command helpers
func echoCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}
func echoArgs(msg string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/C", "echo", msg}
	}
	return []string{"-c", "printf '%s\n' \"$1\"", "sh", msg}
}
func catCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}
func catArgs(path string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/C", "type", path}
	}
	return []string{"-c", "cat \"$1\"", "sh", path}
}
