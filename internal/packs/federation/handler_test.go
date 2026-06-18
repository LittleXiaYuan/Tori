package federationpack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/federation"
	"yunque-agent/pkg/packruntime"
)

func TestFederationPackPeersNilHub(t *testing.T) {
	h := NewProvider(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/federation/peers", nil)
	rec := httptest.NewRecorder()

	h.Peers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "federation not configured" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestFederationPackReceiveUsesPassthroughAuth(t *testing.T) {
	var receiveRoute packruntime.BackendRoute
	for _, route := range NewProvider(nil, nil, nil).Routes() {
		if route.Path == "/v1/federation/receive" {
			receiveRoute = route
			break
		}
	}
	if receiveRoute.Path == "" {
		t.Fatal("receive route not found")
	}
	if receiveRoute.Auth != packruntime.BackendRouteAuthPassthrough {
		t.Fatalf("receive auth = %q, want passthrough", receiveRoute.Auth)
	}
}

func TestFederationPackReceiveDispatchesTransport(t *testing.T) {
	hub := federation.NewHub(federation.HubConfig{LocalAgent: "local", LocalInstance: "local"})
	transport := federation.NewTransport(hub)
	h := NewProvider(func() *federation.Hub { return hub }, nil, func() *federation.Transport { return transport })
	req := httptest.NewRequest(http.MethodPost, "/v1/federation/receive", nil)
	rec := httptest.NewRecorder()

	h.Receive(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFederationPackRoutesAndSpecsStayAligned(t *testing.T) {
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
