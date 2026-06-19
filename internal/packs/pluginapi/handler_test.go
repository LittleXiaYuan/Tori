package pluginapipack

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/plugin"
)

func TestRoutesDeclarePluginAPIBridgeSurface(t *testing.T) {
	h := New(Config{}, NewPluginTokenManager())
	routes := h.Routes()
	specs := RouteSpecs()
	if len(routes) != len(specs) {
		t.Fatalf("Routes length = %d, RouteSpecs length = %d", len(routes), len(specs))
	}
	if len(Paths()) != len(specs) {
		t.Fatalf("Paths length = %d, RouteSpecs length = %d", len(Paths()), len(specs))
	}

	want := map[string]string{
		"/v1/plugin-api/llm":        http.MethodPost,
		"/v1/plugin-api/cron/list":  http.MethodGet,
		"/v1/plugin-api/extensions": http.MethodGet,
	}
	for path, method := range want {
		route := findRoute(t, routes, path)
		if route.Auth != packruntime.BackendRouteAuthPassthrough {
			t.Fatalf("%s auth = %q, want passthrough", path, route.Auth)
		}
		if len(route.Methods) != 1 || route.Methods[0] != method {
			t.Fatalf("%s methods = %v, want [%s]", path, route.Methods, method)
		}
	}
}

func TestPluginTokenPermissionGate(t *testing.T) {
	tokenMgr := NewPluginTokenManager()
	h := New(Config{
		MemoryMgr: plugin.NewPluginMemoryManager(t.TempDir()),
	}, tokenMgr)
	route := findRoute(t, h.Routes(), "/v1/plugin-api/memory/set")

	req := httptest.NewRequest(http.MethodPost, "/v1/plugin-api/memory/set", bytes.NewBufferString(`{"key":"k","value":"v"}`))
	w := httptest.NewRecorder()
	route.Handler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	badToken := tokenMgr.Issue("demo", []string{"search"})
	req = httptest.NewRequest(http.MethodPost, "/v1/plugin-api/memory/set", bytes.NewBufferString(`{"key":"k","value":"v"}`))
	req.Header.Set("Authorization", "Bearer "+badToken)
	w = httptest.NewRecorder()
	route.Handler(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("missing permission: status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestPluginMemoryRoundTrip(t *testing.T) {
	tokenMgr := NewPluginTokenManager()
	token := tokenMgr.Issue("demo", []string{"memory"})
	h := New(Config{
		MemoryMgr: plugin.NewPluginMemoryManager(t.TempDir()),
	}, tokenMgr)

	setRoute := findRoute(t, h.Routes(), "/v1/plugin-api/memory/set")
	setReq := httptest.NewRequest(http.MethodPost, "/v1/plugin-api/memory/set", bytes.NewBufferString(`{"key":"answer","value":"42"}`))
	setReq.Header.Set("Authorization", "Bearer "+token)
	setW := httptest.NewRecorder()
	setRoute.Handler(setW, setReq)
	if setW.Code != http.StatusOK {
		t.Fatalf("set status = %d body=%s", setW.Code, setW.Body.String())
	}

	getRoute := findRoute(t, h.Routes(), "/v1/plugin-api/memory/get")
	getReq := httptest.NewRequest(http.MethodPost, "/v1/plugin-api/memory/get", bytes.NewBufferString(`{"key":"answer"}`))
	getReq.Header.Set("Authorization", "Bearer "+token)
	getW := httptest.NewRecorder()
	getRoute.Handler(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getW.Code, getW.Body.String())
	}
	if got := getW.Body.String(); !bytes.Contains([]byte(got), []byte(`"value":"42"`)) {
		t.Fatalf("get body = %s, want value 42", got)
	}
}

func findRoute(t *testing.T, routes []packruntime.BackendRoute, path string) packruntime.BackendRoute {
	t.Helper()
	for _, route := range routes {
		if route.Path == path {
			return route
		}
	}
	t.Fatalf("missing route %s", path)
	return packruntime.BackendRoute{}
}
