package modulespack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutesMatchSpecs(t *testing.T) {
	h := NewProvider(nil, nil)
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

func TestModulesNilRegistry(t *testing.T) {
	h := NewProvider(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/modules", nil)
	w := httptest.NewRecorder()

	h.Modules(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Modules []any  `json:"modules"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Modules) != 0 || body.Profile != "" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestModulesRejectsWrongMethod(t *testing.T) {
	h := NewProvider(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/modules", nil)
	w := httptest.NewRecorder()

	h.Modules(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
