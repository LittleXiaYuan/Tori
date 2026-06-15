package skillspack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/pkg/packruntime"
)

// TestSkillsV2AndNativeDynamic verifies the skills pack is a v2 Module and that
// /v1/skills/dynamic is now served natively (de-shelled from the gateway bridge),
// returning a well-formed empty list when no registry is configured.
func TestSkillsV2AndNativeDynamic(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := NewHandler(nil) // nil gateway: we exercise only the native dynamic route
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec := httptest.NewRecorder()
	h.handleDynamicGet(rec, httptest.NewRequest(http.MethodGet, "/v1/skills/dynamic", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("dynamic status = %d", rec.Code)
	}
	var out struct {
		Skills []json.RawMessage `json:"skills"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("dynamic body: %v", err)
	}
	if out.Skills == nil {
		t.Fatalf("expected non-nil skills array (de-shelled native handler)")
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
