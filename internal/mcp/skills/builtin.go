package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"yunque-agent/internal/mcp"
)

// Builtin provides built-in MCP tools: web_fetch, code_exec, file_read, file_write, file_list, configure_settings.
type Builtin struct {
	workDir string
	apiBase string
	apiKey  string
	client  *http.Client
}

// NewBuiltin creates a built-in skill provider.
func NewBuiltin(workDir string) *Builtin {
	if workDir == "" {
		workDir = os.TempDir()
	}
	port := os.Getenv("AGENT_PORT")
	if port == "" {
		port = "9090"
	}
	return &Builtin{
		workDir: workDir,
		apiBase: "http://127.0.0.1:" + port,
		apiKey:  os.Getenv("AGENT_INTERNAL_KEY"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// SetAPIKey sets the API key for authenticated loopback calls.
func (b *Builtin) SetAPIKey(key string) { b.apiKey = key }

func (b *Builtin) ListTools(_ context.Context) ([]mcp.Tool, error) {
	return []mcp.Tool{
		{
			Name:        "web_fetch",
			Description: "Fetch the content of a URL and return the body text (max 32KB).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{"type": "string", "description": "The URL to fetch"},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "code_exec",
			Description: "Execute a code snippet in Python or Node.js and return stdout/stderr.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"language": map[string]any{"type": "string", "enum": []string{"python", "node"}, "description": "Programming language"},
					"code":     map[string]any{"type": "string", "description": "Code to execute"},
				},
				"required": []string{"language", "code"},
			},
		},
		{
			Name:        "file_read",
			Description: "Read the contents of a file in the workspace.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Relative file path"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "file_write",
			Description: "Write content to a file in the workspace.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "Relative file path"},
					"content": map[string]any{"type": "string", "description": "File content"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "file_list",
			Description: "List files and directories in a workspace path.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Relative directory path (default: .)"},
				},
			},
		},
		{
			Name: "configure_settings",
			Description: `Read, preview, or update the agent's configuration.

Workflow: read → understand current state → preview changes → confirm with user → write.

Actions:
- read: returns a compact summary of all settings with current values and available keys.
- preview: shows a diff of current vs proposed values WITHOUT applying. Always preview before write.
- write: applies changes and auto-reloads config (no restart needed for LLM/embedding settings).

Common intent mappings:
- "开启深度思考" / "enable deep thinking" → THINKING_LEVEL=deep
- "换模型" / "switch model" → LLM_MODEL=<model_name>
- "开心跳" / "enable heartbeat" → HEARTBEAT_ENABLED=true
- "改心跳间隔" → HEARTBEAT_INTERVAL=<minutes>
- "接入Telegram" → TELEGRAM_BOT_TOKEN=<token>
- "设快速模型" → LLM_FAST_MODEL=<model>, LLM_FAST_URL=<url>, LLM_FAST_KEY=<key>
- "设专家模型" → LLM_EXPERT_MODEL=<model>
- "开云沙箱" → SANDBOX_CLOUD_ENABLED=true, SANDBOX_CLOUD_API_KEY=<key>
- "改搜索引擎" → SEARXNG_URL=<url>

Config groups: Core LLM, Multi-Model Pool, Advanced Features, Embedding, Channels, File System, Security, Storage, Cloud Sandbox, Other.`,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{"type": "string", "enum": []string{"read", "preview", "write"}, "description": "read: compact settings summary; preview: diff before applying; write: apply and hot-reload"},
					"values": map[string]any{"type": "object", "description": "Key-value pairs to set (required for preview and write). Keys are env var names like LLM_MODEL, THINKING_LEVEL, etc.", "additionalProperties": map[string]any{"type": "string"}},
				},
				"required": []string{"action"},
			},
		},
	}, nil
}

