package plugin

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestServiceManager_RejectNonService(t *testing.T) {
	sm := NewServiceManager()
	sp := &ScriptPlugin{
		dir:      t.TempDir(),
		manifest: Manifest{Name: "func-plugin", Type: PluginTypeFunction},
	}
	err := sm.Start(context.Background(), sp)
	if err == nil {
		t.Fatal("expected error for non-service plugin")
	}
}

func TestServiceManager_RejectNoEntrypoint(t *testing.T) {
	sm := NewServiceManager()
	sp := &ScriptPlugin{
		dir:      t.TempDir(),
		manifest: Manifest{Name: "svc-no-entry", Type: PluginTypeService},
	}
	err := sm.Start(context.Background(), sp)
	if err == nil {
		t.Fatal("expected error for missing entrypoint")
	}
}

func TestServiceManager_StatusNotFound(t *testing.T) {
	sm := NewServiceManager()
	_, ok := sm.Status("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestServiceManager_StopNotFound(t *testing.T) {
	sm := NewServiceManager()
	err := sm.Stop("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServiceManager_StartAndStop(t *testing.T) {
	sm := NewServiceManager()
	// Use os.MkdirTemp instead of t.TempDir() to avoid cleanup race with process
	dir, err := os.MkdirTemp("", "tori-svc-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Run ping directly (no shell wrapper) so context cancel kills the actual process
	sp := &ScriptPlugin{
		dir: dir,
		manifest: Manifest{
			Name:       "test-svc",
			Type:       PluginTypeService,
			Entrypoint: "ping -n 100 127.0.0.1",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = sm.Start(ctx, sp)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	info, ok := sm.Status("test-svc")
	if !ok {
		t.Fatal("service not found after start")
	}
	if info.State != ServiceRunning {
		t.Fatalf("expected running, got %s", info.State)
	}

	err = sm.Stop("test-svc")
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	// Wait for process to fully terminate
	time.Sleep(1 * time.Second)

	info2, _ := sm.Status("test-svc")
	if info2.State != ServiceStopped {
		t.Fatalf("expected stopped, got %s", info2.State)
	}
}

func TestServiceManager_AllStatus(t *testing.T) {
	sm := NewServiceManager()
	all := sm.AllStatus()
	if len(all) != 0 {
		t.Fatalf("expected empty, got %d", len(all))
	}
}
