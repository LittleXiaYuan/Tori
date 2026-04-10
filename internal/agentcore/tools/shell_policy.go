package tools

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"

	"yunque-agent/internal/agentcore/approval"
)

// ──────────────────────────────────────────────
// ShellPolicy — host machine execution strategy
//
// Integrates shell analysis + approval + trust:
//   1. Analyze command for dangerous patterns (ShellGuard)
//   2. Check allowlist/denylist rules (RuleStore)
//   3. Gate on approval if needed (Manager)
//   4. Apply OS-specific safety wrappers
//   5. Audit log every execution
//
// Replaces bare exec with a policy-aware executor.
// ──────────────────────────────────────────────

// ShellExecPolicy controls how commands are dispatched to the host.
type ShellExecPolicy struct {
	mu            sync.RWMutex
	approvalMgr   *approval.Manager
	processManager *ProcessManager

	// Configurable limits
	MaxConcurrent int     // max concurrent shell processes (default: 5)
	AllowUserHome bool    // allow commands to access ~/
	AllowSudo     bool    // allow sudo (default: false)
	AllowNetwork  bool    // allow network commands (curl, wget) (default: false)
	SafeMode      bool    // if true, block ALL destructive commands

	// Audit callback
	OnExec func(cmd string, risk approval.ShellRisk, allowed bool)

	running int
}

// NewShellExecPolicy creates a policy-aware shell executor.
func NewShellExecPolicy(approvalMgr *approval.Manager, pm *ProcessManager) *ShellExecPolicy {
	return &ShellExecPolicy{
		approvalMgr:    approvalMgr,
		processManager: pm,
		MaxConcurrent:  5,
		AllowSudo:      false,
		AllowNetwork:   false,
		AllowUserHome:  true,
	}
}

// ExecResult wraps the command result with policy metadata.
type PolicyExecResult struct {
	*ExecResult
	Risk     approval.ShellRisk `json:"risk"`
	Patterns []string           `json:"patterns,omitempty"`
	Approved bool               `json:"approved"`
}

// Execute runs a command through the full policy pipeline.
func (sp *ShellExecPolicy) Execute(ctx context.Context, opts ExecOptions, tenantID string) (*PolicyExecResult, error) {
	// ── Step 1: Shell danger analysis ──
	analysis := approval.AnalyzeShellCommand(opts.Command)

	result := &PolicyExecResult{
		Risk:     analysis.Risk,
		Patterns: analysis.Patterns,
	}

	// ── Step 2: SafeMode blocks everything dangerous ──
	if sp.SafeMode && analysis.Risk >= approval.ShellDanger {
		result.Approved = false
		result.ExecResult = &ExecResult{
			Output:   fmt.Sprintf("Blocked by safe mode: %s", strings.Join(analysis.Patterns, "; ")),
			ExitCode: -1,
			State:    ProcessError,
		}
		sp.audit(opts.Command, analysis.Risk, false)
		return result, nil
	}

	// ── Step 3: Critical commands always blocked ──
	if analysis.Risk == approval.ShellCritical {
		result.Approved = false
		result.ExecResult = &ExecResult{
			Output:   fmt.Sprintf("Command blocked (critical risk): %s", strings.Join(analysis.Patterns, "; ")),
			ExitCode: -1,
			State:    ProcessError,
		}
		sp.audit(opts.Command, analysis.Risk, false)
		return result, nil
	}

	// ── Step 4: Policy checks ──
	if !sp.AllowSudo && containsWord(opts.Command, "sudo", "su", "doas") {
		result.Approved = false
		result.ExecResult = &ExecResult{
			Output:   "Privilege escalation not allowed by policy",
			ExitCode: -1,
			State:    ProcessError,
		}
		sp.audit(opts.Command, approval.ShellDanger, false)
		return result, nil
	}

	if !sp.AllowNetwork && containsWord(opts.Command, "curl", "wget", "nc", "ssh", "scp") {
		result.Approved = false
		result.ExecResult = &ExecResult{
			Output:   "Network commands not allowed by policy",
			ExitCode: -1,
			State:    ProcessError,
		}
		sp.audit(opts.Command, approval.ShellCaution, false)
		return result, nil
	}

	// ── Step 5: Concurrency limit ──
	sp.mu.Lock()
	if sp.MaxConcurrent > 0 && sp.running >= sp.MaxConcurrent {
		sp.mu.Unlock()
		return nil, fmt.Errorf("max concurrent shell processes reached (%d)", sp.MaxConcurrent)
	}
	sp.running++
	sp.mu.Unlock()
	defer func() {
		sp.mu.Lock()
		sp.running--
		sp.mu.Unlock()
	}()

	// ── Step 6: Approval gate for dangerous commands ──
	if analysis.Risk >= approval.ShellDanger && sp.approvalMgr != nil {
		req := &approval.Request{
			Category:  approval.CatCodeExec,
			RiskLevel: approval.RiskHigh,
			Summary:   fmt.Sprintf("Shell命令需要审批: %s", analysis.Command),
			Details: map[string]any{
				"skill_name":     "exec_command",
				"command":        opts.Command,
				"shell_risk":     string(analysis.Risk),
				"shell_patterns": analysis.Patterns,
			},
			Requester: "shell_policy",
			TenantID:  tenantID,
		}

		resolved := sp.approvalMgr.RequestApproval(req)
		if resolved.Status != approval.StatusApproved && resolved.Status != approval.StatusAutoApproved {
			result.Approved = false
			result.ExecResult = &ExecResult{
				Output:   fmt.Sprintf("Command denied by approval: %s", resolved.Reason),
				ExitCode: -1,
				State:    ProcessError,
			}
			sp.audit(opts.Command, analysis.Risk, false)
			return result, nil
		}
	}

	result.Approved = true
	sp.audit(opts.Command, analysis.Risk, true)

	// ── Step 7: Apply OS-specific wrappers ──
	opts.Command = sp.wrapCommand(opts.Command)

	// ── Step 8: Execute ──
	slog.Info("shell_policy: executing",
		"cmd_len", len(opts.Command), "risk", analysis.Risk, "bg", opts.Background)

	execResult, err := sp.processManager.Exec(ctx, opts)
	result.ExecResult = execResult
	return result, err
}

// wrapCommand applies OS-specific safety wrappers.
func (sp *ShellExecPolicy) wrapCommand(cmd string) string {
	if runtime.GOOS == "windows" {
		return cmd // Windows commands go through cmd /c already
	}

	// On Unix, add timeout wrapper if not already present
	if !strings.HasPrefix(cmd, "timeout ") && !strings.Contains(cmd, "timeout ") {
		// Don't wrap interactive commands
		if !containsWord(cmd, "vim", "nano", "vi", "less", "more", "top", "htop") {
			// Wrap with 5-minute timeout by default
			cmd = fmt.Sprintf("timeout 300 %s", cmd)
		}
	}

	return cmd
}

func (sp *ShellExecPolicy) audit(cmd string, risk approval.ShellRisk, allowed bool) {
	if sp.OnExec != nil {
		sp.OnExec(cmd, risk, allowed)
	}
}

func containsWord(s string, words ...string) bool {
	lower := strings.ToLower(s)
	for _, w := range words {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}
