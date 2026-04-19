package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type GenericAdapterConfig struct {
	AdapterName    string          `json:"adapter_name"`
	Binary         string          `json:"binary"`
	LaunchArgs     string          `json:"launch_args,omitempty"`
	MCPConfigPath  string          `json:"mcp_config_path"`
	RulesFilePath  string          `json:"rules_file_path,omitempty"`
	Lifecycle      WorkerLifecycle `json:"lifecycle,omitempty"`
}

type GenericAdapter struct {
	cfg      GenericAdapterConfig
	mu       sync.Mutex
	sessions map[string]*genericSession
}

type genericSession struct {
	cancel context.CancelFunc
}

func NewGenericAdapter(cfg GenericAdapterConfig) *GenericAdapter {
	return &GenericAdapter{
		cfg:      cfg,
		sessions: make(map[string]*genericSession),
	}
}

func (a *GenericAdapter) Name() string { return a.cfg.AdapterName }

func (a *GenericAdapter) Lifecycle() WorkerLifecycle {
	if a.cfg.Lifecycle != "" {
		return a.cfg.Lifecycle
	}
	return LifecycleEphemeral
}

func (a *GenericAdapter) Available() bool {
	_, err := exec.LookPath(a.cfg.Binary)
	return err == nil
}

func (a *GenericAdapter) Launch(ctx context.Context, task LaunchTask) (*LaunchResult, error) {
	if err := a.injectMCPConfig(task.WorkDir, task.MCPEndpoint); err != nil {
		slog.Warn("generic: failed to inject MCP config", "adapter", a.cfg.AdapterName, "err", err)
	}
	if a.cfg.RulesFilePath != "" {
		if err := a.injectRules(task.WorkDir, task.Description, task.MCPEndpoint); err != nil {
			slog.Warn("generic: failed to inject rules", "adapter", a.cfg.AdapterName, "err", err)
		}
	}

	args := []string{task.WorkDir}
	if a.cfg.LaunchArgs != "" {
		args = []string{a.cfg.LaunchArgs, task.WorkDir}
	}

	childCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(childCtx, a.cfg.Binary, args...)

	sessionID := fmt.Sprintf("%s-%s-%d", a.cfg.AdapterName, task.TaskID, time.Now().UnixMilli())

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start %s: %w", a.cfg.Binary, err)
	}

	a.mu.Lock()
	a.sessions[sessionID] = &genericSession{cancel: cancel}
	a.mu.Unlock()

	go func() {
		_ = cmd.Wait()
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

func (a *GenericAdapter) Monitor(_ context.Context, _ string) (<-chan ProgressEvent, error) {
	ch := make(chan ProgressEvent, 1)
	close(ch)
	return ch, nil
}

func (a *GenericAdapter) Stop(sessionID string) error {
	a.mu.Lock()
	s, ok := a.sessions[sessionID]
	if ok {
		s.cancel()
		delete(a.sessions, sessionID)
	}
	a.mu.Unlock()
	return nil
}

func (a *GenericAdapter) injectMCPConfig(workDir, mcpEndpoint string) error {
	configPath := filepath.Join(workDir, a.cfg.MCPConfigPath)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
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
	return os.WriteFile(configPath, data, 0o644)
}

func (a *GenericAdapter) injectRules(workDir, taskDesc, mcpEndpoint string) error {
	rulesPath := filepath.Join(workDir, a.cfg.RulesFilePath)
	if err := os.MkdirAll(filepath.Dir(rulesPath), 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(`# Yunque Worker (%s)

You are an automated worker for Yunque orchestration.
MCP Endpoint: %s

## Workflow
1. Call register_worker to register
2. Call get_pending_tasks to find tasks
3. Call claim_task to take one
4. Execute the task
5. Call report_progress periodically
6. Call submit_result when done

## Task
%s
`, a.cfg.AdapterName, mcpEndpoint, taskDesc)

	return os.WriteFile(rulesPath, []byte(content), 0o644)
}
