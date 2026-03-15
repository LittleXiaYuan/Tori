package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDockerConfigDefaults(t *testing.T) {
	cfg := DefaultDockerConfig()
	if cfg.DefaultImage != "python:3.12-slim" {
		t.Fatalf("wrong default image: %s", cfg.DefaultImage)
	}
	if cfg.PoolSize != 2 {
		t.Fatalf("wrong pool size: %d", cfg.PoolSize)
	}
	if cfg.MaxContainers != 10 {
		t.Fatalf("wrong max containers: %d", cfg.MaxContainers)
	}
	if cfg.PidsLimit != 64 {
		t.Fatalf("wrong pids limit: %d", cfg.PidsLimit)
	}
	if !cfg.ReadOnlyRootfs {
		t.Fatal("read-only rootfs should be true by default")
	}
	if !cfg.NonRootUser {
		t.Fatal("non-root user should be true by default")
	}
	if cfg.NetworkEnabled {
		t.Fatal("network should be disabled by default")
	}
}

func TestDockerRuntime_ResolveImage(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Enabled = true
	dr := &DockerRuntime{cfg: cfg}

	tests := []struct {
		lang     string
		expected string
	}{
		{"python", "python:3.12-slim"},
		{"python3", "python:3.12-slim"},
		{"javascript", "node:20-slim"},
		{"js", "node:20-slim"},
		{"go", "golang:1.22-alpine"},
		{"shell", "alpine:latest"},
		{"unknown", "python:3.12-slim"},
		{"", "python:3.12-slim"},
	}

	for _, tt := range tests {
		got := dr.resolveImage(tt.lang)
		if got != tt.expected {
			t.Errorf("resolveImage(%q) = %q, want %q", tt.lang, got, tt.expected)
		}
	}
}

func TestDockerRuntime_IsImageAllowed(t *testing.T) {
	cfg := DefaultDockerConfig()
	dr := &DockerRuntime{cfg: cfg}

	// Empty whitelist: only LanguageImages + DefaultImage allowed
	if !dr.isImageAllowed("python:3.12-slim") {
		t.Error("default image should be allowed")
	}
	if !dr.isImageAllowed("node:20-slim") {
		t.Error("language image should be allowed")
	}
	if dr.isImageAllowed("malicious:latest") {
		t.Error("unknown image should not be allowed")
	}

	// With explicit whitelist
	cfg.AllowedImages = []string{"python:3.12-slim", "custom:v1"}
	dr.cfg = cfg
	if !dr.isImageAllowed("python:3.12-slim") {
		t.Error("whitelisted image should be allowed")
	}
	if !dr.isImageAllowed("custom:v1") {
		t.Error("custom whitelisted image should be allowed")
	}
	if dr.isImageAllowed("node:20-slim") {
		t.Error("non-whitelisted image should not be allowed")
	}
}

func TestDockerRuntime_BuildDockerRunArgs(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.MemoryLimit = "256m"
	cfg.CPULimit = "1"
	cfg.PidsLimit = 64
	cfg.ReadOnlyRootfs = true
	cfg.NonRootUser = true
	cfg.NetworkEnabled = false
	dr := &DockerRuntime{cfg: cfg}

	req := RunRequest{
		Language: "python",
		Code:     "print('hello')",
	}
	args := dr.buildDockerRunArgs("python:3.12-slim", req)
	joined := strings.Join(args, " ")

	// Must contain security flags
	if !strings.Contains(joined, "--security-opt=no-new-privileges") {
		t.Error("missing no-new-privileges")
	}
	if !strings.Contains(joined, "--read-only") {
		t.Error("missing read-only rootfs")
	}
	if !strings.Contains(joined, "--user 1000:1000") {
		t.Error("missing non-root user")
	}
	if !strings.Contains(joined, "--memory 256m") {
		t.Error("missing memory limit")
	}
	if !strings.Contains(joined, "--memory-swap 256m") {
		t.Error("missing memory-swap limit")
	}
	if !strings.Contains(joined, "--cpus 1") {
		t.Error("missing CPU limit")
	}
	if !strings.Contains(joined, "--pids-limit 64") {
		t.Error("missing pids limit")
	}
	if !strings.Contains(joined, "--network none") {
		t.Error("missing network none")
	}
	if !strings.Contains(joined, "--tmpfs") {
		t.Error("missing tmpfs for read-only rootfs")
	}
}

