package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/apperror"
)

// stickerSendProb returns the probability (0-1) of actually sending a sticker for a given frequency level.
// 0=never, 1=rare(25%), 2=normal(50%), 3=frequent(80%)
func stickerSendProb(freq float64) float64 {
	switch {
	case freq <= 0:
		return 0
	case freq <= 1:
		return 0.25
	case freq <= 2:
		return 0.50
	default:
		return 0.80
	}
}

// mathRandFloat64 returns a random float64 in [0,1). Wraps rand.Float64 for testability.
var mathRandFloat64 = func() float64 { return rand.Float64() }

func (g *Gateway) handleChat(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())

	var httpReq struct {
		Messages      []llm.Message `json:"messages"`
		SessionID     string        `json:"session_id"`
		TaskID        string        `json:"task_id"`
		ClassID       string        `json:"class_id"`
		TeacherID     string        `json:"teacher_id"`
		StudentID     string        `json:"student_id"`
		Platform      string        `json:"platform,omitempty"`
		ThinkingLevel string        `json:"thinking_level,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&httpReq); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	if len(httpReq.Messages) == 0 {
		apperror.WriteCode(w, apperror.CodeMessageEmpty, "messages array is required")
		return
	}
	if len(httpReq.Messages) > 100 {
		apperror.WriteCode(w, apperror.CodeMessageTooMany, "max 100 messages per request")
		return
	}
	for _, m := range httpReq.Messages {
		if len(m.Content) > 32000 {
			apperror.WriteCode(w, apperror.CodeMessageTooLong, "max 32000 chars per message")
			return
		}
	}

	chatReq := &ChatRequest{
		Messages:      httpReq.Messages,
		SessionID:     httpReq.SessionID,
		TaskID:        httpReq.TaskID,
		ClassID:       httpReq.ClassID,
		TeacherID:     httpReq.TeacherID,
		StudentID:     httpReq.StudentID,
		Platform:      httpReq.Platform,
		ThinkingLevel: httpReq.ThinkingLevel,
		TenantID:      tid,
	}

	resp, err := g.ExecuteChatPipeline(r.Context(), chatReq)
	if err != nil {
		if strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "budget") {
			apperror.WriteCode(w, apperror.CodeQuotaExceeded, err.Error())
		} else if strings.Contains(err.Error(), "guardrail") {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		} else {
			slog.Warn("chat pipeline failed", "err", err)
			apperror.Write(w, apperror.New(apperror.CodeLLMError, friendlyChatPipelineError(err)))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if resp.TraceID != "" {
		w.Header().Set("X-Trace-ID", resp.TraceID)
	}

	out := map[string]any{
		"reply":       resp.Reply,
		"skills_used": resp.SkillsUsed,
		"steps":       resp.Steps,
	}
	if len(resp.Actions) > 0 {
		out["actions"] = resp.Actions
	}
	if resp.Rich != nil {
		out["rich"] = resp.Rich
	}
	if resp.Plan != nil {
		out["plan"] = resp.Plan
	}
	if resp.Sandbox != nil {
		out["sandbox"] = resp.Sandbox
	}
	if len(resp.ContextLayers) > 0 {
		out["context_layers"] = resp.ContextLayers
	}
	if resp.EmotionHint != nil && resp.EmotionHint.Emotion != emotion.EmotionNeutral && resp.EmotionHint.Emotion != emotion.EmotionUnknown {
		out["emotion"] = resp.EmotionHint
		if resp.StickerSuggestion != nil {
			out["sticker_suggestion"] = resp.StickerSuggestion
		}
		if resp.StickerMulti != nil && len(resp.StickerMulti) > 0 {
			out["sticker_suggestions"] = resp.StickerMulti
		}
	}
	if resp.BrowserRequired {
		out["browser_requirement"] = resp.BrowserPayload
		out["suggestions"] = []map[string]string{
			{"type": "followup", "label": "Open browser setup"},
		}
	}
	json.NewEncoder(w).Encode(out)
}

func friendlyChatPipelineError(err error) string {
	if err == nil {
		return "任务暂时没有完成，已保留现场，可稍后重试或继续。"
	}
	raw := strings.TrimSpace(err.Error())
	if friendly := plannerKnownFriendlyError(raw); friendly != "" {
		return friendly
	}
	return "任务暂时没有完成，已保留现场，可稍后重试或切换策略继续。"
}

// generateConversationTitle uses a fast LLM call to generate a short title for the conversation.
func (g *Gateway) generateConversationTitle(ctx context.Context, userMsg, assistReply string) string {
	if g == nil || g.planner == nil {
		return ""
	}
	return g.planner.GenerateConversationTitle(ctx, userMsg, assistReply)
}

// storePendingSuggestions saves skill suggestions for a session.
func (g *Gateway) storePendingSuggestions(sessionID string, suggestions []memory.SkillSuggestion) {
	g.pendingSuggestionsMu.Lock()
	defer g.pendingSuggestionsMu.Unlock()
	if g.pendingSuggestions == nil {
		g.pendingSuggestions = make(map[string][]memory.SkillSuggestion)
	}
	g.pendingSuggestions[sessionID] = suggestions
}

// popPendingSuggestions returns and clears skill suggestions for a session.
func (g *Gateway) popPendingSuggestions(sessionID string) []memory.SkillSuggestion {
	g.pendingSuggestionsMu.Lock()
	defer g.pendingSuggestionsMu.Unlock()
	suggestions := g.pendingSuggestions[sessionID]
	delete(g.pendingSuggestions, sessionID)
	return suggestions
}

// handleSkillSuggestions returns pending skill suggestions for a session.
// GET /v1/skill-suggestions?session_id=xxx
func (g *Gateway) handleSkillSuggestions(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	suggestions := g.popPendingSuggestions(sessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"suggestions": suggestions,
	})
}

// ingestFactsToRAG writes extracted conversation facts into the knowledge store
// and persists them to data/knowledge/ so they survive restarts.
func (g *Gateway) ingestFactsToRAG(ctx context.Context, facts []string) {
	if g.knowledgeStore == nil || len(facts) == 0 {
		return
	}
	combined := strings.Join(facts, "\n")
	name := fmt.Sprintf("对话事实 %s", time.Now().Format("2006-01-02 15:04"))
	_, err := g.knowledgeStore.IngestText(name, combined)
	if err != nil {
		slog.Warn("facts→RAG ingest failed", "err", err)
		return
	}
	if err := g.knowledgeStore.BuildIndex(ctx); err != nil {
		slog.Warn("facts→RAG index rebuild failed", "err", err)
	}

	// Persist to disk so facts survive restarts
	if g.knowledgeDir != "" {
		if mkErr := os.MkdirAll(g.knowledgeDir, 0o755); mkErr == nil {
			filename := filepath.Join(g.knowledgeDir, "conversation_facts.md")
			if f, openErr := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); openErr == nil {
				ts := time.Now().Format("2006-01-02 15:04:05")
				fmt.Fprintf(f, "\n## %s\n\n", ts)
				for _, fact := range facts {
					fmt.Fprintf(f, "- %s\n", fact)
				}
				f.Close()
			}
		}
	}

	slog.Info("facts→RAG ingested", "count", len(facts), "source", name)
}
