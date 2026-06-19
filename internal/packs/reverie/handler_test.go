package reveriepack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/cognikernel"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/packruntime"
)

type fakeGW struct{}

func (fakeGW) Reverie() *planner.Reverie     { return nil }
func (fakeGW) ReverieChannelTypes() []string { return nil }
func (fakeGW) CogniKernel() *cognikernel.CogniKernel {
	return nil
}
func (fakeGW) CogniEvolution() *cogni.EvolutionEngine {
	return nil
}

// TestReveriePackV2 verifies the reverie pack is a v2 Module with the expected
// route surface and degrades gracefully when the engine is not initialized.
func TestReveriePackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 8 {
		t.Fatalf("Routes len = %d, want 8", got)
	}
	if got := len(RouteSpecs()); got != 9 {
		t.Fatalf("RouteSpecs len = %d, want 9", got)
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

	dream := httptest.NewRecorder()
	h.handleDreamStatus(dream, httptest.NewRequest(http.MethodGet, "/v1/reverie/dream/status", nil))
	if dream.Code != http.StatusOK {
		t.Fatalf("nil dream status code = %d, want 200", dream.Code)
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
