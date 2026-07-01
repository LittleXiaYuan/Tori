package gateway

import (
	"time"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/session"
)

// sessionFileSearchRoots mirrors the roots AttachFilesToRich resolves
// relative tool-result paths against, so a bare "report.docx" in a skill's
// Output text still matches the file it actually wrote under data/output.
var sessionFileSearchRoots = []string{".", "data", "data/output", "data/uploads", "data/tasks"}

// registerSessionFiles folds a completed chat turn's file traffic — files the
// user attached this turn (workspacePaths) and files any skill just wrote
// (extracted from the plan's tool-result text) — into the session's file
// registry, so later turns in the same conversation can reference them by
// path without the user re-uploading or re-generating (Manus-style file
// reuse within a conversation).
func (g *Gateway) registerSessionFiles(sessionID string, workspacePaths []string, result *planner.PlanResult) {
	if sessionID == "" || g.convStore == nil {
		return
	}
	now := time.Now()
	var files []session.SessionFile
	seen := map[string]bool{}
	for _, raw := range workspacePaths {
		ref, ok := resolveFileRef(raw, seen, sessionFileSearchRoots)
		if !ok {
			continue
		}
		files = append(files, session.SessionFile{Path: ref.Path, Name: ref.Name, Kind: "uploaded", Size: ref.Size, AddedAt: now})
	}
	for _, ref := range CollectGeneratedFileRefs(result, sessionFileSearchRoots) {
		files = append(files, session.SessionFile{Path: ref.Path, Name: ref.Name, Kind: "generated", Size: ref.Size, AddedAt: now})
	}
	if len(files) > 0 {
		g.convStore.AddFiles(sessionID, files...)
	}
}

// sessionFilesForRequest reads back a session's registered files as
// planner.SessionFileRef, ready to thread into PlanRequest.SessionFiles so
// BuildMessages can inject them as context on this turn.
func (g *Gateway) sessionFilesForRequest(sessionID string) []planner.SessionFileRef {
	if sessionID == "" || g.convStore == nil {
		return nil
	}
	stored := g.convStore.Files(sessionID)
	if len(stored) == 0 {
		return nil
	}
	refs := make([]planner.SessionFileRef, len(stored))
	for i, f := range stored {
		refs[i] = planner.SessionFileRef{Path: f.Path, Name: f.Name, Kind: f.Kind}
	}
	return refs
}
