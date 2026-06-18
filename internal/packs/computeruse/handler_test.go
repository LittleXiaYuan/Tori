package computeruse

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/skills"
)

type fakeGateway struct {
	connected bool
	action    json.RawMessage
}

func (f *fakeGateway) TenantOf(context.Context) string { return "test-tenant" }

func (f *fakeGateway) BrowserConnectedForTenant(string) bool { return f.connected }

func (f *fakeGateway) BrowserHealth() map[string]any {
	return map[string]any{"connected": f.connected, "version": "test"}
}

func (f *fakeGateway) SendBrowserActionRaw(_ context.Context, action json.RawMessage) (json.RawMessage, error) {
	f.action = append(f.action[:0], action...)
	return json.RawMessage(`{"ok":true,"screenshot":"data:image/png;base64,abc123"}`), nil
}

func (f *fakeGateway) DesktopSandboxStatus(context.Context) map[string]any {
	return map[string]any{"available": true, "running": false, "status": "configured"}
}

func TestRoutesConsistentWithManifest(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "..", "packs", "official", "computer-use-pack", "pack.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Backend struct {
			Routes     []string `json:"routes"`
			RouteSpecs []struct {
				Method string `json:"method"`
				Path   string `json:"path"`
			} `json:"routeSpecs"`
		} `json:"backend"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	manifestRoutes := map[string]bool{}
	routeMethods := map[string]string{}
	for _, p := range manifest.Backend.Routes {
		manifestRoutes[p] = true
	}
	for _, rt := range manifest.Backend.RouteSpecs {
		routeMethods[rt.Path] = rt.Method
	}
	if len(manifestRoutes) == 0 {
		t.Fatal("manifest declares no backend routes")
	}

	h := New(nil)
	seen := map[string]bool{}
	for _, rt := range h.Routes() {
		if seen[rt.Path] {
			t.Fatalf("duplicate route %q", rt.Path)
		}
		seen[rt.Path] = true
		if rt.Handler == nil {
			t.Fatalf("route %q has nil handler", rt.Path)
		}
		if rt.Method == "" {
			t.Fatalf("route %q declares no method", rt.Path)
		}
		if !manifestRoutes[rt.Path] {
			t.Fatalf("route %q not in manifest backend.routes", rt.Path)
		}
		if routeMethods[rt.Path] != rt.Method {
			t.Fatalf("route %q method = %q, manifest = %q", rt.Path, rt.Method, routeMethods[rt.Path])
		}
	}
	for p := range manifestRoutes {
		if !seen[p] {
			t.Fatalf("manifest route %q missing from Routes()", p)
		}
	}
}

func TestIntentPlanIsNonDestructive(t *testing.T) {
	h := New(&fakeGateway{connected: true})
	body := bytes.NewBufferString(`{"goal":"open the dashboard and summarize status","surface":"browser","allow_execute":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/computer/intent/plan", body)
	w := httptest.NewRecorder()

	h.IntentPlan(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Plan intentPlanReport `json:"plan"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Plan.PlanReady {
		t.Fatal("expected plan_ready")
	}
	if resp.Plan.ExecutionReady {
		t.Fatal("plan route must not mark execution ready")
	}
	if resp.Plan.ControlsLocalDesktop || resp.Plan.ExecutesCommands || resp.Plan.WritesFiles || resp.Plan.NetworkAccess {
		t.Fatalf("plan must stay non-destructive: %#v", resp.Plan)
	}
	if !resp.Plan.AllowExecuteRequested {
		t.Fatal("expected allow_execute request to be reflected")
	}
}

func TestBuildContextOnlyForRelevantComputerUseRequests(t *testing.T) {
	h := New(&fakeGateway{connected: true})
	if got := h.BuildContext(context.Background(), "hello, write a short poem", "tenant"); got != "" {
		t.Fatalf("expected unrelated request to produce no pack context, got %q", got)
	}
	got := h.BuildContext(context.Background(), "帮我看一下浏览器截图再计划下一步", "tenant")
	if !strings.Contains(got, "Computer Use Pack") || !strings.Contains(got, "computer_use_plan") {
		t.Fatalf("expected computer-use guidance, got %q", got)
	}
}

func TestComputerUsePlanSkillIsPlanOnly(t *testing.T) {
	h := New(&fakeGateway{connected: true})
	skillsList := h.Skills()
	if len(skillsList) != 1 {
		t.Fatalf("expected one skill, got %d", len(skillsList))
	}
	out, err := skillsList[0].Execute(context.Background(), map[string]any{
		"goal":          "open settings and click save",
		"surface":       "browser",
		"allow_execute": true,
	}, &skills.Environment{TenantID: "tenant"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, `"execution_ready": false`) {
		t.Fatalf("skill must stay plan-only, got %s", out)
	}
	if !strings.Contains(out, `"controls_local_desktop": false`) || !strings.Contains(out, `"executes_commands": false`) {
		t.Fatalf("skill output must not claim destructive capability, got %s", out)
	}
}

func TestScreenshotProxiesBrowserReadOnly(t *testing.T) {
	fg := &fakeGateway{connected: true}
	h := New(fg)
	req := httptest.NewRequest(http.MethodGet, "/v1/computer/screenshot?surface=browser", nil)
	w := httptest.NewRecorder()

	h.Screenshot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["surface"] != "browser" || resp["screenshot"] != "abc123" {
		t.Fatalf("unexpected screenshot response: %#v", resp)
	}
	if !bytes.Contains(fg.action, []byte(`browser_screenshot`)) {
		t.Fatalf("expected browser_screenshot action, got %s", string(fg.action))
	}
}

func TestComputerUseImplementsPackRuntimeExtensions(t *testing.T) {
	var _ packruntime.ContextProvider = (*Handler)(nil)
	var _ packruntime.SkillProvider = (*Handler)(nil)
}

func TestScreenshotRequiresConnectedBrowser(t *testing.T) {
	h := New(&fakeGateway{connected: false})
	req := httptest.NewRequest(http.MethodGet, "/v1/computer/screenshot?surface=browser", nil)
	w := httptest.NewRecorder()

	h.Screenshot(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}