func TestDockerRuntime_BuildDockerRunArgs_WithNetwork(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.NetworkEnabled = true
	cfg.ReadOnlyRootfs = false
	cfg.NonRootUser = false
	dr := &DockerRuntime{cfg: cfg}

	req := RunRequest{Command: "echo hello"}
	args := dr.buildDockerRunArgs("alpine:latest", req)
	joined := strings.Join(args, " ")

	if strings.Contains(joined, "--network") {
		t.Error("should not have --network when enabled")
	}
	if strings.Contains(joined, "--read-only") {
		t.Error("should not have --read-only when disabled")
	}
	if strings.Contains(joined, "--user") {
		t.Error("should not have --user when non-root disabled")
	}
}

func TestDockerRuntime_BuildExecCommand_Code(t *testing.T) {
	dr := &DockerRuntime{cfg: DefaultDockerConfig()}

	tests := []struct {
		req      RunRequest
		contains string
	}{
		{
			RunRequest{Language: "python", Code: "print('hello')"},
			"python3 /workspace/main.py",
		},
		{
			RunRequest{Language: "javascript", Code: "console.log('hi')"},
			"node /workspace/main.js",
		},
		{
			RunRequest{Language: "go", Code: "package main"},
			"go run main.go",
		},
		{
			RunRequest{Command: "ls", Args: []string{"-la"}},
			"ls -la",
		},
	}

	for _, tt := range tests {
		cmd := dr.buildExecCommand(tt.req)
		if !strings.Contains(cmd, tt.contains) {
			t.Errorf("buildExecCommand: expected %q in %q", tt.contains, cmd)
		}
	}
}

func TestDockerRuntime_BuildExecCommand_UnsupportedLang(t *testing.T) {
	dr := &DockerRuntime{cfg: DefaultDockerConfig()}
	cmd := dr.buildExecCommand(RunRequest{Language: "rust", Code: "fn main() {}"})
	if !strings.Contains(cmd, "unsupported") {
		t.Errorf("expected unsupported language error, got %q", cmd)
	}
}

func TestContainerPool_Stats(t *testing.T) {
	p := &containerPool{
		containers: make(map[string]*containerInfo),
		available:  make(map[string][]string),
		cfg:        DefaultDockerConfig(),
		stopCh:     make(chan struct{}),
	}

	stats := p.stats()
	if stats["total"].(int) != 0 {
		t.Error("empty pool should have 0 total")
	}
	if stats["available"].(int) != 0 {
		t.Error("empty pool should have 0 available")
	}
}

func TestContainerPool_CheckoutEmpty(t *testing.T) {
	p := &containerPool{
		containers: make(map[string]*containerInfo),
		available:  make(map[string][]string),
		cfg:        DefaultDockerConfig(),
		stopCh:     make(chan struct{}),
	}

	id := p.checkout("python:3.12-slim")
	if id != "" {
		t.Error("checkout on empty pool should return empty")
	}
}

func TestContainerPool_MaxContainers(t *testing.T) {
	p := &containerPool{
		containers: make(map[string]*containerInfo),
		available:  make(map[string][]string),
		cfg:        DockerConfig{MaxContainers: 0},
		stopCh:     make(chan struct{}),
	}

	// Manually add containers to exceed max
	// (Won't actually call docker, just test the logic)
	_, err := p.create("test:latest")
	if err == nil {
		// If Docker is available it might succeed, so just check the pool full error
		// In CI without docker this will fail with docker error, not pool full
		t.Skip("docker available, cannot test pool full without real containers")
	}
}

func TestDockerRuntime_ClosedRuntime(t *testing.T) {
	dr := &DockerRuntime{
		cfg:    DefaultDockerConfig(),
		closed: true,
		pool: &containerPool{
			containers: make(map[string]*containerInfo),
			available:  make(map[string][]string),
			cfg:        DefaultDockerConfig(),
			stopCh:     make(chan struct{}),
		},
	}

	_, err := dr.Run(context.Background(), RunRequest{Command: "echo hello"})
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Error("expected closed error")
	}
}

