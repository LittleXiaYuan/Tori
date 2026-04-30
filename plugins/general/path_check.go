package general

import (
	"path/filepath"
	"strings"
)

// isUnderAllowed checks if the target path is under one of the allowed directories.
func isUnderAllowed(absPath string, allowed []string) bool {
	for _, dir := range allowed {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || absPath == absDir {
			return true
		}
	}
	return false
}
