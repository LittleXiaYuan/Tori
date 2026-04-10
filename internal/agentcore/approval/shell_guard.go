package approval

import (
	"regexp"
	"strings"
)

// ──────────────────────────────────────────────
// ShellGuard — Dangerous Shell Syntax Detection
//
// Detects risky shell patterns BEFORE the command reaches the sandbox:
//   - Command chaining via ; && ||
//   - Pipe to destructive commands (| rm, | dd)
//   - Subshell / command substitution $() ``
//   - Privilege escalation (sudo, su, doas)
//   - Environment/PATH manipulation
//   - Reverse shells, network exfiltration
//   - Recursive destructive ops (rm -rf, del /s)
//   - File permission abuse (chmod 777, chown root)
// ──────────────────────────────────────────────

// ShellRisk classifies how risky a shell command is.
type ShellRisk string

const (
	ShellSafe     ShellRisk = "safe"     // no risk patterns detected
	ShellCaution  ShellRisk = "caution"  // minor risk, log warning
	ShellDanger   ShellRisk = "danger"   // needs human approval
	ShellCritical ShellRisk = "critical" // should be blocked outright
)

// ShellCheckResult contains the analysis of a shell command.
type ShellCheckResult struct {
	Risk     ShellRisk `json:"risk"`
	Patterns []string  `json:"patterns"` // matched pattern descriptions
	Command  string    `json:"command"`  // sanitized command preview
}

// dangerPattern pairs a detection regex with description and risk level.
type dangerPattern struct {
	re   *regexp.Regexp
	desc string
	risk ShellRisk
}

