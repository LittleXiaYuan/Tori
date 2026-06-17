package knowledge

// repoimport.go holds the local-repository import path-resolution policy shared
// by the gateway security test and the knowledge pack's native import-repo
// handler. Pure filesystem/env logic: no gateway dependency.

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ResolveRepoPath validates a user-supplied import-repo path against the
// allowed roots and returns the resolved absolute path.
//
// SECURITY: to prevent arbitrary local-file exfiltration via an authenticated
// user setting `path` to `/etc` or `C:\Users`, the resolved path must be rooted
// under one of:
//   - the configured output dir (outputDir)
//   - any directory listed in the KB_IMPORT_ROOTS env (`;` or `:` separated)
//
// Operators who need wider access can opt in with KB_IMPORT_ALLOW_ANY=true,
// which restores the legacy behaviour and is logged loudly on every request.
func ResolveRepoPath(outputDir, userPath string) (string, error) {
	if strings.EqualFold(os.Getenv("KB_IMPORT_ALLOW_ANY"), "true") {
		slog.Warn("knowledge: KB_IMPORT_ALLOW_ANY=true — arbitrary directory import enabled", "path", userPath)
		abs, err := filepath.Abs(userPath)
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}
		return abs, nil
	}

	abs, err := filepath.Abs(userPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	realCandidate, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("path not accessible")
	}

	roots := collectImportRoots(outputDir)
	if len(roots) == 0 {
		return "", fmt.Errorf("no KB import roots configured; set KB_IMPORT_ROOTS or configure outputDir")
	}
	for _, root := range roots {
		realRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(realRoot, realCandidate)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return realCandidate, nil
		}
	}
	return "", fmt.Errorf("path must be inside the configured KB import roots")
}

func collectImportRoots(outputDir string) []string {
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			return
		}
		out = append(out, p)
	}
	add(outputDir)
	raw := os.Getenv("KB_IMPORT_ROOTS")
	if raw != "" {
		for _, part := range filepath.SplitList(raw) {
			add(part)
		}
	}
	return out
}
