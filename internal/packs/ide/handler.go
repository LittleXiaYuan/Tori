// Package idepack mounts the IDE-supervisor HTTP surface (/v1/ide/review,
// /v1/ide/status) as a v2 capability pack (Tier 0 microkernel). Native pack:
// handler logic lives here and reaches the host only through a narrow accessor
// (code review via the planner, skill count + uptime for status) — the gateway
// no longer hosts these routes.
package idepack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.ide"

const fence = "```"

// Gateway is the narrow host surface the IDE pack needs.
type Gateway interface {
	// ReviewPlan runs a code-review prompt through the host LLM pipeline.
	ReviewPlan(ctx context.Context, tenantID, prompt string) (string, error)
	// TenantOf resolves the request tenant.
	TenantOf(ctx context.Context) string
	// SkillCount reports the number of registered skills (for status).
	SkillCount() int
	// Uptime reports server uptime (for status).
	Uptime() time.Duration
}

// Handler is the IDE pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the IDE pack backed by the host accessors.
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
		h.host.Logger().Info("ide pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the IDE-supervisor surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	m := []string{http.MethodGet, http.MethodPost}
	return []packruntime.BackendRoute{
		{Methods: m, Path: "/v1/ide/review", Handler: h.handleReview},
		{Methods: m, Path: "/v1/ide/status", Handler: h.handleStatus},
	}
}

// handleReview provides structured code review for the IDE plugin (POST).
func (h *Handler) handleReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.gw == nil {
		http.Error(w, "ide pack not wired", http.StatusServiceUnavailable)
		return
	}

	tid := h.gw.TenantOf(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, 512*1024)

	var req struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
		Diff     string `json:"diff"`
		Language string `json:"language"`
		Mode     string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Content == "" && req.Diff == "" {
		http.Error(w, "content or diff required", http.StatusBadRequest)
		return
	}
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

	mode := req.Mode
	switch mode {
	case "full", "diff", "quick":
	default:
		if req.Diff != "" {
			mode = "diff"
		} else {
			mode = "full"
		}
	}

	safePath := sanitizeForPrompt(req.FilePath)
	safeLang := sanitizeForPrompt(req.Language)

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
	default:
		prompt = fmt.Sprintf(
			"你是一位资深代码审查专家。请对以下代码进行全面审查。\n\n"+
				"文件: %s\n语言: %s\n\n"+fence+"\n%s\n"+fence+"\n\n"+
				"请以 JSON 格式返回审查结果: %s\n只返回 JSON，不要其他内容。",
			safePath, safeLang, req.Content, jsonFmt)
	}

	reply, err := h.gw.ReviewPlan(r.Context(), tid, prompt)
	if err != nil {
		http.Error(w, "review failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	parsed := parseReviewJSON(reply)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(parsed)
}

// handleStatus returns server-side capabilities available to the IDE plugin (GET).
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	skillCount := 0
	uptime := time.Duration(0)
	if h.gw != nil {
		skillCount = h.gw.SkillCount()
		uptime = h.gw.Uptime()
	}
	status := map[string]any{
		"version":      "0.1.0",
		"connected":    true,
		"capabilities": []string{"review", "tasks", "missions", "approvals", "workflows", "sse"},
		"skills_count": skillCount,
		"server_time":  time.Now().Format(time.RFC3339),
		"uptime_sec":   int(uptime.Seconds()),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

// parseReviewJSON extracts a structured review result from an LLM reply.
func parseReviewJSON(reply string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(reply), &result); err == nil {
		return result
	}
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
	if idx := strings.Index(reply, "{"); idx >= 0 {
		if end := strings.LastIndex(reply, "}"); end > idx {
			jsonStr := reply[idx : end+1]
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
				return result
			}
		}
	}
	return map[string]any{
		"summary": reply,
		"issues":  []any{},
		"score":   0,
	}
}

// sanitizeForPrompt removes characters that could break prompt structure.
func sanitizeForPrompt(s string) string {
	s = strings.ReplaceAll(s, "`", "'")
	s = strings.ReplaceAll(s, "///system:", "")
	s = strings.ReplaceAll(s, "###system", "")
	r := []rune(s)
	if len(r) > 256 {
		return string(r[:256])
	}
	return s
}
