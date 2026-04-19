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

type TraeAdapter struct {
	mu       sync.Mutex
	sessions map[string]*traeSession
}

type traeSession struct {
	workDir string
	cancel  context.CancelFunc
}

func NewTraeAdapter() *TraeAdapter {
	return &TraeAdapter{
		sessions: make(map[string]*traeSession),
	}
}

func (a *TraeAdapter) Name() string { return "trae" }

func (a *TraeAdapter) Lifecycle() WorkerLifecycle { return LifecyclePersistent }

func (a *TraeAdapter) Available() bool {
	return FindBinary("trae") != ""
}

func (a *TraeAdapter) Launch(ctx context.Context, task LaunchTask) (*LaunchResult, error) {
	if err := a.injectConfig(task.WorkDir, task.MCPEndpoint, task.Description, task.Rules); err != nil {
		slog.Warn("trae: failed to inject config", "err", err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(childCtx, traeBinary(), task.WorkDir)

	sessionID := fmt.Sprintf("trae-%s-%d", task.TaskID, time.Now().UnixMilli())

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start trae: %w", err)
	}

	a.mu.Lock()
	a.sessions[sessionID] = &traeSession{workDir: task.WorkDir, cancel: cancel}
	a.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		slog.Info("trae: session ended", "session", sessionID)
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

func (a *TraeAdapter) Monitor(_ context.Context, _ string) (<-chan ProgressEvent, error) {
	ch := make(chan ProgressEvent, 1)
	close(ch)
	return ch, nil
}

func (a *TraeAdapter) Stop(sessionID string) error {
	a.mu.Lock()
	s, ok := a.sessions[sessionID]
	if ok {
		s.cancel()
		delete(a.sessions, sessionID)
	}
	a.mu.Unlock()
	return nil
}

func (a *TraeAdapter) injectConfig(workDir, mcpEndpoint, taskDesc, rules string) error {
	configDir := filepath.Join(workDir, ".trae")
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
	if err := os.WriteFile(filepath.Join(configDir, "mcp.json"), data, 0o644); err != nil {
		return err
	}

	rulesDir := filepath.Join(configDir, "rules")
	os.MkdirAll(rulesDir, 0o755)

	rulesContent := fmt.Sprintf(`# Yunque Worker (Trae)

You are an automated worker for Yunque orchestration.
MCP Endpoint: %s

## Workflow
1. register_worker → get_pending_tasks → claim_task
2. Execute the task
3. report_progress → submit_result

## Task
%s

%s`, mcpEndpoint, taskDesc, rules)

	return os.WriteFile(filepath.Join(rulesDir, "yunque-worker.md"), []byte(rulesContent), 0o644)
}

func traeBinary() string {
	if runtime.GOOS == "windows" {
		return "trae.exe"
	}
	return "trae"
}
