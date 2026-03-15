package sandbox

import (
	"context"
	"fmt"
	"runtime"
)

// Task represents an automation task the agent can execute.
type Task struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Steps       []Step            `json:"steps"`
	Env         map[string]string `json:"env,omitempty"`
}

// Step is a single step in an automation task.
type Step struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	OnFail  string   `json:"on_fail"` // "stop", "continue", "retry"
}

// Automator runs multi-step automation tasks in a sandbox.
type Automator struct {
	baseDir string
	policy  Policy
}

// NewAutomator creates an automator with the given base directory.
func NewAutomator(baseDir string, policy Policy) *Automator {
	return &Automator{baseDir: baseDir, policy: policy}
}

// RunTask executes a multi-step task and returns results per step.
func (a *Automator) RunTask(ctx context.Context, task Task) ([]Result, error) {
	sb, err := New(a.baseDir, a.policy)
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	defer sb.Cleanup()

	var results []Result
	for i, step := range task.Steps {
		result, err := sb.Exec(ctx, step.Command, step.Args...)
		if err != nil {
			results = append(results, Result{ExitCode: -1, Stderr: err.Error()})
			if step.OnFail != "continue" {
				return results, fmt.Errorf("step %d failed: %w", i, err)
			}
			continue
		}
		results = append(results, *result)

		if result.ExitCode != 0 && step.OnFail != "continue" {
			if step.OnFail == "retry" {
				// Single retry
				result2, _ := sb.Exec(ctx, step.Command, step.Args...)
				if result2 != nil {
					results[len(results)-1] = *result2
					if result2.ExitCode != 0 {
						return results, fmt.Errorf("step %d retry failed", i)
					}
				}
			} else {
				return results, fmt.Errorf("step %d exit code %d", i, result.ExitCode)
			}
		}
	}
	return results, nil
}

// SystemInfo returns basic system information the agent can use.
func SystemInfo() map[string]string {
	return map[string]string{
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"cpus":          fmt.Sprintf("%d", runtime.NumCPU()),
		"sandbox_types": "process,docker,wasm",
		"wasm_runtime":  "wazero",
	}
}
