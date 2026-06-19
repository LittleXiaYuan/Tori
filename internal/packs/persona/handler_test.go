package personapack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/pkg/packruntime"
)

type fakeGateway struct {
	persona *persona.Persona
	chain   *persona.PriorityChain
}

func (f fakeGateway) Persona() *persona.Persona { return f.persona }
func (f fakeGateway) PersonaChain() *persona.PriorityChain {
	return f.chain
}

func newTestHandler(t *testing.T) (*Handler, *persona.Persona, *persona.PresetManager) {
	t.Helper()
	p, err := persona.New(t.TempDir())
	if err != nil {
		t.Fatalf("persona.New: %v", err)
	}
	pm := persona.NewPresetManager()
	chain := persona.NewPriorityChain(p, pm)
	return New(fakeGateway{persona: p, chain: chain}), p, pm
}

func TestPersonaPackV2AndRouteSpecs(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(nil)
	if h.PackID() != PackID {
		t.Fatalf("PackID=%q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 5 {
		t.Fatalf("Routes len=%d, want 5 mounted paths", got)
	}
	if got := len(RouteSpecs()); got != 10 {
		t.Fatalf("RouteSpecs len=%d, want 10 method specs", got)
	}

	paths := map[string]bool{}
	for _, route := range h.Routes() {
		paths[route.Path] = true
	}
	for _, spec := range RouteSpecs() {
		if !paths[spec.Path] {
			t.Fatalf("route spec path %s has no mounted route", spec.Path)
		}
	}

	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestPersonaReadUpdateAndSkills(t *testing.T) {
	h, _, _ := newTestHandler(t)

	putReq := httptest.NewRequest(http.MethodPut, "/v1/persona", strings.NewReader(`{"identity":"I am Xiao Yu","soul":"careful"}`))
	putRec := httptest.NewRecorder()
	h.Persona(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put persona status=%d body=%s", putRec.Code, putRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	h.Persona(getRec, httptest.NewRequest(http.MethodGet, "/v1/persona", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get persona status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	var personaBody struct {
		Identity string          `json:"identity"`
		Soul     string          `json:"soul"`
		Skills   []persona.Skill `json:"skills"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &personaBody); err != nil {
		t.Fatalf("decode persona: %v", err)
	}
	if personaBody.Identity != "I am Xiao Yu" || personaBody.Soul != "careful" {
		t.Fatalf("unexpected persona body: %#v", personaBody)
	}

	addSkillReq := httptest.NewRequest(http.MethodPost, "/v1/persona/skills", strings.NewReader(`{"name":"coding","description":"write code","content":"Prefer tests."}`))
	addSkillRec := httptest.NewRecorder()
	h.PersonaSkills(addSkillRec, addSkillReq)
	if addSkillRec.Code != http.StatusCreated {
		t.Fatalf("add skill status=%d body=%s", addSkillRec.Code, addSkillRec.Body.String())
	}

	listSkillRec := httptest.NewRecorder()
	h.PersonaSkills(listSkillRec, httptest.NewRequest(http.MethodGet, "/v1/persona/skills", nil))
	var skillsBody struct {
		Skills []persona.Skill `json:"skills"`
	}
	if err := json.Unmarshal(listSkillRec.Body.Bytes(), &skillsBody); err != nil {
		t.Fatalf("decode skills: %v", err)
	}
	if len(skillsBody.Skills) != 1 || skillsBody.Skills[0].Name != "coding" {
		t.Fatalf("unexpected skills body: %#v", skillsBody)
	}

	deleteSkillReq := httptest.NewRequest(http.MethodDelete, "/v1/persona/skills", strings.NewReader(`{"name":"coding"}`))
	deleteSkillRec := httptest.NewRecorder()
	h.PersonaSkills(deleteSkillRec, deleteSkillReq)
	if deleteSkillRec.Code != http.StatusOK {
		t.Fatalf("delete skill status=%d body=%s", deleteSkillRec.Code, deleteSkillRec.Body.String())
	}
}

func TestPresetsCustomAndFeatures(t *testing.T) {
	h, _, pm := newTestHandler(t)

	listRec := httptest.NewRecorder()
	h.Presets(listRec, httptest.NewRequest(http.MethodGet, "/v1/persona/presets", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list presets status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	switchReq := httptest.NewRequest(http.MethodPost, "/v1/persona/presets", strings.NewReader(`{"id":"jarvis"}`))
	switchRec := httptest.NewRecorder()
	h.Presets(switchRec, switchReq)
	if switchRec.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", switchRec.Code, switchRec.Body.String())
	}
	if pm.ActiveID() != "jarvis" {
		t.Fatalf("active=%s, want jarvis", pm.ActiveID())
	}

	customReq := httptest.NewRequest(http.MethodPost, "/v1/persona/presets/custom", strings.NewReader(`{"id":"reviewer","name":"Reviewer","system_note":"Be precise."}`))
	customRec := httptest.NewRecorder()
	h.CustomPreset(customRec, customReq)
	if customRec.Code != http.StatusOK {
		t.Fatalf("custom status=%d body=%s", customRec.Code, customRec.Body.String())
	}
	if _, ok := pm.Get("reviewer"); !ok {
		t.Fatal("expected custom preset reviewer")
	}

	featureReq := httptest.NewRequest(http.MethodPut, "/v1/persona/presets/features", strings.NewReader(`{"id":"reviewer","features":{"emotion":false}}`))
	featureRec := httptest.NewRecorder()
	h.PresetFeatures(featureRec, featureReq)
	if featureRec.Code != http.StatusOK {
		t.Fatalf("features status=%d body=%s", featureRec.Code, featureRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/persona/presets/custom", strings.NewReader(`{"id":"reviewer"}`))
	deleteRec := httptest.NewRecorder()
	h.CustomPreset(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete custom status=%d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	if _, ok := pm.Get("reviewer"); ok {
		t.Fatal("custom preset should be removed")
	}
}

func TestNilDependenciesPreserveFallbacks(t *testing.T) {
	h := New(nil)

	personaRec := httptest.NewRecorder()
	h.Persona(personaRec, httptest.NewRequest(http.MethodGet, "/v1/persona", nil))
	if personaRec.Code != http.StatusInternalServerError {
		t.Fatalf("nil persona status=%d body=%s", personaRec.Code, personaRec.Body.String())
	}

	presetsRec := httptest.NewRecorder()
	h.Presets(presetsRec, httptest.NewRequest(http.MethodGet, "/v1/persona/presets", nil))
	if presetsRec.Code != http.StatusOK {
		t.Fatalf("nil presets status=%d body=%s", presetsRec.Code, presetsRec.Body.String())
	}
	var body struct {
		Presets []any  `json:"presets"`
		Active  string `json:"active"`
	}
	if err := json.Unmarshal(presetsRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode nil presets: %v", err)
	}
	if len(body.Presets) != 0 || body.Active != "" {
		t.Fatalf("unexpected nil presets body: %#v", body)
	}
}
