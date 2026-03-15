package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/skills"
)

// handleDocGenerate handles direct document generation via skill invocation.
// POST /v1/documents/generate
// { "format": "docx|xlsx|html|pptx", "path": "...", "title": "...", "content": "...", "sheet_name": "..." }
func (g *Gateway) handleDocGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.registry == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "skill registry not available")
		return
	}

	var req struct {
		Format    string `json:"format"`     // docx, xlsx, html, pptx
		Path      string `json:"path"`       // output path
		Title     string `json:"title"`      // document title
		Content   string `json:"content"`    // content body
		SheetName string `json:"sheet_name"` // xlsx only
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.Format == "" || req.Content == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "format and content are required")
		return
	}

	// Map format to skill name
	skillMap := map[string]string{
		"docx": "docx_create",
		"xlsx": "xlsx_create",
		"html": "html_export",
		"pptx": "pptx_create",
	}
	skillName, ok := skillMap[strings.ToLower(req.Format)]
	if !ok {
		apperror.WriteCode(w, apperror.CodeBadRequest, fmt.Sprintf("unsupported format: %s (support: docx, xlsx, html, pptx)", req.Format))
		return
	}

	skill, ok := g.registry.Get(skillName)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, fmt.Sprintf("skill %s not found", skillName))
		return
	}

	// Auto-generate path if not provided
	if req.Path == "" {
		ext := req.Format
		if ext == "html" {
			ext = "html"
		}
		req.Path = filepath.Join("data", "output", fmt.Sprintf("document.%s", ext))
	}

	// Build args
	args := map[string]any{
		"path":    req.Path,
		"content": req.Content,
	}
	if req.Title != "" {
		args["title"] = req.Title
	}
	if req.SheetName != "" {
		args["sheet_name"] = req.SheetName
	}

	result, err := skill.Execute(r.Context(), args, &skills.Environment{})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"result": result,
		"path":   req.Path,
		"format": req.Format,
	})
}
