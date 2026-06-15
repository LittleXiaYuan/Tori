package wasmplugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

var errInstallReject = errors.New("bad signature")

// TestRemoteInstallExecuteFailClosed proves the remote-install executor never
// installs unless every gate passes (Appendix B). This is the RCE-surface guard:
// the only path that calls the installer is "enforce on + approved + cached".
func TestRemoteInstallExecuteFailClosed(t *testing.T) {
	const packID = "yunque.pack.remote-demo"

	newHandler := func(enforce bool, install func(ctx context.Context, cachePath string) error) *Handler {
		return New(Config{DataDir: t.TempDir(), RemoteInstallEnforce: enforce, InstallFromCache: install})
	}
	seedApproved := func(h *Handler) {
		if err := h.saveApprovalQueueRecord(ApprovalQueueRecord{PackID: packID, RequestID: "req-1", RequestKey: "key-1", DecisionKey: "dec-1", Decision: "approved"}); err != nil {
			t.Fatal(err)
		}
	}
	seedDownload := func(h *Handler) {
		if err := h.saveInstallerDownloadRecord(InstallerDownloadRecord{PackID: packID, CachePath: filepath.Join(h.dataDir, "x.yqpack")}); err != nil {
			t.Fatal(err)
		}
	}
	exec := func(h *Handler) map[string]any {
		body, _ := json.Marshal(RemoteInstallExecuteRequest{PackID: packID})
		req := httptest.NewRequest(http.MethodPost, "/v1/wasm-plugin/remote-install/execute", strings.NewReader(string(body)))
		w := httptest.NewRecorder()
		h.RemoteInstallExecute(w, req)
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v (body=%s)", err, w.Body.String())
		}
		return resp
	}

	// 1. enforce off (default) → blocked, installer never called.
	called := false
	h1 := newHandler(false, func(context.Context, string) error { called = true; return nil })
	seedApproved(h1)
	seedDownload(h1)
	if r := exec(h1); r["remote_install_ready"] != false || called {
		t.Fatalf("enforce off must block & not install: %v called=%v", r, called)
	}

	// 2. enforce on but not approved → blocked, installer never called.
	h2 := newHandler(true, func(context.Context, string) error { t.Fatal("must not install unapproved"); return nil })
	seedDownload(h2)
	if r := exec(h2); r["remote_install_ready"] != false || r["approved"] != false {
		t.Fatalf("unapproved must block: %v", r)
	}

	// 3. enforce on, approved, cached, installer ok → ready (the only install path).
	installed := ""
	h3 := newHandler(true, func(_ context.Context, p string) error { installed = p; return nil })
	seedApproved(h3)
	seedDownload(h3)
	if r := exec(h3); r["remote_install_ready"] != true || installed == "" {
		t.Fatalf("all gates pass must install: %v installed=%q", r, installed)
	}

	// 4. enforce on, installer rejects (bad sha/signature) → blocked.
	h4 := newHandler(true, func(context.Context, string) error { return errInstallReject })
	seedApproved(h4)
	seedDownload(h4)
	if r := exec(h4); r["remote_install_ready"] != false {
		t.Fatalf("installer rejection must block: %v", r)
	}
}
