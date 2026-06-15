package cronpack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/pkg/packruntime"
)

type fakeGW struct{ m *cron.Manager }

func (f fakeGW) CronManager() *cron.Manager { return f.m }

// TestCronPackV2 verifies the cron pack is a v2 Module with the expected route
// surface and degrades to a "not configured" body when no manager is wired.
func TestCronPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 4 {
		t.Fatalf("Routes len = %d, want 4", got)
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec := httptest.NewRecorder()
	h.handleList(rec, httptest.NewRequest(http.MethodGet, "/v1/cron/list", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("handleList code = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "cron not configured") {
		t.Fatalf("expected not-configured body, got %q", rec.Body.String())
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
