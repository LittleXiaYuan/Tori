package gateway

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
)

type testBackendPackModule struct {
	id     string
	routes []packruntime.BackendRoute
}

func (m testBackendPackModule) PackID() string { return m.id }

func (m testBackendPackModule) Routes() []packruntime.BackendRoute { return m.routes }

func TestPackRoutesExposeInstalledAndEnabledPacks(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.backup",
		Name:         "Backup Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"backup.info"}, Routes: []string{"/v1/backup/info"}},
		Frontend:     packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "backup", Label: "备份恢复", Path: "/packs/backup"}}},
		SDK:          packruntime.SDKManifest{TypeScript: "yunque-client/backup"},
		Update:       packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	gw := NewFromConfig(GatewayConfig{Packs: registry})

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/installed", nil)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handlePacksList(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Count   int                         `json:"count"`
		Enabled []packruntime.InstalledPack `json:"enabled"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Count != 1 || len(body.Enabled) != 1 || body.Enabled[0].Manifest.SDK.TypeScript != "yunque-client/backup" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestBackendPackModuleCanBeInjectedFromGatewayConfig(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.example",
		Name:         "Example Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Routes: []string{"/v1/example-pack/ping"}},
		Frontend:     packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "example", Label: "示例包", Path: "/packs/example"}}},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	module := testBackendPackModule{
		id: "yunque.pack.example",
		routes: []packruntime.BackendRoute{{
			Method: http.MethodGet,
			Path:   "/v1/example-pack/ping",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, map[string]any{"ok": true, "pack": "example"})
			},
		}},
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry, BackendPacks: []packruntime.BackendModule{module}})
	tenant := tenants.Register("example-pack")

	req := httptest.NewRequest(http.MethodGet, "/v1/example-pack/ping", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected injected backend pack route to be 200, got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/example-pack/ping", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected injected backend pack method gate to be 405, got %d body=%s", w.Code, w.Body.String())
	}

	if _, err := registry.Disable("yunque.pack.example"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/example-pack/ping", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected disabled injected backend pack route to be 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRegisterBackendPackMountsModuleAfterGatewayConstruction(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.runtime-added",
		Name:         "Runtime Added Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Routes: []string{"/v1/runtime-added/ping"}},
		Frontend:     packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "runtime-added", Label: "运行时包", Path: "/packs/runtime-added"}}},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	tenant := tenants.Register("runtime-added-pack")

	module := testBackendPackModule{
		id: "yunque.pack.runtime-added",
		routes: []packruntime.BackendRoute{{
			Method: http.MethodGet,
			Path:   "/v1/runtime-added/ping",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, map[string]any{"ok": true, "pack": "runtime-added"})
			},
		}},
	}
	gw.RegisterBackendPack(module)

	req := httptest.NewRequest(http.MethodGet, "/v1/runtime-added/ping", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected runtime registered backend pack route to be 200, got %d body=%s", w.Code, w.Body.String())
	}

	if _, err := registry.Disable("yunque.pack.runtime-added"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/runtime-added/ping", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected disabled runtime registered backend pack route to be 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRegisterBackendPackIsIdempotentForSamePackRoute(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.idempotent",
		Name:         "Idempotent Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Routes: []string{"/v1/idempotent-pack/ping"}},
		Frontend:     packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "idempotent-pack", Label: "幂等包", Path: "/packs/idempotent"}}},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	tenant := tenants.Register("idempotent-pack")
	module := testBackendPackModule{
		id: "yunque.pack.idempotent",
		routes: []packruntime.BackendRoute{{
			Method: http.MethodGet,
			Path:   "/v1/idempotent-pack/ping",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, map[string]any{"ok": true})
			},
		}},
	}

	gw.RegisterBackendPack(module)
	gw.RegisterBackendPack(module)

	req := httptest.NewRequest(http.MethodGet, "/v1/idempotent-pack/ping", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected idempotent route to stay mounted, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRegisterBackendPackPanicsOnRouteConflict(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	gw, _ := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	first := testBackendPackModule{
		id: "yunque.pack.first",
		routes: []packruntime.BackendRoute{{
			Method:  http.MethodGet,
			Path:    "/v1/conflicting-pack/ping",
			Handler: func(w http.ResponseWriter, r *http.Request) {},
		}},
	}
	second := testBackendPackModule{
		id: "yunque.pack.second",
		routes: []packruntime.BackendRoute{{
			Method:  http.MethodGet,
			Path:    "/v1/conflicting-pack/ping",
			Handler: func(w http.ResponseWriter, r *http.Request) {},
		}},
	}

	gw.RegisterBackendPack(first)
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected route conflict panic")
		}
	}()
	gw.RegisterBackendPack(second)
}

func TestRegisterBackendPackPanicsOnMissingRouteMethod(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	gw, _ := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	module := testBackendPackModule{
		id: "yunque.pack.no-method",
		routes: []packruntime.BackendRoute{{
			Path:    "/v1/no-method-pack/ping",
			Handler: func(w http.ResponseWriter, r *http.Request) {},
		}},
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected missing route method panic")
		}
		if !strings.Contains(fmt.Sprint(recovered), "must declare an HTTP method") {
			t.Fatalf("expected missing method panic, got %v", recovered)
		}
	}()
	gw.RegisterBackendPack(module)
}

func TestPackBackendModulesExposeMountedRoutes(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.example",
		Name:         "Example Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Routes: []string{"/v1/example-pack/ping"}},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	module := testBackendPackModule{
		id: "yunque.pack.example",
		routes: []packruntime.BackendRoute{{
			Method:  http.MethodGet,
			Path:    "/v1/example-pack/ping",
			Handler: func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]any{"ok": true}) },
		}},
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry, BackendPacks: []packruntime.BackendModule{module}})
	tenant := tenants.Register("backend-modules")

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/backend-modules", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Count   int                             `json:"count"`
		Modules []packruntime.BackendModuleInfo `json:"modules"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Count != 1 || len(body.Modules) != 1 {
		t.Fatalf("unexpected modules body: %#v", body)
	}
	if body.Modules[0].PackID != "yunque.pack.example" || len(body.Modules[0].Routes) != 1 || body.Modules[0].Routes[0].Path != "/v1/example-pack/ping" {
		t.Fatalf("unexpected module metadata: %#v", body.Modules[0])
	}
	if body.Modules[0].Routes[0].Method != http.MethodGet {
		t.Fatalf("expected mounted route method to be preserved, got %#v", body.Modules[0].Routes[0])
	}
	if body.Modules[0].Routes[0].Auth != "" {
		t.Fatalf("default mounted route auth should be omitted, got %#v", body.Modules[0].Routes[0])
	}
}

