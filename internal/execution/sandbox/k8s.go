package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// K8s Runtime — Kubernetes Pod sandbox execution
// ──────────────────────────────────────────────

// K8sConfig configures the Kubernetes runtime.
type K8sConfig struct {
	Namespace       string        `json:"namespace"`         // default: "tori-sandbox"
	Image           string        `json:"image"`             // default: "alpine:latest"
	ServiceAccount  string        `json:"service_account,omitempty"`
	CPULimit        string        `json:"cpu_limit"`         // default: "500m"
	MemoryLimit     string        `json:"memory_limit"`      // default: "256Mi"
	Timeout         time.Duration `json:"timeout"`           // default: 5min
	PVCName         string        `json:"pvc_name,omitempty"` // for persistent data
	ImagePullPolicy string       `json:"image_pull_policy"`  // default: "IfNotPresent"
	Labels          map[string]string `json:"labels,omitempty"`
}

// DefaultK8sConfig returns sensible defaults.
func DefaultK8sConfig() K8sConfig {
	return K8sConfig{
		Namespace:       "tori-sandbox",
		Image:           "alpine:latest",
		CPULimit:        "500m",
		MemoryLimit:     "256Mi",
		Timeout:         5 * time.Minute,
		ImagePullPolicy: "IfNotPresent",
	}
}

// ──────────────────────────────────────────────
// PodState
// ──────────────────────────────────────────────

type PodState string

const (
	PodPending    PodState = "pending"
	PodRunning    PodState = "running"
	PodSucceeded  PodState = "succeeded"
	PodFailed     PodState = "failed"
	PodTerminated PodState = "terminated"
)

// PodInfo tracks a sandbox pod.
type PodInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	State     PodState  `json:"state"`
	Image     string    `json:"image"`
	Command   string    `json:"command,omitempty"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	ExitCode  int       `json:"exit_code"`
	CreatedAt time.Time `json:"created_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// ──────────────────────────────────────────────
// K8sExecutor — interface for K8s operations
// ──────────────────────────────────────────────

// K8sExecutor abstracts Kubernetes API calls for testability.
type K8sExecutor interface {
	CreatePod(ctx context.Context, cfg K8sConfig, name string, command []string) error
	WaitPod(ctx context.Context, namespace, name string, timeout time.Duration) (PodState, error)
	GetLogs(ctx context.Context, namespace, name string) (string, error)
	DeletePod(ctx context.Context, namespace, name string) error
	GetExitCode(ctx context.Context, namespace, name string) (int, error)
}

// ──────────────────────────────────────────────
// K8sRuntime — manages sandbox pods
// ──────────────────────────────────────────────

// K8sRuntime manages Kubernetes-based sandbox execution.
type K8sRuntime struct {
	mu       sync.RWMutex
	config   K8sConfig
	executor K8sExecutor
	pods     map[string]*PodInfo
}

// NewK8sRuntime creates a K8s runtime.
func NewK8sRuntime(cfg K8sConfig, executor K8sExecutor) *K8sRuntime {
	return &K8sRuntime{
		config:   cfg,
		executor: executor,
		pods:     make(map[string]*PodInfo),
	}
}

// Execute runs a command in a new sandbox pod and waits for completion.
func (kr *K8sRuntime) Execute(ctx context.Context, command string) (*PodInfo, error) {
	if kr.executor == nil {
		return nil, fmt.Errorf("k8s: no executor configured")
	}

	podID := uuid.New().String()[:8]
	podName := fmt.Sprintf("tori-sandbox-%s", podID)

	pod := &PodInfo{
		ID:        podID,
		Name:      podName,
		Namespace: kr.config.Namespace,
		State:     PodPending,
		Image:     kr.config.Image,
		Command:   command,
		CreatedAt: time.Now(),
	}

	kr.mu.Lock()
	kr.pods[podID] = pod
	kr.mu.Unlock()

	slog.Info("k8s: creating pod", "name", podName, "image", kr.config.Image)

	// Parse command into shell exec
	cmdArgs := []string{"sh", "-c", command}

	// Create pod
	if err := kr.executor.CreatePod(ctx, kr.config, podName, cmdArgs); err != nil {
		pod.State = PodFailed
		pod.Error = err.Error()
		return pod, fmt.Errorf("k8s: create pod: %w", err)
	}

	now := time.Now()
	pod.StartedAt = &now
	pod.State = PodRunning

	// Wait for completion
	state, err := kr.executor.WaitPod(ctx, kr.config.Namespace, podName, kr.config.Timeout)
	if err != nil {
		pod.State = PodFailed
		pod.Error = err.Error()
		return pod, fmt.Errorf("k8s: wait pod: %w", err)
	}

	end := time.Now()
	pod.EndedAt = &end
	pod.Duration = end.Sub(now)
	pod.State = state

	// Get logs
	logs, err := kr.executor.GetLogs(ctx, kr.config.Namespace, podName)
	if err == nil {
		pod.Output = logs
	}

	// Get exit code
	exitCode, err := kr.executor.GetExitCode(ctx, kr.config.Namespace, podName)
	if err == nil {
		pod.ExitCode = exitCode
	}

	if state == PodFailed {
		pod.Error = "pod failed with exit code " + fmt.Sprint(exitCode)
	}

	slog.Info("k8s: pod completed", "name", podName, "state", state, "duration", pod.Duration)
	return pod, nil
}

