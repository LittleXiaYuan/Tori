package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Result is the output of a sandbox execution.
type Result struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration string `json:"duration"`
}

// Policy defines what a sandbox is allowed to do.
type Policy struct {
	AllowCommands  []string // whitelist of allowed commands (empty = all)
	BlockCommands  []string // blacklist of blocked commands
	AllowPaths     []string // allowed file system paths
	HostReadPaths  []string // host paths the sandbox can READ (read-only)
	MaxDuration    time.Duration
	MaxOutputBytes int
	AllowNetwork   bool
}

// DefaultPolicy returns a safe default policy with allowlist-only commands.
func DefaultPolicy() Policy {
	return Policy{
		AllowCommands:  []string{"echo", "cat", "head", "tail", "wc", "sort", "grep", "find", "ls", "dir", "type", "python3", "python", "node"},
		BlockCommands:  []string{"rm -rf /", "format", "shutdown", "reboot", "del /s", "rd /s", "curl", "wget", "nc", "ssh", "scp"},
		MaxDuration:    30 * time.Second,
		MaxOutputBytes: 64 * 1024, // 64KB
		AllowNetwork:   false,
	}
}

// ──────────────────────────────────────────────
// Sandbox Tier Templates
// Three pre-configured security levels for different deployment contexts.
// ──────────────────────────────────────────────

// TierName identifies a pre-configured sandbox security level.
type TierName string

const (
	TierPersonal TierName = "personal" // developer's own machine — relaxed
	TierFamily   TierName = "family"   // shared household — moderate
	TierPublic   TierName = "public"   // public-facing service — strict
)

// PersonalPolicy is relaxed: most commands allowed, network on, longer timeouts.
func PersonalPolicy() Policy {
	return Policy{
		AllowCommands:  nil, // no whitelist → all allowed
		BlockCommands:  []string{"rm -rf /", "format", "shutdown", "reboot"},
		MaxDuration:    2 * time.Minute,
		MaxOutputBytes: 256 * 1024, // 256KB
		AllowNetwork:   true,
	}
}

// FamilyPolicy is moderate: curated command list, no network, standard timeout.
func FamilyPolicy() Policy {
	return Policy{
		AllowCommands:  []string{"echo", "cat", "head", "tail", "wc", "sort", "grep", "find", "ls", "dir", "type", "python3", "python", "node"},
		BlockCommands:  []string{"rm -rf /", "format", "shutdown", "reboot", "del /s", "rd /s", "curl", "wget", "nc", "ssh", "scp", "sudo"},
		MaxDuration:    30 * time.Second,
		MaxOutputBytes: 64 * 1024,
		AllowNetwork:   false,
	}
}

// PublicPolicy is strict: minimal commands, Docker isolation expected, tight limits.
func PublicPolicy() Policy {
	return Policy{
		AllowCommands:  []string{"echo", "cat", "head", "ls"},
		BlockCommands:  []string{"rm", "mv", "cp", "chmod", "chown", "curl", "wget", "nc", "ssh", "scp", "sudo", "su", "shutdown", "reboot", "format", "python", "node", "bash", "sh"},
		MaxDuration:    10 * time.Second,
		MaxOutputBytes: 16 * 1024, // 16KB
		AllowNetwork:   false,
	}
}

// PolicyForTier returns the policy template for a given tier name.
func PolicyForTier(tier TierName) Policy {
	switch tier {
	case TierPersonal:
		return PersonalPolicy()
	case TierFamily:
		return FamilyPolicy()
	case TierPublic:
		return PublicPolicy()
	default:
		return DefaultPolicy()
	}
}

// Sandbox executes commands in a controlled environment.
type Sandbox struct {
	workDir   string
	policy    Policy
	useDocker bool
	dockerImg string
}

