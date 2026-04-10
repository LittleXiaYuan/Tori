package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/pkg/plugin"
)

func (g *Gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g.metrics.Snapshot())
}

func (g *Gateway) handleMetricsPrometheus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(g.metrics.PrometheusFormat()))
}

func (g *Gateway) handleSkills(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	type skillInfo struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	}
	out := make([]skillInfo, 0)
	if g.registry != nil {
		for _, s := range g.registry.All() {
			out = append(out, skillInfo{Name: s.Name(), Description: s.Description(), Parameters: s.Parameters()})
		}
	}
	json.NewEncoder(w).Encode(map[string]any{"skills": out, "count": len(out)})
}

func (g *Gateway) handleSkillsDynamicGet(w http.ResponseWriter, r *http.Request) {
	if g.registry == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "skill registry not configured")
		return
	}
	allSkills := g.registry.All()
	var dynamic []task.DynamicSkillDef
	for _, sk := range allSkills {
		if ds, ok := sk.(*task.DynamicSkill); ok {
			dynamic = append(dynamic, ds.Def())
		}
	}
	if dynamic == nil {
		dynamic = []task.DynamicSkillDef{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"skills": dynamic})
}

func (g *Gateway) handleSkillsDynamicApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Instruction string `json:"instruction,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeInvalidField, "invalid request")
		return
	}
	sk, ok := g.registry.Get(req.Name)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "skill not found")
		return
	}
	if ds, ok := sk.(*task.DynamicSkill); ok {
		ds.SetApprovalStatus("approved")
		if req.Instruction != "" {
			ds.UpdateInstruction(req.Instruction)
		}
		if err := task.SaveDynamicSkills(g.registry, "data/dynamic_skills.json"); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "save dynamic skills", err))
			return
		}
	} else {
		apperror.WriteCode(w, apperror.CodeInvalidField, "not a dynamic skill")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *Gateway) handleSkillsDynamicReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeInvalidField, "invalid request")
		return
	}
	sk, ok := g.registry.Get(req.Name)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "skill not found")
		return
	}
	if _, ok := sk.(*task.DynamicSkill); !ok {
		apperror.WriteCode(w, apperror.CodeInvalidField, "not a dynamic skill")
		return
	}
	g.registry.Remove(req.Name)
	if err := task.SaveDynamicSkills(g.registry, "data/dynamic_skills.json"); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "save dynamic skills", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *Gateway) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	t := g.tenants.Register(req.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

func (g *Gateway) handleListTenants(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	list := g.tenants.List()
	json.NewEncoder(w).Encode(map[string]any{"tenants": list, "count": len(list)})
}

func (g *Gateway) handlePlugins(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"plugins": g.pluginReg.AllIncludeDisabled()})
}

func (g *Gateway) handlePluginToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
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
	ok := g.pluginReg.SetEnabled(req.Name, req.Enabled)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "plugin not found")
		return
	}

	// Rebuild skill registry and planner domain prompt from enabled plugins
	g.registry.Clear()
	for _, s := range g.pluginReg.AllSkills() {
		g.registry.Register(s)
	}
	g.planner.SetDomainPrompt(g.pluginReg.CombinedPrompt())
	slog.Info("plugin toggled, skills rebuilt", "plugin", req.Name, "enabled", req.Enabled, "skills", len(g.registry.All()))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"name": req.Name, "enabled": req.Enabled, "skills_count": len(g.registry.All())})
}

func (g *Gateway) handlePluginCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.pluginLoader == nil {
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

	// Sanitize plugin name (directory-safe)
	safeName := sanitizePluginName(req.Name)
	pluginDir := filepath.Join(g.pluginLoader.Dir(), safeName)
	if _, err := os.Stat(pluginDir); err == nil {
		apperror.WriteCode(w, apperror.CodeInvalidField, "plugin already exists")
		return
	}
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "create plugin dir", err))
		return
	}

	// Write manifest
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

	// Generate boilerplate handler
	handlerName, handlerCode := pluginBoilerplate(req.Language, req.Name, req.Template)
	if err := os.WriteFile(filepath.Join(pluginDir, handlerName), []byte(handlerCode), 0644); err != nil {
		slog.Warn("write handler boilerplate failed", "err", err)
	}

	// Hot-load the new plugin
	g.pluginLoader.LoadAll()
	g.rebuildSkillsFromPlugins()

	slog.Info("plugin created", "name", req.Name, "lang", req.Language, "dir", pluginDir)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"status": "created", "name": req.Name, "dir": safeName})
}

func (g *Gateway) handlePluginDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "DELETE only")
		return
	}
	if g.pluginLoader == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "plugin loader not configured")
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "name is required")
		return
	}

	// Prevent deleting built-in plugins
	if name == "general" || name == "education" {
		apperror.WriteCode(w, apperror.CodeInvalidField, "cannot delete built-in plugin")
		return
	}

	pluginDir := filepath.Join(g.pluginLoader.Dir(), sanitizePluginName(name))
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		apperror.WriteCode(w, apperror.CodeNotFound, "plugin not found")
		return
	}

	if err := os.RemoveAll(pluginDir); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "delete plugin dir", err))
		return
	}

	g.pluginReg.Unregister(name)
	g.rebuildSkillsFromPlugins()

	slog.Info("plugin deleted", "name", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "deleted", "name": name})
}

func (g *Gateway) handlePluginFiles(w http.ResponseWriter, r *http.Request) {
	if g.pluginLoader == nil {
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
		// List files
		pluginDir := filepath.Join(g.pluginLoader.Dir(), sanitizePluginName(name))
		entries, err := os.ReadDir(pluginDir)
		if err != nil {
			// Might be a built-in plugin with no directory
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"files": []any{}, "builtin": true})
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"files": files})

	case http.MethodPut:
		// Save file
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
		pluginDir := filepath.Join(g.pluginLoader.Dir(), sanitizePluginName(pluginName))
		if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
			apperror.WriteCode(w, apperror.CodeNotFound, "plugin not found")
			return
		}
		// Prevent path traversal
		safeFile := filepath.Base(req.File)
		if err := os.WriteFile(filepath.Join(pluginDir, safeFile), []byte(req.Content), 0644); err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "write file", err))
			return
		}
		// Reload plugin
		g.pluginLoader.LoadAll()
		g.rebuildSkillsFromPlugins()
		slog.Info("plugin file saved", "plugin", pluginName, "file", safeFile)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "saved"})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT")
	}
}

// rebuildSkillsFromPlugins rebuilds the skill registry and planner domain prompt.
func (g *Gateway) rebuildSkillsFromPlugins() {
	g.registry.Clear()
	for _, s := range g.pluginReg.AllSkills() {
		g.registry.Register(s)
	}
	g.planner.SetDomainPrompt(g.pluginReg.CombinedPrompt())
}

func (g *Gateway) handleSandboxExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Command == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "command is required")
		return
	}
	sb, err := sandbox.New("", sandbox.DefaultPolicy())
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeSandboxError, "sandbox init failed", err))
		return
	}
	defer sb.Cleanup()
	result, err := sb.Exec(r.Context(), req.Command, req.Args...)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeSandboxError, "sandbox exec failed", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (g *Gateway) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	info := sandbox.SystemInfo()
	breaker := g.planner.LLMBreaker()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"system":  info,
		"breaker": map[string]any{"state": breaker.State(), "failures": breaker.Failures()},
	})
}

func (g *Gateway) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"requests_total": g.reqCount.Load(),
		"tenants":        len(g.tenants.List()),
		"skills":         len(g.registry.All()),
		"plugins":        len(g.pluginReg.AllIncludeDisabled()),
		"scheduler_jobs": len(g.scheduler.List()),
		"conversations":  g.convStore.Count(),
		"memory":         g.memory.Stats(tid),
	})
}

func (g *Gateway) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := map[string]any{}
	if g.planner != nil && g.planner.LLMClient() != nil && g.planner.LLMClient().Cache() != nil {
		stats["llm_response_cache"] = g.planner.LLMClient().Cache().Stats()
	}
	json.NewEncoder(w).Encode(stats)
}

func (g *Gateway) handleTokenGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Authenticate via API Key to issue JWT
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	t := g.tenants.ByAPIKey(apiKey)
	if t == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid api key"})
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Role == "" {
		req.Role = "user"
	}

	// Security: only allow "user" role via API Key token exchange.
	// Admin tokens must be issued through a different mechanism.
	allowedRoles := map[string]bool{"user": true, "viewer": true}
	if !allowedRoles[req.Role] {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "role not allowed via API key exchange"})
		return
	}

	if g.jwtCfg == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "jwt not configured"})
		return
	}

	token, err := GenerateJWT(*g.jwtCfg, t.ID, req.Role)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token, "type": "Bearer"})
}

func (g *Gateway) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	// Max 32MB
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		apperror.WriteCode(w, apperror.CodeMissingField, "file field required")
		return
	}
	defer file.Close()

	filename := filepath.Base(strings.TrimSpace(header.Filename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid filename")
		return
	}

	// Security: validate file extension to prevent dangerous uploads
	ext := strings.ToLower(filepath.Ext(filename))
	blockedExts := map[string]bool{
		".exe": true, ".bat": true, ".cmd": true, ".com": true, ".msi": true,
		".sh": true, ".bash": true, ".ps1": true, ".vbs": true, ".wsf": true,
		".scr": true, ".pif": true, ".dll": true, ".so": true, ".dylib": true,
	}
	if blockedExts[ext] {
		apperror.WriteCode(w, apperror.CodeBadRequest, "file type not allowed: "+ext)
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "read file failed", err))
		return
	}

	// Store in sandbox workspace
	tid := tenantFromCtx(r.Context())
	sb, sbErr := sandbox.New("", sandbox.DefaultPolicy())
	if sbErr != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeSandboxError, "sandbox init failed", sbErr))
		return
	}
	sb.WriteFile(filename, string(content))
	savedPath := filepath.Join(sb.WorkDir(), filename)

	slog.Info("file uploaded", "tenant", tid, "name", filename, "size", len(content))

	resp := map[string]any{
		"filename": filename,
		"size":     len(content),
		"path":     savedPath,
	}

	snippet := TryParseFile(filename, content)
	if isMinerUSupportedExt(ext) && g.documentParser != nil && g.documentParser.Enabled() {
		if parsed, perr := g.parseFileWithMinerU(r.Context(), filename, content); perr != nil {
			slog.Warn("upload MinerU parse failed", "name", filename, "err", perr)
		} else {
			snippet = parsed.Markdown
			resp["parse"] = parsed.Parse
		}
	}
	if g.planner != nil {
		if analysis, aerr := g.planner.AnalyzeUploadedFile(r.Context(), filename, snippet); aerr != nil {
			slog.Debug("upload template analysis skipped", "err", aerr)
		} else {
			actions := planner.AnalysisToActions(savedPath, analysis)
			if len(actions) > 0 {
				resp["analysis"] = analysis
				resp["actions"] = actions
				if rich := RenderAgentActions(actions); rich != nil {
					resp["rich"] = json.RawMessage(rich.ToJSON())
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (g *Gateway) handleSchedulerJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	jobs := g.scheduler.List()
	json.NewEncoder(w).Encode(map[string]any{"jobs": jobs, "count": len(jobs)})
}

func (g *Gateway) handleSchedulerAdd(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	var req struct {
		Name     string `json:"name"`
		Prompt   string `json:"prompt"`
		Interval string `json:"interval"` // e.g. "1h", "30m", "24h"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Prompt == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "name and prompt are required")
		return
	}
	dur, err := time.ParseDuration(req.Interval)
	if err != nil || dur < time.Minute {
		apperror.WriteCode(w, apperror.CodeInvalidField, "invalid interval (min 1m)")
		return
	}
	job := scheduler.Job{
		ID:       fmt.Sprintf("job_%d", time.Now().UnixNano()),
		Name:     req.Name,
		TenantID: tid,
		Interval: dur,
		Prompt:   req.Prompt,
	}
	g.scheduler.Add(job)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (g *Gateway) handleSchedulerRemove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "id is required")
		return
	}
	g.scheduler.Remove(req.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}
