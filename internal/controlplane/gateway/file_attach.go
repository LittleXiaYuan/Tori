package gateway

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"yunque-agent/internal/agentcore/planner"
	channelpkg "yunque-agent/internal/execution/channel"
)

var pathLikePattern = regexp.MustCompile(`(?i)(?:[a-z]:)?(?:\.{0,2}[/\\]|[/\\])(?:[\w.\-]+[/\\])*[\w.\-]+\.(?:png|jpe?g|gif|webp|pdf|xlsx|docx|csv|html?|pptx|zip|txt|md)`)

// AttachFilesToRich scans planner output for filesystem paths and appends image/file components.
// If rm is nil, a new RichMessage is created only when at least one path resolves to a file.
func AttachFilesToRich(rm *channelpkg.RichMessage, result *planner.PlanResult, searchRoots []string) *channelpkg.RichMessage {
	if result == nil {
		return rm
	}
	seen := map[string]bool{}
	textBuf := strings.Builder{}
	textBuf.WriteString(result.Reply)
	for _, step := range result.Plan {
		textBuf.WriteString("\n")
		textBuf.WriteString(step.Result)
	}
	for _, s := range extractPaths(textBuf.String()) {
		rm = tryAddPath(rm, s, seen, searchRoots)
	}
	for _, a := range result.Actions {
		if a.Kind != planner.ActionShowFile {
			continue
		}
		switch fp := a.Payload.(type) {
		case planner.FilePayload:
			rm = tryAddPath(rm, fp.Path, seen, searchRoots)
		case map[string]any:
			p, _ := fp["path"].(string)
			rm = tryAddPath(rm, p, seen, searchRoots)
		}
	}
	return rm
}

func extractPaths(s string) []string {
	found := pathLikePattern.FindAllString(s, -1)
	out := make([]string, 0, len(found))
	for _, p := range found {
		p = strings.Trim(p, `"'`+"`")
		out = append(out, p)
	}
	return out
}

func tryAddPath(rm *channelpkg.RichMessage, raw string, seen map[string]bool, roots []string) *channelpkg.RichMessage {
	if raw == "" || seen[raw] {
		return rm
	}
	candidates := []string{raw}
	if !filepath.IsAbs(raw) {
		for _, root := range roots {
			if root == "" {
				continue
			}
			candidates = append(candidates, filepath.Join(root, raw))
		}
	}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if seen[c] {
			continue
		}
		st, err := os.Stat(c)
		if err != nil || st.IsDir() {
			continue
		}
		seen[c] = true
		seen[raw] = true
		if rm == nil {
			rm = channelpkg.NewRichMessage()
		}
		addFileComponent(rm, planner.FilePayload{
			Path: c,
			Name: filepath.Base(c),
			Size: st.Size(),
		})
		return rm
	}
	return rm
}