func TestBackendPackMultiMethodRouteInfoAndGate(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.multi-method",
		Name:         "Multi Method Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{"/v1/multi-method/config"},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/multi-method/config"},
				{Method: http.MethodPatch, Path: "/v1/multi-method/config"},
			},
		},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	module := testBackendPackModule{
		id: "yunque.pack.multi-method",
		routes: []packruntime.BackendRoute{{
			Methods: []string{http.MethodGet, http.MethodPatch},
			Path:    "/v1/multi-method/config",
			Handler: func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]any{"method": r.Method}) },
		}},
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry, BackendPacks: []packruntime.BackendModule{module}})
	tenant := tenants.Register("multi-method-pack")

	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		req := httptest.NewRequest(method, "/v1/multi-method/config", nil)
		req.Header.Set("X-API-Key", tenant.APIKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected %s multi-method route to pass, got %d body=%s", method, w.Code, w.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/multi-method/config", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected POST multi-method route to be 405, got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/backend-modules", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("modules status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Modules []packruntime.BackendModuleInfo `json:"modules"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Modules) != 1 || len(body.Modules[0].Routes) != 1 {
		t.Fatalf("unexpected modules body: %#v", body)
	}
	got := body.Modules[0].Routes[0]
	if got.Method != http.MethodGet || strings.Join(got.Methods, ",") != "GET,PATCH" {
		t.Fatalf("expected multi-method metadata to preserve primary and methods, got %#v", got)
	}
}

func TestBackendPackPassthroughAuthRouteKeepsPackGate(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.passthrough",
		Name:         "Passthrough Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Routes:     []string{"/v1/passthrough/session"},
			RouteSpecs: []packruntime.BackendRouteSpec{{Method: http.MethodPost, Path: "/v1/passthrough/session"}},
		},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	module := testBackendPackModule{
		id: "yunque.pack.passthrough",
		routes: []packruntime.BackendRoute{{
			Method: http.MethodPost,
			Path:   "/v1/passthrough/session",
			Auth:   packruntime.BackendRouteAuthPassthrough,
			Handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, map[string]any{"token": r.Header.Get("Authorization")})
			},
		}},
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry, BackendPacks: []packruntime.BackendModule{module}})
	tenant := tenants.Register("passthrough-pack")

	req := httptest.NewRequest(http.MethodPost, "/v1/passthrough/session", nil)
	req.Header.Set("Authorization", "Bearer external-token")
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("passthrough route should skip host requireAuth but keep route gate, status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "external-token") {
		t.Fatalf("passthrough route should preserve protocol-specific auth token, body=%s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/passthrough/session", nil)
	req.Header.Set("Authorization", "Bearer external-token")
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("passthrough route should still enforce method gate, status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/backend-modules", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("modules status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Modules []packruntime.BackendModuleInfo `json:"modules"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Modules) != 1 || len(body.Modules[0].Routes) != 1 {
		t.Fatalf("unexpected modules body: %#v", body)
	}
	if body.Modules[0].Routes[0].Auth != string(packruntime.BackendRouteAuthPassthrough) {
		t.Fatalf("expected passthrough auth metadata, got %#v", body.Modules[0].Routes[0])
	}
}