func (b *Builtin) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallResult, error) {
	switch name {
	case "web_fetch":
		return b.webFetch(ctx, args)
	case "code_exec":
		return b.codeExec(ctx, args)
	case "file_read":
		return b.fileRead(args)
	case "file_write":
		return b.fileWrite(args)
	case "file_list":
		return b.fileList(args)
	case "configure_settings":
		return b.configureSettings(ctx, args)
	default:
		return nil, mcp.ErrToolNotFound
	}
}

func (b *Builtin) webFetch(ctx context.Context, args map[string]any) (*mcp.CallResult, error) {
	url := mcp.StringArg(args, "url")
	if url == "" {
		return mcp.ErrorResult("url is required"), nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}
	// Many sites (Stanford, news/CMS, Cloudflare-fronted) 403 or serve a bot
	// wall to non-browser User-Agents, which made web_fetch return useless
	// block pages or fail outright. Present as a normal browser.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8")

	resp, err := b.client.Do(req)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("fetch failed: %v", err)), nil
	}
	defer resp.Body.Close()

	// Read up to 256KB (was 32KB, which truncated most articles before any
	// real body content was captured).
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}

	return mcp.SuccessResult(fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, string(body))), nil
}

func (b *Builtin) codeExec(ctx context.Context, args map[string]any) (*mcp.CallResult, error) {
	lang := mcp.StringArg(args, "language")
	code := mcp.StringArg(args, "code")
	if code == "" {
		return mcp.ErrorResult("code is required"), nil
	}

	var interpreter, ext string
	switch lang {
	case "python":
		interpreter = "python3"
		if runtime.GOOS == "windows" {
			interpreter = "python"
		}
		ext = ".py"
	case "node":
		interpreter = "node"
		ext = ".js"
	default:
		return mcp.ErrorResult("unsupported language: " + lang + " (use python or node)"), nil
	}

	tmpFile, err := os.CreateTemp(b.workDir, "exec_*"+ext)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		return mcp.ErrorResult(err.Error()), nil
	}
	tmpFile.Close()

	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(execCtx, interpreter, tmpPath)
	cmd.Dir = b.workDir

	out, err := cmd.CombinedOutput()
	result := string(out)
	if len(result) > 32*1024 {
		result = result[:32*1024] + "\n...(truncated)"
	}
	if err != nil {
		return mcp.SuccessResult(fmt.Sprintf("Exit error: %v\n\n%s", err, result)), nil
	}
	return mcp.SuccessResult(result), nil
}

func (b *Builtin) fileRead(args map[string]any) (*mcp.CallResult, error) {
	path := mcp.StringArg(args, "path")
	if path == "" {
		return mcp.ErrorResult("path is required"), nil
	}

	fullPath, err := b.resolveWorkPath(path)
	if err != nil {
		return mcp.ErrorResult("path escape not allowed"), nil
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}
	if len(data) > 64*1024 {
		data = append(data[:64*1024], []byte("\n...(truncated)")...)
	}
	return mcp.SuccessResult(string(data)), nil
}

func (b *Builtin) fileWrite(args map[string]any) (*mcp.CallResult, error) {
	path := mcp.StringArg(args, "path")
	content := mcp.StringArg(args, "content")
	if path == "" {
		return mcp.ErrorResult("path is required"), nil
	}

	fullPath, err := b.resolveWorkPath(path)
	if err != nil {
		return mcp.ErrorResult("path escape not allowed"), nil
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("create directory: %v", err)), nil
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}
	return mcp.SuccessResult(fmt.Sprintf("wrote %d bytes to %s", len(content), path)), nil
}

func (b *Builtin) fileList(args map[string]any) (*mcp.CallResult, error) {
	path := mcp.StringArg(args, "path")
	if path == "" {
		path = "."
	}

	fullPath, err := b.resolveWorkPath(path)
	if err != nil {
		return mcp.ErrorResult("path escape not allowed"), nil
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}

	var lines []string
	for _, e := range entries {
		info, _ := e.Info()
		suffix := ""
		size := ""
		if e.IsDir() {
			suffix = "/"
		} else if info != nil {
			size = fmt.Sprintf(" (%d bytes)", info.Size())
		}
		lines = append(lines, fmt.Sprintf("%s%s%s", e.Name(), suffix, size))
	}
	if len(lines) == 0 {
		return mcp.SuccessResult("(empty directory)"), nil
	}
	return mcp.SuccessResult(strings.Join(lines, "\n")), nil
}

