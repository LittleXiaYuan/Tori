package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/modes"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/agentcore/websearch"
	controlplanepack "yunque-agent/internal/packs/controlplane"
	"yunque-agent/pkg/cogni"
)

type nlConfigTestSearch struct{}

func (nlConfigTestSearch) Name() string { return "test" }

func (nlConfigTestSearch) Search(context.Context, string, int) ([]websearch.Result, error) {
	return []websearch.Result{{Title: "ok"}}, nil
}

func TestNLConfigExecSearchToggleChangesRuntimeState(t *testing.T) {
	gw, _ := newTestGateway()
	reg := websearch.NewRegistry()
	reg.Register(nlConfigTestSearch{})
	gw.SetSearchRegistry(reg)

	result := &cogni.NLConfigResult{Params: map[string]any{"enabled": false}}
	gw.execSearchToggle(result)
	if result.ExecError != "" {
		t.Fatalf("exec error: %s", result.ExecError)
	}
	if gw.searchOn.Load() {
		t.Fatal("expected search to be disabled")
	}
}

func TestNLConfigExecUIModeSetsModeManager(t *testing.T) {
	gw, _ := newTestGateway()
	mm := modes.NewModeManager(nil, nil, "zh")
	gw.modeManager = mm

	result := &cogni.NLConfigResult{Params: map[string]any{"mode": "full"}}
	gw.execUIMode(result)
	if result.ExecError != "" {
		t.Fatalf("exec error: %s", result.ExecError)
	}

	got := mm.CurrentMode(context.Background(), "default", "")
	if got != modes.ModeScholar {
		t.Fatalf("expected scholar mode for full UI mode, got %s", got)
	}
}

func TestNLConfigExecModelSwitchFindsProviderByModel(t *testing.T) {
	gw, _ := newTestGateway()
	reg := llm.NewProviderRegistry(nil)
	err := reg.Register(llm.ProviderConfig{
		ID:      "openai",
		Type:    llm.ProviderTypeChat,
		BaseURL: "https://api.openai.com/v1",
		APIKeys: []string{"sk-test"},
		Model:   "gpt-4o",
		Enabled: true,
		Tier:    "smart",
	})
	if err != nil {
		t.Fatalf("register provider: %v", err)
	}
	gw.SetProviderRegistry(reg)

	result := &cogni.NLConfigResult{Params: map[string]any{"model": "gpt-4o"}}
	gw.execModelSwitch(result, "default")
	if result.ExecError != "" {
		t.Fatalf("exec error: %s", result.ExecError)
	}
	if p := reg.Get("openai"); p == nil || p.Client.Model() != "gpt-4o" {
		t.Fatalf("expected provider to remain switched to gpt-4o, got %#v", p)
	}
}

func TestNLConfigExecOutputPreferencesPersistToPersonaPrompt(t *testing.T) {
	gw, _ := newTestGateway()
	p, err := persona.New(t.TempDir())
	if err != nil {
		t.Fatalf("persona new: %v", err)
	}
	gw.persona = p

	langResult := &cogni.NLConfigResult{Params: map[string]any{"language": "en"}}
	gw.execOutputLang(langResult, "default")
	if langResult.ExecError != "" {
		t.Fatalf("language exec error: %s", langResult.ExecError)
	}
	styleResult := &cogni.NLConfigResult{Params: map[string]any{"style": "detailed"}}
	gw.execOutputStyle(styleResult, "default")
	if styleResult.ExecError != "" {
		t.Fatalf("style exec error: %s", styleResult.ExecError)
	}

	prompt := p.SystemPrompt()
	if !strings.Contains(prompt, "默认使用 `en`") {
		t.Fatalf("prompt missing language preference: %s", prompt)
	}
	if !strings.Contains(prompt, "默认采用 `detailed`") {
		t.Fatalf("prompt missing style preference: %s", prompt)
	}
}

func TestNLConfigExecProviderAddRegistersPresetProvider(t *testing.T) {
	gw, _ := newTestGateway()
	reg := llm.NewProviderRegistry(nil)
	gw.SetProviderRegistry(reg)

	result := &cogni.NLConfigResult{Params: map[string]any{
		"provider": "deepseek",
		"api_key":  "sk-test",
		"model":    "deepseek-chat",
	}}
	gw.execProviderAdd(result)
	if result.ExecError != "" {
		t.Fatalf("exec error: %s", result.ExecError)
	}
	if got := reg.Get("deepseek-deepseek-chat"); got == nil || got.Config.Model != "deepseek-chat" {
		t.Fatalf("expected deepseek provider to be registered, got %#v", got)
	}
}

func TestNLConfigExecProviderAddRequiresKeyForRemote(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetProviderRegistry(llm.NewProviderRegistry(nil))

	result := &cogni.NLConfigResult{Params: map[string]any{
		"provider": "deepseek",
		"model":    "deepseek-chat",
	}}
	gw.execProviderAdd(result)
	if result.ExecError == "" {
		t.Fatal("expected missing api key error")
	}
}

func TestExecProviderRejectsUnavailableProvider(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetProviderRegistry(llm.NewProviderRegistry(nil))

	req := httptest.NewRequest(http.MethodPost, "/api/providers/exec", strings.NewReader(`{"provider_id":"local-ollama"}`))
	w := httptest.NewRecorder()
	controlPlaneRoute(t, gw, "/api/providers/exec")(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unavailable provider, got %d: %s", w.Code, w.Body.String())
	}
	if got := gw.ExecProvider(); got != "" {
		t.Fatalf("exec provider should not change, got %q", got)
	}
}

