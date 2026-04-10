package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"yunque-agent/pkg/safego"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Process states
// ──────────────────────────────────────────────

type ProcessState string

const (
	ProcessRunning  ProcessState = "running"
	ProcessFinished ProcessState = "finished"
	ProcessKilled   ProcessState = "killed"
	ProcessError    ProcessState = "error"
)

// ──────────────────────────────────────────────
// ProcessSession — one background process
// ──────────────────────────────────────────────

type ProcessSession struct {
	ID        string       `json:"id"`
	Command   string       `json:"command"`
	State     ProcessState `json:"state"`
	ExitCode  int          `json:"exit_code"`
	StartedAt time.Time    `json:"started_at"`
	EndedAt   *time.Time   `json:"ended_at,omitempty"`
	Cwd       string       `json:"cwd,omitempty"`

	mu       sync.Mutex
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	output   []string // buffered output lines
	newLines int      // unread lines since last poll
	stdin    io.WriteCloser
}

// Output returns all buffered output.
func (p *ProcessSession) Output() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make([]string, len(p.output))
	copy(cp, p.output)
	return cp
}

// Poll returns new output since last poll and resets the counter.
func (p *ProcessSession) Poll() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.newLines == 0 {
		return nil
	}
	start := len(p.output) - p.newLines
	if start < 0 {
		start = 0
	}
	lines := make([]string, p.newLines)
	copy(lines, p.output[start:])
	p.newLines = 0
	return lines
}

func (p *ProcessSession) appendLine(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	const maxLines = 10000
	if len(p.output) >= maxLines {
		p.output = p.output[len(p.output)-maxLines/2:]
	}
	p.output = append(p.output, line)
	p.newLines++
}

// ──────────────────────────────────────────────
// ExecResult — returned from synchronous exec
// ──────────────────────────────────────────────

type ExecResult struct {
	Output   string       `json:"output"`
	ExitCode int          `json:"exit_code"`
	State    ProcessState `json:"state"`
	// For backgrounded commands:
	SessionID string `json:"session_id,omitempty"`
}

// ──────────────────────────────────────────────
// ExecOptions
// ──────────────────────────────────────────────

type ExecOptions struct {
	Command    string        // shell command
	Cwd        string        // working directory
	Background bool          // immediately background
	TimeoutMs  int64         // kill after this duration (0=default 1800s)
	YieldMs    int64         // auto-background after this delay (0=default 10s)
	Env        []string      // extra env vars
}

// ──────────────────────────────────────────────
// ProcessManager
// ──────────────────────────────────────────────

type ProcessManager struct {
	mu       sync.RWMutex
	sessions map[string]*ProcessSession
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		sessions: make(map[string]*ProcessSession),
	}
}

// Exec runs a command. If Background is true or the command runs longer
// than YieldMs, it is backgrounded and a session ID is returned.
func (pm *ProcessManager) Exec(ctx context.Context, opts ExecOptions) (*ExecResult, error) {
	if opts.Command == "" {
		return nil, fmt.Errorf("exec: empty command")
	}

	timeout := time.Duration(opts.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 1800 * time.Second
	}
	yieldDur := time.Duration(opts.YieldMs) * time.Millisecond
	if yieldDur <= 0 {
		yieldDur = 10 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd", "/c", opts.Command)
	} else {
		cmd = exec.CommandContext(execCtx, "sh", "-c", opts.Command)
	}
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(cmd.Environ(), opts.Env...)
	}

	sess := &ProcessSession{
		ID:        uuid.New().String(),
		Command:   opts.Command,
		State:     ProcessRunning,
		StartedAt: time.Now(),
		Cwd:       opts.Cwd,
		cmd:       cmd,
		cancel:    cancel,
	}

	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout // merge stderr into stdout
	stdin, _ := cmd.StdinPipe()
	sess.stdin = stdin

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("exec: start failed: %w", err)
	}

	slog.Info("exec: started", "id", sess.ID, "cmd", truncate(opts.Command, 80))

	// Background immediately
	if opts.Background {
		pm.mu.Lock()
		pm.sessions[sess.ID] = sess
		pm.mu.Unlock()
		go pm.collect(sess, stdout)
		return &ExecResult{
			State:     ProcessRunning,
			SessionID: sess.ID,
		}, nil
	}

	// Foreground with yield
	doneCh := make(chan struct{})
	var resultLines []string

	safego.Go("exec-stdout-"+sess.ID, func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			sess.appendLine(line)
		}
		close(doneCh)
	})

	select {
	case <-doneCh:
		// Completed within yield time
		cmd.Wait()
		now := time.Now()
		sess.EndedAt = &now
		sess.ExitCode = cmd.ProcessState.ExitCode()
		sess.State = ProcessFinished
		cancel()

		resultLines = sess.Output()
		return &ExecResult{
			Output:   strings.Join(resultLines, "\n"),
			ExitCode: sess.ExitCode,
			State:    ProcessFinished,
		}, nil

	case <-time.After(yieldDur):
		// Auto-background
		pm.mu.Lock()
		pm.sessions[sess.ID] = sess
		pm.mu.Unlock()
		safego.Go("exec-bg-wait-"+sess.ID, func() {
			<-doneCh
			cmd.Wait()
			now := time.Now()
			sess.mu.Lock()
			sess.EndedAt = &now
			sess.ExitCode = cmd.ProcessState.ExitCode()
			sess.State = ProcessFinished
			sess.mu.Unlock()
			slog.Info("exec: backgrounded process finished", "id", sess.ID, "exit", sess.ExitCode)
		})
		return &ExecResult{
			Output:    strings.Join(sess.Output(), "\n"),
			State:     ProcessRunning,
			SessionID: sess.ID,
		}, nil
	}
}

