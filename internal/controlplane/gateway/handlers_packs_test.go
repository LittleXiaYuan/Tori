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
	"slices"
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

func writeTestPackManifest(t *testing.T, manifestPath string, manifest packruntime.Manifest) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("MkdirAll manifest dir: %v", err)
	}
	if err := packruntime.SaveManifest(manifestPath, manifest); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
}

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

func TestPackCatalogListsInstallableManifestsAndCapabilityHints(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestPackManifest(t, filepath.Join(sourceDir, "ready-pack", packruntime.ManifestFileName), packruntime.Manifest{
		ID:           "yunque.pack.catalog-ready",
		Name:         "Catalog Ready Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"catalog.ready", "catalog.shared"},
			Routes:       []string{"/v1/catalog/ready"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodGet, Path: "/v1/catalog/ready"}},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "catalog-ready", Label: "Catalog Ready", Path: "/packs/catalog-ready"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/catalog-ready"},
		Distribution: packruntime.DistributionManifest{
			ManifestURL: "https://packs.yunque.local/catalog-ready/pack.json",
			PackageURL:  "https://packs.yunque.local/catalog-ready/catalog-ready-0.1.0.tgz",
			SHA256:      strings.Repeat("a", 64),
			SizeBytes:   1024,
		},
		Update: packruntime.UpdateManifest{Rollback: true},
	})
	writeTestPackManifest(t, filepath.Join(sourceDir, "missing-pack", packruntime.ManifestFileName), packruntime.Manifest{
		ID:           "yunque.pack.catalog-missing",
		Name:         "Catalog Missing Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"catalog.missing"},
			Routes:       []string{"/v1/catalog/missing"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodGet, Path: "/v1/catalog/missing"}},
		},
		SDK:          packruntime.SDKManifest{TypeScript: "yunque-client/catalog-missing"},
		Distribution: packruntime.DistributionManifest{PackageURL: "https://packs.yunque.local/catalog-missing/catalog-missing-0.1.0.tgz", SHA256: strings.Repeat("b", 64)},
		Update:       packruntime.UpdateManifest{Rollback: true},
	})
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.catalog-ready",
		Name:         "Catalog Ready Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"catalog.ready"}, Routes: []string{"/v1/catalog/ready"}},
		SDK:          packruntime.SDKManifest{TypeScript: "yunque-client/catalog-ready"},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	missingSource := filepath.Join(sourceDir, "missing-source")
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.SetPackCatalogSources([]string{sourceDir, " ", missingSource})
	tenant := tenants.Register("pack-catalog")

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/catalog?capability=catalog.missing", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var catalog packruntime.PackCatalogReport
	if err := json.NewDecoder(w.Body).Decode(&catalog); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if catalog.Count != 1 || catalog.Downloadable != 1 || len(catalog.InstallHints) != 1 {
		t.Fatalf("expected missing capability to return one downloadable install hint: %#v", catalog)
	}
	if catalog.InstallHints[0].Manifest.ID != "yunque.pack.catalog-missing" || catalog.InstallHints[0].UpdateAction != "install" {
		t.Fatalf("unexpected install hint: %#v", catalog.InstallHints[0])
	}
	if len(catalog.Sources) != 2 || catalog.Sources[0] != sourceDir || catalog.Sources[1] != missingSource {
		t.Fatalf("expected trimmed catalog sources to be reported: %#v", catalog.Sources)
	}
	if len(catalog.SourceReports) != 2 || !catalog.SourceReports[0].OK || catalog.SourceReports[0].ManifestCount != 2 || catalog.SourceReports[0].MatchedEntries != 1 {
		t.Fatalf("expected successful source report with manifest and matched counts: %#v", catalog.SourceReports)
	}
	if catalog.SourceReports[1].OK || len(catalog.SourceReports[1].Errors) != 1 || len(catalog.Errors) != 1 {
		t.Fatalf("expected missing source to be observable through source reports and errors: %#v errors=%#v", catalog.SourceReports, catalog.Errors)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/catalog?capability=catalog.ready", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	catalog = packruntime.PackCatalogReport{}
	if err := json.NewDecoder(w.Body).Decode(&catalog); err != nil {
		t.Fatalf("decode ready catalog: %v", err)
	}
	if catalog.Count != 1 || catalog.Installed != 1 || len(catalog.EnableHints) != 1 || catalog.EnableHints[0].UpdateAction != "enable" {
		t.Fatalf("expected installed disabled pack to become enable hint: %#v", catalog)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/catalog?q=missing", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	catalog = packruntime.PackCatalogReport{}
	if err := json.NewDecoder(w.Body).Decode(&catalog); err != nil {
		t.Fatalf("decode query catalog: %v", err)
	}
	if catalog.Count != 1 || catalog.Entries[0].Manifest.ID != "yunque.pack.catalog-missing" {
		t.Fatalf("expected query to filter catalog entries: %#v", catalog)
	}
}

func TestPackCapabilityPrepareBuildsOperatorChecklist(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestPackManifest(t, filepath.Join(sourceDir, "install-pack", packruntime.ManifestFileName), packruntime.Manifest{
		ID:           "yunque.pack.install-me",
		Name:         "Install Me Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"prepare.install"},
			Routes:       []string{"/v1/prepare/install"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodGet, Path: "/v1/prepare/install"}},
		},
		SDK: packruntime.SDKManifest{TypeScript: "yunque-client/install-me"},
		Distribution: packruntime.DistributionManifest{
			ManifestURL: "https://packs.yunque.local/install-me/pack.json",
			PackageURL:  "https://packs.yunque.local/install-me/install-me-0.1.0.tgz",
			FrontendURL: "https://packs.yunque.local/install-me/remoteEntry.js",
			SHA256:      strings.Repeat("c", 64),
			SizeBytes:   2048,
		},
		Update: packruntime.UpdateManifest{Rollback: true},
	})

	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.ready",
		Name:         "Ready Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"prepare.ready"},
			Routes:       []string{"/v1/prepare/ready"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodGet, Path: "/v1/prepare/ready"}},
		},
		Update: packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install ready: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.disabled",
		Name:         "Disabled Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"prepare.enable"},
			Routes:       []string{"/v1/prepare/enable"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodGet, Path: "/v1/prepare/enable"}},
		},
		Update: packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install disabled: %v", err)
	}

	module := testBackendPackModule{
		id: "yunque.pack.ready",
		routes: []packruntime.BackendRoute{{
			Method: http.MethodGet,
			Path:   "/v1/prepare/ready",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, map[string]any{"ok": true})
			},
		}},
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry, BackendPacks: []packruntime.BackendModule{module}})
	missingSource := filepath.Join(t.TempDir(), "missing-catalog")
	gw.SetPackCatalogSources([]string{sourceDir, missingSource})
	tenant := tenants.Register("pack-prepare")

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/prepare?capability=prepare.ready&capability=prepare.enable&capability=prepare.install&capability=prepare.unknown", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var prepare packruntime.CapabilityPrepareReport
	if err := json.NewDecoder(w.Body).Decode(&prepare); err != nil {
		t.Fatalf("decode prepare: %v", err)
	}
	if prepare.Action != "install" || prepare.Allowed {
		t.Fatalf("expected install action with blocked workflow: %#v", prepare)
	}
	if prepare.ReadyCount != 1 || prepare.EnableCount != 1 || prepare.InstallCount != 2 || prepare.DownloadCount != 1 {
		t.Fatalf("unexpected prepare counters: %#v", prepare)
	}
	if len(prepare.UseSteps) != 1 || prepare.UseSteps[0].PackID != "yunque.pack.ready" {
		t.Fatalf("unexpected use steps: %#v", prepare.UseSteps)
	}
	if len(prepare.EnableSteps) != 1 || prepare.EnableSteps[0].PackID != "yunque.pack.disabled" {
		t.Fatalf("unexpected enable steps: %#v", prepare.EnableSteps)
	}
	if len(prepare.InstallSteps) != 2 || prepare.InstallSteps[0].PackID != "yunque.pack.install-me" {
		t.Fatalf("unexpected install steps: %#v", prepare.InstallSteps)
	}
	if prepare.InstallSteps[1].Capability != "prepare.unknown" || !strings.Contains(prepare.InstallSteps[1].Reason, "catalog sources scanned:") || !strings.Contains(prepare.InstallSteps[1].Reason, "matched=1") || !strings.Contains(prepare.InstallSteps[1].Reason, "missing-catalog") {
		t.Fatalf("expected unmatched install step to summarize catalog source diagnostics: %#v", prepare.InstallSteps[1])
	}
	if len(prepare.DownloadSteps) != 1 || prepare.DownloadSteps[0].SHA256 != strings.Repeat("c", 64) {
		t.Fatalf("expected downloadable package with sha256: %#v", prepare.DownloadSteps)
	}
	if len(prepare.CatalogSourceReports) != 2 || prepare.CatalogSourceReports[0].ManifestCount != 1 || prepare.CatalogSourceReports[0].MatchedEntries != 1 {
		t.Fatalf("expected prepare report to expose successful catalog source diagnostics: %#v", prepare.CatalogSourceReports)
	}
	if prepare.CatalogSourceReports[1].OK || len(prepare.CatalogSourceReports[1].Errors) != 1 {
		t.Fatalf("expected prepare report to expose missing catalog source diagnostics: %#v", prepare.CatalogSourceReports)
	}
	if len(prepare.Plan.CatalogSourceReports) != len(prepare.CatalogSourceReports) {
		t.Fatalf("expected nested plan to carry the same catalog source diagnostics: plan=%#v prepare=%#v", prepare.Plan.CatalogSourceReports, prepare.CatalogSourceReports)
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

func TestPackCapabilitiesExposeManifestCapabilityIndex(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.capability-index",
		Name:         "Capability Index Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"capability.alpha", "capability.beta"},
			Routes:       []string{"/v1/capability-index/ping"},
			Permissions:  []string{"capability:read"},
		},
		Frontend: packruntime.FrontendManifest{
			Menus:  []packruntime.FrontendMenu{{Key: "capability-index", Label: "能力索引", Path: "/packs/capability-index"}},
			Routes: []packruntime.FrontendRoute{{Path: "/packs/capability-index/detail", Component: "CapabilityIndexPage"}},
		},
		SDK: packruntime.SDKManifest{TypeScript: "yunque-client/capability-index"},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.disabled-capability",
		Name:         "Disabled Capability Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"capability.disabled"}, Routes: []string{"/v1/disabled-capability/ping"}},
	}, "test")
	if err != nil {
		t.Fatalf("Install disabled: %v", err)
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	tenant := tenants.Register("pack-capabilities")

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var report packruntime.CapabilityIndexReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if report.Packs != 2 || report.EnabledPacks != 1 || report.Capabilities != 3 || report.EnabledCapabilities != 2 {
		t.Fatalf("unexpected capability index summary: %#v", report)
	}
	var alpha packruntime.CapabilityIndexEntry
	var disabled packruntime.CapabilityIndexEntry
	for _, entry := range report.Entries {
		switch entry.Capability {
		case "capability.alpha":
			alpha = entry
		case "capability.disabled":
			disabled = entry
		}
	}
	if alpha.PackID != "yunque.pack.capability-index" || !alpha.Enabled || alpha.SDKTypeScript != "yunque-client/capability-index" {
		t.Fatalf("unexpected alpha capability entry: %#v", alpha)
	}
	if len(alpha.Routes) != 1 || alpha.Routes[0] != "/v1/capability-index/ping" || len(alpha.Permissions) != 1 || alpha.Permissions[0] != "capability:read" {
		t.Fatalf("capability entry should include routes and permissions: %#v", alpha)
	}
	if len(alpha.FrontendPaths) != 2 || alpha.FrontendPaths[0] != "/packs/capability-index" || alpha.FrontendPaths[1] != "/packs/capability-index/detail" {
		t.Fatalf("capability entry should include sorted frontend paths: %#v", alpha.FrontendPaths)
	}
	if disabled.PackID != "yunque.pack.disabled-capability" || disabled.Enabled {
		t.Fatalf("disabled capability should stay visible but disabled: %#v", disabled)
	}
}

