package cognisdk

import (
	"strings"
	"testing"
)

func TestPackManagerListEnableDisable(t *testing.T) {
	pm := NewPackManager(BuiltinPacks()...)

	statuses := pm.List()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 built-in packs, got %d", len(statuses))
	}
	if statuses[0].ID != PackXiaoyuCompanion || statuses[1].ID != PackYunqueWork {
		t.Fatalf("packs are not sorted by id: %#v", statuses)
	}

	if err := pm.Disable(PackXiaoyuCompanion); err != nil {
		t.Fatalf("disable companion pack: %v", err)
	}
	merged := pm.Merge()
	if containsString(merged.PackIDs, PackXiaoyuCompanion) {
		t.Fatalf("disabled pack still appears in merge: %#v", merged.PackIDs)
	}

	if err := pm.Enable(PackXiaoyuCompanion); err != nil {
		t.Fatalf("enable companion pack: %v", err)
	}
	merged = pm.Merge()
	if !containsString(merged.PackIDs, PackXiaoyuCompanion) {
		t.Fatalf("enabled pack missing from merge: %#v", merged.PackIDs)
	}
}

func TestValidatePackRejectsBadManifest(t *testing.T) {
	err := ValidatePack(PackManifest{ID: "bad", Version: "v1", Type: "cogni"})
	if err == nil {
		t.Fatal("expected invalid semver to fail")
	}
	if !strings.Contains(err.Error(), "semver") {
		t.Fatalf("expected semver error, got %v", err)
	}
}

func TestMergeMustAvoidIsUnionNotWeakening(t *testing.T) {
	pm := NewPackManager(
		PackManifest{
			ID:      "a-pack",
			Version: "0.1.0",
			Type:    "cogni",
			Boundary: BoundaryPolicy{
				MustAvoid: []string{"never promise permanence"},
			},
		},
		PackManifest{
			ID:      "b-pack",
			Version: "0.1.0",
			Type:    "work",
			Boundary: BoundaryPolicy{
				MustAvoid: []string{"skip confirmation"},
			},
		},
	)

	merged := pm.Merge()
	if !containsString(merged.Boundary.MustAvoid, "never promise permanence") {
		t.Fatalf("first boundary was weakened: %#v", merged.Boundary.MustAvoid)
	}
	if !containsString(merged.Boundary.MustAvoid, "skip confirmation") {
		t.Fatalf("second boundary missing: %#v", merged.Boundary.MustAvoid)
	}
}
