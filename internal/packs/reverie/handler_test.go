package reveriepack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/pkg/packruntime"
)

type fakeGW struct{}

func (fakeGW) Reverie() *planner.Reverie     { return nil }
func (fakeGW) ReverieChannelTypes() []string { return nil }

// TestReveriePackV2 verifies the reverie pack is a v2 Module with the expected
// route surface and degrades gracefully when the engine is not initialized.
func TestReveriePackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 7 {
		t.Fatalf("Routes len = %d, want 7", got)
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec := httptest.NewRecorder()
	h.handleJournal(rec, httptest.NewRequest(http.MethodGet, "/v1/reverie/journal", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("nil reverie handleJournal = %d, want 404", rec.Code)
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
