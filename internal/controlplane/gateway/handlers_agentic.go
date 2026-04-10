package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"yunque-agent/pkg/safego"

	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/apperror"
	channelpkg "yunque-agent/internal/execution/channel"
	"yunque-agent/internal/observe"
)

// ──────────────────────────────────────────────
// Agentic Chat — multi-step streaming with intermediate steps
//
// POST /v1/chat/agentic
//
// Unlike /v1/chat (blocking) or /v1/chat/stream (LLM-token-only streaming),
// this endpoint uses planner.Run() with StepCallback to stream:
//   event: step     → intermediate steps (thinking, tool_start, tool_result, reflect)
//   event: delta    → final reply tokens (streamed)
//   event: done     → complete result
//
// This enables the "Agent thinking process" UX:
//   "好的，让我分析一下..."
//   "🔧 正在调用 [天气查询]..."
//   "✅ [天气查询] 完成"
//   "📧 正在调用 [send_email]..."
//   "✅ [send_email] 完成"
//   "这是最终结果..."
// ──────────────────────────────────────────────

func (g *Gateway) handleAgenticChat(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	ctx, traceSpan := observe.StartTrace(r.Context(), "gateway.handleAgenticChat")
	traceSpan.Attrs["tenant_id"] = tid
	start := time.Now()

	// Quota enforcement
	if !g.usage.CheckQuota(tid) {
		apperror.WriteCode(w, apperror.CodeQuotaExceeded, "quota exceeded")
		observe.EndSpan(traceSpan, nil)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Trace-ID", observe.TraceIDFromContext(ctx))

	flusher, ok := w.(http.Flusher)
	if !ok {
		apperror.WriteCode(w, apperror.CodeInternal, "streaming not supported")
		observe.EndSpan(traceSpan, nil)
		return
	}

	var req struct {
		Messages  []llm.Message `json:"messages"`
		SessionID string        `json:"session_id"`
		TaskID    string        `json:"task_id"`
		Platform  string        `json:"platform,omitempty"`
		Thinking  *bool         `json:"thinking,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sseEvent(w, flusher, "error", `{"code":"BAD_REQUEST","message":"invalid body"}`)
		observe.EndSpan(traceSpan, err)
		return
	}
	if len(req.Messages) == 0 {
		sseEvent(w, flusher, "error", `{"code":"MESSAGES_REQUIRED","message":"messages array is required"}`)
		observe.EndSpan(traceSpan, nil)
		return
	}

	// Chinese guardrail pipeline
	if g.zhGuard != nil && len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1].Content
		guardResult := g.zhGuard.Run(ctx, lastMsg)
		if guardResult.Blocked {
			sseEvent(w, flusher, "error", `{"code":"BLOCKED","message":"内容安全检查未通过"}`)
			observe.EndSpan(traceSpan, nil)
			return
		}
		if guardResult.Redacted != "" {
			req.Messages[len(req.Messages)-1].Content = guardResult.Redacted
		}
	}

	// Session management
	msgs := req.Messages
	if req.SessionID != "" {
		_ = g.convStore.GetOrCreate(req.SessionID, tid)
		if len(req.Messages) <= 1 {
			history := g.convStore.Get(req.SessionID)
			if len(history) > 0 {
				msgs = append(history, req.Messages...)
			}
		}
		if len(req.Messages) > 0 {
			lastMsg := req.Messages[len(req.Messages)-1]
			if lastMsg.Role == "user" {
				g.convStore.Append(req.SessionID, lastMsg)
			}
		}
	}

	msgs = g.augmentMessagesForIntent(msgs, tid)
	// Memory: write user message(s) to short-term
	if g.orchestrator != nil && len(req.Messages) > 0 {
		for _, m := range req.Messages {
			if m.Role == "user" && m.Content != "" {
				_ = g.orchestrator.Ingest(ctx, tid, m.Content, "conversation", "user_input")
			}
		}
	}

	// Smart router
	var routedTier string
	if g.smartRouter != nil && len(msgs) > 0 {
		lastMsg := msgs[len(msgs)-1].Content
		routedModel, tier := g.smartRouter.Route(ctx, lastMsg, false)
		routedTier = tier.String()
		if routedModel != nil {
			traceSpan.Attrs["router_model"] = routedModel.ModelID
		}
	}

	// Task context
	var taskContext string
	if req.TaskID != "" && g.threadMgr != nil {
		threadSID := g.threadMgr.Ensure(req.TaskID, tid)
		req.SessionID = threadSID
		if g.workMemMgr != nil {
			taskContext = g.workMemMgr.RenderForTask(req.TaskID)
		}
	}

	// Emotion analysis
	var emotionHint *emotion.Result
	emotionFeatureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureEmotion)
	if g.emotionAnalyzer != nil && g.emotionAnalyzer.Enabled() && emotionFeatureOK && len(msgs) > 0 {
		lastMsg := msgs[len(msgs)-1]
		if lastMsg.Role == "user" && lastMsg.Content != "" {
			emotionHint, _ = g.emotionAnalyzer.AnalyzeText(ctx, lastMsg.Content)
			if emotionHint != nil && g.emotionHistory != nil {
				g.emotionHistory.Record(req.SessionID, emotionHint.Emotion, emotionHint.Confidence, emotionHint.Source)
			}
		}
	}

	// ── Run planner with StepCallback → SSE ──
	planReq := planner.PlanRequest{
		Messages:        msgs,
		TenantID:        tid,
		ModelOverride:   routedTier,
		EmotionHint:     emotionHint,
		TaskID:          req.TaskID,
		TaskContext:     taskContext,
		ThinkingEnabled: req.Thinking,
		StepCallback: func(event observe.AgentEvent) {
			data, _ := json.Marshal(event)
			sseEvent(w, flusher, event.QualifiedType(), string(data))
			// Record to audit trail
			if g.eventTrail != nil {
				g.eventTrail.Record(event)
			}
		},
		TraceID: traceSpan.TraceID,
	}

	if slashResp, handled, slashErr := g.tryHandleSlashCommand(ctx, planReq); handled {
		if slashErr != nil {
			errData, _ := json.Marshal(map[string]string{"code": "SLASH_COMMAND_ERROR", "message": slashErr.Error()})
			sseEvent(w, flusher, "error", string(errData))
			observe.EndSpan(traceSpan, slashErr)
			return
		}

		reply := slashResp.Result.Reply
		runes := []rune(reply)
		chunkSize := 20
		for i := 0; i < len(runes); i += chunkSize {
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			chunk := string(runes[i:end])
			data, _ := json.Marshal(map[string]string{"content": chunk})
			sseEvent(w, flusher, "delta", string(data))
		}

		doneBytes, _ := json.Marshal(slashResp.Raw)
		sseEvent(w, flusher, "done", string(doneBytes))
		if req.SessionID != "" {
			g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: reply})
		}
		observe.EndSpan(traceSpan, nil)
		return
	}

	result, err := g.planner.Run(ctx, planReq)
	if err != nil {
		slog.Error("agentic planner error", "err", err, "tenant", tid)
		errData, _ := json.Marshal(map[string]string{"code": "PLANNER_ERROR", "message": err.Error()})
		sseEvent(w, flusher, "error", string(errData))
		observe.EndSpan(traceSpan, err)
		return
	}

	// Cache plan result for save_as_workflow skill
	if len(result.Plan) > 0 && g.lastPlanCache != nil {
		g.lastPlanCache.Store(tid, result)
	}

	// Output moderation
	if g.zhGuard != nil && result.Reply != "" {
		outResult := g.zhGuard.Run(ctx, result.Reply)
		if outResult.Redacted != "" {
			result.Reply = outResult.Redacted
		}
	}

	if len(result.Actions) > 0 {
		actJSON, _ := json.Marshal(result.Actions)
		sseEvent(w, flusher, "actions", string(actJSON))
	}

	// Stream reasoning content (thinking) if present
	if result.ReasoningContent != "" {
		thinkData, _ := json.Marshal(map[string]string{"content": result.ReasoningContent})
		sseEvent(w, flusher, "thinking", string(thinkData))
	}

	// Stream final reply as delta events (chunked for smooth UX)
	reply := result.Reply
	runes := []rune(reply)
	chunkSize := 20 // ~20 CJK chars per chunk for smooth streaming
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])
		data, _ := json.Marshal(map[string]string{"content": chunk})
		sseEvent(w, flusher, "delta", string(data))
		time.Sleep(30 * time.Millisecond) // simulate natural typing
	}

	// Done event
	doneData := map[string]any{
		"reply":       reply,
		"skills_used": result.SkillsUsed,
		"steps":       result.Steps,
	}
	if result.ReasoningContent != "" {
		doneData["reasoning_content"] = result.ReasoningContent
	}
	if len(result.Actions) > 0 {
		doneData["actions"] = result.Actions
	}
	if browserSummary := summarizeBrowserPlanArtifact(result.Plan); browserSummary != nil {
		doneData["browser_summary"] = browserSummary
	}
	roots := []string{".", filepath.Join(".", "data"), filepath.Join(".", "data", "output"), filepath.Join(".", "data", "tasks")}
	rich := RenderAgentActions(result.Actions)
	rich = AttachFilesToRich(rich, result, roots)
	if rich != nil && len(rich.Components) > 0 {
		doneData["rich"] = json.RawMessage(rich.ToJSON())
	}
	if result.Plan != nil {
		doneData["plan"] = result.Plan
	}
	if emotionHint != nil && emotionHint.Emotion != emotion.EmotionNeutral {
		doneData["emotion"] = emotionHint
	}
	// Suggestions: follow-up questions + skill save hint
	suggestions := buildSuggestions(result, req.Messages)
	if len(suggestions) > 0 {
		doneData["suggestions"] = suggestions
	}

	doneBytes, _ := json.Marshal(doneData)
	sseEvent(w, flusher, "done", string(doneBytes))

	// Save assistant reply to session
	if req.SessionID != "" {
		g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: reply})
	}

	// Memory: write assistant reply
	if g.orchestrator != nil && reply != "" {
		_ = g.orchestrator.Ingest(ctx, tid, reply, "conversation", "assistant_reply")
	}

	// Memory pipeline (async)
	if g.pipeline != nil && len(req.Messages) > 0 {
		lastUserMsg := ""
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastUserMsg = req.Messages[i].Content
				break
			}
		}
		if lastUserMsg != "" {
			pReply := reply
			pTID := tid
			safego.Go("agentic-memory-pipeline", func() {
				chatMsgs := []memory.ChatMessage{
					{Role: "user", Content: lastUserMsg},
					{Role: "assistant", Content: pReply},
				}
				result, err := g.pipeline.Process(context.Background(), pTID, chatMsgs)
				if err != nil {
					slog.Error("memory pipeline failed", "err", err)
				} else if result != nil && len(result.ExtractedFacts) > 0 {
					slog.Info("memory pipeline extracted", "facts", len(result.ExtractedFacts))
					if g.factHook != nil {
						g.factHook.OnExtracted(result.ExtractedFacts)
					}
				}
			})
		}
	}

	// ReplyHook broadcast
	lastUserMsgForHook := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUserMsgForHook = req.Messages[i].Content
			break
		}
	}
	if g.skillGrow != nil && lastUserMsgForHook != "" {
		growMsg := lastUserMsgForHook
		safego.Go("agentic-skill-grow", func() {
			growCtx, growCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer growCancel()
			g.skillGrow.Observe(growCtx, growMsg)
		})
	}
	g.InvokeReplyHooks(ctx, channelpkg.Message{ChannelType: "webui", Content: lastUserMsgForHook}, channelpkg.Reply{Content: reply})

	// Usage tracking
	estTokens := int64(len(fmt.Sprint(msgs))/4 + len(reply)/4 + 100)
	g.usage.RecordChat(tid, estTokens)
	g.metrics.RecordRequest(time.Since(start), estTokens, estTokens, nil)

	observe.EndSpan(traceSpan, nil)
}

// buildSuggestions generates follow-up suggestions and optional skill-save hint.
func buildSuggestions(result *planner.PlanResult, msgs []llm.Message) []map[string]string {
	if result == nil {
		return nil
	}
	var out []map[string]string

	// Skill save hint: if multiple skills were used, suggest saving as workflow
	if result.Steps >= 3 && len(result.SkillsUsed) >= 2 {
		out = append(out, map[string]string{
			"type":  "save_skill",
			"label": "将此流程保存为可复用技能",
			"icon":  "save",
		})
	}

	userMsg := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			userMsg = msgs[i].Content
			break
		}
	}
	reply := result.Reply
	rUser := []rune(userMsg)
	rReply := []rune(reply)

	// Only generate follow-ups for substantive conversations
	if len(rUser) < 5 || len(rReply) < 20 {
		return out
	}

	// Rule-based follow-up suggestions by detecting task type from skills used
	skillSet := make(map[string]bool)
	for _, s := range result.SkillsUsed {
		skillSet[s] = true
	}

	if skillSet["web_search"] || skillSet["search"] || skillSet["searx"] {
		out = append(out, map[string]string{
			"type": "followup", "label": "帮我深入了解更多细节",
		})
		out = append(out, map[string]string{
			"type": "followup", "label": "整理成一份简报",
		})
	} else if skillSet["file_write"] || skillSet["file_read"] {
		out = append(out, map[string]string{
			"type": "followup", "label": "帮我检查和优化这个文件",
		})
		out = append(out, map[string]string{
			"type": "followup", "label": "还需要做什么修改？",
		})
	} else if skillSet["shell"] || skillSet["run_command"] {
		out = append(out, map[string]string{
			"type": "followup", "label": "执行结果有问题吗？",
		})
		out = append(out, map[string]string{
			"type": "followup", "label": "接下来还需要做什么？",
		})
	} else if len(result.SkillsUsed) > 0 {
		out = append(out, map[string]string{
			"type": "followup", "label": "能再详细解释一下吗？",
		})
		out = append(out, map[string]string{
			"type": "followup", "label": "还有其他相关的事情需要处理吗？",
		})
	} else {
		// Pure conversational reply
		out = append(out, map[string]string{
			"type": "followup", "label": "继续聊这个话题",
		})
	}

	return out
}