func TestPackRoutesInstallPackFromManifestPath(t *testing.T) {
	root := t.TempDir()
	registry, err := packruntime.NewRegistry(filepath.Join(root, "registry"))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	manifestDir := filepath.Join(root, "backup-pack")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifestPath := filepath.Join(manifestDir, packruntime.ManifestFileName)
	manifest := packruntime.Manifest{
		ID:           "yunque.pack.backup",
		Name:         "Backup Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"backup.info"}, Routes: []string{"/v1/backup/info"}},
		Frontend:     packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "backup", Label: "备份恢复", Path: "/packs/backup"}}},
		SDK:          packruntime.SDKManifest{TypeScript: "yunque-client/backup"},
		Update:       packruntime.UpdateManifest{Rollback: true},
	}
	if err := packruntime.SaveManifest(manifestPath, manifest); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	gw := NewFromConfig(GatewayConfig{Packs: registry})

	body := bytes.NewBufferString(`{"manifest_path":` + strconv.Quote(manifestPath) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/packs/install", body)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handlePackInstall(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("install status=%d body=%s", w.Code, w.Body.String())
	}
	pack, ok := registry.Get("yunque.pack.backup")
	if !ok || pack.Status != packruntime.PackStatusEnabled || pack.Manifest.SDK.TypeScript != "yunque-client/backup" {
		t.Fatalf("unexpected installed pack: %#v", pack)
	}
}

