package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ServiceState represents a service plugin's lifecycle state.
type ServiceState string

const (
	ServiceStopped  ServiceState = "stopped"
	ServiceStarting ServiceState = "starting"
	ServiceRunning  ServiceState = "running"
	ServiceFailed   ServiceState = "failed"
)

// ServiceInstance tracks a running service plugin process.
type ServiceInstance struct {
	Plugin   *ScriptPlugin
	State    ServiceState
	Cmd      *exec.Cmd
	Cancel   context.CancelFunc
	StartedAt time.Time
	Restarts  int
	LastError string
}

// ServiceManager manages service-type plugin lifecycles.
type ServiceManager struct {
	mu        sync.RWMutex
	instances map[string]*ServiceInstance // plugin name -> instance
	maxRestarts int
}

// NewServiceManager creates a service manager.
func NewServiceManager() *ServiceManager {
	return &ServiceManager{
		instances:   make(map[string]*ServiceInstance),
		maxRestarts: 5,
	}
}

// Start launches a service plugin as a long-running subprocess.
func (sm *ServiceManager) Start(ctx context.Context, sp *ScriptPlugin) error {
	m := sp.Manifest()
	if m.Type != PluginTypeService {
		return fmt.Errorf("plugin %q is not a service plugin", m.Name)
	}
	if m.Entrypoint == "" {
		return fmt.Errorf("plugin %q has no entrypoint", m.Name)
	}

	sm.mu.Lock()
	if inst, exists := sm.instances[m.Name]; exists && inst.State == ServiceRunning {
		sm.mu.Unlock()
		return fmt.Errorf("plugin %q is already running", m.Name)
	}
	sm.mu.Unlock()

	return sm.startProcess(ctx, sp, 0)
}

func (sm *ServiceManager) startProcess(parentCtx context.Context, sp *ScriptPlugin, restarts int) error {
	m := sp.Manifest()
	ctx, cancel := context.WithCancel(parentCtx)

	entrypoint := m.Entrypoint
	var cmd *exec.Cmd

	switch m.Language {
	case "python":
		interpreter := "python3"
		if runtime.GOOS == "windows" {
			interpreter = "python"
		}
		cmd = exec.CommandContext(ctx, interpreter, entrypoint)
	case "node":
		cmd = exec.CommandContext(ctx, "node", entrypoint)
	case "shell":
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/c", entrypoint)
		} else {
			cmd = exec.CommandContext(ctx, "sh", entrypoint)
		}
	default:
		// Treat entrypoint as a direct command
		parts := strings.Fields(entrypoint)
		cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
	}

	cmd.Dir = sp.Dir()
	cmd.Env = append(os.Environ(),
		"TORI_PLUGIN_NAME="+m.Name,
		"TORI_PLUGIN_TYPE=service",
	)
	if m.Port > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TORI_PLUGIN_PORT=%d", m.Port))
	}
	cmd.Stdout = &prefixWriter{prefix: "[" + m.Name + "] ", logger: slog.Default()}
	cmd.Stderr = &prefixWriter{prefix: "[" + m.Name + "] ", logger: slog.Default()}

	inst := &ServiceInstance{
		Plugin:    sp,
		State:     ServiceStarting,
		Cmd:       cmd,
		Cancel:    cancel,
		StartedAt: time.Now(),
		Restarts:  restarts,
	}

	sm.mu.Lock()
	sm.instances[m.Name] = inst
	sm.mu.Unlock()

	if err := cmd.Start(); err != nil {
		inst.State = ServiceFailed
		inst.LastError = err.Error()
		cancel()
		return fmt.Errorf("start service %q: %w", m.Name, err)
	}

	inst.State = ServiceRunning
	slog.Info("service plugin started", "name", m.Name, "pid", cmd.Process.Pid, "port", m.Port)

	// Monitor process in background
	go sm.monitor(parentCtx, sp, inst)

	// Health check if configured
	if m.HealthCheck != "" && m.Port > 0 {
		go sm.healthCheckLoop(ctx, m.Name, m.Port, m.HealthCheck)
	}

	return nil
}

