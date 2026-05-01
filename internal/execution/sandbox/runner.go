package sandbox

import (
	"context"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// Unified Runner Interface
//
// Runner is the polymorphic interface used by the general-purpose
// "run code / run command" path. Current concrete implementations:
//
//   - ProcessRunner  — local process sandbox (default)
//   - DockerRuntime  — container pool
//   - CloudRunner    — E2B-compatible remote sandbox (optional)
//   - FallbackRunner — wraps cloud + local so cloud failures degrade
//
// The K8s (k8s.go) and WASM (wasm.go) backends expose their own
// Execute(...) methods tailored to Pod specs / wasm bytes and are
// invoked via the specialised APIs in internal/execution/sandbox
// rather than through this interface. Do not assume a generic Runner
// factory can produce them.
// ──────────────────────────────────────────────

// RunRequest describes what to execute in a sandbox.
type RunRequest struct {
	Language string            `json:"language,omitempty"` // "python", "javascript", "go", "shell"
	Code     string            `json:"code,omitempty"`     // source code (auto writes & runs)
	Command  string            `json:"command,omitempty"`  // raw command (mutually exclusive with Code)
	Args     []string          `json:"args,omitempty"`     // command arguments
	Stdin    string            `json:"stdin,omitempty"`    // standard input
	Files    map[string]string `json:"files,omitempty"`    // additional files: filename -> content
	Env      map[string]string `json:"env,omitempty"`      // environment variables
	Timeout  time.Duration     `json:"timeout,omitempty"`  // override default (0 = backend default)
}

// RunResult is the unified execution result.
type RunResult struct {
	ExitCode int           `json:"exit_code"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Duration time.Duration `json:"duration"`
}

// Runner is the unified interface for the generic code/command sandboxing
// path. K8s and WASM backends are NOT Runners — see package doc above.
type Runner interface {
	// Run executes code or a command in the sandbox.
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
	// Type returns the backend identifier ("process", "docker", "cloud",
	// or a composite name for wrappers such as "fallback").
	Type() string
	// Close releases resources (container pool, runtime, etc).
	Close() error
}

// ──────────────────────────────────────────────
// Language runner mapping
// ──────────────────────────────────────────────

type langRunner struct {
	Cmd string
	Ext string
}

var defaultLangRunners = map[string]langRunner{
	"python":     {Cmd: "python3", Ext: ".py"},
	"python3":    {Cmd: "python3", Ext: ".py"},
	"javascript": {Cmd: "node", Ext: ".js"},
	"js":         {Cmd: "node", Ext: ".js"},
	"go":         {Cmd: "go", Ext: ".go"},
	"shell":      {Cmd: "sh", Ext: ".sh"},
	"bash":       {Cmd: "bash", Ext: ".sh"},
}

// ──────────────────────────────────────────────
// ProcessRunner — wraps existing Sandbox as Runner
// ──────────────────────────────────────────────

// ProcessRunner runs code in a local process sandbox.
type ProcessRunner struct {
	baseDir string
	policy  Policy
}

// NewProcessRunner creates a process-based Runner.
func NewProcessRunner(baseDir string, policy Policy) *ProcessRunner {
	return &ProcessRunner{baseDir: baseDir, policy: policy}
}

func (r *ProcessRunner) Type() string { return "process" }
func (r *ProcessRunner) Close() error { return nil }

func (r *ProcessRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	sb, err := New(r.baseDir, r.policy)
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	defer sb.Cleanup()

	// Write additional files
	for name, content := range req.Files {
		if err := sb.WriteFile(name, content); err != nil {
			return nil, fmt.Errorf("write file %s: %w", name, err)
		}
	}

	// Code execution mode: write code, determine runner, execute
	if req.Code != "" {
		return r.runCode(ctx, sb, req)
	}

	// Command execution mode
	if req.Command != "" {
		result, err := sb.Exec(ctx, req.Command, req.Args...)
		if err != nil {
			return nil, err
		}
		return toRunResult(result), nil
	}

	return nil, fmt.Errorf("either Code or Command must be specified")
}

func (r *ProcessRunner) runCode(ctx context.Context, sb *Sandbox, req RunRequest) (*RunResult, error) {
	lr, ok := defaultLangRunners[req.Language]
	if !ok {
		return &RunResult{ExitCode: -1, Stderr: fmt.Sprintf("unsupported language: %s", req.Language)}, nil
	}

	filename := "main" + lr.Ext
	if err := sb.WriteFile(filename, req.Code); err != nil {
		return nil, fmt.Errorf("write code: %w", err)
	}

	var result *Result
	var err error
	if req.Language == "go" {
		result, err = sb.Exec(ctx, lr.Cmd, "run", filename)
	} else {
		result, err = sb.Exec(ctx, lr.Cmd, filename)
	}
	if err != nil {
		return nil, err
	}
	return toRunResult(result), nil
}

// ──────────────────────────────────────────────
// Factory
// ──────────────────────────────────────────────

// NewRunner creates a Runner from a SandboxConfig.
// Priority: cloud > docker > process.
// When cloud is enabled, a CircuitRunner wraps the cloud runner so repeated
// failures skip it immediately, then a FallbackRunner degrades to local.
func NewRunner(cfg SandboxConfig) (Runner, error) {
	local := localRunner(cfg)

	if cfg.Cloud.Enabled {
		cr, err := NewCloudRunner(cfg.Cloud)
		if err == nil {
			wrapped := NewCircuitRunner(cr, DefaultCircuitConfig())
			return NewFallbackRunner(wrapped, local), nil
		}
		fmt.Printf("sandbox: cloud unavailable (%v), using local only\n", err)
	}
	return local, nil
}

func localRunner(cfg SandboxConfig) Runner {
	if cfg.Docker.Enabled {
		dr, err := NewDockerRuntime(cfg.Docker)
		if err == nil {
			return dr
		}
		fmt.Printf("sandbox: docker unavailable (%v), falling back to process\n", err)
	}
	return NewProcessRunner(cfg.BaseDir, cfg.Policy)
}

// NewRunnerForBackend creates a specific backend Runner. Only backends
// that implement the Runner interface can be returned here — k8s and
// wasm are exposed via their own Execute(...) APIs and are rejected
// explicitly so callers fail fast with a useful error instead of
// silently getting nothing.
func NewRunnerForBackend(backend string, cfg SandboxConfig) (Runner, error) {
	switch backend {
	case "cloud":
		return NewCloudRunner(cfg.Cloud)
	case "docker":
		return NewDockerRuntime(cfg.Docker)
	case "process":
		return NewProcessRunner(cfg.BaseDir, cfg.Policy), nil
	case "k8s", "wasm":
		return nil, fmt.Errorf("sandbox backend %q does not implement the Runner interface; use the dedicated Execute API in k8s.go / wasm.go instead", backend)
	default:
		return nil, fmt.Errorf("unknown sandbox backend: %s", backend)
	}
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func toRunResult(r *Result) *RunResult {
	d, _ := time.ParseDuration(r.Duration)
	return &RunResult{
		ExitCode: r.ExitCode,
		Stdout:   r.Stdout,
		Stderr:   r.Stderr,
		Duration: d,
	}
}
