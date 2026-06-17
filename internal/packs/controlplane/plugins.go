package controlplanepack

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/plugin"
)

func (h *Handler) pluginGateway() pluginGateway {
	g, _ := h.gateway.(pluginGateway)
	return g
}

func (h *Handler) pluginRegistry() *plugin.Registry {
	g := h.pluginGateway()
	if g == nil {
		return nil
	}
	return g.PluginRegistry()
}

func (h *Handler) pluginLoader() *plugin.Loader {
	g := h.pluginGateway()
	if g == nil {
		return nil
	}
	return g.PluginLoader()
}

func (h *Handler) rebuildSkillsFromPlugins() int {
	g := h.pluginGateway()
	if g == nil {
		return 0
	}
	return g.RebuildSkillsFromPlugins()
}

func (h *Handler) handlePlugins(w http.ResponseWriter, r *http.Request) {
	reg := h.pluginRegistry()
	if reg == nil {
		writeJSON(w, map[string]any{"plugins": []any{}})
		return
	}
	writeJSON(w, map[string]any{"plugins": reg.AllIncludeDisabled()})
}

func (h *Handler) handlePluginToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	reg := h.pluginRegistry()
	if reg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "plugin registry not configured")
		return
	}
	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "name is required")
		return
	}
	if !reg.SetEnabled(req.Name, req.Enabled) {
		apperror.WriteCode(w, apperror.CodeNotFound, "plugin not found")
		return
	}

	skillsCount := h.rebuildSkillsFromPlugins()
	slog.Info("plugin toggled, skills rebuilt", "plugin", req.Name, "enabled", req.Enabled, "skills", skillsCount)
	writeJSON(w, map[string]any{"name": req.Name, "enabled": req.Enabled, "skills_count": skillsCount})
}

