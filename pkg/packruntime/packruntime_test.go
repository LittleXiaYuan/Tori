package packruntime

import (
	"path/filepath"
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
		SDK:    SDKManifest{TypeScript: "yunque-client/backup"},
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
