package modespack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/modes"
	"yunque-agent/pkg/packruntime"
)

type fakeGW struct{ m *modes.ModeManager }

func (f fakeGW) ModeManager() *modes.ModeManager { return f.m }

// TestModesPackV2 verifies the persona-modes pack is a v2 Module with the
// expected route surface and that it degrades to 404 when the mode manager is
// not configured (native handler, de-shelled from the gateway).
func TestModesPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{}) // nil mode manager
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

	rec := httptest.NewRecorder()
	h.handleList(rec, httptest.NewRequest(http.MethodGet, "/v1/persona/modes", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("nil manager handleList = %d, want 404", rec.Code)
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
