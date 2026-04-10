// Package webui embeds the Next.js static export from heroui-web/out/.
// Build the frontend first with `make web-build` (requires Node.js).
// Without a build, a minimal placeholder is embedded and the gateway
// falls back to the pure-HTML dashboard.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:out
var outFS embed.FS

// FS returns the embedded static filesystem rooted at "out/".
func FS() (fs.FS, error) {
	return fs.Sub(outFS, "out")
}

// HasContent reports whether a real Next.js build is embedded
// (i.e. out/ contains _next/ directory, not just the placeholder).
func HasContent() bool {
	entries, err := fs.ReadDir(outFS, "out")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() == "_next" && e.IsDir() {
			return true
		}
	}
	return false
}
