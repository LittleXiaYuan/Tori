package cognisdk

import (
	"strings"
	"testing"
)

func TestDiffPackBundlesReportsAddRemoveChangeAndToggle(t *testing.T) {
	baseCompanion := XiaoyuCompanionPack()
	oldOnly := PackManifest{ID: "old-only", Version: "0.1.0", Type: "cogni"}
	from, err := NewPackBundle("before", []PackManifest{baseCompanion, oldOnly}, []string{PackXiaoyuCompanion, "old-only"})
	if err != nil {
		t.Fatalf("from bundle: %v", err)
	}

	changedCompanion := baseCompanion
	changedCompanion.Version = "0.2.0"
	newOnly := PackManifest{ID: "new-only", Version: "0.1.0", Type: "work"}
	to, err := NewPackBundle("after", []PackManifest{changedCompanion, newOnly}, []string{"new-only"})
	if err != nil {
		t.Fatalf("to bundle: %v", err)
	}

	diff, err := DiffPackBundles(from, to)
	if err != nil {
		t.Fatalf("diff bundles: %v", err)
	}
	if len(diff.AddedPacks) != 1 || diff.AddedPacks[0].ID != "new-only" {
		t.Fatalf("added packs mismatch: %#v", diff.AddedPacks)
	}
	if len(diff.RemovedPacks) != 1 || diff.RemovedPacks[0].ID != "old-only" {
		t.Fatalf("removed packs mismatch: %#v", diff.RemovedPacks)
	}
	if len(diff.ChangedPacks) != 1 || diff.ChangedPacks[0].ID != PackXiaoyuCompanion || diff.ChangedPacks[0].Reason != "version changed" {
		t.Fatalf("changed packs mismatch: %#v", diff.ChangedPacks)
	}
	if !containsString(diff.EnabledPacks, "new-only") {
		t.Fatalf("enabled pack missing: %#v", diff.EnabledPacks)
	}
	if !containsString(diff.DisabledPacks, PackXiaoyuCompanion) || !containsString(diff.DisabledPacks, "old-only") {
		t.Fatalf("disabled packs mismatch: %#v", diff.DisabledPacks)
	}
}

func TestRenderPackBundleDiffMarkdown(t *testing.T) {
	from, err := NewPackBundle("before", []PackManifest{XiaoyuCompanionPack()}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("from bundle: %v", err)
	}
	to, err := NewPackBundle("after", []PackManifest{XiaoyuCompanionPack(), YunqueWorkPack()}, []string{PackXiaoyuCompanion, PackYunqueWork})
	if err != nil {
		t.Fatalf("to bundle: %v", err)
	}
	diff, err := DiffPackBundles(from, to)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	markdown := RenderPackBundleDiffMarkdown(diff)
	for _, want := range []string{"Cogni Pack Bundle Diff", "Added Packs", PackYunqueWork, "Enabled Packs"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("rendered diff missing %q:\n%s", want, markdown)
		}
	}
}

func TestDiffPackBundlesRejectsInvalidBundle(t *testing.T) {
	valid, err := NewPackBundle("valid", []PackManifest{XiaoyuCompanionPack()}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("valid bundle: %v", err)
	}
	invalid := valid
	invalid.EnabledPacks = []string{"missing"}
	if _, err := DiffPackBundles(valid, invalid); err == nil {
		t.Fatal("expected invalid target bundle to fail")
	}
}
