package sandbox

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// mockK8sExecutor implements K8sExecutor for testing
type mockK8sExecutor struct {
	createErr error
	waitState PodState
	waitErr   error
	logs      string
	logsErr   error
	exitCode  int
	deleteErr error
	deleted   []string
}

func (m *mockK8sExecutor) CreatePod(_ context.Context, _ K8sConfig, name string, _ []string) error {
	return m.createErr
}

func (m *mockK8sExecutor) WaitPod(_ context.Context, _, _ string, _ time.Duration) (PodState, error) {
	return m.waitState, m.waitErr
}

func (m *mockK8sExecutor) GetLogs(_ context.Context, _, _ string) (string, error) {
	return m.logs, m.logsErr
}

func (m *mockK8sExecutor) DeletePod(_ context.Context, _, name string) error {
	m.deleted = append(m.deleted, name)
	return m.deleteErr
}

func (m *mockK8sExecutor) GetExitCode(_ context.Context, _, _ string) (int, error) {
	return m.exitCode, nil
}

func TestK8sExecuteSuccess(t *testing.T) {
	exec := &mockK8sExecutor{waitState: PodSucceeded, logs: "hello world"}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)

	pod, err := rt.Execute(context.Background(), "echo hello")
	if err != nil {
		t.Fatal(err)
	}
	if pod.State != PodSucceeded {
		t.Fatalf("expected succeeded, got %s", pod.State)
	}
	if pod.Output != "hello world" {
		t.Fatal("wrong output")
	}
	// Duration may be 0 in mock; just verify it's non-negative
	if pod.Duration < 0 {
		t.Fatal("negative duration")
	}
}

func TestK8sExecuteCreateFail(t *testing.T) {
	exec := &mockK8sExecutor{createErr: fmt.Errorf("quota exceeded")}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)

	pod, err := rt.Execute(context.Background(), "echo x")
	if err == nil {
		t.Fatal("expected error")
	}
	if pod.State != PodFailed {
		t.Fatal("should be failed")
	}
}

func TestK8sExecuteWaitFail(t *testing.T) {
	exec := &mockK8sExecutor{waitErr: fmt.Errorf("timeout")}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)

	_, err := rt.Execute(context.Background(), "sleep 999")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestK8sExecutePodFailed(t *testing.T) {
	exec := &mockK8sExecutor{waitState: PodFailed, exitCode: 1, logs: "error output"}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)

	pod, _ := rt.Execute(context.Background(), "exit 1")
	if pod.State != PodFailed {
		t.Fatal("should be failed")
	}
	if pod.ExitCode != 1 {
		t.Fatal("wrong exit code")
	}
}

func TestK8sNoExecutor(t *testing.T) {
	rt := NewK8sRuntime(DefaultK8sConfig(), nil)
	_, err := rt.Execute(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestK8sCleanup(t *testing.T) {
	exec := &mockK8sExecutor{waitState: PodSucceeded}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)

	pod, _ := rt.Execute(context.Background(), "echo x")
	if err := rt.Cleanup(context.Background(), pod.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := rt.GetPod(pod.ID)
	if got.State != PodTerminated {
		t.Fatal("should be terminated")
	}
}

func TestK8sCleanupNotFound(t *testing.T) {
	rt := NewK8sRuntime(DefaultK8sConfig(), nil)
	err := rt.Cleanup(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestK8sCleanupAll(t *testing.T) {
	exec := &mockK8sExecutor{waitState: PodSucceeded}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)
	rt.Execute(context.Background(), "a")
	rt.Execute(context.Background(), "b")

	cleaned := rt.CleanupAll(context.Background())
	if cleaned != 2 {
		t.Fatalf("expected 2 cleaned, got %d", cleaned)
	}
}

func TestK8sListPods(t *testing.T) {
	exec := &mockK8sExecutor{waitState: PodSucceeded}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)
	rt.Execute(context.Background(), "a")
	rt.Execute(context.Background(), "b")

	pods := rt.ListPods()
	if len(pods) != 2 {
		t.Fatalf("expected 2, got %d", len(pods))
	}
}

func TestK8sGetPod(t *testing.T) {
	exec := &mockK8sExecutor{waitState: PodSucceeded}
	rt := NewK8sRuntime(DefaultK8sConfig(), exec)
	pod, _ := rt.Execute(context.Background(), "test")

	got, ok := rt.GetPod(pod.ID)
	if !ok || got.Command != "test" {
		t.Fatal("get failed")
	}

	_, ok = rt.GetPod("nope")
	if ok {
		t.Fatal("should not find")
	}
}

func TestGeneratePodYAML(t *testing.T) {
	cfg := DefaultK8sConfig()
	cfg.Labels = map[string]string{"app": "tori"}
	cfg.PVCName = "tori-data"

	yaml := GeneratePodYAML(cfg, "test-pod", []string{"sh", "-c", "echo hello"})
	if !strings.Contains(yaml, "name: test-pod") {
		t.Fatal("missing pod name")
	}
	if !strings.Contains(yaml, "tori-sandbox") {
		t.Fatal("missing namespace")
	}
	if !strings.Contains(yaml, "app: tori") {
		t.Fatal("missing label")
	}
	if !strings.Contains(yaml, "tori-data") {
		t.Fatal("missing PVC")
	}
}

func TestDefaultK8sConfig(t *testing.T) {
	cfg := DefaultK8sConfig()
	if cfg.Namespace != "tori-sandbox" {
		t.Fatal("wrong default namespace")
	}
	if cfg.Timeout != 5*time.Minute {
		t.Fatal("wrong default timeout")
	}
}
