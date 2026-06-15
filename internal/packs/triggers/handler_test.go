package triggerspack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/pkg/packruntime"
)

type fakeGW struct{}

func (fakeGW) TriggerRuntime() *trigger.Runtime { return nil }
func (fakeGW) TriggerManager() *trigger.Manager { return nil }

// TestTriggersPackV2 verifies the triggers pack is a v2 Module with the expected
// route surface and degrades to 404 when the subsystems are not wired.
func TestTriggersPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 6 {
		t.Fatalf("Routes len = %d, want 6", got)
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec := httptest.NewRecorder()
	h.handleTriggers(rec, httptest.NewRequest(http.MethodGet, "/v1/triggers", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("nil runtime handleTriggers = %d, want 404", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	h.handleV2(rec2, httptest.NewRequest(http.MethodGet, "/v1/triggers/v2", nil))
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("nil manager handleV2 = %d, want 404", rec2.Code)
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