func (sm *ServiceManager) monitor(parentCtx context.Context, sp *ScriptPlugin, inst *ServiceInstance) {
	name := sp.Manifest().Name
	err := inst.Cmd.Wait()

	sm.mu.Lock()
	if inst.State == ServiceRunning {
		inst.State = ServiceFailed
		if err != nil {
			inst.LastError = err.Error()
		}
		slog.Warn("service plugin exited", "name", name, "err", err, "restarts", inst.Restarts)

		// Auto-restart if under limit
		if inst.Restarts < sm.maxRestarts {
			sm.mu.Unlock()
			time.Sleep(time.Duration(inst.Restarts+1) * 2 * time.Second) // backoff
			slog.Info("restarting service plugin", "name", name, "attempt", inst.Restarts+1)
			if restartErr := sm.startProcess(parentCtx, sp, inst.Restarts+1); restartErr != nil {
				slog.Error("failed to restart service plugin", "name", name, "err", restartErr)
			}
			return
		}
		slog.Error("service plugin exceeded max restarts", "name", name, "max", sm.maxRestarts)
	}
	sm.mu.Unlock()
}

func (sm *ServiceManager) healthCheckLoop(ctx context.Context, name string, port int, path string) {
	url := fmt.Sprintf("http://localhost:%d%s", port, path)
	client := &http.Client{Timeout: 5 * time.Second}

	// Wait for startup
	time.Sleep(3 * time.Second)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := client.Get(url)
			if err != nil {
				slog.Warn("service health check failed", "name", name, "err", err)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				slog.Warn("service health check unhealthy", "name", name, "status", resp.StatusCode)
			}
		}
	}
}

// Stop terminates a running service plugin.
func (sm *ServiceManager) Stop(name string) error {
	sm.mu.Lock()
	inst, ok := sm.instances[name]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("service %q not found", name)
	}
	inst.State = ServiceStopped
	inst.Restarts = sm.maxRestarts + 1 // prevent auto-restart
	sm.mu.Unlock()

	inst.Cancel()
	slog.Info("service plugin stopped", "name", name)
	return nil
}

// StopAll stops all running service plugins.
func (sm *ServiceManager) StopAll() {
	sm.mu.RLock()
	names := make([]string, 0, len(sm.instances))
	for name := range sm.instances {
		names = append(names, name)
	}
	sm.mu.RUnlock()

	for _, name := range names {
		if err := sm.Stop(name); err != nil {
			slog.Warn("failed to stop service", "name", name, "err", err)
		}
	}
}

// Status returns info about a service plugin.
func (sm *ServiceManager) Status(name string) (ServiceInfo, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	inst, ok := sm.instances[name]
	if !ok {
		return ServiceInfo{}, false
	}
	return ServiceInfo{
		Name:      name,
		State:     inst.State,
		StartedAt: inst.StartedAt,
		Restarts:  inst.Restarts,
		LastError: inst.LastError,
		Port:      inst.Plugin.Manifest().Port,
	}, true
}

// AllStatus returns status of all service plugins.
func (sm *ServiceManager) AllStatus() []ServiceInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	out := make([]ServiceInfo, 0, len(sm.instances))
	for name, inst := range sm.instances {
		out = append(out, ServiceInfo{
			Name:      name,
			State:     inst.State,
			StartedAt: inst.StartedAt,
			Restarts:  inst.Restarts,
			LastError: inst.LastError,
			Port:      inst.Plugin.Manifest().Port,
		})
		_ = name
	}
	return out
}

// ServiceInfo is serializable status for a service plugin.
type ServiceInfo struct {
	Name      string       `json:"name"`
	State     ServiceState `json:"state"`
	StartedAt time.Time    `json:"started_at"`
	Restarts  int          `json:"restarts"`
	LastError string       `json:"last_error,omitempty"`
	Port      int          `json:"port,omitempty"`
}

// prefixWriter logs subprocess output with a plugin name prefix.
type prefixWriter struct {
	prefix string
	logger *slog.Logger
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	lines := strings.Split(strings.TrimRight(string(p), "\n"), "\n")
	for _, line := range lines {
		if line != "" {
			pw.logger.Info(pw.prefix + line)
		}
	}
	return len(p), nil
}
