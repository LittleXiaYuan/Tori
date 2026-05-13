package packruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func backupManifest(version string) Manifest {
	return Manifest{
		ID:           "yunque.pack.backup",
		Name:         "Backup Pack",
		Version:      version,
		Description:  "Backup info/export/import as an optional pack.",
		RequiresCore: ">=0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: BackendManifest{
			Capabilities: []string{"backup.info", "backup.export", "backup.import"},
			Routes:       []string{"/v1/backup/info", "/v1/backup/export", "/v1/backup/import"},
			RouteSpecs: []BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/backup/info", Description: "Read backup pack runtime status."},
				{Method: http.MethodGet, Path: "/v1/backup/export", Description: "Export a backup archive."},
				{Method: http.MethodPost, Path: "/v1/backup/import", Description: "Import backup payload data."},
			},
			Permissions: []string{"backup:read", "backup:write"},
		},
		Frontend: FrontendManifest{
			Menus:  []FrontendMenu{{Key: "backup", Label: "备份恢复", Path: "/packs/backup", Icon: "archive", Order: 90}},
			Routes: []FrontendRoute{{Path: "/packs/backup", Component: "backup/BackupPage", Title: "备份恢复"}},
			Assets: FrontendAssets{Type: "builtin", Entry: "backup/BackupPage"},
		},
		SDK: SDKManifest{TypeScript: "yunque-client/backup"},
		Distribution: DistributionManifest{
			ManifestURL: "https://packs.yunque.local/backup/pack.json",
			PackageURL:  "https://packs.yunque.local/backup/backup-pack-0.1.0.tgz",
			FrontendURL: "https://packs.yunque.local/backup/frontend/remoteEntry.js",
			SHA256:      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			SizeBytes:   4096,
		},
		Update: UpdateManifest{Channel: "stable", Rollback: true},
	}
}

func TestManifestValidateAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFileName)
	manifest := backupManifest("0.1.0")
	if err := SaveManifest(path, manifest); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if loaded.ID != manifest.ID || loaded.SDK.TypeScript != "yunque-client/backup" || len(loaded.Backend.RouteSpecs) != 3 {
		t.Fatalf("unexpected manifest roundtrip: %#v", loaded)
	}
	if loaded.Distribution.PackageURL == "" || loaded.Distribution.SHA256 == "" || loaded.Distribution.FrontendURL == "" {
		t.Fatalf("expected distribution metadata to roundtrip: %#v", loaded.Distribution)
	}
}

func TestManifestValidateRequiresChecksumForPackageURL(t *testing.T) {
	manifest := backupManifest("0.1.0")
	manifest.Distribution.SHA256 = ""
	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected distribution checksum validation error")
	}
}

func TestManifestValidateRejectsNegativeDistributionSize(t *testing.T) {
	manifest := backupManifest("0.1.0")
	manifest.Distribution.SizeBytes = -1
	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected distribution size validation error")
	}
}

func TestBackendManifestAllowsRouteUsesMethodAwareSpecs(t *testing.T) {
	backend := BackendManifest{
		Routes:     []string{"/v1/backup/import", "/v1/legacy/ping"},
		RouteSpecs: []BackendRouteSpec{{Method: http.MethodPost, Path: "/v1/backup/import"}},
	}
	if !backend.AllowsRoute(http.MethodPost, "/v1/backup/import") {
		t.Fatalf("expected POST import route to be allowed")
	}
	if backend.AllowsRoute(http.MethodGet, "/v1/backup/import") {
		t.Fatalf("expected GET import route to be rejected when routeSpecs declares POST")
	}
	if !backend.AllowsRoute(http.MethodGet, "/v1/legacy/ping") {
		t.Fatalf("expected legacy path-only route to remain compatible")
	}
}

func TestManifestValidateRequiresBackendRouteSpecMethodAndPath(t *testing.T) {
	manifest := backupManifest("0.1.0")
	manifest.Backend.RouteSpecs = []BackendRouteSpec{{Path: "/v1/backup/info"}}
	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected missing backend.routeSpecs method validation error")
	}
	manifest = backupManifest("0.1.0")
	manifest.Backend.RouteSpecs = []BackendRouteSpec{{Method: http.MethodGet, Path: "v1/backup/info"}}
	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected invalid backend.routeSpecs path validation error")
	}
}

func TestRegistryCachesDistributionPackage(t *testing.T) {
	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	payload := []byte("pack artifact payload")
	sha := sha256.Sum256(payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backup-pack-0.1.0.tgz" {
			t.Fatalf("unexpected package path: %s", r.URL.Path)
		}
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	manifest := backupManifest("0.1.0")
	manifest.Distribution.PackageURL = srv.URL + "/backup-pack-0.1.0.tgz"
	manifest.Distribution.SHA256 = hex.EncodeToString(sha[:])
	artifacts, err := registry.CacheDistribution(context.Background(), manifest)
	if err != nil {
		t.Fatalf("CacheDistribution: %v", err)
	}
	if artifacts == nil || artifacts.SHA256 != manifest.Distribution.SHA256 || artifacts.SizeBytes != int64(len(payload)) {
		t.Fatalf("unexpected artifacts: %#v", artifacts)
	}
	if !strings.Contains(artifacts.PackagePath, filepath.Join("artifacts", "yunque.pack.backup", "0.1.0")) {
		t.Fatalf("unexpected artifact path: %s", artifacts.PackagePath)
	}
	cached, err := os.ReadFile(artifacts.PackagePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(cached) != string(payload) {
		t.Fatalf("unexpected cached payload: %q", cached)
	}
}

func TestRegistryCacheDistributionRejectsSHA256Mismatch(t *testing.T) {
	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("different payload"))
	}))
	defer srv.Close()
	manifest := backupManifest("0.1.0")
	manifest.Distribution.PackageURL = srv.URL + "/backup-pack-0.1.0.tgz"
	manifest.Distribution.SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := registry.CacheDistribution(context.Background(), manifest); err == nil {
		t.Fatalf("expected sha256 mismatch")
	}
}