func (b *Builtin) configureSettings(ctx context.Context, args map[string]any) (*mcp.CallResult, error) {
	action := mcp.StringArg(args, "action")
	switch action {
	case "read":
		return b.settingsReadCompact(ctx)

	case "preview":
		return b.settingsPreview(ctx, args)

	case "write":
		valuesRaw, _ := args["values"]
		if valuesRaw == nil {
			return nil, fmt.Errorf("values required for write action")
		}
		proposed, _ := valuesRaw.(map[string]any)
		var changedKeys []string
		for k := range proposed {
			changedKeys = append(changedKeys, k)
		}
		sort.Strings(changedKeys)

		body, err := json.Marshal(map[string]any{"values": valuesRaw})
		if err != nil {
			return nil, fmt.Errorf("marshal values: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, b.apiBase+"/api/settings/config", strings.NewReader(string(body)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		b.setAuthHeader(req)
		resp, err := b.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("write config: %w", err)
		}
		defer resp.Body.Close()
		result, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("write config failed (%d): %s", resp.StatusCode, string(result))
		}

		reloadMsg := ""
		if reloadResp, err := b.apiPost(ctx, "/v1/config/reload"); err == nil {
			var rr map[string]any
			if json.Unmarshal([]byte(reloadResp), &rr) == nil && rr["success"] == true {
				reloadMsg = "Hot-reloaded."
			} else {
				reloadMsg = "Saved, hot-reload failed (restart may be needed)."
			}
		} else {
			reloadMsg = "Saved, hot-reload unavailable."
		}

		summary := fmt.Sprintf("Updated %d setting(s): %s. %s", len(changedKeys), strings.Join(changedKeys, ", "), reloadMsg)
		return mcp.SuccessResult(summary), nil

	default:
		return nil, fmt.Errorf("unknown action: %s (use 'read', 'preview', or 'write')", action)
	}
}

// settingsPreview returns a diff-style comparison of current vs proposed values.
func (b *Builtin) settingsPreview(ctx context.Context, args map[string]any) (*mcp.CallResult, error) {
	valuesRaw, _ := args["values"]
	if valuesRaw == nil {
		return mcp.ErrorResult("values required for preview action"), nil
	}
	proposed, ok := valuesRaw.(map[string]any)
	if !ok {
		return mcp.ErrorResult("values must be a key-value object"), nil
	}

	configResp, err := b.apiGet(ctx, "/api/settings/config")
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var configData struct {
		Values map[string]string `json:"values"`
	}
	json.Unmarshal([]byte(configResp), &configData)
	current := configData.Values
	if current == nil {
		current = map[string]string{}
	}

	schemaResp, err := b.apiGet(ctx, "/api/settings/schema")
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	var schemaData struct {
		Groups []struct {
			Fields []struct {
				Key     string `json:"key"`
				LabelZh string `json:"label_zh"`
				Label   string `json:"label"`
			} `json:"fields"`
		} `json:"groups"`
	}
	json.Unmarshal([]byte(schemaResp), &schemaData)
	labelMap := map[string]string{}
	for _, g := range schemaData.Groups {
		for _, f := range g.Fields {
			if f.LabelZh != "" {
				labelMap[f.Key] = f.LabelZh
			} else {
				labelMap[f.Key] = f.Label
			}
		}
	}

	var lines []string
	lines = append(lines, "Configuration changes preview:")
	lines = append(lines, "")
	for k, v := range proposed {
		newVal := fmt.Sprint(v)
		oldVal := current[k]
		label := labelMap[k]
		if label == "" {
			label = k
		}
		if oldVal == newVal {
			lines = append(lines, fmt.Sprintf("  %s (%s): %s (unchanged)", label, k, newVal))
		} else if oldVal == "" {
			lines = append(lines, fmt.Sprintf("  %s (%s): (empty) → %s", label, k, newVal))
		} else {
			lines = append(lines, fmt.Sprintf("  %s (%s): %s → %s", label, k, oldVal, newVal))
		}
	}
	lines = append(lines, "")
	lines = append(lines, "Call with action='write' to apply these changes.")
	return mcp.SuccessResult(strings.Join(lines, "\n")), nil
}

