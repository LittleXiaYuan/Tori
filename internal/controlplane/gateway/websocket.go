package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"fmt"

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

// wsConn is a minimal WebSocket connection using raw HTTP hijack + text frames.
// For production, use gorilla/websocket or nhooyr.io/websocket.
// This implementation uses Server-Sent Events (SSE) as a simpler streaming alternative.

func (g *Gateway) handleStreamChat(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	ctx, traceSpan := observe.StartTrace(r.Context(), "gateway.handleStreamChat")
	traceSpan.Attrs["tenant_id"] = tid
	traceSpan.Attrs["req_id"] = RequestID(r.Context())
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
		Messages      []llm.Message `json:"messages"`
		SessionID     string        `json:"session_id"`
		ClassID       string        `json:"class_id"`
		TeacherID     string        `json:"teacher_id"`
		StudentID     string        `json:"student_id"`
		ThinkingLevel string        `json:"thinking_level,omitempty"` // "none" | "auto" | "deep"
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
	if len(req.Messages) > 100 {
		sseEvent(w, flusher, "error", `{"code":"TOO_MANY_MESSAGES","message":"max 100 messages"}`)
		observe.EndSpan(traceSpan, nil)
		return
	}
	for _, m := range req.Messages {
		if len(m.Content) > 32000 {
			sseEvent(w, flusher, "error", `{"code":"MESSAGE_TOO_LONG","message":"max 32000 chars"}`)
			observe.EndSpan(traceSpan, nil)
			return
		}
	}

	// Chinese guardrail pipeline: check input safety (same as handleChat)
	if g.zhGuard != nil && len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1].Content
		guardResult := g.zhGuard.Run(ctx, lastMsg)
		if guardResult.Blocked {
			sseEvent(w, flusher, "error", `{"code":"BLOCKED","message":"内容安全检查未通过: `+guardResult.Rule+`"}`)
			observe.EndSpan(traceSpan, nil)
			return
		}
		if guardResult.Redacted != "" {
			req.Messages[len(req.Messages)-1].Content = guardResult.Redacted
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Session management (same logic as handleChat)
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

	// Memory: write user message(s) to short-term (Mem0-style Auto-Capture)
	if g.orchestrator != nil && len(req.Messages) > 0 {
		for _, m := range req.Messages {
			if m.Role == "user" && m.Content != "" {
				_ = g.orchestrator.Ingest(ctx, tid, m.Content, "conversation", "user_input")
			}
		}
	}

	// Thinking level: per-request override > env default > smart router.
	thinkingLevel := req.ThinkingLevel
	if thinkingLevel == "" {
		thinkingLevel = os.Getenv("THINKING_LEVEL")
	}

	var routedTier string
	switch thinkingLevel {
	case "deep":
		routedTier = "expert"
		traceSpan.Attrs["thinking_level"] = "deep"
	case "none":
		routedTier = "fast"
		traceSpan.Attrs["thinking_level"] = "none"
	default:
		if g.smartRouter != nil && len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1].Content
			routedModel, tier := g.smartRouter.Route(ctx, lastMsg, false)
			routedTier = tier.String()
			traceSpan.Attrs["router_tier"] = routedTier
			if routedModel != nil {
				traceSpan.Attrs["router_model"] = routedModel.ModelID
			}
		}
		traceSpan.Attrs["thinking_level"] = "auto"
	}

	// Emotion analysis (streaming path): detect user emotion before planning
	var streamEmotionHint *struct {
		Emotion    string  `json:"emotion"`
		Confidence float64 `json:"confidence"`
		Source     string  `json:"source"`
	}
	emotionFeatureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureEmotion)
	if g.emotionAnalyzer != nil && g.emotionAnalyzer.Enabled() && emotionFeatureOK && len(msgs) > 0 {
		lastMsg := msgs[len(msgs)-1]
		if lastMsg.Role == "user" && lastMsg.Content != "" {
			if hint, err := g.emotionAnalyzer.AnalyzeText(ctx, lastMsg.Content); err == nil && hint != nil {
				streamEmotionHint = &struct {
					Emotion    string  `json:"emotion"`
					Confidence float64 `json:"confidence"`
					Source     string  `json:"source"`
				}{
					Emotion:    string(hint.Emotion),
					Confidence: hint.Confidence,
					Source:     hint.Source,
				}
				if g.emotionHistory != nil {
					g.emotionHistory.Record(req.SessionID, hint.Emotion, hint.Confidence, hint.Source)
				}
				// Event-driven Reverie trigger: detect significant emotion shifts.
				if g.emotionShift != nil {
					g.emotionShift.Observe(req.SessionID, string(hint.Emotion), hint.Confidence)
				}
			}
		}
	}

	// ── SSE-aware Planner execution via StepCallback ──
	// Use Planner.Run() so tools (docx_create, web_search, etc.) are properly invoked.
	// StepCallback pushes intermediate events (thinking, tool_start, tool_result) as SSE.
	traceID := observe.TraceIDFromContext(ctx)
	stepCB := func(event observe.AgentEvent) {
		stepData, _ := json.Marshal(map[string]any{
			"type":    event.QualifiedType(),
			"summary": event.Summary,
			"detail":  event.Detail,
		})
		sseEvent(w, flusher, "step", string(stepData))
	}

	var emotionHintForPlanner *emotion.Result
	if streamEmotionHint != nil {
		emotionHintForPlanner = &emotion.Result{
			Emotion:    emotion.Emotion(streamEmotionHint.Emotion),
			Confidence: streamEmotionHint.Confidence,
			Source:     streamEmotionHint.Source,
		}
	}

	result, err := g.planner.Run(ctx, planner.PlanRequest{
		Messages:      msgs,
		ClassID:       req.ClassID,
		TeacherID:     req.TeacherID,
		StudentID:     req.StudentID,
		TenantID:      tid,
		ModelOverride: routedTier,
		EmotionHint:   emotionHintForPlanner,
		StepCallback:  stepCB,
		TraceID:       traceID,
	})
	if err != nil {
		slog.Error("planner error (stream)", "err", err, "tenant", tid)
		sseEvent(w, flusher, "error", `{"code":"LLM_ERROR","message":"`+err.Error()+`"}`)
		g.metrics.RecordRequest(time.Since(start), 0, 0, err)
		observe.EndSpan(traceSpan, err)
		return
	}

	reply := result.Reply

	// Stream the final reply text as delta events for smooth UI rendering
	replyChunks := chunkText(reply, 20)
	for _, chunk := range replyChunks {
		data, _ := json.Marshal(map[string]string{"content": chunk})
		sseEvent(w, flusher, "delta", string(data))
	}

	// Output moderation: check agent reply for safety (post-stream)
	if g.zhGuard != nil && reply != "" {
		outResult := g.zhGuard.Run(ctx, reply)
		if outResult.Redacted != "" {
			reply = outResult.Redacted
			result.Reply = reply
		}
	}

	// Send done event with full reply and real planner results
	doneData := map[string]any{
		"reply":       reply,
		"skills_used": result.SkillsUsed,
		"steps":       result.Steps,
	}
	if len(result.Actions) > 0 {
		doneData["actions"] = result.Actions
	}
	roots := []string{".", "data", "data/output", "data/tasks"}
	rich := RenderAgentActions(result.Actions)
	rich = AttachFilesToRich(rich, result, roots)
	if rich != nil && len(rich.Components) > 0 {
		doneData["rich"] = json.RawMessage(rich.ToJSON())
	}
	if result.Plan != nil {
		doneData["plan"] = result.Plan
	}
	if streamEmotionHint != nil && streamEmotionHint.Emotion != "neutral" && streamEmotionHint.Emotion != "unknown" {
		doneData["emotion"] = streamEmotionHint
		stickerFeatureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureSticker)
		wsFred := 2.0
		if g.personaChain != nil {
			wsFred = g.personaChain.FloatFeature(persona.FeatureStickerFrequency, 2)
		}
		if g.stickerMap != nil && stickerFeatureOK && mathRandFloat64() < stickerSendProb(wsFred) {
			if s := g.stickerMap.Suggest(emotion.Emotion(streamEmotionHint.Emotion), "line"); s != nil {
				doneData["sticker_suggestion"] = s
			}
		}
	}
	doneBytes, _ := json.Marshal(doneData)
	sseEvent(w, flusher, "done", string(doneBytes))

	// Save assistant reply to session (with ExecutionSummary for multi-turn continuity)
	if req.SessionID != "" {
		assistantContent := result.Reply
		if summary := result.ExecutionSummary(); summary != "" {
			assistantContent = summary + "\n\n" + assistantContent
		}
		g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: assistantContent})
	}

	// Memory: write assistant reply to short-term
	if g.orchestrator != nil && reply != "" {
		_ = g.orchestrator.Ingest(ctx, tid, reply, "conversation", "assistant_reply")
	}

	// Memory pipeline: extract facts from the LATEST exchange only
	if g.pipeline != nil && len(req.Messages) > 0 {
		lastUserMsg := ""
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastUserMsg = req.Messages[i].Content
				break
			}
		}
		if lastUserMsg != "" {
			pipelineReply := reply
			pipelineTID := tid
		safego.Go("ws-memory-pipeline", func() {
				chatMsgs := []memory.ChatMessage{
					{Role: "user", Content: lastUserMsg},
					{Role: "assistant", Content: pipelineReply},
				}
				result, err := g.pipeline.Process(context.Background(), pipelineTID, chatMsgs)
				if err != nil {
					slog.Error("memory pipeline failed", "err", err, "tenant", pipelineTID)
				} else if result != nil && len(result.ExtractedFacts) > 0 {
					slog.Info("memory pipeline extracted", "facts", len(result.ExtractedFacts), "added", result.Added, "tenant", pipelineTID)
					if g.factHook != nil {
						g.factHook.OnExtracted(result.ExtractedFacts)
					}
				}
			})
		}
	}

	// Learning loop: extract lessons
	if g.learning != nil && len(req.Messages) > 0 {
		userMsg := req.Messages[len(req.Messages)-1].Content
		quality := 7
		if g.learning.Reflect() != nil {
			if eval, err := g.learning.Reflect().Evaluate(ctx, userMsg, reply, nil); err == nil {
				quality = eval.Quality
			}
		}
		g.learning.AfterInteraction(ctx, userMsg, reply, result.SkillsUsed, quality)
	}

	// Adaptive loop: observe interaction
	if g.adaptiveLoop != nil && len(req.Messages) > 0 {
		userMsg := req.Messages[len(req.Messages)-1].Content
	safego.Go("ws-adaptive-loop", func() {
		g.adaptiveLoop.ObserveInteraction(context.Background(), userMsg, reply)
	})
	}

	// ReplyHook broadcast
	lastUserMsgForHook := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUserMsgForHook = req.Messages[i].Content
			break
		}
	}
	g.InvokeReplyHooks(ctx, channelpkg.Message{ChannelType: "webui", Content: lastUserMsgForHook}, channelpkg.Reply{Content: reply})

	// Record usage
	estTokensIn := int64(len(fmt.Sprint(msgs))/4 + 50)
	estTokensOut := int64(len(reply)/4 + 50)
	g.usage.RecordStream(tid, estTokensIn+estTokensOut)
	g.metrics.RecordRequest(time.Since(start), estTokensIn, estTokensOut, nil)
	for _, sk := range result.SkillsUsed {
		g.metrics.RecordSkillCall(sk, 0, nil)
	}

	// Cost tracking (use routed model)
	if g.costTracker != nil {
		model := g.planner.LLMClientFor(routedTier).Model()
		cost, alert := g.costTracker.Record(model, tid, "", req.SessionID, int(estTokensIn), int(estTokensOut), time.Since(start))
		traceSpan.Attrs["cost_usd"] = fmt.Sprintf("%.6f", cost)
		if alert != nil {
			slog.Warn("cost alert", "type", alert.Type, "message", alert.Message)
		}
	}

	observe.EndSpan(traceSpan, nil)
	slog.Info("stream chat done", "tenant", tid, "skills", result.SkillsUsed, "steps", result.Steps, "trace_id", traceSpan.TraceID)
}

type streamEvent struct {
	Type string
	Data any
}

func sseEvent(w http.ResponseWriter, f http.Flusher, event, data string) {
	w.Write([]byte("event: " + event + "\n"))
	// Handle multi-line data
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		w.Write([]byte("data: " + scanner.Text() + "\n"))
	}
	w.Write([]byte("\n"))
	f.Flush()
}

func chunkText(text string, runesPerChunk int) []string {
	runes := []rune(text)
	var chunks []string
	for i := 0; i < len(runes); i += runesPerChunk {
		end := i + runesPerChunk
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	if len(chunks) == 0 {
		chunks = []string{text}
	}
	return chunks
}
