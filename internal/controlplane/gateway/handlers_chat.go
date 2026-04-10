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

	"yunque-agent/pkg/safego"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/apperror"
	channelpkg "yunque-agent/internal/execution/channel"
	"yunque-agent/internal/observe"
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
	ctx, traceSpan := observe.StartTrace(r.Context(), "gateway.handleChat")
	traceSpan.Attrs["tenant_id"] = tid
	traceSpan.Attrs["req_id"] = RequestID(r.Context())
	r = r.WithContext(ctx)
	start := time.Now()

	// Quota enforcement
	if !g.usage.CheckQuota(tid) {
		apperror.WriteCode(w, apperror.CodeQuotaExceeded, "quota exceeded")
		return
	}

	var req struct {
		Messages      []llm.Message `json:"messages"`
		SessionID     string        `json:"session_id"`
		TaskID        string        `json:"task_id"`
		ClassID       string        `json:"class_id"`
		TeacherID     string        `json:"teacher_id"`
		StudentID     string        `json:"student_id"`
		Platform      string        `json:"platform,omitempty"`       // target platform for sticker suggestions
		ThinkingLevel string        `json:"thinking_level,omitempty"` // "none" | "auto" | "deep" → override model tier
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	if len(req.Messages) == 0 {
		apperror.WriteCode(w, apperror.CodeMessageEmpty, "messages array is required")
		return
	}
	if len(req.Messages) > 100 {
		apperror.WriteCode(w, apperror.CodeMessageTooMany, "max 100 messages per request")
		return
	}
	for _, m := range req.Messages {
		if len(m.Content) > 32000 {
			apperror.WriteCode(w, apperror.CodeMessageTooLong, "max 32000 chars per message")
			return
		}
	}

	// Chinese guardrail pipeline: check input safety
	if g.zhGuard != nil && len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1].Content
		guardResult := g.zhGuard.Run(r.Context(), lastMsg)
		if guardResult.Blocked {
			apperror.WriteCode(w, apperror.CodeBadRequest, "内容安全检查未通过: "+guardResult.Rule)
			return
		}
		// Apply redaction if any
		if guardResult.Redacted != "" {
			req.Messages[len(req.Messages)-1].Content = guardResult.Redacted
		}
	}

	// Task thread: when task_id is set, use the task's dedicated conversation thread.
	// This overrides session_id — the task thread becomes the session.
	var taskContext string
	if req.TaskID != "" && g.threadMgr != nil {
		threadSID := g.threadMgr.Ensure(req.TaskID, tid)
		req.SessionID = threadSID
		// Inject task working memory as context
		if g.workMemMgr != nil {
			taskContext = g.workMemMgr.RenderForTask(req.TaskID)
			// Extract user confirmations from the message into working memory
			if len(req.Messages) > 0 {
				lastMsg := req.Messages[len(req.Messages)-1]
				if lastMsg.Role == "user" {
					g.workMemMgr.ExtractConfirmFromThread(req.TaskID, lastMsg.Content)
				}
			}
		}
	}

	// Session management: two modes based on how the client sends messages.
	//  - Web UI sends full conversation history → use it directly, save only the new message
	//  - API clients may send a single message → load history from session store
	msgs := req.Messages
	if req.SessionID != "" {
		_ = g.convStore.GetOrCreate(req.SessionID, tid)
		if len(req.Messages) <= 1 {
			// Single message: API-style call, load server-side history
			history := g.convStore.Get(req.SessionID)
			if len(history) > 0 {
				msgs = append(history, req.Messages...)
			}
		}
		// Save only the last user message (avoid duplicating full history)
		if len(req.Messages) > 0 {
			lastMsg := req.Messages[len(req.Messages)-1]
			if lastMsg.Role == "user" {
				g.convStore.Append(req.SessionID, lastMsg)
			}
		}
	}

	msgs = g.augmentMessagesForIntent(msgs, tid)
	intent := requestIntent{}
	if len(req.Messages) > 0 {
		intent = g.detectRequestIntent(req.Messages[len(req.Messages)-1].Content, tid)
	}
	if intent.RequiresBrowser && !intent.BrowserConnected {
		reply := browserRequirementReply()
		resp := map[string]any{
			"reply":               reply,
			"skills_used":         []string{},
			"steps":               1,
			"browser_requirement": browserRequirementPayload(),
			"suggestions": []map[string]string{
				{"type": "followup", "label": "Open browser setup"},
			},
		}
		if req.SessionID != "" {
			g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: reply})
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Trace-ID", observe.TraceIDFromContext(r.Context()))
		observe.EndSpan(traceSpan, nil)
		json.NewEncoder(w).Encode(resp)
		return
	}
	// ── Memory: write user message(s) to short-term (Mem0-style Auto-Capture) ──
	if g.orchestrator != nil && len(req.Messages) > 0 {
		for _, m := range req.Messages {
			if m.Role == "user" && m.Content != "" {
				_ = g.orchestrator.Ingest(r.Context(), tid, m.Content, "conversation", "user_input")
			}
		}
	}

	// Thinking level: per-request override > env default > smart router.
	// "deep" → expert, "none" → fast, "auto"/empty → smart router.
	thinkingLevel := req.ThinkingLevel
	if thinkingLevel == "" {
		thinkingLevel = os.Getenv("THINKING_LEVEL") // global default from settings
	}

	var routedTier string
	var routedModelID string
	switch thinkingLevel {
	case "deep":
		routedTier = "expert"
		traceSpan.Attrs["thinking_level"] = "deep"
	case "none":
		routedTier = "fast"
		traceSpan.Attrs["thinking_level"] = "none"
	default:
		// Auto: use smart router
		if g.smartRouter != nil && len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1].Content
			routedModel, tier := g.smartRouter.Route(r.Context(), lastMsg, false)
			routedTier = tier.String()
			if routedModel != nil {
				routedModelID = routedModel.ModelID
			}
		}
		traceSpan.Attrs["thinking_level"] = "auto"
	}
	traceSpan.Attrs["router_tier"] = routedTier
	traceSpan.Attrs["router_model"] = routedModelID

	// Budget pre-check
	if g.costTracker != nil {
		estIn := len(fmt.Sprint(msgs))/4 + 50
		estOut := 500 // conservative estimate
		model := g.planner.LLMClientFor(routedTier).Model()
		if g.costTracker.WouldExceedBudget(model, estIn, estOut) {
			apperror.WriteCode(w, apperror.CodeQuotaExceeded, "cost budget would be exceeded")
			return
		}
	}

	// Emotion analysis: detect user emotion from last message (non-blocking, best-effort)
	// Respects per-persona feature toggle: business/tech_expert presets disable emotion by default.
	var emotionHint *emotion.Result
	emotionFeatureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureEmotion)
	if g.emotionAnalyzer != nil && g.emotionAnalyzer.Enabled() && emotionFeatureOK && len(msgs) > 0 {
		lastMsg := msgs[len(msgs)-1].Content
		if msgs[len(msgs)-1].Role == "user" && lastMsg != "" {
			emotionHint, _ = g.emotionAnalyzer.AnalyzeText(r.Context(), lastMsg)
			if emotionHint != nil && g.emotionHistory != nil {
				g.emotionHistory.Record(req.SessionID, emotionHint.Emotion, emotionHint.Confidence, emotionHint.Source)
			}
			// Event-driven Reverie trigger: detect significant emotion shifts.
			if emotionHint != nil && g.emotionShift != nil {
				g.emotionShift.Observe(req.SessionID, string(emotionHint.Emotion), emotionHint.Confidence)
			}
			// Filter by per-persona minimum confidence threshold.
			if emotionHint != nil {
				minConf := 0.5 // default
				if g.personaChain != nil {
					minConf = g.personaChain.FloatFeature(persona.FeatureEmotionMinConfidence, 0.5)
				}
				if emotionHint.Confidence < minConf {
					emotionHint = nil
				}
			}
		}
	}

	planReq := planner.PlanRequest{
		Messages:      msgs,
		ClassID:       req.ClassID,
		TeacherID:     req.TeacherID,
		StudentID:     req.StudentID,
		TenantID:      tid,
		ModelOverride: routedTier,
		EmotionHint:   emotionHint,
		TaskID:        req.TaskID,
		TaskContext:   taskContext,
		TraceID:       traceSpan.TraceID,
	}
	if slashResp, handled, slashErr := g.tryHandleSlashCommand(r.Context(), planReq); handled {
		if slashErr != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeLLMError, "slash command execution failed", slashErr))
			return
		}
		if req.SessionID != "" {
			g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: slashResp.Result.Reply})
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Trace-ID", observe.TraceIDFromContext(r.Context()))
		json.NewEncoder(w).Encode(slashResp.Raw)
		return
	}
	result, err := g.planner.Run(r.Context(), planReq)
	if err != nil {
		slog.Error("planner error", "err", err, "tenant", tid)
		g.metrics.RecordRequest(time.Since(start), 0, 0, err)
		observe.EndSpan(traceSpan, err)

		// Self-heal: attempt to generate a plugin for unsupported tasks
		if g.healer != nil && len(req.Messages) > 0 {
			lastMsg := req.Messages[len(req.Messages)-1].Content
			errMsg := err.Error()
			safego.Go("selfheal", func() {
				healCtx, healCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer healCancel()
				if g.healer.ShouldHeal(lastMsg, errMsg) {
					generated, healErr := g.healer.GenerateAndInstall(healCtx, lastMsg+"\nError: "+errMsg)
					if healErr != nil {
						slog.Warn("selfheal: generation failed", "err", healErr)
					} else {
						slog.Info("selfheal: plugin generated and installed", "name", generated.Name)
					}
				}
			})
		}

		apperror.Write(w, apperror.Wrap(apperror.CodeLLMError, "planner execution failed", err))
		return
	}

	// Record usage (estimate tokens: ~4 chars per token for mixed CJK/EN)
	estTokensIn := int64(len(fmt.Sprint(msgs))/4 + 50)
	estTokensOut := int64(len(result.Reply)/4 + 50)
	g.usage.RecordChat(tid, estTokensIn+estTokensOut)
	g.metrics.RecordRequest(time.Since(start), estTokensIn, estTokensOut, nil)
	for _, sk := range result.SkillsUsed {
		g.metrics.RecordSkillCall(sk, 0, nil)
	}

	// ReplyHook broadcast
	lastUserMsgForHook := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUserMsgForHook = req.Messages[i].Content
			break
		}
	}
	g.InvokeReplyHooks(ctx, channelpkg.Message{ChannelType: "webui", Content: lastUserMsgForHook}, channelpkg.Reply{Content: result.Reply})

	// Cost tracking: record actual token usage (use routed model)
	if g.costTracker != nil {
		model := g.planner.LLMClientFor(routedTier).Model()
		cost, alert := g.costTracker.Record(model, tid, "", req.SessionID, int(estTokensIn), int(estTokensOut), time.Since(start))
		traceSpan.Attrs["cost_usd"] = fmt.Sprintf("%.6f", cost)
		if alert != nil {
			slog.Warn("cost alert", "type", alert.Type, "message", alert.Message)
		}
	}

	// Output moderation: check agent reply for safety
	if g.zhGuard != nil && result.Reply != "" {
		outResult := g.zhGuard.Run(r.Context(), result.Reply)
		if outResult.Redacted != "" {
			result.Reply = outResult.Redacted
		}
	}

	// Save assistant reply to session (with ExecutionSummary for multi-turn continuity)
	if req.SessionID != "" {
		assistantContent := result.Reply
		if summary := result.ExecutionSummary(); summary != "" {
			assistantContent = summary + "\n\n" + assistantContent
		}
		g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: assistantContent})

		// Auto-title: generate a conversation title after the first exchange (like ChatGPT).
		sess := g.convStore.GetSession(req.SessionID)
		if sess != nil && sess.Name == "" && len(req.Messages) > 0 {
			userMsg := req.Messages[len(req.Messages)-1].Content
			assistReply := result.Reply
			sessionID := req.SessionID
			safego.Go("auto-title", func() {
				titleCtx, titleCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer titleCancel()
				title := g.generateConversationTitle(titleCtx, userMsg, assistReply)
				if title != "" {
					g.convStore.Rename(sessionID, title)
					slog.Debug("auto-titled conversation", "session", sessionID, "title", title)
				}
			})
		}
	}

	// ── Memory: write assistant reply to short-term ──
	if g.orchestrator != nil && result.Reply != "" {
		_ = g.orchestrator.Ingest(r.Context(), tid, result.Reply, "conversation", "assistant_reply")
	}

	// Memory pipeline: extract facts from the LATEST exchange only (not full history).
	// Sending full conversation causes token bloat and extraction failure.
	if g.pipeline != nil && len(req.Messages) > 0 {
		lastUserMsg := ""
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastUserMsg = req.Messages[i].Content
				break
			}
		}
		if lastUserMsg != "" {
			pipelineReply := result.Reply
			pipelineTID := tid
			safego.Go("memory-pipeline", func() {
				pipeCtx, pipeCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer pipeCancel()
				chatMsgs := []memory.ChatMessage{
					{Role: "user", Content: lastUserMsg},
					{Role: "assistant", Content: pipelineReply},
				}
				result, err := g.pipeline.Process(pipeCtx, pipelineTID, chatMsgs)
				if err != nil {
					slog.Error("memory pipeline failed", "err", err, "tenant", pipelineTID)
				} else if result != nil && len(result.ExtractedFacts) > 0 {
					slog.Info("memory pipeline extracted", "facts", len(result.ExtractedFacts), "added", result.Added, "tenant", pipelineTID)
					if g.factHook != nil {
						g.factHook.OnExtracted(result.ExtractedFacts)
					}
					g.ingestFactsToRAG(pipeCtx, result.ExtractedFacts)
				}
			})
		}
	}

	// Learning loop: extract lessons with dynamic quality from Reflect
	if g.learning != nil && len(req.Messages) > 0 {
		userMsg := req.Messages[len(req.Messages)-1].Content
		quality := 7 // default
		if g.learning.Reflect() != nil {
			if eval, err := g.learning.Reflect().Evaluate(r.Context(), userMsg, result.Reply, nil); err == nil {
				quality = eval.Quality
			}
		}
		g.learning.AfterInteraction(r.Context(), userMsg, result.Reply, result.SkillsUsed, quality)
	}

	// Adaptive loop: observe interaction for behavior adaptation
	if g.adaptiveLoop != nil && len(req.Messages) > 0 {
		userMsg := req.Messages[len(req.Messages)-1].Content
		safego.Go("adaptive-loop", func() {
			adaptCtx, adaptCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer adaptCancel()
			g.adaptiveLoop.ObserveInteraction(adaptCtx, userMsg, result.Reply)
		})
		// Feed emotion signal into adaptive emoji dimension
		if emotionHint != nil && emotionHint.IsPositive() {
			g.adaptiveLoop.RecordFeedback(adaptive.Feedback{
				Type:        adaptive.FeedbackPreference,
				Dimension:   adaptive.DimEmoji,
				UserMessage: req.Messages[len(req.Messages)-1].Content,
				Correction:  "with_emoji",
			})
		}
	}

	// Skill growth detector: judge whether this user task is becoming a reusable workflow pattern.
	if g.skillGrow != nil && len(req.Messages) > 0 {
		lastUserMsg := ""
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastUserMsg = req.Messages[i].Content
				break
			}
		}
		if lastUserMsg != "" {
			safego.Go("skill-grow", func() {
				growCtx, growCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer growCancel()
				g.skillGrow.Observe(growCtx, lastUserMsg)
			})
		}
	}

	// Skill suggestion: analyze if this conversation could become a reusable skill
	if g.skillSuggester != nil && len(req.Messages) > 0 && len(result.Reply) > 200 {
		lastUserMsg := ""
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastUserMsg = req.Messages[i].Content
				break
			}
		}
		suggestReply := result.Reply
		suggestSkills := result.SkillsUsed
		suggestUserMsg := lastUserMsg
		suggestSID := req.SessionID
		safego.Go("skill-suggest", func() {
			suggestCtx, suggestCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer suggestCancel()
			suggestions, err := g.skillSuggester.Analyze(suggestCtx, suggestUserMsg, suggestReply, suggestSkills)
			if err != nil {
				slog.Debug("skill suggest failed", "err", err)
				return
			}
			if len(suggestions.Suggestions) > 0 {
				g.storePendingSuggestions(suggestSID, suggestions.Suggestions)
				slog.Info("skill suggestions ready", "count", len(suggestions.Suggestions), "session", suggestSID)
			}
		})
	}

	observe.EndSpan(traceSpan, nil)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Trace-ID", observe.TraceIDFromContext(r.Context()))

	// Wrap response: include planner result + optional emotion metadata
	resp := map[string]any{
		"reply":       result.Reply,
		"skills_used": result.SkillsUsed,
		"steps":       result.Steps,
	}
	if len(result.Actions) > 0 {
		resp["actions"] = result.Actions
	}
	roots := []string{".", filepath.Join(".", "data"), filepath.Join(".", "data", "output"), filepath.Join(".", "data", "tasks")}
	rich := RenderAgentActions(result.Actions)
	rich = AttachFilesToRich(rich, result, roots)
	if rich != nil && len(rich.Components) > 0 {
		resp["rich"] = json.RawMessage(rich.ToJSON())
	}
	if result.Plan != nil {
		resp["plan"] = result.Plan
	}
	if emotionHint != nil && emotionHint.Emotion != emotion.EmotionNeutral && emotionHint.Emotion != emotion.EmotionUnknown {
		resp["emotion"] = emotionHint
		// Include sticker suggestion: if platform specified, return that one; otherwise return all platforms.
		stickerFeatureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureSticker)
		freq := 2.0 // default: normal
		if g.personaChain != nil {
			freq = g.personaChain.FloatFeature(persona.FeatureStickerFrequency, 2)
		}
		if g.stickerMap != nil && stickerFeatureOK && mathRandFloat64() < stickerSendProb(freq) {
			if req.Platform != "" {
				if s := g.stickerMap.Suggest(emotionHint.Emotion, req.Platform); s != nil {
					resp["sticker_suggestion"] = s
				}
			} else {
				if multi := g.stickerMap.SuggestMulti(emotionHint.Emotion); len(multi) > 0 {
					resp["sticker_suggestions"] = multi
				}
			}
		}
	}
	json.NewEncoder(w).Encode(resp)
}

