package packruntime

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func minimalManifest(id, version string) Manifest {
	return Manifest{
		ID:           id,
		Name:         "Test Pack",
		Version:      version,
		RequiresCore: ">=0.1.0",
		Optional:     true,
		DefaultState: "disabled",
	}
}

func TestPackToYqpackIsDeterministic(t *testing.T) {
	srcDir := t.TempDir()
	if err := SaveManifest(filepath.Join(srcDir, ManifestFileName), minimalManifest("yunque.pack.test", "0.1.0")); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	// Add a few files with mixed timestamps to confirm we strip them.
	if err := os.MkdirAll(filepath.Join(srcDir, "backend", "linux-amd64"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "backend", "linux-amd64", "binary"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}

	out1 := filepath.Join(t.TempDir(), "a.yqpack")
	out2 := filepath.Join(t.TempDir(), "b.yqpack")
	sha1, err := PackToYqpack(srcDir, out1)
	if err != nil {
		t.Fatalf("pack 1: %v", err)
	}
	sha2, err := PackToYqpack(srcDir, out2)
	if err != nil {
		t.Fatalf("pack 2: %v", err)
	}
	if sha1 != sha2 {
		t.Fatalf("non-deterministic: %s != %s", sha1, sha2)
	}
	if len(sha1) != 64 {
		t.Fatalf("sha256 hex length: got %d", len(sha1))
	}
}

func TestInstallFromYqpackRoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	if err := SaveManifest(filepath.Join(srcDir, ManifestFileName), minimalManifest("yunque.pack.test", "0.1.0")); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgPath := filepath.Join(t.TempDir(), "test.yqpack")
	sha, err := PackToYqpack(srcDir, pkgPath)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	pack, err := registry.InstallFromYqpack(pkgPath, InstallOptions{ExpectedSHA256: sha})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if pack.Manifest.ID != "yunque.pack.test" {
		t.Fatalf("unexpected id: %s", pack.Manifest.ID)
	}
	if pack.Artifacts == nil || pack.Artifacts.SHA256 != sha {
		t.Fatalf("artifacts not recorded: %#v", pack.Artifacts)
	}
	installedReadme := filepath.Join(registry.root, "installed", "yunque.pack.test-0.1.0", "README.md")
	if _, err := os.Stat(installedReadme); err != nil {
		t.Fatalf("README not extracted: %v", err)
	}
}