// Pre-compiled patterns for performance.
var shellPatterns = []dangerPattern{
	// ── Critical: immediate block ──
	{regexp.MustCompile(`(?i)\brm\s+(-\w*r\w*f|--force|-\w*f\w*r)\b.*(/|\*)`), "recursive force delete on root/wildcard", ShellCritical},
	{regexp.MustCompile(`(?i)\bdd\s+.*\bof=/dev/`), "raw disk write via dd", ShellCritical},
	{regexp.MustCompile(`(?i)\bmkfs\b`), "filesystem format", ShellCritical},
	{regexp.MustCompile(`(?i)\b(shutdown|reboot|halt|poweroff|init\s+[06])\b`), "system shutdown/reboot", ShellCritical},
	{regexp.MustCompile(`(?i)/dev/(sda|nvme|vda|hd[a-z])\b`), "raw block device access", ShellCritical},
	{regexp.MustCompile(`(?i)\bformat\s+[a-z]:`), "Windows format drive", ShellCritical},
	{regexp.MustCompile(`(?i)(rd|rmdir)\s+/s\s+/q`), "Windows recursive directory delete", ShellCritical},
	{regexp.MustCompile(`(?i)del\s+/[sfq]+\s.*(\\|\*)`), "Windows recursive file delete", ShellCritical},

	// ── Critical: reverse shell / exfiltration ──
	{regexp.MustCompile(`(?i)\b(bash|sh|zsh)\s+(-i\s+)?>&?\s*/dev/tcp/`), "reverse shell via /dev/tcp", ShellCritical},
	{regexp.MustCompile(`(?i)\bnc\s+(-\w+\s+)*\w+\s+\d+\s*(-e|-c)\b`), "netcat reverse shell", ShellCritical},
	{regexp.MustCompile(`(?i)\bpython[23]?\s+.*socket.*connect\b`), "Python reverse shell", ShellCritical},
	{regexp.MustCompile(`(?i)\bcurl\s+.*\|\s*(bash|sh)\b`), "pipe curl to shell", ShellCritical},
	{regexp.MustCompile(`(?i)\bwget\s+.*\|\s*(bash|sh)\b`), "pipe wget to shell", ShellCritical},

	// ── Danger: needs approval ──
	{regexp.MustCompile(`(?i)\bsudo\b`), "privilege escalation (sudo)", ShellDanger},
	{regexp.MustCompile(`(?i)\bsu\s+(-|root)\b`), "switch to root user", ShellDanger},
	{regexp.MustCompile(`(?i)\bdoas\b`), "privilege escalation (doas)", ShellDanger},
	{regexp.MustCompile(`(?i)\bchmod\s+[0-7]*7[0-7]*\b`), "world-writable permission", ShellDanger},
	{regexp.MustCompile(`(?i)\bchown\s+root\b`), "ownership change to root", ShellDanger},
	{regexp.MustCompile(`(?i)\biptables\b`), "firewall rule modification", ShellDanger},
	{regexp.MustCompile(`(?i)\bufw\b`), "firewall management", ShellDanger},
	{regexp.MustCompile(`(?i)\bsystemctl\s+(stop|disable|mask)\b`), "service management (stop/disable)", ShellDanger},
	{regexp.MustCompile(`(?i)\bssh\s+`), "SSH connection", ShellDanger},
	{regexp.MustCompile(`(?i)\bscp\s+`), "SCP file transfer", ShellDanger},
	{regexp.MustCompile(`(?i)\brm\s+(-\w*r|--recursive)\b`), "recursive delete", ShellDanger},
	{regexp.MustCompile(`(?i)\bfind\s+.*-delete\b`), "find with delete", ShellDanger},
	{regexp.MustCompile(`(?i)\b(crontab|at)\s+`), "scheduled task modification", ShellDanger},
	{regexp.MustCompile(`(?i)\bkill\s+(-9\s+)?(-1|1)\b`), "kill all processes", ShellDanger},
	{regexp.MustCompile(`(?i)\bpkill\s+`), "process kill by name", ShellDanger},
	{regexp.MustCompile(`(?i)>\s*/etc/`), "write to /etc/", ShellDanger},
	{regexp.MustCompile(`(?i)\bexport\s+PATH=`), "PATH manipulation", ShellDanger},
	{regexp.MustCompile(`(?i)\bexport\s+LD_PRELOAD=`), "LD_PRELOAD injection", ShellDanger},

	// ── Caution: log and monitor ──
	{regexp.MustCompile(`(?i)\bcurl\b`), "network request (curl)", ShellCaution},
	{regexp.MustCompile(`(?i)\bwget\b`), "network download (wget)", ShellCaution},
	{regexp.MustCompile(`(?i)\bpip\s+install\b`), "package install (pip)", ShellCaution},
	{regexp.MustCompile(`(?i)\bnpm\s+install\b`), "package install (npm)", ShellCaution},
	{regexp.MustCompile(`(?i)\bapt(-get)?\s+install\b`), "package install (apt)", ShellCaution},
	{regexp.MustCompile(`(?i)\byum\s+install\b`), "package install (yum)", ShellCaution},
	{regexp.MustCompile(`(?i)\bdocker\s+`), "Docker command", ShellCaution},
	{regexp.MustCompile(`(?i)\bgit\s+push\b`), "git push", ShellCaution},
	{regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`), "git hard reset", ShellCaution},
	{regexp.MustCompile(`;\s*\w`), "command chaining (;)", ShellCaution},
	{regexp.MustCompile(`&&`), "conditional chaining (&&)", ShellCaution},
	{regexp.MustCompile(`\|\|`), "fallback chaining (||)", ShellCaution},
	{regexp.MustCompile(`\$\(`), "command substitution $()", ShellCaution},
	{regexp.MustCompile("`"), "backtick command substitution", ShellCaution},
}

// AnalyzeShellCommand checks a command for dangerous patterns.
func AnalyzeShellCommand(cmd string) ShellCheckResult {
	result := ShellCheckResult{
		Risk:    ShellSafe,
		Command: sanitizeForLog(cmd),
	}

	for _, p := range shellPatterns {
		if p.re.MatchString(cmd) {
			result.Patterns = append(result.Patterns, p.desc)
			if riskOrder(p.risk) > riskOrder(result.Risk) {
				result.Risk = p.risk
			}
		}
	}

	return result
}

// riskOrder returns numeric priority (higher = more dangerous).
func riskOrder(r ShellRisk) int {
	switch r {
	case ShellCritical:
		return 3
	case ShellDanger:
		return 2
	case ShellCaution:
		return 1
	default:
		return 0
	}
}

// sanitizeForLog truncates and cleans a command for safe logging.
func sanitizeForLog(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	runes := []rune(cmd)
	if len(runes) > 120 {
		return string(runes[:120]) + "..."
	}
	return cmd
}