func TestPackCapabilityCogniProfileCompactsCapabilityContext(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.computer-profile",
		Name:         "Computer Profile Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"computer.use.plan"},
			Routes:       []string{"/v1/computer/intent/plan"},
			Permissions:  []string{"computer:plan", "browser:read"},
		},
		SDK: packruntime.SDKManifest{TypeScript: "yunque-client/computer-use"},
	}, "test")
	if err != nil {
		t.Fatalf("Install computer profile: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.disabled-memory-profile",
		Name:         "Disabled Memory Profile Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"memory.recall"},
			Routes:       []string{"/v1/memory/search"},
			Permissions:  []string{"memory:read"},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "memory", Label: "记忆", Path: "/memory"}}},
	}, "test")
	if err != nil {
		t.Fatalf("Install memory profile: %v", err)
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	tenant := tenants.Register("pack-cogni-profile")

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/cogni-profile", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var report packruntime.CapabilityCogniProfileReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if report.Count != 2 || len(report.Entries) != 2 {
		t.Fatalf("unexpected profile count: %#v", report)
	}
	var computer, memory packruntime.CapabilityCogniProfileEntry
	for _, entry := range report.Entries {
		switch entry.Capability {
		case "computer.use.plan":
			computer = entry
		case "memory.recall":
			memory = entry
		}
	}
	if computer.SourceType != "pack" || computer.SourceID != "yunque.pack.computer-profile" || computer.Action != "use" || computer.Risk != "high" {
		t.Fatalf("unexpected computer profile: %#v", computer)
	}
	if computer.InvokeHint != "sdk:yunque-client/computer-use" || !strings.Contains(computer.TokenHint, "computer.use.plan via yunque.pack.computer-profile") {
		t.Fatalf("computer profile should be compact and invokable: %#v", computer)
	}
	if !slices.Contains(computer.Constraints, "高风险能力需要用户授权，不能静默扩大权限") {
		t.Fatalf("computer profile should carry high-risk constraint: %#v", computer.Constraints)
	}
	if memory.Action != "enable" || memory.Enabled || memory.Risk != "low" || memory.InvokeHint != "route:/v1/memory/search" {
		t.Fatalf("unexpected memory profile: %#v", memory)
	}
	if !slices.Contains(memory.Constraints, "能力包未启用，使用前先引导启用") {
		t.Fatalf("disabled profile should explain enablement constraint: %#v", memory.Constraints)
	}
}

