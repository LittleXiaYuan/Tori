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
			Permissions:  []string{"backup:read", "backup:write"},
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
	if loaded.ID != manifest.ID || loaded.SDK.TypeScript != "yunque-client/backup" {
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
	updatedManifest := backupManifest("0.2.0")
	updatedManifest.DefaultState = "disabled"
	updated, err := registry.Install(updatedManifest, "downloaded://backup-0.2.0")
	if err != nil {
		t.Fatalf("Install update: %v", err)
	}
	if updated.Manifest.Version != "0.2.0" || updated.PreviousVersion != "0.1.0" {
		t.Fatalf("unexpected update state: %#v", updated)
	}
	rolledBack, err := registry.Rollback("yunque.pack.backup")
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolledBack.Manifest.Version != "0.1.0" || rolledBack.PreviousVersion != "0.2.0" {
		t.Fatalf("unexpected rollback state: %#v", rolledBack)
	}
}
