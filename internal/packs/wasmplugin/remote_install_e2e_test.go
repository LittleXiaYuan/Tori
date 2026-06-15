package wasmplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
)

// TestRemoteInstallExecuteEndToEnd drives the executor through the REAL trusted
// installer (Registry.InstallFromYqpack), not a fake: a genuine .yqpack is built,
// installed when all gates pass, and rejected (nothing registered) once tampered.
// This nails down the RCE-surface path end to end.
func TestRemoteInstallExecuteEndToEnd(t *testing.T) {
	const packID = "yunque.pack.e2e-demo"

	// Build a real .yqpack archive.
	srcDir := t.TempDir()
	m := packruntime.Manifest{ID: packID, Name: "E2E Demo", Version: "0.1.0", RequiresCore: ">=0.1.0", Optional: true, DefaultState: "disabled"}
	if err := packruntime.SaveManifest(filepath.Join(srcDir, packruntime.ManifestFileName), m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# e2e"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgPath := filepath.Join(t.TempDir(), "demo.yqpack")
	sha, err := packruntime.PackToYqpack(srcDir, pkgPath)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	installedIn := func(reg *packruntime.Registry) bool {
		for _, p := range reg.List() {
			if p.Manifest.ID == packID {
				return true
			}
		}
		return false
	}
	exec := func(h *Handler) map[string]any {
		body, _ := json.Marshal(RemoteInstallExecuteRequest{PackID: packID})
		req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/execute", strings.NewReader(string(body)))
		w := httptest.NewRecorder()
		h.RemoteInstallExecute(w, req)
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v (%s)", err, w.Body.String())
		}
		return resp
	}
	seed := func(h *Handler) {
		if err := h.saveApprovalQueueRecord(ApprovalQueueRecord{PackID: packID, RequestID: "r", RequestKey: "k", DecisionKey: "d", Decision: "approved"}); err != nil {
			t.Fatal(err)
		}
		if err := h.saveInstallerDownloadRecord(InstallerDownloadRecord{PackID: packID, CachePath: pkgPath}); err != nil {
			t.Fatal(err)
		}
	}

	// Happy path: all gates pass → the real installer registers the pack.
	reg, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	h := New(Config{DataDir: t.TempDir(), RemoteInstallEnforce: true, InstallFromCache: func(_ context.Context, p string) error {
		_, e := reg.InstallFromYqpack(p, packruntime.InstallOptions{ExpectedSHA256: sha})
		return e
	}})
	seed(h)
	if r := exec(h); r["remote_install_ready"] != true {
		t.Fatalf("expected install, got %v", r)
	}
	if !installedIn(reg) {
		t.Fatalf("pack not registered after successful install")
	}

	// Tamper: corrupt the cached bytes → installer rejects → blocked, nothing registered.
	if err := os.WriteFile(pkgPath, []byte("corrupted-not-a-yqpack"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg2, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("registry2: %v", err)
	}
	h2 := New(Config{DataDir: t.TempDir(), RemoteInstallEnforce: true, InstallFromCache: func(_ context.Context, p string) error {
		_, e := reg2.InstallFromYqpack(p, packruntime.InstallOptions{ExpectedSHA256: sha})
		return e
	}})
	seed(h2)
	if r := exec(h2); r["remote_install_ready"] != false {
		t.Fatalf("tampered package must be rejected, got %v", r)
	}
	if installedIn(reg2) {
		t.Fatalf("tampered pack must NOT be registered")
	}
}
