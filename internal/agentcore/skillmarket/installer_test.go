package skillmarket

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestInstaller(t *testing.T) (*Installer, string) {
	t.Helper()
	dir := t.TempDir()
	market := NewMarket(filepath.Join(dir, "market.json"))
	inst := NewInstaller(filepath.Join(dir, "skills"), nil, nil, market)
	return inst, dir
}

func seedInstalledSkill(t *testing.T, inst *Installer, slug, version string) {
	t.Helper()
	inst.mu.Lock()
	defer inst.mu.Unlock()

	inst.installed[slug] = &InstalledSkill{
		Slug:        slug,
		Name:        slug,
		Version:     version,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
		Enabled:     true,
	}
	inst.saveInstalled()

	// Write version to disk
	dir := filepath.Join(inst.dataDir, slug)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+slug+" v"+version), 0644)
	meta := map[string]string{"name": slug, "version": version, "description": "test skill"}
	data, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(dir, "meta.json"), data, 0644)

	// Archive version
	vDir := filepath.Join(dir, "versions", version)
	os.MkdirAll(vDir, 0755)
	os.WriteFile(filepath.Join(vDir, "SKILL.md"), []byte("# "+slug+" v"+version), 0644)
	os.WriteFile(filepath.Join(vDir, "meta.json"), data, 0644)
}

func TestListVersions(t *testing.T) {
	inst, _ := setupTestInstaller(t)
	seedInstalledSkill(t, inst, "test-skill", "1.0.0")

	// Add another version archive
	vDir := filepath.Join(inst.dataDir, "test-skill", "versions", "0.9.0")
	os.MkdirAll(vDir, 0755)
	os.WriteFile(filepath.Join(vDir, "SKILL.md"), []byte("old"), 0644)

	versions, err := inst.ListVersions("test-skill")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}

	// Verify current is marked
	currentFound := false
	for _, v := range versions {
		if v.Version == "1.0.0" && v.Current {
			currentFound = true
		}
	}
	if !currentFound {
		t.Error("current version not marked")
	}
}

func TestListVersionsNotInstalled(t *testing.T) {
	inst, _ := setupTestInstaller(t)
	_, err := inst.ListVersions("nope")
	if err == nil {
		t.Error("expected error for uninstalled skill")
	}
}

func TestRollback(t *testing.T) {
	inst, _ := setupTestInstaller(t)
	seedInstalledSkill(t, inst, "test-skill", "2.0.0")

	// Add old version archive
	vDir := filepath.Join(inst.dataDir, "test-skill", "versions", "1.0.0")
	os.MkdirAll(vDir, 0755)
	oldMeta, _ := json.Marshal(map[string]string{
		"name": "test-skill", "version": "1.0.0", "description": "old version",
	})
	os.WriteFile(filepath.Join(vDir, "SKILL.md"), []byte("# old"), 0644)
	os.WriteFile(filepath.Join(vDir, "meta.json"), oldMeta, 0644)

	// Rollback to 1.0.0
	err := inst.Rollback("test-skill", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	// Verify version changed
	info, ok := inst.GetInstalled("test-skill")
	if !ok {
		t.Fatal("skill not found after rollback")
	}
	if info.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", info.Version)
	}

	// Verify SKILL.md was restored
	content, err := os.ReadFile(filepath.Join(inst.dataDir, "test-skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# old" {
		t.Errorf("expected restored content, got %q", string(content))
	}
}

func TestRollbackSameVersion(t *testing.T) {
	inst, _ := setupTestInstaller(t)
	seedInstalledSkill(t, inst, "test-skill", "1.0.0")

	err := inst.Rollback("test-skill", "1.0.0")
	if err == nil {
		t.Error("expected error for same version rollback")
	}
}

func TestRollbackMissingVersion(t *testing.T) {
	inst, _ := setupTestInstaller(t)
	seedInstalledSkill(t, inst, "test-skill", "1.0.0")

	err := inst.Rollback("test-skill", "9.9.9")
	if err == nil {
		t.Error("expected error for missing version archive")
	}
}

func TestRollbackNotInstalled(t *testing.T) {
	inst, _ := setupTestInstaller(t)

	err := inst.Rollback("nope", "1.0.0")
	if err == nil {
		t.Error("expected error for uninstalled skill")
	}
}

func TestOnInstallCallbackOnRollback(t *testing.T) {
	inst, _ := setupTestInstaller(t)
	seedInstalledSkill(t, inst, "test-skill", "2.0.0")

	// Add old version
	vDir := filepath.Join(inst.dataDir, "test-skill", "versions", "1.0.0")
	os.MkdirAll(vDir, 0755)
	os.WriteFile(filepath.Join(vDir, "SKILL.md"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(vDir, "meta.json"), []byte(`{}`), 0644)

	called := false
	inst.SetOnInstall(func(slug string) { called = true })

	err := inst.Rollback("test-skill", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("onInstall callback not called after rollback")
	}
}

func TestInstalledPersistence(t *testing.T) {
	inst, dir := setupTestInstaller(t)
	seedInstalledSkill(t, inst, "persist-test", "1.0.0")

	// Create new installer with same dir
	market := NewMarket(filepath.Join(dir, "market.json"))
	inst2 := NewInstaller(filepath.Join(dir, "skills"), nil, nil, market)

	info, ok := inst2.GetInstalled("persist-test")
	if !ok {
		t.Fatal("installed skill not persisted")
	}
	if info.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", info.Version)
	}
}

func TestSetEnabled(t *testing.T) {
	inst, _ := setupTestInstaller(t)
	seedInstalledSkill(t, inst, "test-skill", "1.0.0")

	err := inst.SetEnabled("test-skill", false)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := inst.GetInstalled("test-skill")
	if info.Enabled {
		t.Error("expected disabled")
	}

	err = inst.SetEnabled("test-skill", true)
	if err != nil {
		t.Fatal(err)
	}
	info, _ = inst.GetInstalled("test-skill")
	if !info.Enabled {
		t.Error("expected enabled")
	}
}

func TestIsInstalled(t *testing.T) {
	inst, _ := setupTestInstaller(t)
	seedInstalledSkill(t, inst, "test-skill", "1.0.0")

	if !inst.IsInstalled("test-skill") {
		t.Error("expected installed")
	}
	if inst.IsInstalled("nope") {
		t.Error("expected not installed")
	}
}