func (pm *ProcessManager) collect(sess *ProcessSession, r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		sess.appendLine(scanner.Text())
	}
	sess.cmd.Wait()
	now := time.Now()
	sess.mu.Lock()
	sess.EndedAt = &now
	if sess.cmd.ProcessState != nil {
		sess.ExitCode = sess.cmd.ProcessState.ExitCode()
	}
	sess.State = ProcessFinished
	sess.mu.Unlock()
	slog.Info("exec: background process finished", "id", sess.ID, "exit", sess.ExitCode)
}

// ──────────────────────────────────────────────
// Process management actions
// ──────────────────────────────────────────────

// List returns all sessions.
func (pm *ProcessManager) List() []*ProcessSession {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]*ProcessSession, 0, len(pm.sessions))
	for _, s := range pm.sessions {
		out = append(out, s)
	}
	return out
}

// PollSession returns new output for a session.
func (pm *ProcessManager) PollSession(id string) ([]string, ProcessState, error) {
	pm.mu.RLock()
	s, ok := pm.sessions[id]
	pm.mu.RUnlock()
	if !ok {
		return nil, "", fmt.Errorf("process: session %q not found", id)
	}
	return s.Poll(), s.State, nil
}

// Log returns all output for a session.
func (pm *ProcessManager) Log(id string) ([]string, error) {
	pm.mu.RLock()
	s, ok := pm.sessions[id]
	pm.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("process: session %q not found", id)
	}
	return s.Output(), nil
}

// Write sends stdin data to a running session.
func (pm *ProcessManager) Write(id string, data string) error {
	pm.mu.RLock()
	s, ok := pm.sessions[id]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("process: session %q not found", id)
	}
	if s.State != ProcessRunning {
		return fmt.Errorf("process: session %q is not running", id)
	}
	if s.stdin == nil {
		return fmt.Errorf("process: no stdin pipe")
	}
	_, err := io.WriteString(s.stdin, data)
	return err
}

// Kill terminates a running session.
func (pm *ProcessManager) Kill(id string) error {
	pm.mu.RLock()
	s, ok := pm.sessions[id]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("process: session %q not found", id)
	}
	if s.State != ProcessRunning {
		return fmt.Errorf("process: session %q is not running", id)
	}
	s.cancel()
	s.mu.Lock()
	s.State = ProcessKilled
	now := time.Now()
	s.EndedAt = &now
	s.mu.Unlock()
	slog.Info("exec: killed", "id", id)
	return nil
}

// Clear removes a finished session from memory.
func (pm *ProcessManager) Clear(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	s, ok := pm.sessions[id]
	if !ok {
		return fmt.Errorf("process: session %q not found", id)
	}
	if s.State == ProcessRunning {
		return fmt.Errorf("process: session %q is still running, kill first", id)
	}
	delete(pm.sessions, id)
	return nil
}

// Remove kills if running, clears if finished.
func (pm *ProcessManager) Remove(id string) error {
	pm.mu.RLock()
	s, ok := pm.sessions[id]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("process: session %q not found", id)
	}
	if s.State == ProcessRunning {
		pm.Kill(id)
		time.Sleep(100 * time.Millisecond)
	}
	return pm.Clear(id)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