// settingsReadCompact returns a concise summary of current settings.
func (b *Builtin) settingsReadCompact(ctx context.Context) (*mcp.CallResult, error) {
	schemaResp, err := b.apiGet(ctx, "/api/settings/schema")
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	configResp, err := b.apiGet(ctx, "/api/settings/config")
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var schemaData struct {
		Groups []struct {
			Key     string `json:"key"`
			Label   string `json:"label"`
			LabelZh string `json:"label_zh"`
			Fields  []struct {
				Key       string   `json:"key"`
				Label     string   `json:"label"`
				LabelZh   string   `json:"label_zh"`
				Type      string   `json:"type"`
				Options   []string `json:"options"`
				Sensitive bool     `json:"sensitive"`
				Required  bool     `json:"required"`
				Hint      string   `json:"hint"`
			} `json:"fields"`
		} `json:"groups"`
	}
	json.Unmarshal([]byte(schemaResp), &schemaData)

	var configData struct {
		Values map[string]string `json:"values"`
	}
	json.Unmarshal([]byte(configResp), &configData)
	vals := configData.Values
	if vals == nil {
		vals = map[string]string{}
	}

	var lines []string
	lines = append(lines, "Agent Settings Summary")
	lines = append(lines, "======================")
	for _, g := range schemaData.Groups {
		groupLabel := g.LabelZh
		if groupLabel == "" {
			groupLabel = g.Label
		}
		var fieldLines []string
		for _, f := range g.Fields {
			v := vals[f.Key]
			label := f.LabelZh
			if label == "" {
				label = f.Label
			}
			status := ""
			if f.Sensitive && v != "" {
				status = "(set)"
			} else if v != "" {
				status = v
			} else if f.Required {
				status = "⚠ (empty, required)"
			} else {
				continue
			}
			fieldLines = append(fieldLines, fmt.Sprintf("  %s [%s]: %s", label, f.Key, status))
		}
		if len(fieldLines) > 0 {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("── %s ──", groupLabel))
			lines = append(lines, fieldLines...)
		}
	}

	// Available config keys for reference
	lines = append(lines, "")
	lines = append(lines, "All configurable keys:")
	for _, g := range schemaData.Groups {
		for _, f := range g.Fields {
			opts := ""
			if len(f.Options) > 0 {
				opts = " options=" + strings.Join(f.Options, "|")
			}
			lines = append(lines, fmt.Sprintf("  %s (%s)%s", f.Key, f.Type, opts))
		}
	}

	return mcp.SuccessResult(strings.Join(lines, "\n")), nil
}

func (b *Builtin) setAuthHeader(req *http.Request) {
	if b.apiKey != "" {
		req.Header.Set("X-API-Key", b.apiKey)
	}
}

func (b *Builtin) apiGet(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.apiBase+path, nil)
	if err != nil {
		return "", err
	}
	b.setAuthHeader(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s returned %d: %s", path, resp.StatusCode, string(body))
	}
	return string(body), nil
}

func (b *Builtin) apiPost(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.apiBase+path, nil)
	if err != nil {
		return "", err
	}
	b.setAuthHeader(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("POST %s returned %d: %s", path, resp.StatusCode, string(body))
	}
	return string(body), nil
}

func (b *Builtin) resolveWorkPath(path string) (string, error) {
	fullPath := filepath.Join(b.workDir, filepath.Clean(path))
	rel, err := filepath.Rel(b.workDir, fullPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escape not allowed")
	}
	return fullPath, nil
}
