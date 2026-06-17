package controlplanepack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

type pluginTestGateway struct {
	toolsGateway
	reg     *plugin.Registry
	loader  *plugin.Loader
	rebuild int
}

func (g *pluginTestGateway) PluginRegistry() *plugin.Registry { return g.reg }

func (g *pluginTestGateway) PluginLoader() *plugin.Loader { return g.loader }

func (g *pluginTestGateway) RebuildSkillsFromPlugins() int {
	g.rebuild++
	if g.reg == nil {
		return 0
	}
	return len(g.reg.AllSkills())
}

type testPlugin struct {
	name   string
	skills []skills.Skill
	tabs   []plugin.UITab
}

func (p testPlugin) Name() string { return p.name }

func (p testPlugin) Description() string { return "test plugin" }

func (p testPlugin) Skills() []skills.Skill { return p.skills }

func (p testPlugin) SystemPrompt() string { return "" }

func (p testPlugin) UITabs() []plugin.UITab { return p.tabs }

func (p testPlugin) HTTPHandlers() map[string]http.HandlerFunc { return nil }

type testSkill struct{ name string }

func (s testSkill) Name() string { return s.name }

func (s testSkill) Description() string { return "test skill" }

func (s testSkill) Parameters() map[string]any { return nil }

func (s testSkill) Execute(context.Context, map[string]any, *skills.Environment) (string, error) {
	return "ok", nil
}

func TestPluginRoutesAreNative(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.Register(testPlugin{name: "native-plugin"})
	gateway := &pluginTestGateway{reg: reg}
	h := NewHandler(gateway)

	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}
	for _, path := range []string{"/v1/plugins", "/v1/plugins/toggle", "/v1/plugins/create", "/v1/plugins/delete", "/v1/plugins/files", "/v1/plugins/ui", "/v1/plugins/reload", "/v1/plugins/open-folder"} {
		if byPath[path] == nil {
			t.Fatalf("route %s not mounted", path)
		}
	}
	rec := httptest.NewRecorder()
	byPath["/v1/plugins"](rec, httptest.NewRequest(http.MethodGet, "/v1/plugins", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "native-plugin") {
		t.Fatalf("plugins status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gateway.bridged != 0 {
		t.Fatalf("plugin route should not call bridge, calls=%d", gateway.bridged)
	}
}

func TestPluginToggleRebuildsSkills(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.Register(testPlugin{name: "toggle-me", skills: []skills.Skill{testSkill{name: "s1"}}})
	gateway := &pluginTestGateway{reg: reg}
	h := NewHandler(gateway)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/plugins/toggle", strings.NewReader(`{"name":"toggle-me","enabled":false}`))
	h.handlePluginToggle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if reg.IsEnabled("toggle-me") {
		t.Fatalf("plugin should be disabled")
	}
	if gateway.rebuild != 1 {
		t.Fatalf("rebuild calls=%d, want 1", gateway.rebuild)
	}
	if !strings.Contains(rec.Body.String(), `"skills_count":0`) {
		t.Fatalf("expected rebuilt skill count, got %s", rec.Body.String())
	}
}

func TestPluginFilesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name":"demo","description":"demo"}`), 0644); err != nil {
		t.Fatal(err)
	}

	reg := plugin.NewRegistry()
	loader := plugin.NewLoader(dir, reg, nil)
	gateway := &pluginTestGateway{reg: reg, loader: loader}
	h := NewHandler(gateway)

	rec := httptest.NewRecorder()
	h.handlePluginFiles(rec, httptest.NewRequest(http.MethodPut, "/v1/plugins/files?name=demo", strings.NewReader(`{"file":"handler.py","content":"print('hi')"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gateway.rebuild != 1 {
		t.Fatalf("rebuild calls=%d, want 1", gateway.rebuild)
	}

	rec = httptest.NewRecorder()
	h.handlePluginFiles(rec, httptest.NewRequest(http.MethodGet, "/v1/plugins/files?name=demo", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "handler.py") || !strings.Contains(rec.Body.String(), "print('hi')") {
		t.Fatalf("get status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPluginUIListsNativeTabs(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.Register(testPlugin{
		name: "ui-plugin",
		tabs: []plugin.UITab{{Key: "custom", Label: "Custom", Icon: "Box"}},
	})
	h := NewHandler(&pluginTestGateway{reg: reg})

	rec := httptest.NewRecorder()
	h.handlePluginUI(rec, httptest.NewRequest(http.MethodGet, "/v1/plugins/ui", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"plugin":"ui-plugin"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
