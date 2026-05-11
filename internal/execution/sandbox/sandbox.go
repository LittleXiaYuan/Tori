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

type Result struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration string `json:"duration"`
}

// Policy controls what the sandbox can and can't do.
//
// AllowCommands and BlockCommands are matched against the *base name* of the
// resolved command (case-insensitive), not against a substring of the full
// command line. This avoids classic substring-bypass tricks like
//
//	cat /tmp/notes-about-rm-rf.txt
//
// being blocked because the words "rm" appear, or
//
//	rm  -rf  /
//
// (extra whitespace) bypassing a "rm -rf /" string contains check.
//
// Dangerous argument combinations (currently `rm`-style root deletion and the
// Windows `del /s` / `rd /s` family) are vetted separately by inspecting the
// individual argv elements rather than the joined string.
type Policy struct {
	AllowCommands  []string // base-name allowlist (empty = all base names allowed)
	BlockCommands  []string // base-name blocklist
	AllowPaths     []string // allowed file system paths
	HostReadPaths  []string // host paths the sandbox can READ (read-only)
	MaxDuration    time.Duration
	MaxOutputBytes int
	AllowNetwork   bool
}

// DefaultPolicy is conservative: base-name allowlist, no network. Dangerous
// argv combinations (rm -rf /, del /s, rd /s) are caught by the
// argv-aware check inside Exec, so they no longer need to live in the
// substring-based BlockCommands list.
func DefaultPolicy() Policy {
	return Policy{
		AllowCommands:  []string{"echo", "cat", "head", "tail", "wc", "sort", "grep", "find", "ls", "dir", "type", "python3", "python", "node"},
		BlockCommands:  []string{"format", "shutdown", "reboot", "curl", "wget", "nc", "ssh", "scp"},
		MaxDuration:    30 * time.Second,
		MaxOutputBytes: 64 * 1024, // 64KB
		AllowNetwork:   false,
	}
}

// ---- tier templates ----

// TierName is one of personal/family/public.
type TierName string

const (
	TierPersonal TierName = "personal" // developer's own machine — relaxed
	TierFamily   TierName = "family"   // shared household — moderate
	TierPublic   TierName = "public"   // public-facing service — strict
)

// PersonalPolicy: your own machine, mostly unrestricted.
func PersonalPolicy() Policy {
	return Policy{
		AllowCommands:  nil, // no whitelist → all base names allowed
		BlockCommands:  []string{"format", "shutdown", "reboot"},
		MaxDuration:    2 * time.Minute,
		MaxOutputBytes: 256 * 1024, // 256KB
		AllowNetwork:   true,
	}
}

// FamilyPolicy: shared device, curated allowlist, no network.
func FamilyPolicy() Policy {
	return Policy{
		AllowCommands:  []string{"echo", "cat", "head", "tail", "wc", "sort", "grep", "find", "ls", "dir", "type", "python3", "python", "node"},
		BlockCommands:  []string{"format", "shutdown", "reboot", "curl", "wget", "nc", "ssh", "scp", "sudo"},
		MaxDuration:    30 * time.Second,
		MaxOutputBytes: 64 * 1024,
		AllowNetwork:   false,
	}
}

// PublicPolicy: public-facing, very locked down. Pair with Docker.
func PublicPolicy() Policy {
	return Policy{
		AllowCommands:  []string{"echo", "cat", "head", "ls"},
		BlockCommands:  []string{"rm", "mv", "cp", "chmod", "chown", "curl", "wget", "nc", "ssh", "scp", "sudo", "su", "shutdown", "reboot", "format", "python", "node", "bash", "sh"},
		MaxDuration:    10 * time.Second,
		MaxOutputBytes: 16 * 1024, // 16KB
		AllowNetwork:   false,
	}
}

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

type Sandbox struct {
	workDir   string
	policy    Policy
	useDocker bool
	dockerImg string
}

