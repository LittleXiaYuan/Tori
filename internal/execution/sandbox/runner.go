package sandbox

import (
	"context"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// Unified Runner Interface
// All sandbox backends (Process / Docker / K8s / WASM)
// implement this interface for polymorphic code execution.
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

// Runner is the unified interface for all sandbox backends.
type Runner interface {
	// Run executes code or a command in the sandbox.
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
	// Type returns the backend identifier ("process", "docker", "k8s", "wasm").
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
// It selects the best available backend: docker > process.
func NewRunner(cfg SandboxConfig) (Runner, error) {
	if cfg.Docker.Enabled {
		dr, err := NewDockerRuntime(cfg.Docker)
		if err == nil {
			return dr, nil
		}
		// Docker unavailable — fall back to process
		fmt.Printf("sandbox: docker unavailable (%v), falling back to process\n", err)
	}
	return NewProcessRunner(cfg.BaseDir, cfg.Policy), nil
}

// NewRunnerForBackend creates a specific backend Runner.
func NewRunnerForBackend(backend string, cfg SandboxConfig) (Runner, error) {
	switch backend {
	case "docker":
		return NewDockerRuntime(cfg.Docker)
	case "process":
		return NewProcessRunner(cfg.BaseDir, cfg.Policy), nil
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
