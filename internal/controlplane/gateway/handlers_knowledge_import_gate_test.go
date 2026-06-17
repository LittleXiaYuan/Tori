package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	knowledgepack "yunque-agent/internal/packs/knowledge"
	"yunque-agent/pkg/packruntime"
)

// TestKnowledgeImportRoutesNativeGating verifies the de-shelled import-url /
// import-repo routes are owned by the knowledge pack's Pack Runtime gate:
// enabled + authed reaches the native handler (not 404), and disabling the pack
// removes the surface (authed → 404).
func TestKnowledgeImportRoutesNativeGating(t *testing.T) {
	for _, probe := range []string{"/v1/knowledge/import-url", "/v1/knowledge/import-repo"} {
		// Enabled + authed → reaches the native handler. The test gateway has no
		// KB store wired, so the handler returns a 200 JSON "not configured"
		// error — importantly NOT a 404 from the pack gate.
		gw, tm := newTestGatewayWithMigrationPack(t, knowledgepack.PackID, packruntime.PackStatusEnabled)
		key := tm.Register("kb-import-en-" + probe).APIKey
		req := httptest.NewRequest(http.MethodPost, probe, strings.NewReader(`{}`))
		req.Header.Set("X-API-Key", key)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code == http.StatusNotFound {
			t.Fatalf("%s enabled+authed: got 404, want the native handler to be reached", probe)
		}

		// Disabled + authed → 404 (pack enable gate removes the surface).
		gwD, tmD := newTestGatewayWithMigrationPack(t, knowledgepack.PackID, packruntime.PackStatusDisabled)
		keyD := tmD.Register("kb-import-dis-" + probe).APIKey
		reqD := httptest.NewRequest(http.MethodPost, probe, strings.NewReader(`{}`))
		reqD.Header.Set("X-API-Key", keyD)
		wD := httptest.NewRecorder()
		gwD.ServeHTTP(wD, reqD)
		if wD.Code != http.StatusNotFound {
			t.Fatalf("%s disabled+authed: got %d, want 404", probe, wD.Code)
		}
	}
}
