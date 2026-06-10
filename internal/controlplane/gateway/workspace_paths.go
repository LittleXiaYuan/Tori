package gateway

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"yunque-agent/internal/agentcore/llm"
)

var windowsAbsPathPattern = regexp.MustCompile(`(?i)[a-z]:[\\/][^\r\n"'<>|*?]+`)

func inferWorkspacePathsFromMessages(existing []string, messages []llm.Message) []string {
	out := append([]string{}, existing...)
	seen := map[string]bool{}
	for _, p := range out {
		if abs, ok := existingDirectoryPath(p); ok {
			seen[abs] = true
		}
	}
	for _, msg := range messages {
		if msg.Role != "user" || msg.Content == "" {
			continue
		}
		for _, candidate := range extractLocalPathCandidates(msg.Content) {
			dir, ok := existingDirectoryPath(candidate)
			if !ok || seen[dir] {
				continue
			}
			out = append(out, dir)
			seen[dir] = true
		}
	}
	return out
}

func extractLocalPathCandidates(text string) []string {
	matches := windowsAbsPathPattern.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, raw := range matches {
		if p := trimLocalPathCandidate(raw); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func trimLocalPathCandidate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	start := len(filepath.VolumeName(raw))
	if start < 0 {
		start = 0
	}
	for idx, r := range raw {
		if idx < start {
			continue
		}
		if strings.ContainsRune(",，。；;：:、!?！？", r) {
			raw = raw[:idx]
			break
		}
	}
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, `"'`+"`"+`)]}）】》`)
	return raw
}

func existingDirectoryPath(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !filepath.IsAbs(raw) {
		return "", false
	}
	clean := filepath.Clean(raw)
	info, err := os.Stat(clean)
	if err != nil {
		return "", false
	}
	if !info.IsDir() {
		clean = filepath.Dir(clean)
		info, err = os.Stat(clean)
		if err != nil || !info.IsDir() {
			return "", false
		}
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", false
	}
	return abs, true
}