func TestProcessRunner(t *testing.T) {
	r := NewProcessRunner(t.TempDir(), DefaultPolicy())
	if r.Type() != "process" {
		t.Fatalf("wrong type: %s", r.Type())
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProcessRunner_NoCodeNoCommand(t *testing.T) {
	r := NewProcessRunner(t.TempDir(), DefaultPolicy())
	_, err := r.Run(context.Background(), RunRequest{})
	if err == nil {
		t.Error("expected error for empty request")
	}
}

func TestProcessRunner_UnsupportedLanguage(t *testing.T) {
	r := NewProcessRunner(t.TempDir(), DefaultPolicy())
	result, err := r.Run(context.Background(), RunRequest{Language: "cobol", Code: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != -1 {
		t.Error("expected exit code -1 for unsupported language")
	}
}

func TestNewRunner_DefaultsToProcess(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.Docker.Enabled = false
	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if runner.Type() != "process" {
		t.Fatalf("expected process runner, got %s", runner.Type())
	}
	runner.Close()
}

func TestNewRunner_DockerFallsBack(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.Docker.Enabled = true
	// This will fail if Docker is not installed, falling back to process
	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Type is either "docker" (if docker available) or "process" (fallback)
	if runner.Type() != "docker" && runner.Type() != "process" {
		t.Fatalf("unexpected runner type: %s", runner.Type())
	}
	runner.Close()
}

func TestLoadConfig_Defaults(t *testing.T) {
	cfg := LoadConfig("")
	if cfg.Docker.DefaultImage != "python:3.12-slim" {
		t.Fatalf("wrong default image: %s", cfg.Docker.DefaultImage)
	}
	if cfg.Docker.Timeout != 30*time.Second {
		t.Fatalf("wrong timeout: %v", cfg.Docker.Timeout)
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	t.Setenv("SANDBOX_DOCKER_ENABLED", "true")
	t.Setenv("SANDBOX_DOCKER_IMAGE", "custom:v1")
	t.Setenv("SANDBOX_DOCKER_POOL_SIZE", "5")
	t.Setenv("SANDBOX_DOCKER_MEMORY", "512m")
	t.Setenv("SANDBOX_DOCKER_NETWORK", "true")
	t.Setenv("SANDBOX_DOCKER_READONLY_ROOTFS", "false")
	t.Setenv("SANDBOX_DOCKER_NON_ROOT", "false")
	t.Setenv("SANDBOX_DOCKER_PIDS_LIMIT", "128")
	t.Setenv("SANDBOX_DOCKER_TIMEOUT", "1m")
	t.Setenv("SANDBOX_DOCKER_IMAGE_PYTHON", "my-python:latest")
	t.Setenv("SANDBOX_TIER", "public")

	cfg := LoadConfig("")
	if !cfg.Docker.Enabled {
		t.Error("docker should be enabled")
	}
	if cfg.Docker.DefaultImage != "custom:v1" {
		t.Errorf("wrong image: %s", cfg.Docker.DefaultImage)
	}
	if cfg.Docker.PoolSize != 5 {
		t.Errorf("wrong pool size: %d", cfg.Docker.PoolSize)
	}
	if cfg.Docker.MemoryLimit != "512m" {
		t.Errorf("wrong memory: %s", cfg.Docker.MemoryLimit)
	}
	if !cfg.Docker.NetworkEnabled {
		t.Error("network should be enabled")
	}
	if cfg.Docker.ReadOnlyRootfs {
		t.Error("read-only rootfs should be false")
	}
	if cfg.Docker.NonRootUser {
		t.Error("non-root should be false")
	}
	if cfg.Docker.PidsLimit != 128 {
		t.Errorf("wrong pids limit: %d", cfg.Docker.PidsLimit)
	}
	if cfg.Docker.Timeout != 1*time.Minute {
		t.Errorf("wrong timeout: %v", cfg.Docker.Timeout)
	}
	if cfg.Docker.LanguageImages["python"] != "my-python:latest" {
		t.Errorf("wrong python image: %s", cfg.Docker.LanguageImages["python"])
	}
	// Public tier should have strict policy
	if len(cfg.Policy.AllowCommands) == 0 || cfg.Policy.AllowCommands[0] != "echo" {
		t.Error("public tier should have strict allowlist")
	}
}
