package skillhubpack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/skillmarket"
)

func TestSkillHubPackSearchUsesLocalMarket(t *testing.T) {
	market := skillmarket.NewMarket(t.TempDir())
	if err := market.Publish(skillmarket.SkillMeta{
		Name:        "repo-auditor",
		Version:     "0.1.0",
		Description: "Audit repositories",
		Author:      "Yunque",
	}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{skillMarket: market}

	req := httptest.NewRequest(http.MethodGet, "/api/skillhub/search?q=repo", nil)
	rec := httptest.NewRecorder()

	h.handleSkillHubSearch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Results []struct {
			Name      string `json:"name"`
			Source    string `json:"source"`
			Installed bool   `json:"installed"`
		} `json:"results"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Results) != 1 {
		t.Fatalf("unexpected search body: %+v", body)
	}
	if body.Results[0].Name != "repo-auditor" || body.Results[0].Source != "local" || body.Results[0].Installed {
		t.Fatalf("unexpected search result: %+v", body.Results[0])
	}
}

func TestSkillHubPackInstalledNilInstaller(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/skillhub/installed", nil)
	rec := httptest.NewRecorder()

	h.handleSkillHubInstalled(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Skills []any `json:"skills"`
		Count  int   `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 0 || len(body.Skills) != 0 {
		t.Fatalf("unexpected installed body: %+v", body)
	}
}

func TestSkillHubPackPolicyCheckAllowsWithoutPolicy(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/skillhub/policy/check?slug=demo", nil)
	rec := httptest.NewRecorder()

	h.handleSkillHubPolicyCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Allowed {
		t.Fatalf("expected policy check to allow without policy: %+v", body)
	}
}

func TestSkillHubPackRoutesAndSpecsStayAligned(t *testing.T) {
	routes := map[string]map[string]bool{}
	for _, route := range (&Handler{}).Routes() {
		if routes[route.Path] == nil {
			routes[route.Path] = map[string]bool{}
		}
		if route.Method != "" {
			routes[route.Path][route.Method] = true
		}
		for _, method := range route.Methods {
			routes[route.Path][method] = true
		}
	}

	for _, spec := range RouteSpecs() {
		if !routes[spec.Path][spec.Method] {
			t.Fatalf("routeSpec %s %s not mounted by Routes()", spec.Method, spec.Path)
		}
	}
}
