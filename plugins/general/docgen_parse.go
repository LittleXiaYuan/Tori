package general

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
)

var mdImageRe = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)\)`)

// extractChartBlock removes a ```chart ... ``` fenced block and returns JSON inside.
func extractChartBlock(s string) (string, json.RawMessage) {
	idx := strings.Index(s, "```chart")
	if idx < 0 {
		return s, nil
	}
	rest := s[idx+len("```chart"):]
	end := strings.Index(rest, "```")
	if end < 0 {
		return s, nil
	}
	raw := strings.TrimSpace(rest[:end])
	if !json.Valid([]byte(raw)) {
		return s, nil
	}
	out := strings.TrimSpace(s[:idx] + rest[end+3:])
	return out, json.RawMessage(raw)
}

// extractMarkdownTable finds the first GFM-style pipe table and removes it from text.
func extractMarkdownTable(s string) (string, [][]string) {
	lines := strings.Split(s, "\n")
	start := -1
	for i, line := range lines {
		if isTableRow(line) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			return spliceTable(lines, start, i)
		}
	}
	if start >= 0 {
		return spliceTable(lines, start, len(lines))
	}
	return s, nil
}

func spliceTable(lines []string, start, end int) (string, [][]string) {
	chunk := lines[start:end]
	var rows [][]string
	for _, ln := range chunk {
		if isSeparatorRow(ln) {
			continue
		}
		if r := splitPipeRow(ln); len(r) >= 2 {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return strings.Join(lines, "\n"), nil
	}
	before := strings.Join(lines[:start], "\n")
	after := strings.Join(lines[end:], "\n")
	return strings.TrimSpace(strings.TrimSpace(before) + "\n" + strings.TrimSpace(after)), rows
}

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

// extractFirstImagePath returns the first markdown image path and text without that line.
func extractFirstImagePath(s string) (string, string) {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		m := mdImageRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		path := strings.TrimSpace(m[1])
		rest := append(lines[:i], lines[i+1:]...)
		return path, strings.TrimSpace(strings.Join(rest, "\n"))
	}
	return "", s
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
