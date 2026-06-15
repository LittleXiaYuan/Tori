package nightschoolpack

import (
	"context"
	"testing"

	"yunque-agent/pkg/packruntime"
)

// TestNightSchoolIsV2ModuleWithLifecycle verifies the Night School pack satisfies
// the v2 Module contract and its enable/disable lifecycle flips state.
func TestNightSchoolIsV2ModuleWithLifecycle(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(Config{})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 3 {
		t.Fatalf("Routes len = %d, want 3", got)
	}

	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !h.started.Load() {
		t.Fatalf("expected started after Start")
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if h.started.Load() {
		t.Fatalf("expected stopped after Stop")
	}
}