func (h *Handler) handlePluginCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	loader := h.pluginLoader()
	if loader == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "plugin loader not configured")
		return
	}
	var req struct {
		Name         string                 `json:"name"`
		Description  string                 `json:"description"`
		Language     string                 `json:"language"`
		Template     string                 `json:"template"`
		SystemPrompt string                 `json:"system_prompt"`
		Skills       []plugin.SkillManifest `json:"skills"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "name is required")
		return
	}
	if req.Language == "" {
		req.Language = "python"
	}

	safeName := sanitizePluginName(req.Name)
	pluginDir := filepath.Join(loader.Dir(), safeName)
	if _, err := os.Stat(pluginDir); err == nil {
		apperror.WriteCode(w, apperror.CodeInvalidField, "plugin already exists")
		return
	}
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "create plugin dir", err))
		return
	}

	manifest := plugin.Manifest{
		Name:         req.Name,
		Description:  req.Description,
		Language:     req.Language,
		SystemPrompt: req.SystemPrompt,
		Skills:       req.Skills,
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), manifestData, 0644); err != nil {
		os.RemoveAll(pluginDir)
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "write manifest", err))
		return
	}

	handlerName, handlerCode := pluginBoilerplate(req.Language, req.Name, req.Template)
	if err := os.WriteFile(filepath.Join(pluginDir, handlerName), []byte(handlerCode), 0644); err != nil {
		slog.Warn("write handler boilerplate failed", "err", err)
	}
	scaffoldPluginDir(pluginDir, req.Language, req.Name, req.Description)

	loader.LoadAll()
	h.rebuildSkillsFromPlugins()

	slog.Info("plugin created", "name", req.Name, "lang", req.Language, "dir", pluginDir)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{"status": "created", "name": req.Name, "dir": safeName, "full_path": pluginDir})
}

func (h *Handler) handlePluginOpenFolder(w http.ResponseWriter, r *http.Request) {
	loader := h.pluginLoader()
	if loader == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "plugin loader not configured")
		return
	}
	name := r.URL.Query().Get("name")
	var dir string
	if name != "" {
		dir = filepath.Join(loader.Dir(), sanitizePluginName(name))
	} else {
		dir = loader.Dir()
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		apperror.WriteCode(w, apperror.CodeNotFound, "directory not found")
		return
	}
	go openFileExplorer(dir)
	writeJSON(w, map[string]any{"ok": true, "path": dir})
}

func openFileExplorer(dir string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	_ = cmd.Start()
}

func (h *Handler) handlePluginDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "DELETE only")
		return
	}
	loader := h.pluginLoader()
	reg := h.pluginRegistry()
	if loader == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "plugin loader not configured")
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "name is required")
		return
	}

	if name == "general" || name == "education" {
		apperror.WriteCode(w, apperror.CodeInvalidField, "cannot delete built-in plugin")
		return
	}

	pluginDir := filepath.Join(loader.Dir(), sanitizePluginName(name))
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		apperror.WriteCode(w, apperror.CodeNotFound, "plugin not found")
		return
	}

	if err := os.RemoveAll(pluginDir); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "delete plugin dir", err))
		return
	}

	if reg != nil {
		reg.Unregister(name)
	}
	h.rebuildSkillsFromPlugins()

	slog.Info("plugin deleted", "name", name)
	writeJSON(w, map[string]any{"status": "deleted", "name": name})
}

func (h *Handler) handlePluginFiles(w http.ResponseWriter, r *http.Request) {
	loader := h.pluginLoader()
	if loader == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "plugin loader not configured")
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "name is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		pluginDir := filepath.Join(loader.Dir(), sanitizePluginName(name))
		entries, err := os.ReadDir(pluginDir)
		if err != nil {
			writeJSON(w, map[string]any{"files": []any{}, "builtin": true})
			return
		}
		type fileInfo struct {
			Name    string `json:"name"`
			Content string `json:"content"`
			Size    int64  `json:"size"`
		}
		files := make([]fileInfo, 0)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			fi, _ := e.Info()
			data, err := os.ReadFile(filepath.Join(pluginDir, e.Name()))
			content := ""
			if err == nil {
				content = string(data)
			}
			size := int64(0)
			if fi != nil {
				size = fi.Size()
			}
			files = append(files, fileInfo{Name: e.Name(), Content: content, Size: size})
		}
		writeJSON(w, map[string]any{"files": files})

	case http.MethodPut:
		var req struct {
			Plugin  string `json:"plugin"`
			File    string `json:"file"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" {
			apperror.WriteCode(w, apperror.CodeMissingField, "file is required")
			return
		}
		pluginName := name
		if req.Plugin != "" {
			pluginName = req.Plugin
		}
		pluginDir := filepath.Join(loader.Dir(), sanitizePluginName(pluginName))
		if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
			apperror.WriteCode(w, apperror.CodeNotFound, "plugin not found")
			return
		}
		safeFile := filepath.Base(req.File)
		if err := os.WriteFile(filepath.Join(pluginDir, safeFile), []byte(req.Content), 0644); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "write file", err))
			return
		}
		loader.LoadAll()
		h.rebuildSkillsFromPlugins()
		slog.Info("plugin file saved", "plugin", pluginName, "file", safeFile)
		writeJSON(w, map[string]any{"status": "saved"})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT")
	}
}

func (h *Handler) handlePluginReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	loader := h.pluginLoader()
	if loader == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "plugin loader not configured")
		return
	}
	loader.LoadAll()
	skillsCount := h.rebuildSkillsFromPlugins()
	slog.Info("plugins reloaded via API")
	writeJSON(w, map[string]any{"status": "reloaded", "skills": skillsCount})
}

func (h *Handler) handlePluginUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET required")
		return
	}
	reg := h.pluginRegistry()
	if reg == nil {
		writeJSON(w, map[string]any{"tabs": []any{}})
		return
	}
	writeJSON(w, map[string]any{"tabs": reg.AllUITabs()})
}
