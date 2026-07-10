package general

import (
	"path/filepath"
	"regexp"
	"strings"
)

var mdImageRe = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)\)`)

func isTableRow(line string) bool {
	line = strings.TrimSpace(line)
	return strings.Count(line, "|") >= 2 && strings.Contains(line, "|")
}

func isSeparatorRow(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, "|") {
		return false
	}
	parts := splitPipeRow(line)
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		for _, r := range p {
			if r != '-' && r != ':' && r != ' ' {
				return false
			}
		}
	}
	return true
}

func splitPipeRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func sanitizeImagePath(path string, allowed []string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	if !isUnderAllowed(abs, allowed) {
		return ""
	}
	return abs
}