func TestPackCapabilityResolveReturnsPreferredAction(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.disabled-resolver",
		Name:         "Disabled Resolver Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"resolver.demo"}, Routes: []string{"/v1/resolver/disabled"}},
		SDK:          packruntime.SDKManifest{TypeScript: "yunque-client/resolver-disabled"},
	}, "test")
	if err != nil {
		t.Fatalf("Install disabled: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.enabled-resolver",
		Name:         "Enabled Resolver Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"resolver.demo"}, Routes: []string{"/v1/resolver/enabled"}},
		SDK:          packruntime.SDKManifest{TypeScript: "yunque-client/resolver-enabled"},
	}, "test")
	if err != nil {
		t.Fatalf("Install enabled: %v", err)
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	tenant := tenants.Register("pack-capability-resolve")

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/resolve?capability=resolver.demo", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var report packruntime.CapabilityResolveReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !report.Found || !report.Enabled || report.Action != "use" || report.Preferred == nil {
		t.Fatalf("expected enabled resolver action, got %#v", report)
	}
	if report.Preferred.PackID != "yunque.pack.enabled-resolver" || len(report.Entries) != 2 || len(report.EnabledEntries) != 1 {
		t.Fatalf("expected enabled pack to be preferred while keeping all providers visible: %#v", report)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/resolve?capability=capability.not-installed", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("missing status=%d body=%s", w.Code, w.Body.String())
	}
	report = packruntime.CapabilityResolveReport{}
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode missing: %v", err)
	}
	if report.Found || report.Enabled || report.Action != "install" || report.Preferred != nil {
		t.Fatalf("expected missing capability to suggest install: %#v", report)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/resolve", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected missing capability query to be 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPackCapabilityGateChecksEnabledStateAndRouteAudit(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.gated",
		Name:         "Gated Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"gated.ready"},
			Routes:       []string{"/v1/gated/ready"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodGet, Path: "/v1/gated/ready"}},
		},
	}, "test")
	if err != nil {
		t.Fatalf("Install gated: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.gated-disabled",
		Name:         "Gated Disabled Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"gated.disabled"}, Routes: []string{"/v1/gated/disabled"}},
	}, "test")
	if err != nil {
		t.Fatalf("Install disabled: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.gated-mismatch",
		Name:         "Gated Mismatch Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"gated.mismatch"},
			Routes:       []string{"/v1/gated/mismatch"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodPost, Path: "/v1/gated/mismatch"}},
		},
	}, "test")
	if err != nil {
		t.Fatalf("Install mismatch: %v", err)
	}
	module := testBackendPackModule{
		id: "yunque.pack.gated",
		routes: []packruntime.BackendRoute{{
			Method:  http.MethodGet,
			Path:    "/v1/gated/ready",
			Handler: func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]any{"ok": true}) },
		}},
	}
	mismatchModule := testBackendPackModule{
		id: "yunque.pack.gated-mismatch",
		routes: []packruntime.BackendRoute{{
			Method:  http.MethodGet,
			Path:    "/v1/gated/mismatch",
			Handler: func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]any{"ok": true}) },
		}},
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry, BackendPacks: []packruntime.BackendModule{module, mismatchModule}})
	tenant := tenants.Register("pack-capability-gate")

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/gate?capability=gated.ready", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ready status=%d body=%s", w.Code, w.Body.String())
	}
	var gate packruntime.CapabilityGateReport
	if err := json.NewDecoder(w.Body).Decode(&gate); err != nil {
		t.Fatalf("decode ready: %v", err)
	}
	if !gate.Allowed || gate.Action != "use" || gate.Reason != "capability is available through an enabled pack" || len(gate.RouteAudit) != 1 {
		t.Fatalf("expected ready capability to be allowed with route audit evidence: %#v", gate)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/gate?capability=gated.disabled", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if err := json.NewDecoder(w.Body).Decode(&gate); err != nil {
		t.Fatalf("decode disabled: %v", err)
	}
	if gate.Allowed || gate.Action != "enable" || gate.Reason != "capability is provided only by disabled packs" {
		t.Fatalf("expected disabled capability to be blocked with enable action: %#v", gate)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/gate?capability=gated.mismatch", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	gate = packruntime.CapabilityGateReport{}
	if err := json.NewDecoder(w.Body).Decode(&gate); err != nil {
		t.Fatalf("decode mismatch: %v", err)
	}
	if gate.Allowed || gate.Action != "use" || gate.Reason != "capability pack has backend route audit issues" || len(gate.RouteAudit) != 1 || gate.RouteAudit[0].Status != "method-mismatch" {
		t.Fatalf("expected route audit mismatch to block capability: %#v", gate)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/gate?capability=gated.missing", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	gate = packruntime.CapabilityGateReport{}
	if err := json.NewDecoder(w.Body).Decode(&gate); err != nil {
		t.Fatalf("decode missing: %v", err)
	}
	if gate.Allowed || gate.Action != "install" || gate.Reason != "capability is not provided by any installed pack" {
		t.Fatalf("expected missing capability to be blocked with install action: %#v", gate)
	}
}

func TestPackCapabilityPlanAggregatesWorkflowPreflight(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestPackManifest(t, filepath.Join(sourceDir, "plan-missing-pack", packruntime.ManifestFileName), packruntime.Manifest{
		ID:           "yunque.pack.plan-missing",
		Name:         "Plan Missing Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"plan.missing"},
			Routes:       []string{"/v1/plan/missing"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodPost, Path: "/v1/plan/missing"}},
		},
		Frontend: packruntime.FrontendManifest{
			Menus:  []packruntime.FrontendMenu{{Key: "plan-missing", Label: "Plan Missing", Path: "/packs/plan-missing"}},
			Routes: []packruntime.FrontendRoute{{Path: "/packs/plan-missing", Component: "PlanMissingPackPage"}},
		},
		SDK: packruntime.SDKManifest{TypeScript: "yunque-client/plan-missing"},
		Distribution: packruntime.DistributionManifest{
			ManifestURL: "https://packs.yunque.local/plan-missing/pack.json",
			PackageURL:  "https://packs.yunque.local/plan-missing/plan-missing-0.1.0.tgz",
			FrontendURL: "https://packs.yunque.local/plan-missing/frontend/remoteEntry.js",
			SHA256:      strings.Repeat("c", 64),
			SizeBytes:   2048,
		},
		Update: packruntime.UpdateManifest{Rollback: true},
	})
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.plan-ready",
		Name:         "Plan Ready Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"plan.ready"},
			Routes:       []string{"/v1/plan/ready"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodGet, Path: "/v1/plan/ready"}},
		},
		SDK: packruntime.SDKManifest{TypeScript: "yunque-client/plan-ready"},
	}, "test")
	if err != nil {
		t.Fatalf("Install ready: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.plan-disabled",
		Name:         "Plan Disabled Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"plan.disabled"},
			Routes:       []string{"/v1/plan/disabled"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodGet, Path: "/v1/plan/disabled"}},
		},
		SDK: packruntime.SDKManifest{TypeScript: "yunque-client/plan-disabled"},
	}, "test")
	if err != nil {
		t.Fatalf("Install disabled: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.plan-audit",
		Name:         "Plan Audit Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"plan.audit"},
			Routes:       []string{"/v1/plan/audit"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodPost, Path: "/v1/plan/audit"}},
		},
		SDK: packruntime.SDKManifest{TypeScript: "yunque-client/plan-audit"},
	}, "test")
	if err != nil {
		t.Fatalf("Install audit: %v", err)
	}
	readyModule := testBackendPackModule{
		id: "yunque.pack.plan-ready",
		routes: []packruntime.BackendRoute{{
			Method:  http.MethodGet,
			Path:    "/v1/plan/ready",
			Handler: func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]any{"ok": true}) },
		}},
	}
	auditModule := testBackendPackModule{
		id: "yunque.pack.plan-audit",
		routes: []packruntime.BackendRoute{{
			Method:  http.MethodGet,
			Path:    "/v1/plan/audit",
			Handler: func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]any{"ok": true}) },
		}},
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry, BackendPacks: []packruntime.BackendModule{readyModule, auditModule}})
	missingSource := filepath.Join(t.TempDir(), "missing-plan-catalog")
	gw.SetPackCatalogSources([]string{sourceDir, missingSource})
	tenant := tenants.Register("pack-capability-plan")

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/plan?capability=plan.ready&capability=plan.disabled&capability=plan.audit&capability=plan.missing", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var plan packruntime.CapabilityPlanReport
	if err := json.NewDecoder(w.Body).Decode(&plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if plan.Allowed || plan.Action != "install" || plan.AllowedCount != 1 || plan.BlockedCount != 3 {
		t.Fatalf("expected mixed plan to require install with three blockers: %#v", plan)
	}
	if plan.UseCount != 2 || plan.EnableCount != 1 || plan.InstallCount != 1 || plan.RouteAuditIssueCount != 1 {
		t.Fatalf("unexpected plan counts: %#v", plan)
	}
	if len(plan.RequiredPacks) != 1 || plan.RequiredPacks[0].PackID != "yunque.pack.plan-ready" {
		t.Fatalf("expected ready pack in required packs: %#v", plan.RequiredPacks)
	}
	if len(plan.EnablePacks) != 1 || plan.EnablePacks[0].PackID != "yunque.pack.plan-disabled" {
		t.Fatalf("expected disabled pack in enable packs: %#v", plan.EnablePacks)
	}
	if len(plan.InstallCapabilities) != 1 || plan.InstallCapabilities[0] != "plan.missing" {
		t.Fatalf("expected missing capability to require install: %#v", plan.InstallCapabilities)
	}
	if len(plan.CatalogInstallHints) != 1 || plan.CatalogInstallHints[0].Manifest.ID != "yunque.pack.plan-missing" {
		t.Fatalf("expected missing capability to include installable catalog hint: %#v", plan.CatalogInstallHints)
	}
	if len(plan.CatalogDownloadHints) != 1 || !plan.CatalogDownloadHints[0].Downloadable {
		t.Fatalf("expected missing capability to include downloadable catalog hint: %#v", plan.CatalogDownloadHints)
	}
	if len(plan.CatalogSourceReports) != 2 || plan.CatalogSourceReports[0].Source != sourceDir || !plan.CatalogSourceReports[0].OK || plan.CatalogSourceReports[0].ManifestCount != 1 || plan.CatalogSourceReports[0].MatchedEntries != 1 {
		t.Fatalf("expected capability plan to expose successful catalog source diagnostics: %#v", plan.CatalogSourceReports)
	}
	if plan.CatalogSourceReports[1].Source != missingSource || plan.CatalogSourceReports[1].OK || len(plan.CatalogSourceReports[1].Errors) != 1 {
		t.Fatalf("expected capability plan to expose missing catalog source diagnostics: %#v", plan.CatalogSourceReports)
	}
	if len(plan.RouteAuditIssues) != 1 || plan.RouteAuditIssues[0].Status != "method-mismatch" {
		t.Fatalf("expected route audit issue to be surfaced: %#v", plan.RouteAuditIssues)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/plan?capabilities=plan.ready,plan.ready", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	plan = packruntime.CapabilityPlanReport{}
	if err := json.NewDecoder(w.Body).Decode(&plan); err != nil {
		t.Fatalf("decode deduped plan: %v", err)
	}
	if !plan.Allowed || plan.Action != "use" || len(plan.Capabilities) != 1 || len(plan.Gates) != 1 {
		t.Fatalf("expected deduped ready plan to be allowed: %#v", plan)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/packs/capabilities/plan", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected empty plan query to be 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPackBackendRouteAuditComparesManifestAndMountedRoutes(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.audit",
		Name:         "Audit Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{"/v1/audit/ok", "/v1/audit/missing", "/v1/audit/mismatch"},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/audit/ok", Description: "ok route"},
				{Method: http.MethodPost, Path: "/v1/audit/missing", Description: "missing route"},
				{Method: http.MethodPost, Path: "/v1/audit/mismatch", Description: "mismatch route"},
			},
		},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	module := testBackendPackModule{
		id: "yunque.pack.audit",
		routes: []packruntime.BackendRoute{
			{Method: http.MethodGet, Path: "/v1/audit/ok", Handler: func(w http.ResponseWriter, r *http.Request) {}},
			{Method: http.MethodGet, Path: "/v1/audit/mismatch", Handler: func(w http.ResponseWriter, r *http.Request) {}},
			{Method: http.MethodGet, Path: "/v1/audit/extra", Handler: func(w http.ResponseWriter, r *http.Request) {}},
		},
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry, BackendPacks: []packruntime.BackendModule{module}})
	tenant := tenants.Register("backend-route-audit")

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/backend-route-audit", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var report packruntime.BackendRouteAuditReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if report.Packs != 1 || report.EnabledPacks != 1 || report.MountedModules != 1 {
		t.Fatalf("unexpected report summary: %#v", report)
	}
	if report.OKRoutes != 1 || report.MissingRoutes != 1 || report.MethodMismatches != 1 || report.UndeclaredRoutes != 1 {
		t.Fatalf("expected ok/missing/mismatch/undeclared counts, got %#v", report)
	}
	statusByPath := map[string]string{}
	for _, entry := range report.Entries {
		statusByPath[entry.Path] = entry.Status
	}
	if statusByPath["/v1/audit/ok"] != "ok" || statusByPath["/v1/audit/missing"] != "missing" || statusByPath["/v1/audit/mismatch"] != "method-mismatch" || statusByPath["/v1/audit/extra"] != "undeclared" {
		t.Fatalf("unexpected audit entries: %#v", report.Entries)
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

func TestPackStudioPlanBuildsReadOnlyPlanForInstalledPack(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           "yunque.pack.computer-use",
		Name:         "Computer Use",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Description:  "Generate computer use plans.",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"computer.use.plan"},
			Permissions:  []string{"computer:plan", "browser:read"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodPost, Path: "/v1/computer-use/plan", Description: "Plan computer use"}},
		},
		Frontend: packruntime.FrontendManifest{
			Menus:  []packruntime.FrontendMenu{{Key: "computer-use", Label: "Computer Use", Path: "/packs/computer-use"}},
			Routes: []packruntime.FrontendRoute{{Path: "/packs/computer-use", Component: "ComputerUsePackPage", Title: "Computer Use"}},
			Assets: packruntime.FrontendAssets{Type: packruntime.FrontendAssetsTypeIframeBundle, Entry: "index.html"},
		},
		Update: packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	tenant := tenants.Register("pack-studio")

	body := bytes.NewBufferString(`{"pack_id":"yunque.pack.computer-use","goal":"补齐结果区和授权说明"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/packs/studio/plan", body)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var report packruntime.PackStudioPlanReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if report.PackID != "yunque.pack.computer-use" || !report.Installed || !report.Enabled || report.RiskLevel != "high" {
		t.Fatalf("unexpected studio report identity/risk: %#v", report)
	}
	if !slices.Contains(report.Surfaces, "iframe-bundle") || !slices.Contains(report.Surfaces, "backend") {
		t.Fatalf("expected iframe/backend surfaces, got %#v", report.Surfaces)
	}
	if !strings.Contains(report.DiffPreview, "补齐结果区和授权说明") || !strings.Contains(report.XiaoyuPrompt, "当前不执行本机控制") {
		t.Fatalf("expected diff and prompt to include goal/computer-use boundary: diff=%s prompt=%s", report.DiffPreview, report.XiaoyuPrompt)
	}
	if len(report.PackageSteps) == 0 || !strings.Contains(strings.Join(report.Guarded, " "), "不直接修改已签名或已安装包") {
		t.Fatalf("expected package steps and guarded boundary: %#v", report)
	}
}

func TestPackStudioPlanCanUseCatalogOrRequestManifest(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestPackManifest(t, filepath.Join(sourceDir, "studio-pack", packruntime.ManifestFileName), packruntime.Manifest{
		ID:           "yunque.pack.studio-catalog",
		Name:         "Studio Catalog",
		Version:      "0.2.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"studio.catalog"}, Permissions: []string{"network:read"}},
		Frontend:     packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "studio-catalog", Label: "Studio Catalog", Path: "/packs/studio-catalog"}}},
	})
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{})
	gw.SetPackCatalogSources([]string{sourceDir})
	tenant := tenants.Register("pack-studio-catalog")

	req := httptest.NewRequest(http.MethodPost, "/v1/packs/studio/plan", bytes.NewBufferString(`{"pack_id":"yunque.pack.studio-catalog"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("catalog status=%d body=%s", w.Code, w.Body.String())
	}
	var catalogReport packruntime.PackStudioPlanReport
	if err := json.NewDecoder(w.Body).Decode(&catalogReport); err != nil {
		t.Fatalf("decode catalog report: %v", err)
	}
	if catalogReport.Installed || catalogReport.Enabled || catalogReport.Source != sourceDir || catalogReport.RiskLevel != "medium" {
		t.Fatalf("unexpected catalog report: %#v", catalogReport)
	}

	requestManifest := `{"manifest":{"id":"yunque.pack.request-only","name":"Request Only","version":"0.1.0","optional":true,"backend":{"capabilities":["request.only"]},"frontend":{}},"goal":"只读分析"}`
	req = httptest.NewRequest(http.MethodPost, "/v1/packs/studio/plan", bytes.NewBufferString(requestManifest))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("request manifest status=%d body=%s", w.Code, w.Body.String())
	}
	var manifestReport packruntime.PackStudioPlanReport
	if err := json.NewDecoder(w.Body).Decode(&manifestReport); err != nil {
		t.Fatalf("decode manifest report: %v", err)
	}
	if manifestReport.Source != "request-manifest" || manifestReport.PackID != "yunque.pack.request-only" || manifestReport.Installed {
		t.Fatalf("unexpected request manifest report: %#v", manifestReport)
	}
}

