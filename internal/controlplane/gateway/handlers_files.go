package gateway

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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

func (g *Gateway) handleFileList(w http.ResponseWriter, r *http.Request) {
	if g.outputDir == "" {
		http.Error(w, `{"error":"output dir not configured"}`, http.StatusInternalServerError)
		return
	}

	subPath := r.URL.Query().Get("path")
	if subPath == "" {
		subPath = "."
	}

	fullPath, err := safeResolve(g.outputDir, subPath)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"files": []any{}})
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"files": files})
}

func (g *Gateway) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "path required"})
		return
	}

	if g.outputDir == "" {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "output dir not configured"})
		return
	}

	fullPath, err := safeResolve(g.outputDir, filePath)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	info, statErr := os.Stat(fullPath)
	if statErr != nil || info.IsDir() {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(fullPath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fullPath)
}

func (g *Gateway) handleFilePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET required"})
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "path required"})
		return
	}
	if g.outputDir == "" {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "output dir not configured"})
		return
	}
	fullPath, err := safeResolve(g.outputDir, filePath)
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	info, statErr := os.Stat(fullPath)
	if statErr != nil || info.IsDir() {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}
	if info.Size() > 20<<20 {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "file too large for preview"})
		return
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	ext := strings.ToLower(filepath.Ext(fullPath))
	parseResult := TryParseFileResult(filepath.Base(fullPath), data)
	preview := parseResult.Preview
	truncated := false
	runes := []rune(preview)
	if len(runes) > 6000 {
		preview = string(runes[:6000]) + "\n\n...已截断"
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
	if parseMeta := fileParseMetadata(parseResult, 6000); parseMeta != nil {
		resp["parse"] = parseMeta
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
