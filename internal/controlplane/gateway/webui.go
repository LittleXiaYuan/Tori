package gateway

import (
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	webui "yunque-agent/web"
)

// serveWebUI serves the embedded Next.js static export.
// Resolution order:
//  1. Exact static file match (e.g. /_next/static/xxx.js, /favicon.ico)
//  2. path.html — Next.js generates chat.html, settings.html, etc.
//  3. path/index.html — directory-style route
//  4. SPA fallback — index.html (client-side router handles the path)
//
// If no real frontend build is embedded, falls back to the pure HTML dashboard.
func (g *Gateway) serveWebUI(w http.ResponseWriter, r *http.Request) {
	staticFS, err := webui.FS()
	if err != nil || !webui.HasContent() {
		// No frontend build → fall back to embedded pure HTML dashboard.
		g.handleDashboard(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")

	// 1. Exact file match (static assets, _next/*, etc.)
	if path != "" {
		if data, err := fs.ReadFile(staticFS, path); err == nil {
			serveBytes(w, path, data)
			return
		}
	}

	// 2. Try path.html (Next.js static export: /chat → chat.html)
	if path != "" {
		htmlPath := path + ".html"
		if data, err := fs.ReadFile(staticFS, htmlPath); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
	}

	// 3. Try path/index.html (directory-style)
	if path != "" {
		indexPath := path + "/index.html"
		if data, err := fs.ReadFile(staticFS, indexPath); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
	}

	// 4. SPA fallback: serve root index.html for client-side routing
	data, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		// Even index.html missing → pure HTML dashboard
		g.handleDashboard(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// serveBytes writes raw bytes with a Content-Type inferred from the file extension.
func serveBytes(w http.ResponseWriter, name string, data []byte) {
	ext := filepath.Ext(name)
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		ct = "application/octet-stream"
		if ext == ".js" || ext == ".mjs" {
			ct = "application/javascript"
		} else if ext == ".css" {
			ct = "text/css; charset=utf-8"
		} else if ext == ".svg" {
			ct = "image/svg+xml"
		} else if ext == ".json" {
			ct = "application/json"
		} else if ext == ".woff2" {
			ct = "font/woff2"
		}
	}
	w.Header().Set("Content-Type", ct)
	// Cache static assets aggressively (_next/ files are content-hashed).
	if strings.HasPrefix(name, "_next/static/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	w.Write(data)
}
