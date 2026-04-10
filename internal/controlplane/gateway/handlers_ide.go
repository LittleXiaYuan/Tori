package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
)

// ── IDE Supervisor 专用接口 ──────────────────────────────────

const fence = "```"

// handleIDEReviewCode 为 IDE 插件提供结构化代码审查
// POST /v1/ide/review
func (g *Gateway) handleIDEReviewCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tid := tenantFromCtx(r.Context())

	// Limit request body size (max 512KB)
	r.Body = http.MaxBytesReader(w, r.Body, 512*1024)

	var req struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
		Diff     string `json:"diff"`
		Language string `json:"language"`
		Mode     string `json:"mode"` // "full" | "diff" | "quick"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" && req.Diff == "" {
		http.Error(w, "content or diff required", http.StatusBadRequest)
		return
	}

	// Input validation
	if len(req.FilePath) > 512 {
		http.Error(w, "file_path too long", http.StatusBadRequest)
		return
	}
	if len(req.Content) > 200*1024 {
		http.Error(w, "content too large (max 200KB)", http.StatusBadRequest)
		return
	}
	if len(req.Diff) > 200*1024 {
		http.Error(w, "diff too large (max 200KB)", http.StatusBadRequest)
		return
	}
	if len(req.Language) > 50 {
		req.Language = req.Language[:50]
	}

	// Whitelist mode
	mode := req.Mode
	switch mode {
	case "full", "diff", "quick":
		// valid
	default:
		if req.Diff != "" {
			mode = "diff"
		} else {
			mode = "full"
		}
	}

	// Sanitize file path for prompt (prevent path traversal in display)
	safePath := sanitizeForPrompt(req.FilePath)
	safeLang := sanitizeForPrompt(req.Language)

	// 构造 review 提示
	var prompt string
	jsonFmt := `{"summary":"一句话总结","issues":[{"line":N,"severity":"error|warning|info","message":"问题描述","suggestion":"建议修复"}],"score":1-10,"improvements":["改进建议"]}`

	switch mode {
	case "diff":
		prompt = fmt.Sprintf(
			"你是一位资深代码审查专家。请审查以下 Git Diff，返回结构化的审查结果。\n\n"+
				"文件: %s\n语言: %s\n\n"+fence+"diff\n%s\n"+fence+"\n\n"+
				"请以 JSON 格式返回审查结果: %s\n只返回 JSON，不要其他内容。",
			safePath, safeLang, req.Diff, jsonFmt)

	case "quick":
		content := req.Content
		if len(content) > 2000 {
			content = content[:2000] + "\n... (已截断)"
		}
		prompt = fmt.Sprintf(
			"快速检查以下代码片段的明显问题（安全漏洞、严重bug、性能问题），忽略代码风格。\n\n"+
				"文件: %s\n"+fence+"\n%s\n"+fence+"\n\n"+
				"JSON 格式返回: %s\n只返回 JSON。",
			safePath, content, jsonFmt)

	default: // "full"
		prompt = fmt.Sprintf(
			"你是一位资深代码审查专家。请对以下代码进行全面审查。\n\n"+
				"文件: %s\n语言: %s\n\n"+fence+"\n%s\n"+fence+"\n\n"+
				"请以 JSON 格式返回审查结果: %s\n只返回 JSON，不要其他内容。",
			safePath, safeLang, req.Content, jsonFmt)
	}

	// 调用 Planner（复用现有 LLM 管线）
	result, err := g.planner.Run(r.Context(), planner.PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: prompt}},
		TenantID: tid,
	})
	if err != nil {
		slog.Error("ide review failed", "error", err, "file", req.FilePath)
		http.Error(w, "review failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 尝试解析 LLM 返回的 JSON
	parsed := parseReviewJSON(result.Reply)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parsed)
}

// handleIDEStatus 返回 IDE 连接可用的服务端能力
// GET /v1/ide/status
func (g *Gateway) handleIDEStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	skillCount := 0
	if g.registry != nil {
		skillCount = len(g.registry.All())
	}

	status := map[string]any{
		"version":      "0.1.0",
		"connected":    true,
		"capabilities": []string{"review", "tasks", "missions", "approvals", "workflows", "sse"},
		"skills_count": skillCount,
		"server_time":  time.Now().Format(time.RFC3339),
		"uptime_sec":   int(time.Since(g.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// parseReviewJSON 从 LLM 回复中提取结构化 review 结果
func parseReviewJSON(reply string) map[string]any {
	// 尝试直接解析
	var result map[string]any
	if err := json.Unmarshal([]byte(reply), &result); err == nil {
		return result
	}

	// 尝试从 ```json ... ``` 中提取
	jsonTag := fence + "json"
	if idx := strings.Index(reply, jsonTag); idx >= 0 {
		start := idx + len(jsonTag)
		if end := strings.Index(reply[start:], fence); end >= 0 {
			jsonStr := strings.TrimSpace(reply[start : start+end])
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
				return result
			}
		}
	}

	// 尝试从 { ... } 中提取
	if idx := strings.Index(reply, "{"); idx >= 0 {
		if end := strings.LastIndex(reply, "}"); end > idx {
			jsonStr := reply[idx : end+1]
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
				return result
			}
		}
	}

	// 无法解析，返回文本形式
	return map[string]any{
		"summary": reply,
		"issues":  []any{},
		"score":   0,
	}
}

// sanitizeForPrompt removes characters that could break prompt structure.
func sanitizeForPrompt(s string) string {
	// Remove backticks (could break code fences)
	s = strings.ReplaceAll(s, "`", "'")
	// Remove common prompt injection markers
	s = strings.ReplaceAll(s, "///system:", "")
	s = strings.ReplaceAll(s, "###system", "")
	// Limit length
	r := []rune(s)
	if len(r) > 256 {
		return string(r[:256])
	}
	return s
}
