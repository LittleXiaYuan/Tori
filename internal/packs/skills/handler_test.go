package skillspack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/skills"
)

// fakeRegistry is a minimal SkillsRegistry whose All() reports a fixed count, so
// the native scan handler's total_skills can be asserted without a real registry.
type fakeRegistry struct{ n int }

func (f fakeRegistry) All() []skills.Skill                 { return make([]skills.Skill, f.n) }
func (f fakeRegistry) CategoryOf(string) string            { return "" }
func (f fakeRegistry) Categories() []*skills.SkillCategory { return nil }
func (f fakeRegistry) Get(string) (skills.Skill, bool)     { return nil, false }
func (f fakeRegistry) Remove(string)                       {}

// TestSkillsV2AndNativeDynamic verifies the skills pack is a v2 Module and that
// /v1/skills/dynamic is served natively, returning a well-formed empty list when
// no registry is configured.
func TestSkillsV2AndNativeDynamic(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := NewHandler() // no services: exercise only the native dynamic route
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

// TestSkillsScanNative verifies /v1/skills/scan is served natively (de-shelled
// from the gateway bridge): POST rescans via the injected hook and reports the
// loaded + total counts; GET is method-gated (405); and an unwired scanner
// reports the original "not configured" 500.
func TestSkillsScanNative(t *testing.T) {
	const loaded = 7
	h := NewHandlerWithService(fakeRegistry{n: 3}, nil)
	h.SetScan(func() (int, bool) { return loaded, true })

	rec := httptest.NewRecorder()
	h.handleScan(rec, httptest.NewRequest(http.MethodPost, "/v1/skills/scan", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("scan POST status = %d, want 200", rec.Code)
	}
	var out struct {
		Status       string `json:"status"`
		SkillsLoaded int    `json:"skills_loaded"`
		TotalSkills  int    `json:"total_skills"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("scan body: %v", err)
	}
	if out.Status != "scanned" || out.SkillsLoaded != loaded || out.TotalSkills != 3 {
		t.Fatalf("scan resp = %+v, want {scanned %d 3}", out, loaded)
	}

	// Method gate: GET is rejected with 405.
	recGet := httptest.NewRecorder()
	h.handleScan(recGet, httptest.NewRequest(http.MethodGet, "/v1/skills/scan", nil))
	if recGet.Code != http.StatusMethodNotAllowed {
		t.Fatalf("scan GET status = %d, want 405", recGet.Code)
	}

	// No scanner wired (loader not configured) → 500.
	recNC := httptest.NewRecorder()
	NewHandler().handleScan(recNC, httptest.NewRequest(http.MethodPost, "/v1/skills/scan", nil))
	if recNC.Code != http.StatusInternalServerError {
		t.Fatalf("scan (no loader) status = %d, want 500", recNC.Code)
	}
}

// TestSkillsRoutesConsistent asserts the pack mounts exactly the manifest's
// /v1/skills/* path set (see packs/official/skills-pack/pack.json), each with a
// non-nil native handler and declared methods — i.e. manifest backend.routes ↔
// handler.Routes() consistency, including the now-native scan route.
func TestSkillsRoutesConsistent(t *testing.T) {
	want := map[string]bool{
		"/v1/skills":         false,
		"/v1/skills/scan":    false,
		"/v1/skills/dynamic": false,
		"/v1/skills/approve": false,
		"/v1/skills/reject":  false,
	}
	for _, rt := range NewHandlerWithService(fakeRegistry{}, nil).Routes() {
		seen, ok := want[rt.Path]
		if !ok {
			t.Fatalf("unexpected route %q (not in manifest)", rt.Path)
		}
		if seen {
			t.Fatalf("duplicate route %q", rt.Path)
		}
		want[rt.Path] = true
		if rt.Handler == nil {
			t.Fatalf("route %q has nil handler", rt.Path)
		}
		if len(rt.Methods) == 0 {
			t.Fatalf("route %q declares no methods", rt.Path)
		}
	}
	for p, seen := range want {
		if !seen {
			t.Fatalf("missing route %q (in manifest, not in Routes())", p)
		}
	}
}
