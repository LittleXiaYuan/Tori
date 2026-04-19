package orchestrator

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type ClaudeCodeAdapter struct {
	mu       sync.Mutex
	sessions map[string]*claudeSession
}

type claudeSession struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

func NewClaudeCodeAdapter() *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{
		sessions: make(map[string]*claudeSession),
	}
}

func (a *ClaudeCodeAdapter) Name() string { return "claude_code" }

func (a *ClaudeCodeAdapter) Lifecycle() WorkerLifecycle { return LifecycleEphemeral }

func (a *ClaudeCodeAdapter) Available() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func (a *ClaudeCodeAdapter) Launch(ctx context.Context, task LaunchTask) (*LaunchResult, error) {
	if err := a.injectMCPConfig(task.WorkDir, task.MCPEndpoint); err != nil {
		slog.Warn("claude_code: failed to inject MCP config", "err", err)
	}

	if err := a.injectRules(task.WorkDir, task.Rules, task.MCPEndpoint); err != nil {
		slog.Warn("claude_code: failed to inject rules", "err", err)
	}

	prompt := buildClaudePrompt(task)

	childCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(childCtx, "claude", "-p", prompt, "--output-format", "json")
	cmd.Dir = task.WorkDir

	sessionID := fmt.Sprintf("claude-%s-%d", task.TaskID, time.Now().UnixMilli())

	a.mu.Lock()
	a.sessions[sessionID] = &claudeSession{cmd: cmd, cancel: cancel}
	a.mu.Unlock()

	if err := cmd.Start(); err != nil {
		cancel()
		a.mu.Lock()
		delete(a.sessions, sessionID)
		a.mu.Unlock()
		return nil, fmt.Errorf("start claude CLI: %w", err)
	}

	go func() {
		err := cmd.Wait()
		if err != nil {
			slog.Info("claude_code: session ended", "session", sessionID, "err", err)
		} else {
			slog.Info("claude_code: session completed", "session", sessionID)
		}
		a.mu.Lock()
		delete(a.sessions, sessionID)
		a.mu.Unlock()
	}()

	return &LaunchResult{
		SessionID: sessionID,
		WorkerID:  sessionID,
		PID:       cmd.Process.Pid,
		StartedAt: time.Now(),
	}, nil
}

func (a *ClaudeCodeAdapter) Monitor(ctx context.Context, sessionID string) (<-chan ProgressEvent, error) {
	ch := make(chan ProgressEvent, 32)
	a.mu.Lock()
	s, ok := a.sessions[sessionID]
	a.mu.Unlock()
	if !ok {
		close(ch)
		return ch, fmt.Errorf("session %q not found", sessionID)
	}

	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		close(ch)
		return ch, err
	}

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- ProgressEvent{
				SessionID: sessionID,
				Type:      "output",
				Data:      scanner.Text(),
				Timestamp: time.Now(),
			}:
			}
		}
		ch <- ProgressEvent{
			SessionID: sessionID,
			Type:      "done",
			Timestamp: time.Now(),
		}
	}()

	return ch, nil
}

func (a *ClaudeCodeAdapter) Stop(sessionID string) error {
	a.mu.Lock()
	s, ok := a.sessions[sessionID]
	if ok {
		s.cancel()
		delete(a.sessions, sessionID)
	}
	a.mu.Unlock()
	if !ok {
		return nil
	}
	return nil
}

func (a *ClaudeCodeAdapter) injectMCPConfig(workDir, mcpEndpoint string) error {
	configDir := filepath.Join(workDir, ".mcp")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"yunque-dispatch": map[string]any{
				"type": "streamable-http",
				"url":  mcpEndpoint,
			},
		},
	}

	data, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "mcp.json"), data, 0o644)
}

func (a *ClaudeCodeAdapter) injectRules(workDir, rules, mcpEndpoint string) error {
	rulesContent := fmt.Sprintf(`# Yunque Worker Rules

You are a worker connected to the Yunque orchestration system.

## Connection
- MCP Endpoint: %s

## Workflow
1. Call register_worker to register yourself
2. Call get_pending_tasks to see available tasks
3. Call claim_task to take a task
4. Work on the task in this repository
5. Call report_progress periodically
6. Call submit_result when done

## Important
- Focus on the claimed task only
- Report progress after each significant step
- Submit complete results with file paths and descriptions

%s`, mcpEndpoint, rules)

	return os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), []byte(rulesContent), 0o644)
}

func buildClaudePrompt(task LaunchTask) string {
	prompt := fmt.Sprintf("你是云雀(Yunque)的自动化 Worker。请执行以下任务：\n\n%s", task.Description)
	if len(task.Steps) > 0 {
		prompt += "\n\n步骤：\n"
		for i, s := range task.Steps {
			prompt += fmt.Sprintf("%d. %s\n", i+1, s)
		}
	}
	prompt += "\n\n请先调用 register_worker 注册为 Worker，然后通过 get_pending_tasks 获取任务，claim_task 领取后开始工作。完成后调用 submit_result 提交结果。"
	return prompt
}

func claudeCLIPath() string {
	if runtime.GOOS == "windows" {
		return "claude.exe"
	}
	return "claude"
}
