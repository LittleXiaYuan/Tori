// Package filespack mounts the agent output-file surface
// (/api/files, /api/files/preview, /api/files/download) as a v2 capability pack
// (Tier 0 microkernel). It is a native pack: the list/preview/download logic
// lives here and reads the configured output directory through a narrow host
// accessor — the gateway no longer hosts these routes.
package filespack

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"yunque-agent/internal/fileparse"
	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.files"

// previewMaxBytes caps the size of a file that can be previewed inline.
const previewMaxBytes = 20 << 20

// previewMaxRunes caps how much decoded preview text is returned to the client.
const previewMaxRunes = 6000

// Gateway is the narrow host surface the files pack needs: the configured agent
// output directory, resolved per request so registration order does not matter.
type Gateway interface {
	OutputDir() string
}

// Handler is the files pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the files pack backed by the host's output-dir accessor.
func New(gw Gateway) *Handler { return &Handler{gw: gw} }

// compile-time assertion: this is a valid v2 Module.
var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

// Init wires the pack against the kernel Host.
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("files pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the agent output-file surface natively. All three are read-only
// GET endpoints; the Pack Runtime gate applies the host's normal auth.
func (h *Handler) Routes() []packruntime.BackendRoute {
	get := []string{http.MethodGet}
	return []packruntime.BackendRoute{
		{Methods: get, Path: "/api/files", Handler: h.handleList},
		{Methods: get, Path: "/api/files/preview", Handler: h.handlePreview},
		{Methods: get, Path: "/api/files/download", Handler: h.handleDownload},
	}
}

func (h *Handler) outputDir() string {
	if h.gw == nil {
		return ""
	}
	return h.gw.OutputDir()
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	outputDir := h.outputDir()
	if outputDir == "" {
		http.Error(w, `{"error":"output dir not configured"}`, http.StatusInternalServerError)
		return
	}

	subPath := r.URL.Query().Get("path")
	if subPath == "" {
		subPath = "."
	}

	fullPath, err := safeResolve(outputDir, subPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"files": []any{}})
		return
	}

	type fileInfo struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		Size  int64  `json:"size"`
		IsDir bool   `json:"is_dir"`
	}
	files := make([]fileInfo, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		relPath := filepath.Join(subPath, e.Name())
		if subPath == "." {
			relPath = e.Name()
		}
		files = append(files, fileInfo{
			Name:  e.Name(),
			Path:  relPath,
			Size:  size,
			IsDir: e.IsDir(),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path required"})
		return
	}

	outputDir := h.outputDir()
	if outputDir == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "output dir not configured"})
		return
	}

	fullPath, err := safeResolve(outputDir, filePath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	info, statErr := os.Stat(fullPath)
	if statErr != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(fullPath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fullPath)
}

func (h *Handler) handlePreview(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path required"})
		return
	}
	outputDir := h.outputDir()
	if outputDir == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "output dir not configured"})
		return
	}
	fullPath, err := safeResolve(outputDir, filePath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	info, statErr := os.Stat(fullPath)
	if statErr != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}
	if info.Size() > previewMaxBytes {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large for preview"})
		return
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	ext := strings.ToLower(filepath.Ext(fullPath))
	parseResult := fileparse.Parse(filepath.Base(fullPath), data)
	preview := parseResult.Preview
	truncated := false
	runes := []rune(preview)
	if len(runes) > previewMaxRunes {
		preview = string(runes[:previewMaxRunes]) + "\n\n...已截断"
		truncated = true
	}
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	resp := map[string]any{
		"name":         filepath.Base(fullPath),
		"path":         filePath,
		"size":         info.Size(),
		"ext":          strings.TrimPrefix(ext, "."),
		"kind":         previewKind(ext),
		"content_type": contentType,
		"preview":      preview,
		"truncated":    truncated,
		"editable":     previewEditable(ext),
	}
	if parseMeta := fileparse.Metadata(parseResult, previewMaxRunes); parseMeta != nil {
		resp["parse"] = parseMeta
	}
	writeJSON(w, http.StatusOK, resp)
}

// safeResolve resolves a user-supplied relative path against baseDir,
// returning the cleaned absolute path only if it stays within baseDir.
// Rejects absolute paths, ".." traversals, and symlink escapes.
func safeResolve(baseDir, userPath string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("base directory not configured")
	}
	if filepath.IsAbs(userPath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	joined := filepath.Join(baseDir, filepath.Clean(userPath))

	realBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		realBase, err = filepath.Abs(baseDir)
		if err != nil {
			return "", err
		}
	}
	realTarget, err := filepath.EvalSymlinks(joined)
	if err != nil {
		realTarget, err = filepath.Abs(joined)
		if err != nil {
			return "", err
		}
	}

	rel, err := filepath.Rel(realBase, realTarget)
	if err != nil {
		return "", fmt.Errorf("path escape detected")
	}
	if rel == ".." || len(rel) > 2 && rel[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("path escape detected")
	}
	return realTarget, nil
}

func previewKind(ext string) string {
	switch ext {
	case ".csv", ".xlsx", ".xls":
		return "table"
	case ".docx", ".doc", ".pdf", ".ppt", ".pptx":
		return "document"
	case ".txt", ".md", ".markdown", ".log", ".json", ".xml", ".yaml", ".yml":
		return "text"
	default:
		return "file"
	}
}

func previewEditable(ext string) bool {
	switch ext {
	case ".csv", ".xlsx", ".xls", ".docx", ".doc", ".txt", ".md", ".markdown", ".json", ".xml", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
