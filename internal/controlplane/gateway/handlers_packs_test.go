package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"yunque-agent/pkg/packruntime"
)

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
	manifest := packruntime.Manifest{
		ID:           "yunque.pack.remote-backup",
		Name:         "Remote Backup Pack",
		Version:      "0.2.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Capabilities: []string{"backup.info"}, Routes: []string{"/v1/backup/info"}},
		Frontend:     packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "backup", Label: "备份恢复", Path: "/packs/backup"}}},
		SDK:          packruntime.SDKManifest{TypeScript: "yunque-client/backup"},
		Update:       packruntime.UpdateManifest{Rollback: true},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pack.json" {
			t.Fatalf("unexpected manifest URL path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(manifest)
	}))
	defer srv.Close()
	gw := NewFromConfig(GatewayConfig{Packs: registry})

	body := bytes.NewBufferString(`{"manifest_url":` + strconv.Quote(srv.URL+"/pack.json") + `}`)
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
