package microagentpack

import (
	"context"
	"testing"

	"yunque-agent/pkg/packruntime"
)

func TestMicroAgentIsV2ModuleWithLifecycle(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(Config{})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if len(h.Routes()) == 0 {
		t.Fatalf("expected routes")
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !h.started.Load() {
		t.Fatalf("expected started")
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if h.started.Load() {
		t.Fatalf("expected stopped")
	}
}
