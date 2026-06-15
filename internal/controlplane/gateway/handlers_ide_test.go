package gateway

import (
	"context"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/pkg/skills"
)

// IDE handler tests moved to internal/packs/ide (the IDE surface is now a pack).

// ── WireTaskSSE ────────────────────────────────────────────

func TestWireTaskSSE_BridgesEvents(t *testing.T) {
	gw, _ := newTestGateway()

	// Create SSE broker
	broker := NewSSEBroker()
	gw.SetSSEBroker(broker)

	// Create a minimal task runner
	dir := t.TempDir()
	store := task.NewStore(dir)
	reg := skills.NewRegistry()
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		return "ok", nil
	}
	runner := task.NewRunner(store, reg, mockLLM, nil)
	gw.SetTaskRunner(runner)

	// Wire them
	gw.WireTaskSSE()

	// Subscribe to SSE
	_, ch, _ := broker.Subscribe()

	// Broadcast a simulated event
	broker.Broadcast(SSEEvent{Type: "task.step_completed", Data: map[string]string{"task_id": "test-1"}})

	select {
	case event := <-ch:
		if event.Type != "task.step_completed" {
			t.Fatalf("expected task.step_completed, got %s", event.Type)
		}
		data, ok := event.Data.(map[string]string)
		if !ok {
			t.Fatal("expected map[string]string data")
		}
		if data["task_id"] != "test-1" {
			t.Fatalf("expected task_id test-1, got %s", data["task_id"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

func TestSSEEventVisibleToTenantFiltersTenantScopedEvents(t *testing.T) {
	if !sseEventVisibleToTenant(SSEEvent{Type: "task.step_completed"}, "tenant-a") {
		t.Fatal("global SSE events should remain visible")
	}
	if !sseEventVisibleToTenant(SSEEvent{Type: "planner.resume_plan_event", TenantID: "tenant-a"}, "tenant-a") {
		t.Fatal("tenant-owned SSE event should be visible to same tenant")
	}
	if sseEventVisibleToTenant(SSEEvent{Type: "planner.resume_plan_event", TenantID: "tenant-a"}, "tenant-b") {
		t.Fatal("tenant-scoped SSE event should not be visible to a different tenant")
	}
}

func TestWireTaskSSE_NilSafe(t *testing.T) {
	gw, _ := newTestGateway()
	// Should not panic with nil runner or broker
	gw.WireTaskSSE()
	gw.SetSSEBroker(NewSSEBroker())
	gw.WireTaskSSE() // still nil runner
}

// ── parseCallbackData ──────────────────────────────────────

func TestParseCallbackData_Valid(t *testing.T) {
	act, id := parseCallbackData("retry_task:abc-123")
	if act != "retry_task" || id != "abc-123" {
		t.Fatalf("got %q %q", act, id)
	}
}

func TestParseCallbackData_NoColon(t *testing.T) {
	act, id := parseCallbackData("nocolon")
	if act != "" || id != "" {
		t.Fatalf("expected empty, got %q %q", act, id)
	}
}

func TestParseCallbackData_MultiColon(t *testing.T) {
	act, id := parseCallbackData("approve:task:sub")
	if act != "approve" || id != "task:sub" {
		t.Fatalf("got %q %q", act, id)
	}
}

// ── WireTelegramCallbackActions ────────────────────────────

func TestWireTelegramCallbackActions_NilSafe(t *testing.T) {
	gw, _ := newTestGateway()
	// Should not panic with nil channelReg
	gw.WireTelegramCallbackActions()
}

func TestWireTelegramCallbackActions_NoTelegram(t *testing.T) {
	gw, _ := newTestGateway()
	reg := channel.NewRegistry()
	gw.SetChannelRegistry(reg)
	// No telegram channel → should not panic
	gw.WireTelegramCallbackActions()
}