func TestProviderSessionOverrideRejectsUnavailableProvider(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetProviderRegistry(llm.NewProviderRegistry(nil))

	req := httptest.NewRequest(http.MethodPost, "/api/providers/session", strings.NewReader(`{"session_id":"s1","provider_id":"local-ollama"}`))
	w := httptest.NewRecorder()
	controlPlaneRoute(t, gw, "/api/providers/session")(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unavailable provider, got %d: %s", w.Code, w.Body.String())
	}
	if got := gw.providerReg.GetForSession("s1"); got != nil {
		t.Fatalf("session provider should not change, got %#v", got)
	}
}

func controlPlaneRoute(t *testing.T, gw *Gateway, path string) http.HandlerFunc {
	t.Helper()
	for _, route := range controlplanepack.NewHandler(gw).Routes() {
		if route.Path == path {
			return route.Handler
		}
	}
	t.Fatalf("control-plane route %s not found", path)
	return nil
}

func TestNLConfigExecSystemInfoReturnsRuntimeState(t *testing.T) {
	gw, _ := newTestGateway()
	reg := llm.NewProviderRegistry(nil)
	if err := reg.Register(llm.ProviderConfig{
		ID:      "local",
		Type:    llm.ProviderTypeChat,
		BaseURL: "http://127.0.0.1:11434/v1",
		Model:   "llama3",
		Enabled: true,
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	gw.SetProviderRegistry(reg)
	gw.searchOn.Store(false)

	result := &cogni.NLConfigResult{Params: map[string]any{}}
	gw.execSystemInfo(result)
	if result.ExecError != "" {
		t.Fatalf("exec error: %s", result.ExecError)
	}
	info, ok := result.ExecResult.(map[string]any)
	if !ok {
		t.Fatalf("expected map exec result, got %T", result.ExecResult)
	}
	if info["providers_total"] != 1 || info["providers_enabled"] != 1 {
		t.Fatalf("expected provider counts, got %#v", info)
	}
	if info["search_enabled"] != false {
		t.Fatalf("expected search disabled, got %#v", info["search_enabled"])
	}
}

func TestNLConfigExecUsageStatsReturnsTrackerRecords(t *testing.T) {
	gw, _ := newTestGateway()
	gw.usage.RecordChat("default", 123)
	gw.usage.RecordStream("tenant-b", 456)

	result := &cogni.NLConfigResult{Params: map[string]any{}}
	gw.execUsageStats(result)
	if result.ExecError != "" {
		t.Fatalf("exec error: %s", result.ExecError)
	}
	info, ok := result.ExecResult.(map[string]any)
	if !ok {
		t.Fatalf("expected map exec result, got %T", result.ExecResult)
	}
	if info["count"] != 2 {
		t.Fatalf("expected two usage records, got %#v", info)
	}
}

func TestNLConfigExecSkillInstallSearchesLocalMarket(t *testing.T) {
	gw, _ := newTestGateway()
	market := skillmarket.NewMarket(t.TempDir())
	if err := market.Publish(skillmarket.SkillMeta{
		Name:        "translator",
		Version:     "1.0.0",
		Description: "Translate text between Chinese and English",
		Author:      "Yunque",
		Category:    skillmarket.CatLanguage,
		Tags:        []string{"translate", "language"},
	}); err != nil {
		t.Fatalf("publish skill: %v", err)
	}
	gw.SetSkillMarket(market)

	result := &cogni.NLConfigResult{Params: map[string]any{
		"skill_name": "translator",
	}}
	gw.execSkillInstall(result)
	if result.ExecError != "" {
		t.Fatalf("exec error: %s", result.ExecError)
	}
	info, ok := result.ExecResult.(map[string]any)
	if !ok {
		t.Fatalf("expected map exec result, got %T", result.ExecResult)
	}
	if info["candidate_count"] != 1 {
		t.Fatalf("expected one candidate, got %#v", info)
	}
	local, ok := info["local_results"].([]skillmarket.SkillMeta)
	if !ok || len(local) != 1 || local[0].Name != "translator" {
		t.Fatalf("expected translator local result, got %#v", info["local_results"])
	}
}

func TestNLConfigExecDataBackupCreatesZip(t *testing.T) {
	gw, _ := newTestGateway()
	backupDir := t.TempDir()

	result := &cogni.NLConfigResult{Params: map[string]any{
		"backup_dir":  backupDir,
		"max_backups": float64(2),
	}}
	gw.execDataBackup(result)
	if result.ExecError != "" {
		t.Fatalf("exec error: %s", result.ExecError)
	}
	info, ok := result.ExecResult.(map[string]any)
	if !ok {
		t.Fatalf("expected map exec result, got %T", result.ExecResult)
	}
	path, ok := info["backup_path"].(string)
	if !ok || path == "" {
		t.Fatalf("expected backup path, got %#v", info)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected backup zip to exist: %v", err)
	}
	if info["backup_exists"] != true {
		t.Fatalf("expected backup_exists true, got %#v", info)
	}
}

func TestNLConfigExecAuditLogReturnsRecentEntries(t *testing.T) {
	gw, _ := newTestGateway()
	trail := audit.NewTrail(t.TempDir())
	trail.Record(audit.TrailEntry{Operation: "nl_config", Result: "ok", Actor: "default"})
	gw.SetAuditTrail(trail)

	result := &cogni.NLConfigResult{Params: map[string]any{"limit": float64(5)}}
	gw.execAuditLog(result)
	if result.ExecError != "" {
		t.Fatalf("exec error: %s", result.ExecError)
	}
	info, ok := result.ExecResult.(map[string]any)
	if !ok {
		t.Fatalf("expected map exec result, got %T", result.ExecResult)
	}
	if info["count"] != 1 {
		t.Fatalf("expected one audit entry, got %#v", info)
	}
}