func TestPackRoutesInstallPackFromManifestURL(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	payload := []byte("remote backup artifact")
	sha := sha256.Sum256(payload)
	manifest := packruntime.Manifest{
		ID:           "yunque.pack.remote-backup",
		Name:         "Remote Backup Pack",
		Version:      "0.2.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"backup.info"}, Routes: []string{"/v1/backup/info"}},
		Frontend:     packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "backup", Label: "备份恢复", Path: "/packs/backup"}}},
		SDK:          packruntime.SDKManifest{TypeScript: "yunque-client/backup"},
		Distribution: packruntime.DistributionManifest{SHA256: hex.EncodeToString(sha[:])},
		Update:       packruntime.UpdateManifest{Rollback: true},
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pack.json":
			manifest.Distribution.PackageURL = srv.URL + "/remote-backup-0.2.0.tgz"
			_ = json.NewEncoder(w).Encode(manifest)
		case "/remote-backup-0.2.0.tgz":
			_, _ = w.Write(payload)
		default:
			t.Fatalf("unexpected URL path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	gw := NewFromConfig(GatewayConfig{Packs: registry})

	body := bytes.NewBufferString(`{"manifest_url":` + strconv.Quote(srv.URL+"/pack.json") + `,"download":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/packs/install", body)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handlePackInstall(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("install status=%d body=%s", w.Code, w.Body.String())
	}
	pack, ok := registry.Get("yunque.pack.remote-backup")
	if !ok || pack.Status != packruntime.PackStatusEnabled || pack.Source != srv.URL+"/pack.json" {
		t.Fatalf("unexpected downloaded pack: %#v", pack)
	}
	if pack.Artifacts == nil || pack.Artifacts.SHA256 != hex.EncodeToString(sha[:]) || pack.Artifacts.SizeBytes != int64(len(payload)) {
		t.Fatalf("expected downloaded artifacts to be recorded: %#v", pack.Artifacts)
	}
}

func TestPackRoutesPruneArtifacts(t *testing.T) {
	root := t.TempDir()
	registry, err := packruntime.NewRegistry(root)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	keepDir := filepath.Join(root, "artifacts", "yunque.pack.backup", "0.1.0")
	oldDir := filepath.Join(root, "artifacts", "yunque.pack.backup", "0.0.9")
	if err := os.MkdirAll(keepDir, 0o755); err != nil {
		t.Fatalf("MkdirAll keep: %v", err)
	}
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("MkdirAll old: %v", err)
	}
	keepPath := filepath.Join(keepDir, "keep.tgz")
	oldPath := filepath.Join(oldDir, "old.tgz")
	if err := os.WriteFile(keepPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile keep: %v", err)
	}
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile old: %v", err)
	}
	_, err = registry.InstallWithArtifacts(packruntime.Manifest{
		ID:           "yunque.pack.backup",
		Name:         "Backup Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Update:       packruntime.UpdateManifest{Rollback: true},
	}, "test", &packruntime.PackArtifacts{PackagePath: keepPath, SHA256: "keep", SizeBytes: 4})
	if err != nil {
		t.Fatalf("InstallWithArtifacts: %v", err)
	}
	gw := NewFromConfig(GatewayConfig{Packs: registry})
	req := httptest.NewRequest(http.MethodPost, "/v1/packs/prune", nil)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handlePackPrune(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("prune status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		RemovedCount int `json:"removed_count"`
		KeptCount    int `json:"kept_count"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.RemovedCount != 1 || body.KeptCount != 1 {
		t.Fatalf("unexpected prune body: %#v", body)
	}
}

func TestPackRoutesTogglePackStatus(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{ID: "yunque.pack.backup", Name: "Backup Pack", Version: "0.1.0", Optional: true, DefaultState: "enabled", Update: packruntime.UpdateManifest{Rollback: true}}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	gw := NewFromConfig(GatewayConfig{Packs: registry})

	body := bytes.NewBufferString(`{"id":"yunque.pack.backup"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/packs/disable", body)
	req = req.WithContext(ctxWithTenant(req.Context(), "t1"))
	w := httptest.NewRecorder()
	gw.handlePackDisable(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", w.Code, w.Body.String())
	}
	pack, ok := registry.Get("yunque.pack.backup")
	if !ok || pack.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected disabled pack, got %#v", pack)
	}
}

func TestBackendPackRouteSpecsGateByMethod(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.method-spec",
		Name:         "Method Spec Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Routes:     []string{"/v1/method-spec/import"},
			RouteSpecs: []packruntime.BackendRouteSpec{{Method: http.MethodPost, Path: "/v1/method-spec/import"}},
		},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	module := testBackendPackModule{
		id: "yunque.pack.method-spec",
		routes: []packruntime.BackendRoute{{
			Method:  http.MethodGet,
			Path:    "/v1/method-spec/import",
			Handler: func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]any{"ok": true}) },
		}},
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry, BackendPacks: []packruntime.BackendModule{module}})
	tenant := tenants.Register("method-spec-pack")

	req := httptest.NewRequest(http.MethodGet, "/v1/method-spec/import", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected method-aware manifest route gate to reject GET, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestBackupRoutesArePackGated(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	gw, tenants := newTestGateway()
	gw.SetPackRegistry(registry)
	tenant := tenants.Register("pack-gated-backup")

	req := httptest.NewRequest(http.MethodGet, "/v1/backup/info", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected disabled pack route to be 404, got %d body=%s", w.Code, w.Body.String())
	}

	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.backup",
		Name:         "Backup Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Routes: []string{"/v1/backup/info"}},
		Frontend:     packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "backup", Label: "备份恢复", Path: "/packs/backup"}}},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/backup/info", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected enabled pack route to be 200, got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/backup/info", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected registered backend pack method gate to be 405, got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/backup/export", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected undeclared manifest route to be 404, got %d body=%s", w.Code, w.Body.String())
	}
}
