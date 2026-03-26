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

// sanitizePluginName makes a name safe for use as directory name.
func sanitizePluginName(name string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if safe == "" {
		safe = "plugin"
	}
	return safe
}

// pluginBoilerplate generates a starter handler file for the given language and template.
func pluginBoilerplate(lang, pluginName, template string) (filename, code string) {
	// Template-specific Python boilerplate
	if lang == "python" && template != "" && template != "custom" {
		return "handler.py", pythonTemplateCode(pluginName, template)
	}

	switch lang {
	case "python":
		return "handler.py", fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s

Arguments are passed via:
  - stdin (JSON)
  - env var PLUGIN_ARGS (JSON)
  - env var PLUGIN_SKILL (skill name)

Print your result to stdout.
"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    user_input = args.get("input", "")

    # --- Your logic here ---
    result = f"Processed: {user_input}"

    print(result)

if __name__ == "__main__":
    main()
`, pluginName)

	case "node":
		if template == "node_tool" {
			return "handler.js", fmt.Sprintf(`#!/usr/bin/env node
/**
 * Plugin: %s
 *
 * Node.js tool plugin using npm ecosystem.
 * Install dependencies: npm init -y && npm install axios cheerio
 */
const axios = require('axios');

let data = '';
process.stdin.on('data', chunk => { data += chunk; });
process.stdin.on('end', async () => {
  try {
    const args = JSON.parse(data);
    const url = args.url || args.input || '';

    // Example: HTTP GET request
    const response = await axios.get(url, { timeout: 10000 });
    const result = {
      status: response.status,
      content_length: response.data.length,
      preview: typeof response.data === 'string'
        ? response.data.slice(0, 500)
        : JSON.stringify(response.data).slice(0, 500),
    };

    console.log(JSON.stringify(result));
  } catch (err) {
    console.error(JSON.stringify({ error: err.message }));
    process.exit(1);
  }
});
`, pluginName)
		}
		return "handler.js", fmt.Sprintf(`#!/usr/bin/env node
/**
 * Plugin: %s
 *
 * Arguments arrive via stdin (JSON) and env PLUGIN_ARGS.
 * Print your result to stdout.
 */
let data = '';
process.stdin.on('data', chunk => { data += chunk; });
process.stdin.on('end', () => {
  const args = JSON.parse(data);
  const input = args.input || '';

  // --- Your logic here ---
  const result = 'Processed: ' + input;

  console.log(result);
});
`, pluginName)

	case "shell":
		return "handler.sh", fmt.Sprintf(`#!/bin/sh
# Plugin: %s
# Arguments come via stdin (JSON) and env $PLUGIN_ARGS

read INPUT

# --- Your logic here ---
echo "Processed: $INPUT"
`, pluginName)

	default:
		return "handler.py", "# Unsupported language, using Python template\nimport sys\nprint(sys.stdin.read())\n"
	}
}

// pythonTemplateCode generates template-specific Python boilerplate.
func pythonTemplateCode(pluginName, template string) string {
	switch template {
	case "word_doc":
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s — Word Document Processing

Dependencies: pip install python-docx
"""
import json
import sys
from pathlib import Path

def main():
    args = json.loads(sys.stdin.read())
    action = args.get("action", "create")  # create, read, modify
    filepath = args.get("filepath", "output.docx")
    content = args.get("content", "")

    try:
        from docx import Document
    except ImportError:
        print(json.dumps({"error": "python-docx not installed. Run: pip install python-docx"}))
        return

    if action == "create":
        doc = Document()
        doc.add_heading(args.get("title", "Document"), level=1)
        for paragraph in content.split("\n"):
            if paragraph.strip():
                doc.add_paragraph(paragraph.strip())
        doc.save(filepath)
        print(json.dumps({"status": "created", "filepath": filepath}))

    elif action == "read":
        doc = Document(filepath)
        text = "\n".join([p.text for p in doc.paragraphs])
        print(json.dumps({"text": text, "paragraphs": len(doc.paragraphs)}))

    elif action == "modify":
        doc = Document(filepath)
        doc.add_paragraph(content)
        doc.save(filepath)
        print(json.dumps({"status": "modified", "filepath": filepath}))

    else:
        print(json.dumps({"error": f"Unknown action: {action}"}))

if __name__ == "__main__":
    main()
`, pluginName)

	case "excel":
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s — Excel Spreadsheet Processing

Dependencies: pip install openpyxl
"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    action = args.get("action", "create")  # create, read, modify
    filepath = args.get("filepath", "output.xlsx")

    try:
        from openpyxl import Workbook, load_workbook
    except ImportError:
        print(json.dumps({"error": "openpyxl not installed. Run: pip install openpyxl"}))
        return

    if action == "create":
        wb = Workbook()
        ws = wb.active
        ws.title = args.get("sheet_name", "Sheet1")
        headers = args.get("headers", [])
        rows = args.get("rows", [])
        if headers:
            ws.append(headers)
        for row in rows:
            ws.append(row)
        wb.save(filepath)
        print(json.dumps({"status": "created", "filepath": filepath, "rows": len(rows)}))

    elif action == "read":
        wb = load_workbook(filepath)
        ws = wb.active
        data = []
        for row in ws.iter_rows(values_only=True):
            data.append(list(row))
        print(json.dumps({"sheet": ws.title, "rows": len(data), "data": data[:50]}))

    elif action == "modify":
        wb = load_workbook(filepath)
        ws = wb.active
        row_data = args.get("row", [])
        ws.append(row_data)
        wb.save(filepath)
        print(json.dumps({"status": "modified", "total_rows": ws.max_row}))

    else:
        print(json.dumps({"error": f"Unknown action: {action}"}))

if __name__ == "__main__":
    main()
`, pluginName)

	case "api_call":
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s — REST API Caller

Dependencies: pip install requests
"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    url = args.get("url", "")
    method = args.get("method", "GET").upper()
    headers = args.get("headers", {})
    body = args.get("body")

    if not url:
        print(json.dumps({"error": "url is required"}))
        return

    try:
        import requests
    except ImportError:
        print(json.dumps({"error": "requests not installed. Run: pip install requests"}))
        return

    try:
        resp = requests.request(
            method, url,
            headers=headers,
            json=body if body else None,
            timeout=30,
        )
        result = {
            "status_code": resp.status_code,
            "headers": dict(resp.headers),
            "body": resp.text[:2000],
        }
        try:
            result["json"] = resp.json()
        except ValueError:
            pass
        print(json.dumps(result))
    except requests.RequestException as e:
        print(json.dumps({"error": str(e)}))

if __name__ == "__main__":
    main()
`, pluginName)

	case "data_analysis":
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s — Data Analysis

