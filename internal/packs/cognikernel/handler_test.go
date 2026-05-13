package cognikernelpack

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeCogniGateway struct {
	called int
}

func (g *fakeCogniGateway) HandleCogniKernelPack(w http.ResponseWriter, _ *http.Request) {
	g.called++
	w.WriteHeader(http.StatusNoContent)
}

func TestCogniKernelHandlerRoutesExposeSurface(t *testing.T) {
	gateway := &fakeCogniGateway{}
	handler := NewHandler(gateway)

	if handler.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", handler.PackID(), PackID)
	}

	routes := handler.Routes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 Cogni Kernel routes, got %d", len(routes))
	}
	if routes[0].Path != "/v1/cognis" {
		t.Fatalf("collection route path = %q", routes[0].Path)
	}
	if routes[1].Path != "/v1/cognis/" {
		t.Fatalf("sub-resource route path = %q", routes[1].Path)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis", nil)
	w := httptest.NewRecorder()
	routes[0].Handler(w, req)
	if w.Code != http.StatusNoContent || gateway.called != 1 {
		t.Fatalf("expected route to delegate to gateway, status=%d called=%d", w.Code, gateway.called)
	}
}