// Cleanup deletes a completed pod.
func (kr *K8sRuntime) Cleanup(ctx context.Context, podID string) error {
	kr.mu.RLock()
	pod, ok := kr.pods[podID]
	kr.mu.RUnlock()
	if !ok {
		return fmt.Errorf("k8s: pod %q not found", podID)
	}

	if err := kr.executor.DeletePod(ctx, pod.Namespace, pod.Name); err != nil {
		return fmt.Errorf("k8s: delete pod: %w", err)
	}

	kr.mu.Lock()
	pod.State = PodTerminated
	kr.mu.Unlock()

	return nil
}

// CleanupAll deletes all tracked pods.
func (kr *K8sRuntime) CleanupAll(ctx context.Context) int {
	kr.mu.RLock()
	pods := make([]*PodInfo, 0, len(kr.pods))
	for _, p := range kr.pods {
		pods = append(pods, p)
	}
	kr.mu.RUnlock()

	cleaned := 0
	for _, p := range pods {
		if err := kr.executor.DeletePod(ctx, p.Namespace, p.Name); err == nil {
			cleaned++
		}
	}
	return cleaned
}

// GetPod returns pod info by ID.
func (kr *K8sRuntime) GetPod(podID string) (*PodInfo, bool) {
	kr.mu.RLock()
	defer kr.mu.RUnlock()
	p, ok := kr.pods[podID]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// ListPods returns all tracked pods.
func (kr *K8sRuntime) ListPods() []*PodInfo {
	kr.mu.RLock()
	defer kr.mu.RUnlock()
	out := make([]*PodInfo, 0, len(kr.pods))
	for _, p := range kr.pods {
		cp := *p
		out = append(out, &cp)
	}
	return out
}

// ──────────────────────────────────────────────
// PodSpec generation (for kubectl or API)
// ──────────────────────────────────────────────

// GeneratePodYAML generates a Kubernetes Pod YAML spec.
func GeneratePodYAML(cfg K8sConfig, podName string, command []string) string {
	var sb strings.Builder
	sb.WriteString("apiVersion: v1\n")
	sb.WriteString("kind: Pod\n")
	sb.WriteString("metadata:\n")
	sb.WriteString(fmt.Sprintf("  name: %s\n", podName))
	sb.WriteString(fmt.Sprintf("  namespace: %s\n", cfg.Namespace))
	if len(cfg.Labels) > 0 {
		sb.WriteString("  labels:\n")
		for k, v := range cfg.Labels {
			sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
		}
	}
	sb.WriteString("spec:\n")
	sb.WriteString("  restartPolicy: Never\n")
	if cfg.ServiceAccount != "" {
		sb.WriteString(fmt.Sprintf("  serviceAccountName: %s\n", cfg.ServiceAccount))
	}
	sb.WriteString("  containers:\n")
	sb.WriteString("  - name: sandbox\n")
	sb.WriteString(fmt.Sprintf("    image: %s\n", cfg.Image))
	sb.WriteString(fmt.Sprintf("    imagePullPolicy: %s\n", cfg.ImagePullPolicy))
	if len(command) > 0 {
		sb.WriteString("    command:\n")
		for _, c := range command {
			sb.WriteString(fmt.Sprintf("    - %q\n", c))
		}
	}
	sb.WriteString("    resources:\n")
	sb.WriteString("      limits:\n")
	sb.WriteString(fmt.Sprintf("        cpu: %s\n", cfg.CPULimit))
	sb.WriteString(fmt.Sprintf("        memory: %s\n", cfg.MemoryLimit))
	if cfg.PVCName != "" {
		sb.WriteString("    volumeMounts:\n")
		sb.WriteString("    - name: data\n")
		sb.WriteString("      mountPath: /data\n")
		sb.WriteString("  volumes:\n")
		sb.WriteString("  - name: data\n")
		sb.WriteString("    persistentVolumeClaim:\n")
		sb.WriteString(fmt.Sprintf("      claimName: %s\n", cfg.PVCName))
	}
	return sb.String()
}
