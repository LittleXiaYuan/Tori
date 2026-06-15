// Package documentspack mounts the document-generation HTTP surface
// (/v1/documents/*) as a v2 capability pack (Tier 0 microkernel). Native pack:
// it owns doc generation (via skill invocation) + the template catalog, reaching
// the skill registry through a narrow accessor. Split out of the misnamed
// registerTaskRoutes grab-bag.
package documentspack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/skills"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.documents"

// Gateway is the narrow host surface the documents pack needs.
type Gateway interface {
	SkillsRegistry() *skills.Registry
}

// Handler is the documents pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the documents pack backed by the host's skill-registry accessor.
func New(gw Gateway) *Handler { return &Handler{gw: gw} }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("documents pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the document surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodPost, Path: "/v1/documents/generate", Handler: h.handleGenerate},
		{Method: http.MethodGet, Path: "/v1/documents/templates", Handler: h.handleTemplates},
	}
}

func (h *Handler) registry() *skills.Registry {
	if h.gw == nil {
		return nil
	}
	return h.gw.SkillsRegistry()
}

// handleGenerate generates a document directly via skill invocation.
func (h *Handler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	reg := h.registry()
	if reg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "skill registry not available")
		return
	}
	var req struct {
		Format    string `json:"format"`
		Path      string `json:"path"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		SheetName string `json:"sheet_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.Format == "" || req.Content == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "format and content are required")
		return
	}
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
	skill, ok := reg.Get(skillName)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, fmt.Sprintf("skill %s not found", skillName))
		return
	}
	if req.Path == "" {
		ext := req.Format
		base := "document"
		if req.Title != "" {
			base = sanitizeDocFilename(req.Title)
		}
		req.Path = filepath.Join("data", "output", fmt.Sprintf("%s-%d.%s", base, time.Now().Unix(), ext))
	}
	args := map[string]any{"path": req.Path, "content": req.Content}
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
	_ = json.NewEncoder(w).Encode(map[string]string{"result": result, "path": req.Path, "format": req.Format})
}

// handleTemplates returns the template catalog for document generation.
func (h *Handler) handleTemplates(w http.ResponseWriter, r *http.Request) {
	catalogPath := filepath.Join("data", "templates", "catalog.json")
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"templates": []any{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func sanitizeDocFilename(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	replacer := strings.NewReplacer(
		"\\", "-", "/", "-", ":", "-", "*", "-", "?", "-",
		"\"", "-", "<", "-", ">", "-", "|", "-", " ", "-",
	)
	s = replacer.Replace(s)
	s = strings.Trim(s, "-.")
	if s == "" {
		return "document"
	}
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}
