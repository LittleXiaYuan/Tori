package statepack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStateRoutesConsistentWithManifest(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "..", "packs", "official", "state-pack", "pack.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Backend struct {
			Routes []string `json:"routes"`
		} `json:"backend"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	manifestRoutes := map[string]bool{}
	for _, p := range manifest.Backend.Routes {
		manifestRoutes[p] = true
	}
	if len(manifestRoutes) == 0 {
		t.Fatal("manifest declares no backend routes")
	}

	h := New(nil)
	seen := map[string]bool{}
	for _, rt := range h.Routes() {
		if seen[rt.Path] {
			t.Fatalf("duplicate route %q", rt.Path)
		}
		seen[rt.Path] = true
		if rt.Handler == nil {
			t.Fatalf("route %q has nil handler", rt.Path)
		}
		if len(rt.Methods) == 0 {
			t.Fatalf("route %q declares no methods", rt.Path)
		}
		if !manifestRoutes[rt.Path] {
			t.Fatalf("route %q not in manifest backend.routes", rt.Path)
		}
	}
	for p := range manifestRoutes {
		if !seen[p] {
			t.Fatalf("manifest route %q missing from Routes()", p)
		}
	}
}
