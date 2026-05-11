package general

import (
	"path/filepath"
	"strings"
)

// isUnderAllowed checks if the target path is under one of the allowed directories.
// Both paths are resolved through EvalSymlinks to prevent symlink-based escapes.
// When the target file does not yet exist (write path), the parent directory is
// resolved instead so that Windows 8.3 short names (e.g. ADMINI~1) match their
// long-name equivalents correctly.
func isUnderAllowed(absPath string, allowed []string) bool {
	resolved := resolvePathBestEffort(absPath)
	for _, dir := range allowed {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		resolvedDir, err := filepath.EvalSymlinks(absDir)
		if err != nil {
			resolvedDir = filepath.Clean(absDir)
		}
		if strings.HasPrefix(resolved, resolvedDir+string(filepath.Separator)) || resolved == resolvedDir {
			return true
		}
	}
	return false
}

// resolvePathBestEffort resolves a path via EvalSymlinks.  When the path does
// not exist (common for write targets), it resolves the nearest existing
// ancestor and appends the remaining tail.  This ensures Windows 8.3 short
// names in the ancestor portion are expanded to long names.
func resolvePathBestEffort(p string) string {
	resolved, err := filepath.EvalSymlinks(p)
	if err == nil {
		return resolved
	}
	parent := filepath.Dir(p)
	if parent == p {
		return filepath.Clean(p)
	}
	resolvedParent := resolvePathBestEffort(parent)
	return filepath.Join(resolvedParent, filepath.Base(p))
}
