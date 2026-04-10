package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSandboxExec(t *testing.T) {
	policy := DefaultPolicy()
	policy.AllowCommands = append(policy.AllowCommands, "cmd", "sh", "where", "which")
	sb, err := New(os.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	var result *Result
	if runtime.GOOS == "windows" {
		result, err = sb.Exec(context.Background(), "cmd", "/c", "echo", "hello")
	} else {
		result, err = sb.Exec(context.Background(), "echo", "hello")
	}
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Stderr)
	}
}

func TestSandboxBlockedCommand(t *testing.T) {
	sb, err := New(os.TempDir(), DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	result, err := sb.Exec(context.Background(), "rm", "-rf", "/")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != -1 {
		t.Fatal("expected blocked command")
	}
}

func TestSandboxWriteReadFile(t *testing.T) {
	sb, err := New(os.TempDir(), DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	sb.WriteFile("test.txt", "hello world")
	content, err := sb.ReadFile("test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", content)
	}
}

func TestSandboxPathEscape(t *testing.T) {
	sb, err := New(os.TempDir(), DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	_, err = sb.ReadFile("../../etc/passwd")
	if err == nil {
		t.Fatal("expected path escape error")
	}
}

func TestSandboxPathEscapeSiblingPrefix(t *testing.T) {
	sb, err := New(os.TempDir(), DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	if err := sb.WriteFile("../"+filepath.Base(sb.WorkDir())+"_escape/evil.txt", "oops"); err == nil {
		t.Fatal("expected sibling prefix escape to be rejected")
	}
}

func TestHostReadAccess(t *testing.T) {
	// Create a temp file on "host"
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "host.txt"), []byte("host data"), 0644)

	policy := DefaultPolicy()
	policy.HostReadPaths = []string{tmpDir}

	sb, err := New(os.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	content, err := sb.ReadHostFile(filepath.Join(tmpDir, "host.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if content != "host data" {
		t.Fatalf("expected 'host data', got '%s'", content)
	}
}

func TestHostReadDenied(t *testing.T) {
	policy := DefaultPolicy()
	policy.HostReadPaths = []string{"/tmp/allowed"}

	sb, err := New(os.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	_, err = sb.ReadHostFile("C:\\Windows\\System32\\config")
	if err == nil {
		t.Fatal("expected access denied")
	}
}
