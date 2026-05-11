package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/pkg/cogni"
)

func TestCogniExperienceConfirmPattern(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("cogni-experience-confirm")

	reg := cogni.NewRegistry()
	if err := reg.Add(&cogni.Declaration{ID: "reviewer", DisplayName: "Reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni declaration: %v", err)
	}
	gw.SetCogniRegistry(reg, t.TempDir())

	store := cogni.NewExperienceStore("reviewer", cogni.ExperienceConfig{
		StoreDir:      t.TempDir(),
		RequireReview: true,
	})
	store.SuggestPattern(cogni.BehaviorPattern{
		ID:       "pat-timeout",
		Trigger:  "响应超时",
		Response: "保留轨迹并切换备用模型",
	})
	gw.SetCogniExperiences(map[string]*cogni.ExperienceStore{"reviewer": store})

	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/experience/patterns/pat-timeout/confirm", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()

	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" || body["confirmed"] != true {
		t.Fatalf("unexpected response: %#v", body)
	}
	patterns := store.Patterns()
	if len(patterns) != 1 || !patterns[0].Confirmed {
		t.Fatalf("pattern was not confirmed: %+v", patterns)
	}
}

func TestCogniExperienceConfirmPatternNotFound(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("cogni-experience-confirm-missing")

	reg := cogni.NewRegistry()
	if err := reg.Add(&cogni.Declaration{ID: "reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni declaration: %v", err)
	}
	gw.SetCogniRegistry(reg, t.TempDir())
	gw.SetCogniExperiences(map[string]*cogni.ExperienceStore{
		"reviewer": cogni.NewExperienceStore("reviewer", cogni.ExperienceConfig{StoreDir: t.TempDir()}),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/experience/patterns/missing/confirm", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()

	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}
