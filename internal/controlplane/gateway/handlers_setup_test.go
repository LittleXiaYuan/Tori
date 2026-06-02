package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chdirTemp(t *testing.T) string {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	return tmp
}

func TestHandleSetupTemplatesReturnsEnvelope(t *testing.T) {
	g := &Gateway{}
	req := httptest.NewRequest(http.MethodGet, "/v1/setup/templates", nil)
	rr := httptest.NewRecorder()

	g.handleSetupTemplates(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}

	var resp struct {
		Templates []map[string]any `json:"templates"`
		Count     int              `json:"count"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count == 0 || len(resp.Templates) != resp.Count {
		t.Fatalf("unexpected envelope: count=%d len=%d", resp.Count, len(resp.Templates))
	}
}

func TestHandleSetupApplyAcceptsOverridesAndWritesEnv(t *testing.T) {
	g := &Gateway{}
	tmp := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("DEFAULT_API_KEY=keep-me\n"), 0o600); err != nil {
		t.Fatalf("seed .env: %v", err)
	}

	body := strings.NewReader(`{"template_id":"personal-assistant","overrides":{"base_url":"https://api.example.com/v1","api_key":"sk-test","model":"gpt-test"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/setup/apply", body)
	rr := httptest.NewRecorder()

	g.handleSetupApply(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		OK              bool   `json:"ok"`
		Status          string `json:"status"`
		Applied         string `json:"applied"`
		Persisted       bool   `json:"persisted"`
		RestartRequired bool   `json:"restart_required"`
		Message         string `json:"message"`
		EnvContent      string `json:"env_content"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || resp.Status != "applied" || resp.Applied != "personal-assistant" || !resp.Persisted || !resp.RestartRequired {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !strings.Contains(resp.EnvContent, "LLM_BASE_URL=https://api.example.com/v1") {
		t.Fatalf("env preview missing base url: %s", resp.EnvContent)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	env := string(data)
	for _, want := range []string{
		"LLM_BASE_URL=https://api.example.com/v1",
		"LLM_API_KEY=sk-test",
		"LLM_MODEL=gpt-test",
		"AGENT_ADDR=:9090",
		"REACT_ENABLED=true",
		"NATIVE_FC=true",
		"SANDBOX_TIER=personal",
		"DEFAULT_API_KEY=keep-me",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("expected %q in .env, got:\n%s", want, env)
		}
	}
}

func TestHandleSetupTestProviderUsesBackendRequest(t *testing.T) {
	g := &Gateway{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	body := strings.NewReader(`{"base_url":"` + server.URL + `","api_key":"sk-test","model":"gpt-test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/setup/test-provider", body)
	rr := httptest.NewRecorder()

	g.handleSetupTestProvider(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		OK       bool `json:"ok"`
		Provider struct {
			Name      string `json:"name"`
			BaseURL   string `json:"base_url"`
			Model     string `json:"model"`
			Available bool   `json:"available"`
			Latency   string `json:"latency"`
		} `json:"provider"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || !resp.Provider.Available || resp.Provider.BaseURL != server.URL || resp.Provider.Model != "gpt-test" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestRequireSetupOrAuthTreatsKeylessConfigAsConfigured(t *testing.T) {
	g := &Gateway{}
	tmp := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("LLM_BASE_URL=http://localhost:11434/v1\nLLM_MODEL=qwen2.5\n"), 0o600); err != nil {
		t.Fatalf("seed .env: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/setup/test-provider", nil)
	g.requireSetupOrAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(rr, req)

	if rr.Code == http.StatusNoContent {
		t.Fatalf("expected configured keyless setup to require auth")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestConfiguredSetupRoutesRejectAnonymousRequests(t *testing.T) {
	g := &Gateway{mux: http.NewServeMux()}
	tmp := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("LLM_BASE_URL=http://localhost:11434/v1\nLLM_MODEL=qwen2.5\n"), 0o600); err != nil {
		t.Fatalf("seed .env: %v", err)
	}
	g.registerSetupRoutes()
	g.registerSystemRoutes()

	for _, path := range []string{
		"/v1/setup/detect",
		"/v1/setup/health",
		"/v1/setup/templates",
		"/api/settings/check",
	} {
		t.Run(path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)

			g.mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 for %s, got %d body=%s", path, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestHandleSettingsCheckSupportsKeylessProviders(t *testing.T) {
	g := &Gateway{}
	tmp := chdirTemp(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	env := "LLM_BASE_URL=" + server.URL + "\nLLM_MODEL=qwen2.5\n"
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte(env), 0o600); err != nil {
		t.Fatalf("seed .env: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/check", nil)
	g.handleSettingsCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		EnvExists   bool `json:"env_exists"`
		HasLLMKey   bool `json:"has_llm_key"`
		HasLLMURL   bool `json:"has_llm_url"`
		HasLLMModel bool `json:"has_llm_model"`
		APIOK       bool `json:"api_ok"`
		SetupNeeded bool `json:"setup_needed"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.EnvExists || !resp.HasLLMURL || !resp.HasLLMModel || resp.HasLLMKey || !resp.APIOK || resp.SetupNeeded {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestHandleSettingsConfigSavesEnvValues(t *testing.T) {
	g, tm := newTestGateway()
	tenant := tm.Register("settings-save")
	tmp := chdirTemp(t)
	envPath := filepath.Join(tmp, ".env")
	if err := os.WriteFile(envPath, []byte("LLM_BASE_URL=https://old.example/v1\nLLM_API_KEY=sk-old\nLLM_MODEL=old-model\n"), 0o600); err != nil {
		t.Fatalf("seed .env: %v", err)
	}

	body := strings.NewReader(`{"values":{"HOST_READ_PATHS":"C:\\Users\\Administrator\\Documents","LLM_API_KEY":"****"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/config", body)
	req.Header.Set("X-API-Key", tenant.APIKey)
	rr := httptest.NewRecorder()

	g.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	env := string(data)
	if !strings.Contains(env, `HOST_READ_PATHS=C:\Users\Administrator\Documents`) {
		t.Fatalf("saved env missing HOST_READ_PATHS: %s", env)
	}
	if strings.Contains(env, "# ── Extra ──\nHOST_READ_PATHS=") {
		t.Fatalf("HOST_READ_PATHS should be a first-class ordered env key, got extra section:\n%s", env)
	}
	if !strings.Contains(env, "LLM_API_KEY=sk-old") {
		t.Fatalf("masked sensitive value should preserve existing key: %s", env)
	}
}
