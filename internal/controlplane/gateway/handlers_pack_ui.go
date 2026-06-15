package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

// handlePackUIAsset serves a Pack's iframe-bundle frontend assets for the DLC
// host (see docs/spec/pack-frontend-dlc.md).
//
// Route: GET /v1/packs/{id}/ui/{path...}
//
// It is intentionally public (no token): the bundle is non-sensitive static UI,
// and every privileged action goes through the authed postMessage bridge. Files
// are served only from an *enabled* pack whose frontend.assets.type is
// "iframe-bundle", with strict path containment under the pack's frontend/ dir.
//
// The response headers override the gateway's global securityHeaders so the
// bundle can be framed by the same-origin shell while staying locked down:
//   - X-Frame-Options removed (CSP frame-ancestors governs framing instead)
//   - CSP frame-ancestors = the shell origin (the iframe runs at an opaque
//     origin under sandbox, so 'self' would not match — we pin the real host)
//   - connect-src 'none' so a bundle cannot phone home; it must use the bridge
func (g *Gateway) handlePackUIAsset(w http.ResponseWriter, r *http.Request) {
	g.servePackUIAsset(w, r, requestOrigin(r))
}

// servePackUIAsset is the shared bundle-serving core for the main mux route and
// the dedicated isolation listener; frameAncestors parametrizes who may embed.
func (g *Gateway) servePackUIAsset(w http.ResponseWriter, r *http.Request, frameAncestors string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.packRegistry == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "pack registry not configured")
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	pack, ok := g.packRegistry.Get(id)
	if !ok || pack.Status != packruntime.PackStatusEnabled {
		apperror.WriteCode(w, apperror.CodeNotFound, "pack not found or not enabled")
		return
	}
	if pack.Manifest.Frontend.Assets.Type != packruntime.FrontendAssetsTypeIframeBundle {
		apperror.WriteCode(w, apperror.CodeNotFound, "pack has no iframe-bundle frontend")
		return
	}

	base := filepath.Join(g.packRegistry.InstalledDir(id, pack.Manifest.Version), "frontend")
	rel := strings.Trim(strings.TrimSpace(r.PathValue("path")), "/")
	if rel == "" {
		entry := strings.TrimSpace(pack.Manifest.Frontend.Assets.Entry)
		if entry == "" {
			entry = "index.html"
		}
		rel = entry
	}

	// Path containment: resolve under base and reject any escape (.. / symlink-ish).
	target := filepath.Clean(filepath.Join(base, filepath.FromSlash(rel)))
	baseClean := filepath.Clean(base)
	if target != baseClean && !strings.HasPrefix(target, baseClean+string(os.PathSeparator)) {
		apperror.WriteCode(w, apperror.CodeNotFound, "not found")
		return
	}

	f, err := os.Open(target)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "not found")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		apperror.WriteCode(w, apperror.CodeNotFound, "not found")
		return
	}

	g.setPackUIHeaders(w, r, frameAncestors)
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// setPackUIHeaders overrides the gateway's global security headers for pack UI
// bundle responses so the shell can frame them while the bundle stays sandboxed.
// frameAncestors is the CSP source list of origins allowed to embed the bundle:
// the shell's own origin on the main listener, or the local-shell origin set on
// the dedicated isolation listener (whose own origin differs from the shell's).
func (g *Gateway) setPackUIHeaders(w http.ResponseWriter, r *http.Request, frameAncestors string) {
	origin := requestOrigin(r)
	h := w.Header()
	// The global middleware sets X-Frame-Options: DENY, which would block the
	// shell from framing this bundle; CSP frame-ancestors replaces it.
	h.Del("X-Frame-Options")
	h.Set("Content-Security-Policy", strings.Join([]string{
		"default-src 'none'",
		// The sandboxed iframe has an opaque origin, so 'self' will not match
		// the gateway host. Pin the actual origin for the bundle's own assets.
		fmt.Sprintf("script-src %s 'unsafe-inline' 'unsafe-eval'", origin),
		fmt.Sprintf("style-src %s 'unsafe-inline'", origin),
		fmt.Sprintf("img-src %s data: blob:", origin),
		fmt.Sprintf("font-src %s data:", origin),
		// No network for the bundle — all backend access goes through the bridge.
		"connect-src 'none'",
		fmt.Sprintf("frame-ancestors %s", frameAncestors),
	}, "; "))
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("X-Content-Type-Options", "nosniff")
}