// New spins up a sandbox with its own workdir under baseDir.
func New(baseDir string, policy Policy) (*Sandbox, error) {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = os.TempDir()
	}
	workDir := filepath.Join(baseDir, fmt.Sprintf("sandbox_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}
	return &Sandbox{workDir: workDir, policy: policy}, nil
}

// NewDocker tries Docker isolation, falls back to local process if docker isn't available.
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

func (s *Sandbox) WorkDir() string { return s.workDir }

func (s *Sandbox) Exec(ctx context.Context, command string, args ...string) (*Result, error) {
	base := commandBaseName(command)

	// 1. Base-name blocklist: matches the resolved binary name only, so
	//    `cat /tmp/notes-mentioning-rm.txt` is not collateral-blocked just
	//    because "rm" appears in the args.
	for _, blocked := range s.policy.BlockCommands {
		if strings.EqualFold(base, blocked) {
			return &Result{ExitCode: -1, Stderr: fmt.Sprintf("blocked command: %s", blocked)}, nil
		}
	}

	// 2. Argv-aware combination check: catches root-deletion variants that
	//    a string blocklist cannot reliably express.
	if reason := dangerousArgvCombo(base, args); reason != "" {
		return &Result{ExitCode: -1, Stderr: "blocked dangerous argv combination: " + reason}, nil
	}

	if len(s.policy.AllowCommands) > 0 {
		allowed := false
		for _, a := range s.policy.AllowCommands {
			if strings.EqualFold(base, a) {
				allowed = true
				break
			}
		}
		if !allowed {
			return &Result{ExitCode: -1, Stderr: fmt.Sprintf("command not in allowlist: %s", base)}, nil
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

func (s *Sandbox) WriteFile(name, content string) error {
	path, err := s.resolveWorkPath(name)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func (s *Sandbox) ReadFile(name string) (string, error) {
	path, err := s.resolveWorkPath(name)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

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

// ReadHostFile reads from the host filesystem. Respects HostReadPaths allowlist.
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

func (s *Sandbox) resolveWorkPath(name string) (string, error) {
	path := filepath.Join(s.workDir, filepath.Clean(name))
	rel, err := filepath.Rel(s.workDir, path)
	if err != nil {
		return "", fmt.Errorf("path escape: %s", name)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escape: %s", name)
	}
	return path, nil
}

func (s *Sandbox) isHostPathAllowed(abs string) bool {
	if len(s.policy.HostReadPaths) == 0 {
		return false
	}
	// Resolve symlinks to prevent link-based escapes
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		resolved = filepath.Clean(abs)
	} else {
		resolved = filepath.Clean(resolved)
	}
	for _, allowed := range s.policy.HostReadPaths {
		allowedClean := filepath.Clean(allowed)
		allowedResolved, resolveErr := filepath.EvalSymlinks(allowedClean)
		if resolveErr != nil {
			allowedResolved = allowedClean
		}
		rel, relErr := filepath.Rel(allowedResolved, resolved)
		if relErr != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)) {
			return true
		}
	}
	return false
}

func (s *Sandbox) execDocker(ctx context.Context, start time.Time, command string, args ...string) (*Result, error) {
	dockerArgs := []string{
		"run", "--rm",
		"-v", s.workDir + ":/workspace",
		"-w", "/workspace",
		"--read-only",
		"--tmpfs", "/tmp:rw,noexec,nosuid,size=64m",
		"--pids-limit", "64",
		"--security-opt", "no-new-privileges",
		"--user", "65534:65534",
	}
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

func (s *Sandbox) UseDocker() bool { return s.useDocker }

func (s *Sandbox) Cleanup() error {
	return os.RemoveAll(s.workDir)
}

func truncate(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n...[truncated]"
}

// commandBaseName returns the lowercased basename of a command path with
// any trailing .exe (Windows) or .bat/.cmd extension stripped, so that
// `C:\Windows\System32\cmd.exe`, `cmd.exe` and `cmd` all resolve the same.
func commandBaseName(command string) string {
	if command == "" {
		return ""
	}
	b := filepath.Base(command)
	b = strings.ToLower(b)
	for _, ext := range []string{".exe", ".bat", ".cmd", ".com"} {
		if strings.HasSuffix(b, ext) {
			b = strings.TrimSuffix(b, ext)
			break
		}
	}
	return b
}

// interpreterInlineFlags lists flags that allow inline code execution for
// each interpreter. These bypass file-based sandboxing because the user
// can embed arbitrary OS commands in the inline string.
var interpreterInlineFlags = map[string][]string{
	"python":  {"-c", "--command"},
	"python3": {"-c", "--command"},
	"node":    {"-e", "--eval", "-p", "--print"},
	"ruby":    {"-e"},
	"perl":    {"-e"},
}

// dangerousArgvCombo inspects argv per-token (not as a concatenated string)
// for command + flag + target combinations that are too risky for the
// sandbox to ever run. Returns a non-empty reason when a match is found.
func dangerousArgvCombo(base string, args []string) string {
	if flags, ok := interpreterInlineFlags[base]; ok {
		for _, a := range args {
			la := strings.ToLower(a)
			for _, f := range flags {
				if la == f {
					return base + " inline code execution via " + f
				}
			}
		}
	}

	switch base {
	case "rm":
		// Block recursive force-delete of root or root-relative paths.
		recursive := false
		force := false
		for _, a := range args {
			la := strings.ToLower(a)
			if la == "-r" || la == "--recursive" {
				recursive = true
			}
			if la == "-rf" || la == "-fr" {
				recursive = true
				force = true
			}
			if la == "-f" || la == "--force" {
				force = true
			}
		}
		if recursive {
			for _, a := range args {
				if a == "/" || a == "/*" || a == "/." || a == "C:\\" || strings.EqualFold(a, "c:\\windows") {
					_ = force // captured for completeness; recursive on root is enough
					return "rm recursive on root"
				}
			}
		}
	case "del", "erase":
		// Windows `del /s /q C:\` family.
		recursive := false
		for _, a := range args {
			if strings.EqualFold(a, "/s") || strings.EqualFold(a, "/q") {
				recursive = true
			}
		}
		if recursive {
			for _, a := range args {
				if strings.EqualFold(a, "C:\\") || strings.EqualFold(a, "C:\\*") {
					return "del recursive on system root"
				}
			}
		}
	case "rd", "rmdir":
		recursive := false
		for _, a := range args {
			if strings.EqualFold(a, "/s") || strings.EqualFold(a, "/q") {
				recursive = true
			}
		}
		if recursive {
			for _, a := range args {
				if strings.EqualFold(a, "C:\\") {
					return "rd recursive on system root"
				}
			}
		}
	}
	return ""
}