func TestInstallFromYqpackRejectsBadSHA(t *testing.T) {
	srcDir := t.TempDir()
	if err := SaveManifest(filepath.Join(srcDir, ManifestFileName), minimalManifest("yunque.pack.test", "0.1.0")); err != nil {
		t.Fatal(err)
	}
	pkgPath := filepath.Join(t.TempDir(), "test.yqpack")
	if _, err := PackToYqpack(srcDir, pkgPath); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.InstallFromYqpack(pkgPath, InstallOptions{ExpectedSHA256: strings.Repeat("0", 64)})
	if err == nil {
		t.Fatal("expected sha mismatch error")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectYqpackFileBuildsStudioReportWithoutInstalling(t *testing.T) {
	srcDir := t.TempDir()
	manifest := minimalManifest("yunque.pack.inspect", "0.1.0")
	manifest.Name = "Inspect Pack"
	manifest.Backend = BackendManifest{
		Capabilities: []string{"inspect.demo"},
		Permissions:  []string{"wasm:execute"},
		Runtime:      &BackendRuntime{Type: RuntimeTypeWasm, Module: "backend/plugin.wasm", SHA256: strings.Repeat("a", 64)},
		RouteSpecs:   []BackendRouteSpec{{Method: "POST", Path: "/v1/inspect/run"}},
	}
	manifest.Frontend = FrontendManifest{
		Menus:  []FrontendMenu{{Key: "inspect", Label: "Inspect", Path: "/packs/inspect"}},
		Routes: []FrontendRoute{{Path: "/packs/inspect", Component: "InspectPackPage"}},
		Assets: FrontendAssets{Type: FrontendAssetsTypeIframeBundle, Entry: "index.html"},
	}
	if err := SaveManifest(filepath.Join(srcDir, ManifestFileName), manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "frontend", "index.html"), []byte("<main>Inspect</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "backend", "plugin.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# inspect"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgPath := filepath.Join(t.TempDir(), "inspect.yqpack")
	sha, err := PackToYqpack(srcDir, pkgPath)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	report, err := InspectYqpackFile(pkgPath, strings.Repeat("0", 64), "补齐可用说明")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if report.Manifest.ID != manifest.ID || report.SHA256 != sha || report.SHA256Match {
		t.Fatalf("unexpected identity or sha match: %#v", report)
	}
	if report.EntryCount == 0 || report.EditableCount == 0 || report.GuardedCount == 0 {
		t.Fatalf("expected mixed entry classification: %#v", report)
	}
	if !containsYqpackEntry(report.Entries, "pack.json", "manifest", true) {
		t.Fatalf("expected editable manifest entry: %#v", report.Entries)
	}
	if !containsYqpackEntry(report.Entries, "frontend/index.html", "frontend", true) {
		t.Fatalf("expected editable frontend entry: %#v", report.Entries)
	}
	if !containsYqpackEntry(report.Entries, "backend/plugin.wasm", "wasm", false) {
		t.Fatalf("expected guarded wasm entry: %#v", report.Entries)
	}
	if report.Plan.PackID != manifest.ID || !strings.Contains(report.Plan.XiaoyuPrompt, "补齐可用说明") {
		t.Fatalf("expected embedded studio plan: %#v", report.Plan)
	}
	if len(report.Warnings) == 0 || !strings.Contains(report.Warnings[0], "sha256 mismatch") {
		t.Fatalf("expected sha warning, got %#v", report.Warnings)
	}
}

func TestPrepareStudioWorkspaceFromYqpackDoesNotInstall(t *testing.T) {
	srcDir := t.TempDir()
	manifest := minimalManifest("yunque.pack.workspace", "0.1.0")
	manifest.Name = "Workspace Pack"
	manifest.Frontend = FrontendManifest{
		Menus:  []FrontendMenu{{Key: "workspace", Label: "Workspace", Path: "/packs/workspace"}},
		Routes: []FrontendRoute{{Path: "/packs/workspace", Component: "WorkspacePackPage"}},
		Assets: FrontendAssets{Type: FrontendAssetsTypeIframeBundle, Entry: "index.html"},
	}
	if err := SaveManifest(filepath.Join(srcDir, ManifestFileName), manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "frontend", "index.html"), []byte("<main>Workspace</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "manifest.sig"), []byte("sig"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgPath := filepath.Join(t.TempDir(), "workspace.yqpack")
	sha, err := PackToYqpack(srcDir, pkgPath)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report, err := registry.PrepareStudioWorkspaceFromYqpack(pkgPath, sha, "准备可编辑副本")
	if err != nil {
		t.Fatalf("prepare workspace: %v", err)
	}
	if report.WorkspacePath == "" || report.WorkspaceID == "" || report.Manifest.ID != manifest.ID {
		t.Fatalf("unexpected workspace report: %#v", report)
	}
	if _, err := os.Stat(filepath.Join(report.WorkspacePath, ManifestFileName)); err != nil {
		t.Fatalf("expected manifest extracted to workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(report.WorkspacePath, "frontend", "index.html")); err != nil {
		t.Fatalf("expected frontend extracted to workspace: %v", err)
	}
	if len(report.EditableFiles) == 0 || len(report.GuardedFiles) == 0 {
		t.Fatalf("expected editable and guarded files: %#v", report)
	}
	if _, ok := registry.Get(manifest.ID); ok {
		t.Fatal("workspace prepare must not install the pack")
	}
	if _, err := registry.PrepareStudioWorkspaceFromYqpack(pkgPath, strings.Repeat("0", 64), "bad sha"); err == nil {
		t.Fatal("expected sha mismatch to block workspace preparation")
	}
}

func containsYqpackEntry(entries []YqpackEntryReport, path string, kind string, editable bool) bool {
	for _, entry := range entries {
		if entry.Path == path && entry.Kind == kind && entry.Editable == editable {
			return true
		}
	}
	return false
}

func TestSignAndVerifyManifest(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := minimalManifest("yunque.pack.test", "0.1.0")
	if err := SignManifest(&m, priv, "test-publisher", "test-key-1"); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if m.Signing == nil || m.Signing.Signature == "" {
		t.Fatal("signing block not populated")
	}
	if m.Signing.Algorithm != "ed25519" {
		t.Fatalf("unexpected algorithm: %s", m.Signing.Algorithm)
	}

	tr := NewTrustRoot(t.TempDir())
	if err := tr.AddDiskKey("test-publisher", "test-key-1", pub); err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifest(m, tr); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Tamper detection: flip a byte in the signature.
	tampered := m
	sigBytes, _ := hex.DecodeString(hex.EncodeToString([]byte(m.Signing.Signature)))
	if len(sigBytes) > 0 {
		sigBytes[0] ^= 0xFF
	}
	tampered.Description = "tampered description"
	if err := VerifyManifest(tampered, tr); err == nil {
		t.Fatal("expected verify failure for tampered manifest")
	}
}

func TestVerifyManifestUnknownPublisher(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := minimalManifest("yunque.pack.test", "0.1.0")
	if err := SignManifest(&m, priv, "test-publisher", "test-key-1"); err != nil {
		t.Fatal(err)
	}
	tr := NewTrustRoot(t.TempDir())
	if err := VerifyManifest(m, tr); err == nil {
		t.Fatal("expected unknown publisher failure")
	}
}

func TestInstallFromYqpackRejectsUnsignedWhenPolicyDemands(t *testing.T) {
	srcDir := t.TempDir()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := minimalManifest("yunque.pack.test", "0.1.0")
	if err := SignManifest(&m, priv, "publisher-x", "key-1"); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(filepath.Join(srcDir, ManifestFileName), m); err != nil {
		t.Fatal(err)
	}
	pkgPath := filepath.Join(t.TempDir(), "test.yqpack")
	if _, err := PackToYqpack(srcDir, pkgPath); err != nil {
		t.Fatal(err)
	}

	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// With no trust root, signed manifest must fail closed.
	if _, err := registry.InstallFromYqpack(pkgPath, InstallOptions{}); err == nil {
		t.Fatal("expected failure when trust root absent")
	}

	tr := NewTrustRoot(t.TempDir())
	if err := tr.AddDiskKey("publisher-x", "key-1", pub); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.InstallFromYqpack(pkgPath, InstallOptions{TrustRoot: tr}); err != nil {
		t.Fatalf("install with trust root: %v", err)
	}
}