Dependencies: pip install pandas
"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    action = args.get("action", "describe")  # describe, filter, aggregate
    filepath = args.get("filepath", "")
    data_inline = args.get("data")

    try:
        import pandas as pd
    except ImportError:
        print(json.dumps({"error": "pandas not installed. Run: pip install pandas"}))
        return

    # Load data
    if filepath:
        if filepath.endswith(".csv"):
            df = pd.read_csv(filepath)
        elif filepath.endswith(".xlsx"):
            df = pd.read_excel(filepath)
        elif filepath.endswith(".json"):
            df = pd.read_json(filepath)
        else:
            print(json.dumps({"error": f"Unsupported file format: {filepath}"}))
            return
    elif data_inline:
        df = pd.DataFrame(data_inline)
    else:
        print(json.dumps({"error": "filepath or data is required"}))
        return

    if action == "describe":
        desc = df.describe(include="all").to_dict()
        result = {
            "shape": list(df.shape),
            "columns": list(df.columns),
            "dtypes": {k: str(v) for k, v in df.dtypes.items()},
            "describe": desc,
            "head": df.head(5).to_dict(orient="records"),
        }
    elif action == "filter":
        column = args.get("column", "")
        value = args.get("value")
        op = args.get("op", "eq")
        if op == "eq":
            filtered = df[df[column] == value]
        elif op == "gt":
            filtered = df[df[column] > value]
        elif op == "lt":
            filtered = df[df[column] < value]
        elif op == "contains":
            filtered = df[df[column].astype(str).str.contains(str(value), na=False)]
        else:
            filtered = df
        result = {"rows": len(filtered), "data": filtered.head(50).to_dict(orient="records")}
    elif action == "aggregate":
        group_by = args.get("group_by", "")
        agg_col = args.get("agg_column", "")
        agg_func = args.get("agg_func", "sum")
        grouped = df.groupby(group_by)[agg_col].agg(agg_func).reset_index()
        result = {"data": grouped.to_dict(orient="records")}
    else:
        result = {"error": f"Unknown action: {action}"}

    print(json.dumps(result, default=str))

if __name__ == "__main__":
    main()
`, pluginName)

	default:
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    user_input = args.get("input", "")
    result = f"Processed: {user_input}"
    print(result)

if __name__ == "__main__":
    main()
`, pluginName)
	}
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
		"requests_total": g.reqCount,
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
	sb.WriteFile(header.Filename, string(content))
	savedPath := filepath.Join(sb.WorkDir(), header.Filename)

	slog.Info("file uploaded", "tenant", tid, "name", header.Filename, "size", len(content))

	resp := map[string]any{
		"filename": header.Filename,
		"size":     len(content),
		"path":     savedPath,
	}

	snippet := TryParseFile(header.Filename, content)
	if g.planner != nil {
		if analysis, aerr := g.planner.AnalyzeUploadedFile(r.Context(), header.Filename, snippet); aerr != nil {
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