func TestPackStudioInspectYqpackIsReadOnly(t *testing.T) {
	srcDir := t.TempDir()
	manifest := packruntime.Manifest{
		ID:           "yunque.pack.inspect-http",
		Name:         "Inspect HTTP",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Capabilities: []string{"inspect.http"},
			Permissions:  []string{"network:read"},
			RouteSpecs:   []packruntime.BackendRouteSpec{{Method: http.MethodGet, Path: "/v1/inspect-http"}},
		},
		Frontend: packruntime.FrontendManifest{
			Menus:  []packruntime.FrontendMenu{{Key: "inspect-http", Label: "Inspect", Path: "/packs/inspect-http"}},
			Routes: []packruntime.FrontendRoute{{Path: "/packs/inspect-http", Component: "InspectHTTPPackPage"}},
			Assets: packruntime.FrontendAssets{Type: packruntime.FrontendAssetsTypeIframeBundle, Entry: "index.html"},
		},
	}
	writeTestPackManifest(t, filepath.Join(srcDir, packruntime.ManifestFileName), manifest)
	if err := os.MkdirAll(filepath.Join(srcDir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "frontend", "index.html"), []byte("<main>Inspect</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgPath := filepath.Join(t.TempDir(), "inspect-http.yqpack")
	sha, err := packruntime.PackToYqpack(srcDir, pkgPath)
	if err != nil {
		t.Fatalf("PackToYqpack: %v", err)
	}
	yqpackBytes, err := os.ReadFile(pkgPath)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	gw, tenants := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	tenant := tenants.Register("pack-studio-inspect")

	body := bytes.NewBufferString(`{"package_path":` + strconv.Quote(pkgPath) + `,"sha256":"` + sha + `","goal":"检查真实包内容"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/packs/studio/inspect", body)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("local inspect status=%d body=%s", w.Code, w.Body.String())
	}
	var report packruntime.YqpackInspectReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode local report: %v", err)
	}
	if report.Manifest.ID != manifest.ID || !report.SHA256Match || report.EntryCount == 0 || report.Plan.PackID != manifest.ID {
		t.Fatalf("unexpected local inspect report: %#v", report)
	}
	if _, ok := registry.Get(manifest.ID); ok {
		t.Fatal("inspect must not install the yqpack")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(yqpackBytes)
	}))
	defer srv.Close()
	body = bytes.NewBufferString(`{"package_url":` + strconv.Quote(srv.URL+"/inspect-http.yqpack") + `,"sha256":"` + strings.Repeat("0", 64) + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/packs/studio/inspect", body)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("remote inspect status=%d body=%s", w.Code, w.Body.String())
	}
	report = packruntime.YqpackInspectReport{}
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode remote report: %v", err)
	}
	if report.SHA256Match || len(report.Warnings) == 0 || !strings.Contains(report.Warnings[0], "sha256 mismatch") {
		t.Fatalf("expected sha mismatch warning for remote inspect: %#v", report)
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