// New creates a sandbox with an isolated working directory.
func New(baseDir string, policy Policy) (*Sandbox, error) {
	workDir := filepath.Join(baseDir, fmt.Sprintf("sandbox_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}
	return &Sandbox{workDir: workDir, policy: policy}, nil
}

// NewDocker creates a Docker-isolated sandbox. Falls back to local if Docker unavailable.
func NewDocker(baseDir string, policy Policy, image string) (*Sandbox, error) {
	sb, err := New(baseDir, policy)
	if err != nil {
		return nil, err
	}
	if image == "" {
		image = "alpine:latest"
	}
	// Check if Docker is available
	check := exec.Command("docker", "version")
	if check.Run() == nil {
		sb.useDocker = true
		sb.dockerImg = image
	}
	return sb, nil
}

// WorkDir returns the sandbox working directory.
func (s *Sandbox) WorkDir() string { return s.workDir }

// Exec runs a command inside the sandbox.
func (s *Sandbox) Exec(ctx context.Context, command string, args ...string) (*Result, error) {
	// Security check
	fullCmd := command + " " + strings.Join(args, " ")
	for _, blocked := range s.policy.BlockCommands {
		if strings.Contains(strings.ToLower(fullCmd), strings.ToLower(blocked)) {
			return &Result{ExitCode: -1, Stderr: fmt.Sprintf("blocked command: %s", blocked)}, nil
		}
	}

	if len(s.policy.AllowCommands) > 0 {
		allowed := false
		for _, a := range s.policy.AllowCommands {
			if strings.EqualFold(command, a) {
				allowed = true
				break
			}
		}
		if !allowed {
			return &Result{ExitCode: -1, Stderr: fmt.Sprintf("command not in allowlist: %s", command)}, nil
		}
	}

	timeout := s.policy.MaxDuration
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// Docker isolation path
	if s.useDocker {
		return s.execDocker(ctx, start, command, args...)
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = s.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Stdout:   truncate(stdout.String(), s.policy.MaxOutputBytes),
		Stderr:   truncate(stderr.String(), s.policy.MaxOutputBytes),
		Duration: duration.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Stderr = err.Error()
		}
	}
	return result, nil
}

// WriteFile creates a file in the sandbox.
func (s *Sandbox) WriteFile(name, content string) error {
	path := filepath.Join(s.workDir, filepath.Clean(name))
	if !strings.HasPrefix(path, s.workDir) {
		return fmt.Errorf("path escape: %s", name)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadFile reads a file from the sandbox.
func (s *Sandbox) ReadFile(name string) (string, error) {
	path := filepath.Join(s.workDir, filepath.Clean(name))
	if !strings.HasPrefix(path, s.workDir) {
		return "", fmt.Errorf("path escape: %s", name)
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

// ListFiles lists files in the sandbox directory.
func (s *Sandbox) ListFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(s.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(s.workDir, path)
		if rel != "." {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

// ReadHostFile reads a file from the host system (read-only).
// Only paths under HostReadPaths in the policy are allowed.
func (s *Sandbox) ReadHostFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if !s.isHostPathAllowed(abs) {
		return "", fmt.Errorf("access denied: %s not in allowed host read paths", abs)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return truncate(string(b), s.policy.MaxOutputBytes), nil
}

// CopyFromHost copies a host file into the sandbox (read-only copy).
func (s *Sandbox) CopyFromHost(hostPath, sandboxName string) error {
	abs, err := filepath.Abs(hostPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if !s.isHostPathAllowed(abs) {
		return fmt.Errorf("access denied: %s not in allowed host read paths", abs)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	return s.WriteFile(sandboxName, string(data))
}

// ListHostDir lists files in a host directory (read-only).
func (s *Sandbox) ListHostDir(path string) ([]string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	if !s.isHostPathAllowed(abs) {
		return nil, fmt.Errorf("access denied: %s not in allowed host read paths", abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		suffix := ""
		if e.IsDir() {
			suffix = "/"
		}
		names = append(names, e.Name()+suffix)
	}
	return names, nil
}

// SearchHostFiles searches for files matching a pattern under allowed host paths.
func (s *Sandbox) SearchHostFiles(rootPath, pattern string) ([]string, error) {
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	if !s.isHostPathAllowed(abs) {
		return nil, fmt.Errorf("access denied: %s not in allowed host read paths", abs)
	}
	pattern = strings.ToLower(pattern)
	var matches []string
	_ = filepath.Walk(abs, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if len(matches) >= 100 {
			return filepath.SkipAll
		}
		if strings.Contains(strings.ToLower(info.Name()), pattern) {
			rel, _ := filepath.Rel(abs, p)
			matches = append(matches, rel)
		}
		return nil
	})
	return matches, nil
}

// GrepHostFile searches file content for a query string (read-only).
func (s *Sandbox) GrepHostFile(filePath, query string) ([]string, error) {
	content, err := s.ReadHostFile(filePath)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(query)
	lines := strings.Split(content, "\n")
	var matches []string
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, fmt.Sprintf("%d: %s", i+1, line))
		}
		if len(matches) >= 50 {
			break
		}
	}
	return matches, nil
}

func (s *Sandbox) isHostPathAllowed(abs string) bool {
	if len(s.policy.HostReadPaths) == 0 {
		return false
	}
	abs = filepath.Clean(abs)
	for _, allowed := range s.policy.HostReadPaths {
		allowed = filepath.Clean(allowed)
		if strings.HasPrefix(strings.ToLower(abs), strings.ToLower(allowed)) {
			return true
		}
	}
	return false
}

// execDocker runs a command inside a Docker container with the sandbox workdir mounted.
func (s *Sandbox) execDocker(ctx context.Context, start time.Time, command string, args ...string) (*Result, error) {
	dockerArgs := []string{"run", "--rm", "-v", s.workDir + ":/workspace", "-w", "/workspace"}
	if !s.policy.AllowNetwork {
		dockerArgs = append(dockerArgs, "--network", "none")
	}
	dockerArgs = append(dockerArgs, "--memory", "256m", "--cpus", "1")
	dockerArgs = append(dockerArgs, s.dockerImg, command)
	dockerArgs = append(dockerArgs, args...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Stdout:   truncate(stdout.String(), s.policy.MaxOutputBytes),
		Stderr:   truncate(stderr.String(), s.policy.MaxOutputBytes),
		Duration: duration.String(),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Stderr = err.Error()
		}
	}
	return result, nil
}

// UseDocker returns whether Docker isolation is active.
func (s *Sandbox) UseDocker() bool { return s.useDocker }

// Cleanup removes the sandbox directory.
func (s *Sandbox) Cleanup() error {
	return os.RemoveAll(s.workDir)
}

func truncate(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n...[truncated]"
}
