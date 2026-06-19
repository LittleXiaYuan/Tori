package identitypack

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/identity"
)

func TestRoutesMatchSpecs(t *testing.T) {
	h := NewProvider(nil)
	routes := h.Routes()
	specs := RouteSpecs()
	if len(routes) != len(specs) {
		t.Fatalf("routes=%d specs=%d", len(routes), len(specs))
	}
	for i := range routes {
		if routes[i].Method != specs[i].Method || routes[i].Path != specs[i].Path {
			t.Fatalf("route[%d]=%s %s spec=%s %s", i, routes[i].Method, routes[i].Path, specs[i].Method, specs[i].Path)
		}
	}
}

func TestResolveCreatesProfile(t *testing.T) {
	resolver := identity.NewResolver()
	h := NewProvider(func() *identity.Resolver { return resolver })
	req := httptest.NewRequest(http.MethodPost, "/v1/identity/resolve", bytes.NewBufferString(`{"channel":"feishu","user_id":"u1","display_name":"Tori"}`))
	w := httptest.NewRecorder()

	h.Resolve(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var profile identity.Profile
	if err := json.Unmarshal(w.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if profile.UnifiedID == "" || profile.DisplayName != "Tori" || profile.Channels["feishu"] != "u1" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}

func TestProfilesNilResolver(t *testing.T) {
	h := NewProvider(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/identity/profiles", nil)
	w := httptest.NewRecorder()

	h.Profiles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Profiles []any `json:"profiles"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Profiles) != 0 {
		t.Fatalf("profiles=%d", len(body.Profiles))
	}
}

func TestProfilesRejectsWrongMethod(t *testing.T) {
	h := NewProvider(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/identity/profiles", nil)
	w := httptest.NewRecorder()

	h.Profiles(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
