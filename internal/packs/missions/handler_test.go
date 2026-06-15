package missionspack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
)

type fakeGW struct{}

func (fakeGW) ParseMissionIntent(_ context.Context, desc string) (any, error) {
	return map[string]string{"intent": desc}, nil
}

func TestMissionsPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q", h.PackID())
	}
	if got := len(h.Routes()); got != 1 {
		t.Fatalf("Routes = %d, want 1", got)
	}
	_ = h.Init(nil)
	if err := h.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer h.Stop(context.Background())

	// wrong method → 405
	rec := httptest.NewRecorder()
	h.handleParse(rec, httptest.NewRequest(http.MethodGet, "/v1/missions/parse", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d, want 405", rec.Code)
	}

	// empty description → 400
	rec = httptest.NewRecorder()
	h.handleParse(rec, httptest.NewRequest(http.MethodPost, "/v1/missions/parse", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty = %d, want 400", rec.Code)
	}

	// valid → 200 + parsed result
	rec = httptest.NewRecorder()
	h.handleParse(rec, httptest.NewRequest(http.MethodPost, "/v1/missions/parse", strings.NewReader(`{"description":"ship it"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ship it") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}
