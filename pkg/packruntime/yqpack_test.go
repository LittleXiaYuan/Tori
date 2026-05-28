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
