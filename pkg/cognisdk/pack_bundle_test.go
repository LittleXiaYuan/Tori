package cognisdk

import (
	"path/filepath"
	"testing"
)

func TestPackManagerExportBundleRoundTrip(t *testing.T) {
	pm := NewPackManager(BuiltinPacks()...)
	if err := pm.Disable(PackXiaoyuCompanion); err != nil {
		t.Fatalf("disable companion pack: %v", err)
	}

	bundle, err := pm.ExportBundle("work-only-bundle")
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}
	if bundle.ID != "work-only-bundle" {
		t.Fatalf("bundle id = %q", bundle.ID)
	}
	if len(bundle.Packs) != 2 {
		t.Fatalf("expected 2 packs in bundle, got %d", len(bundle.Packs))
	}
	if len(bundle.EnabledPacks) != 1 || bundle.EnabledPacks[0] != PackYunqueWork {
		t.Fatalf("enabled packs not preserved: %#v", bundle.EnabledPacks)
	}

	restored, err := NewPackManagerFromBundle(bundle)
	if err != nil {
		t.Fatalf("restore bundle: %v", err)
	}
	merged := restored.Merge()
	if containsString(merged.PackIDs, PackXiaoyuCompanion) {
		t.Fatalf("disabled pack was re-enabled: %#v", merged.PackIDs)
	}
	if !containsString(merged.PackIDs, PackYunqueWork) {
		t.Fatalf("enabled work pack missing: %#v", merged.PackIDs)
	}
}

func TestSaveAndLoadPackBundle(t *testing.T) {
	bundle, err := NewPackBundle("portable", []PackManifest{XiaoyuCompanionPack()}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("new bundle: %v", err)
	}
	path := filepath.Join(t.TempDir(), "portable.cogni-bundle.json")
	if err := SavePackBundle(bundle, path); err != nil {
		t.Fatalf("save bundle: %v", err)
	}
	loaded, err := LoadPackBundle(path)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if loaded.ID != bundle.ID || len(loaded.Packs) != 1 || loaded.Packs[0].ID != PackXiaoyuCompanion {
		t.Fatalf("loaded bundle mismatch: %#v", loaded)
	}
}

func TestValidatePackBundleRejectsMissingEnabledPack(t *testing.T) {
	bundle, err := NewPackBundle("bad", []PackManifest{XiaoyuCompanionPack()}, nil)
	if err != nil {
		t.Fatalf("new bundle: %v", err)
	}
	bundle.EnabledPacks = []string{"missing-pack"}
	if err := ValidatePackBundle(bundle); err == nil {
		t.Fatal("expected missing enabled pack to fail")
	}
}
