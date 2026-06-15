package documentspack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/skills"
)

type fakeGW struct{}

func (fakeGW) SkillsRegistry() *skills.Registry { return nil }

func TestDocumentsPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q", h.PackID())
	}
	if got := len(h.Routes()); got != 2 {
		t.Fatalf("Routes = %d, want 2", got)
	}
	_ = h.Init(nil)
	if err := h.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer h.Stop(context.Background())

	// generate: wrong method → 405
	rec := httptest.NewRecorder()
	h.handleGenerate(rec, httptest.NewRequest(http.MethodGet, "/v1/documents/generate", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("generate GET = %d, want 405", rec.Code)
	}

	// generate: valid POST but nil registry → 404
	rec = httptest.NewRecorder()
	body := strings.NewReader(`{"format":"docx","content":"hello"}`)
	h.handleGenerate(rec, httptest.NewRequest(http.MethodPost, "/v1/documents/generate", body))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("generate nil-registry = %d, want 404", rec.Code)
	}

	// templates: missing catalog → 200 with empty list
	rec = httptest.NewRecorder()
	h.handleTemplates(rec, httptest.NewRequest(http.MethodGet, "/v1/documents/templates", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("templates = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "templates") {
		t.Fatalf("templates body = %q", rec.Body.String())
	}
}

func TestSanitizeDocFilename(t *testing.T) {
	if got := sanitizeDocFilename("My Report: v2/final"); strings.ContainsAny(got, " /:") {
		t.Fatalf("unsanitized: %q", got)
	}
	if got := sanitizeDocFilename("   "); got != "document" {
		t.Fatalf("empty fallback = %q", got)
	}
}