func TestRegistryPruneArtifactsRemovesUnreferencedFiles(t *testing.T) {
	dir := t.TempDir()
	registry, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	keepDir := filepath.Join(dir, "artifacts", "yunque.pack.backup", "0.1.0")
	oldDir := filepath.Join(dir, "artifacts", "yunque.pack.backup", "0.0.9")
	if err := os.MkdirAll(keepDir, 0o755); err != nil {
		t.Fatalf("MkdirAll keep: %v", err)
	}
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("MkdirAll old: %v", err)
	}
	keepPath := filepath.Join(keepDir, "backup-pack-0.1.0.tgz")
	previousPath := filepath.Join(keepDir, "backup-pack-0.0.10.tgz")
	oldPath := filepath.Join(oldDir, "backup-pack-0.0.9.tgz")
	for _, path := range []string{keepPath, previousPath, oldPath} {
		if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
	}
	manifest := backupManifest("0.1.0")
	pack, err := registry.InstallWithArtifacts(manifest, "test", &PackArtifacts{PackagePath: keepPath, SHA256: "keep", SizeBytes: 1, CachedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("InstallWithArtifacts: %v", err)
	}
	pack.PreviousArtifacts = &PackArtifacts{PackagePath: previousPath, SHA256: "previous", SizeBytes: 1, CachedAt: time.Now().UTC()}
	registry.snapshot.Packs[0] = pack
	report := registry.PruneArtifacts()
	if len(report.Errors) > 0 {
		t.Fatalf("unexpected prune errors: %#v", report.Errors)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected unreferenced artifact removed, stat err=%v", err)
	}
	for _, path := range []string{keepPath, previousPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected referenced artifact kept %s: %v", path, err)
		}
	}
	if len(report.Removed) != 1 || len(report.Kept) != 2 {
		t.Fatalf("unexpected prune report: %#v", report)
	}
}

func TestRegistryInstallEnableDisableAndRollback(t *testing.T) {
	dir := t.TempDir()
	registry, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	base := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	registry.now = func() time.Time { return base }
	installed, err := registry.Install(backupManifest("0.1.0"), "packs/examples/backup-pack")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if installed.Status != PackStatusEnabled {
		t.Fatalf("expected default enabled status, got %s", installed.Status)
	}
	if len(registry.Enabled()) != 1 {
		t.Fatalf("expected one enabled pack")
	}
	disabled, err := registry.Disable("yunque.pack.backup")
	if err != nil || disabled.Status != PackStatusDisabled {
		t.Fatalf("Disable: %v %#v", err, disabled)
	}
	baseArtifacts := &PackArtifacts{PackagePath: filepath.Join(dir, "base.tgz"), SHA256: "base-sha", SizeBytes: 10, CachedAt: base}
	if _, err := registry.InstallWithArtifacts(backupManifest("0.1.1"), "downloaded://backup-0.1.1", baseArtifacts); err != nil {
		t.Fatalf("Install artifacts base: %v", err)
	}
	updatedManifest := backupManifest("0.2.0")
	updatedManifest.DefaultState = "disabled"
	updatedArtifacts := &PackArtifacts{PackagePath: filepath.Join(dir, "updated.tgz"), SHA256: "updated-sha", SizeBytes: 20, CachedAt: base.Add(time.Minute)}
	updated, err := registry.InstallWithArtifacts(updatedManifest, "downloaded://backup-0.2.0", updatedArtifacts)
	if err != nil {
		t.Fatalf("Install update: %v", err)
	}
	if updated.Manifest.Version != "0.2.0" || updated.PreviousVersion != "0.1.1" {
		t.Fatalf("unexpected update state: %#v", updated)
	}
	if updated.Artifacts == nil || updated.Artifacts.SHA256 != "updated-sha" || updated.PreviousArtifacts == nil || updated.PreviousArtifacts.SHA256 != "base-sha" {
		t.Fatalf("expected current and previous artifacts to be recorded: %#v", updated)
	}
	rolledBack, err := registry.Rollback("yunque.pack.backup")
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolledBack.Manifest.Version != "0.1.1" || rolledBack.PreviousVersion != "0.2.0" {
		t.Fatalf("unexpected rollback state: %#v", rolledBack)
	}
	if rolledBack.Artifacts == nil || rolledBack.Artifacts.SHA256 != "base-sha" || rolledBack.PreviousArtifacts == nil || rolledBack.PreviousArtifacts.SHA256 != "updated-sha" {
		t.Fatalf("expected rollback to swap artifacts: %#v", rolledBack)
	}
}
