package orchestrator

import (
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

type CursorAdapter struct {
	mu       sync.Mutex
	sessions map[string]*cursorSession
}

type cursorSession struct {
	workDir string
	cancel  context.CancelFunc
}

func NewCursorAdapter() *CursorAdapter {
	return &CursorAdapter{
		sessions: make(map[string]*cursorSession),
	}
}

func (a *CursorAdapter) Name() string { return "cursor" }

func (a *CursorAdapter) Lifecycle() WorkerLifecycle { return LifecyclePersistent }

func (a *CursorAdapter) Available() bool {
	_, err := exec.LookPath(cursorBinary())
	return err == nil
}

func (a *CursorAdapter) Launch(ctx context.Context, task LaunchTask) (*LaunchResult, error) {
	if err := a.injectMCPConfig(task.WorkDir, task.MCPEndpoint); err != nil {
		slog.Warn("cursor: failed to inject MCP config", "err", err)
	}
	if err := a.injectRules(task.WorkDir, task.Rules, task.MCPEndpoint, task.Description); err != nil {
		slog.Warn("cursor: failed to inject rules", "err", err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(childCtx, cursorBinary(), "--folder", task.WorkDir)

	sessionID := fmt.Sprintf("cursor-%s-%d", task.TaskID, time.Now().UnixMilli())

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start cursor: %w", err)
	}

	a.mu.Lock()
	a.sessions[sessionID] = &cursorSession{workDir: task.WorkDir, cancel: cancel}
	a.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		slog.Info("cursor: session ended", "session", sessionID)
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

func (a *CursorAdapter) Monitor(_ context.Context, sessionID string) (<-chan ProgressEvent, error) {
	ch := make(chan ProgressEvent, 1)
	close(ch)
	return ch, nil
}

func (a *CursorAdapter) Stop(sessionID string) error {
	a.mu.Lock()
	s, ok := a.sessions[sessionID]
	if ok {
		s.cancel()
		delete(a.sessions, sessionID)
	}
	a.mu.Unlock()
	return nil
}

func (a *CursorAdapter) injectMCPConfig(workDir, mcpEndpoint string) error {
	configDir := filepath.Join(workDir, ".cursor")
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

func (a *CursorAdapter) injectRules(workDir, rules, mcpEndpoint, taskDesc string) error {
	rulesDir := filepath.Join(workDir, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(`---
description: Yunque Worker auto-injected rules
alwaysApply: true
---

# Yunque Worker

You are an automated worker connected to Yunque orchestration.
MCP Endpoint: %s

## Auto-Start
On session start, immediately:
1. Call register_worker(id: "cursor-worker", name: "Cursor", type: "cursor", capabilities: ["coding", "testing", "review"])
2. Call get_pending_tasks() to see available work
3. Call claim_task(task_id: <id>) for the first matching task
4. Execute the task in this repository
5. Call report_progress() after each significant step
6. Call submit_result() when done

## Current Task
%s

%s`, mcpEndpoint, taskDesc, rules)

	return os.WriteFile(filepath.Join(rulesDir, "yunque-worker.mdc"), []byte(content), 0o644)
}

func cursorBinary() string {
	if runtime.GOOS == "windows" {
		return "cursor.exe"
	}
	return "cursor"
}