// handlePackBridgeViolation files one refused DLC-bridge request (unknown
// method, undeclared route, quota or rate breach) into the tamper-evident
// audit chain, as required by docs/spec/pack-frontend-dlc.md §7.3. The report
// comes from the authed host shell — never from the sandboxed bundle itself.
//
// Route: POST /v1/packs/{id}/bridge-violation (auth required)
func (g *Gateway) handlePackBridgeViolation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if g.packRegistry == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "pack registry not configured")
		return
	}
	if _, ok := g.packRegistry.Get(id); !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "pack not found")
		return
	}
	var req struct {
		Method  string `json:"method"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
		return
	}
	clip := func(s string, n int) string {
		s = strings.TrimSpace(s)
		if len(s) > n {
			return s[:n]
		}
		return s
	}
	code := clip(req.Code, 32)
	if code == "" {
		code = "forbidden"
	}
	detail := fmt.Sprintf("code=%s method=%s %s", code, clip(req.Method, 64), clip(req.Message, 512))
	if g.auditChain != nil {
		g.auditChain.Append(audit.EventAuth, "pack:"+id, "bridge_violation", detail)
	}
	slog.Warn("pack bridge violation", "pack", id, "code", code, "method", clip(req.Method, 64))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// packUIShellAncestors is the frame-ancestors source list used by the dedicated
// pack-UI listener: only a local-first shell (browser tab on loopback, or the
// packaged Tauri webview) may embed bundles. Web origins on the internet can't.
const packUIShellAncestors = "http://localhost:* http://127.0.0.1:* https://localhost:* https://127.0.0.1:* tauri://localhost http://tauri.localhost"

// PackUIOrigin returns the dedicated pack-UI listener origin ("" = disabled,
// bundles are served same-origin from the main listener).
func (g *Gateway) PackUIOrigin() string {
	g.routesMu.RLock()
	defer g.routesMu.RUnlock()
	return g.packUIOrigin
}

// handlePackUIOrigin reports the dedicated pack-UI origin so the shell can
// point DLC iframes at the isolated listener when it is enabled.
//
// GET /v1/packs/ui-origin → {"origin": "http://127.0.0.1:PORT"} or {"origin": ""}
func (g *Gateway) handlePackUIOrigin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"origin": g.PackUIOrigin()})
}

// StartPackUIServer starts the optional dedicated pack-UI listener on addr
// (e.g. "127.0.0.1:0"). Serving bundles from their own port gives DLC iframes a
// real cross-origin boundary to the shell — defense in depth on top of the
// sandbox attribute's opaque origin. The listener serves ONLY pack UI assets.
// Returns the listener origin; callers stop it by closing the returned server.
func (g *Gateway) StartPackUIServer(addr string) (string, *http.Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, err
	}
	mux := http.NewServeMux()
	serve := func(w http.ResponseWriter, r *http.Request) {
		g.servePackUIAsset(w, r, packUIShellAncestors)
	}
	mux.HandleFunc("GET /v1/packs/{id}/ui", serve)
	mux.HandleFunc("GET /v1/packs/{id}/ui/{path...}", serve)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	origin := "http://" + loopbackHostPort(ln.Addr().String())
	g.routesMu.Lock()
	g.packUIOrigin = origin
	g.routesMu.Unlock()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Warn("pack ui server stopped", "err", err)
		}
	}()
	slog.Info("pack ui isolation listener started", "origin", origin)
	return origin, srv, nil
}

// loopbackHostPort rewrites a listener address like "[::]:1234" or
// "0.0.0.0:1234" to a dialable loopback host:port.
func loopbackHostPort(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "::", "0.0.0.0":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// requestOrigin reconstructs the scheme://host origin of the current request,
// used to pin CSP source lists for sandboxed (opaque-origin) pack bundles.
func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); fwd != "" {
		host = fwd
	}
	if host == "" {
		return "'self'"
	}
	return scheme + "://" + host
}