// generateConversationTitle uses a fast LLM call to generate a short title for the conversation.
func (g *Gateway) generateConversationTitle(ctx context.Context, userMsg, assistReply string) string {
	client := g.planner.LLMClientFor("fast")
	if client == nil {
		client = g.planner.LLMClientFor("")
	}
	if client == nil {
		return ""
	}

	// Truncate inputs to save tokens
	if len(userMsg) > 300 {
		userMsg = userMsg[:300]
	}
	if len(assistReply) > 300 {
		assistReply = assistReply[:300]
	}

	msgs := []llm.Message{
		{Role: "system", Content: "你是一个对话标题生成器。根据用户的第一条消息和助手的回复，生成一个简短的对话标题（5-15个字）。只输出标题文本，不要加引号、标点或解释。"},
		{Role: "user", Content: fmt.Sprintf("用户消息：%s\n助手回复：%s", userMsg, assistReply)},
	}

	title, err := client.Chat(ctx, msgs, 0.3)
	if err != nil {
		slog.Debug("auto-title generation failed", "err", err)
		return ""
	}

	// Clean up: remove quotes, trim whitespace, limit length
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'「」《》【】")
	title = strings.TrimSpace(title)
	if len([]rune(title)) > 30 {
		title = string([]rune(title)[:30])
	}
	return title
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
// as a persistent RAG source so they can be retrieved in future queries.
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
	slog.Info("facts→RAG ingested", "count", len(facts), "source", name)
}
