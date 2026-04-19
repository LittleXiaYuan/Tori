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

type WindsurfAdapter struct {
	mu       sync.Mutex
	sessions map[string]*windsurfSession
}

type windsurfSession struct {
	workDir string
	cancel  context.CancelFunc
}

func NewWindsurfAdapter() *WindsurfAdapter {
	return &WindsurfAdapter{
		sessions: make(map[string]*windsurfSession),
	}
}

func (a *WindsurfAdapter) Name() string { return "windsurf" }

func (a *WindsurfAdapter) Available() bool {
	_, err := exec.LookPath(windsurfBinary())
	return err == nil
}

func (a *WindsurfAdapter) Launch(ctx context.Context, task LaunchTask) (*LaunchResult, error) {
	if err := a.injectConfig(task.WorkDir, task.MCPEndpoint, task.Description, task.Rules); err != nil {
		slog.Warn("windsurf: failed to inject config", "err", err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(childCtx, windsurfBinary(), task.WorkDir)

	sessionID := fmt.Sprintf("windsurf-%s-%d", task.TaskID, time.Now().UnixMilli())

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start windsurf: %w", err)
	}

	a.mu.Lock()
	a.sessions[sessionID] = &windsurfSession{workDir: task.WorkDir, cancel: cancel}
	a.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		slog.Info("windsurf: session ended", "session", sessionID)
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

func (a *WindsurfAdapter) Monitor(_ context.Context, sessionID string) (<-chan ProgressEvent, error) {
	ch := make(chan ProgressEvent, 1)
	close(ch)
	return ch, nil
}

func (a *WindsurfAdapter) Stop(sessionID string) error {
	a.mu.Lock()
	s, ok := a.sessions[sessionID]
	if ok {
		s.cancel()
		delete(a.sessions, sessionID)
	}
	a.mu.Unlock()
	return nil
}

func (a *WindsurfAdapter) injectConfig(workDir, mcpEndpoint, taskDesc, rules string) error {
	configDir := filepath.Join(workDir, ".windsurf")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"yunque-dispatch": map[string]any{
				"serverUrl": mcpEndpoint,
			},
		},
	}
	data, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(configDir, "mcp_config.json"), data, 0o644); err != nil {
		return err
	}

	rulesContent := fmt.Sprintf(`# Yunque Worker (Windsurf)

You are an automated worker for Yunque orchestration.
MCP Endpoint: %s

## Workflow
1. register_worker → get_pending_tasks → claim_task
2. Execute the task
3. report_progress → submit_result

## Task
%s

%s`, mcpEndpoint, taskDesc, rules)

	return os.WriteFile(filepath.Join(workDir, ".windsurfrules"), []byte(rulesContent), 0o644)
}

func windsurfBinary() string {
	if runtime.GOOS == "windows" {
		return "windsurf.exe"
	}
	return "windsurf"
}
