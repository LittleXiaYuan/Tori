package packruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDlcDemoPackArtifact verifies the reference DLC demo pack
// (packs/official/dlc-demo-pack) is a well-formed iframe-bundle pack whose
// frontend speaks the bridge ABI. It guards the M4 deliverable end-to-end at the
// artifact level without requiring a compiled module.wasm.
func TestDlcDemoPackArtifact(t *testing.T) {
	root := filepath.Join("..", "..", "packs", "official", "dlc-demo-pack")

	raw, err := os.ReadFile(filepath.Join(root, "pack.json"))
	if err != nil {
		t.Fatalf("read pack.json: %v", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse pack.json: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("demo pack manifest invalid: %v", err)
	}
	if m.Frontend.Assets.Type != FrontendAssetsTypeIframeBundle {
		t.Fatalf("assets.type = %q, want %q", m.Frontend.Assets.Type, FrontendAssetsTypeIframeBundle)
	}
	if m.Frontend.Assets.Entry == "" {
		t.Fatal("iframe-bundle pack must declare frontend.assets.entry")
	}
	if len(m.Backend.RouteSpecs) == 0 {
		t.Fatal("demo pack should declare at least one backend route for backend.call")
	}

	entry := m.Frontend.Assets.Entry
	html, err := os.ReadFile(filepath.Join(root, "frontend", entry))
	if err != nil {
		t.Fatalf("read frontend/%s: %v", entry, err)
	}
	body := string(html)
	for _, want := range []string{"host.handshake", "backend.call", "postMessage"} {
		if !strings.Contains(body, want) {
			t.Errorf("frontend %s should exercise bridge method %q", entry, want)
		}
	}
}
