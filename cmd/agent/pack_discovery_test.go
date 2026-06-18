package main

import (
	"slices"
	"testing"
)

// TestDiscoverBuiltinPackManifestPaths verifies Phase-A brick 1: builtin packs
// are sourced from on-disk manifests under packs/official, the manual-install
// dlc-demo reference pack stays excluded, and the new control-plane Pro pack
// (Phase A2) is picked up.
func TestDiscoverBuiltinPackManifestPaths(t *testing.T) {
	paths := discoverBuiltinPackManifestPaths()
	if len(paths) == 0 {
		t.Fatal("expected to discover builtin pack manifests, got none")
	}

	want := []string{
		"packs/official/backup-pack/pack.json",
		"packs/official/connectors-pack/pack.json",
		"packs/official/control-plane-pack/pack.json",
		"packs/official/cost-pack/pack.json",
		"packs/official/forks-pack/pack.json",
		"packs/official/inner-life-pack/pack.json",
		"packs/official/mcp-dispatch-pack/pack.json",
		"packs/official/work-pack/pack.json",
		"packs/official/skills-pack/pack.json",
		"packs/official/memory-pack/pack.json",
		"packs/official/knowledge-pack/pack.json",
		"packs/official/notifications-pack/pack.json",
		"packs/official/orchestrator-pack/pack.json",
		"packs/official/reflection-pack/pack.json",
		"packs/official/scheduler-pack/pack.json",
		"packs/official/cogni-console-pack/pack.json",
		"packs/official/workspace-pack/pack.json",
	}
	for _, w := range want {
		if !slices.Contains(paths, w) {
			t.Errorf("discovered paths missing %q; got %v", w, paths)
		}
	}

	if slices.Contains(paths, "packs/official/dlc-demo-pack/pack.json") {
		t.Error("dlc-demo-pack must stay excluded from auto-discovery")
	}

	// Paths must be deterministically sorted.
	if !slices.IsSorted(paths) {
		t.Errorf("discovered paths must be sorted, got %v", paths)
	}
}

// TestControlPlanePackManifest verifies the control-plane manifest loads and
// validates. Since the governance route slice was migrated into the pack, it is
// now an always-on core surface: default-enabled (so audit/trust/usage stay
// available out of the box) and declaring its backend routes + ops menus.
func TestControlPlanePackManifest(t *testing.T) {
	manifest, err := loadBuiltinPackManifest("packs/official/control-plane-pack/pack.json")
	if err != nil {
		t.Fatalf("control-plane manifest failed to load: %v", err)
	}
	if manifest.ID != "yunque.pack.control-plane" {
		t.Errorf("unexpected id %q", manifest.ID)
	}
	if manifest.DefaultState != "enabled" {
		t.Errorf("control-plane pack must be default-enabled (always-on governance), got %q", manifest.DefaultState)
	}
	if len(manifest.Backend.Routes) == 0 {
		t.Error("control-plane pack should declare its migrated governance backend routes")
	}
	if len(manifest.Frontend.Menus) == 0 {
		t.Error("control-plane pack should declare frontend menus for the ops surfaces")
	}
}
