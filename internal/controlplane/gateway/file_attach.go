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

// FileRef is a validated on-disk file reference extracted from planner
// output text (Reply + PlanStep.Result) or explicit show_file actions.
type FileRef struct {
	Path string // absolute, on-disk, confirmed to exist
	Name string
	Size int64
}

// CollectGeneratedFileRefs scans planner output the same way AttachFilesToRich
// does, but returns plain validated file references instead of building IM
// rich-message components. Used to register files a chat turn produced into
// the session file registry (see session_files.go) so later turns can reuse
// them by path.
func CollectGeneratedFileRefs(result *planner.PlanResult, searchRoots []string) []FileRef {
	if result == nil {
		return nil
	}
	seen := map[string]bool{}
	var refs []FileRef
	textBuf := strings.Builder{}
	textBuf.WriteString(result.Reply)
	for _, step := range result.Plan {
		textBuf.WriteString("\n")
		textBuf.WriteString(step.Result)
	}
	for _, s := range extractPaths(textBuf.String()) {
		if ref, ok := resolveFileRef(s, seen, searchRoots); ok {
			refs = append(refs, ref)
		}
	}
	for _, a := range result.Actions {
		if a.Kind != planner.ActionShowFile {
			continue
		}
		var path string
		switch fp := a.Payload.(type) {
		case planner.FilePayload:
			path = fp.Path
		case map[string]any:
			path, _ = fp["path"].(string)
		}
		if ref, ok := resolveFileRef(path, seen, searchRoots); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

func resolveFileRef(raw string, seen map[string]bool, roots []string) (FileRef, bool) {
	if raw == "" || seen[raw] {
		return FileRef{}, false
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
		return FileRef{Path: c, Name: filepath.Base(c), Size: st.Size()}, true
	}
	return FileRef{}, false
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
