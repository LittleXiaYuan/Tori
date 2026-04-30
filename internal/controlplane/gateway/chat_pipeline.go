package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/guardrails"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	channelpkg "yunque-agent/internal/execution/channel"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/safego"
)

// ChatRequest is the HTTP-independent representation of a chat request.
type ChatRequest struct {
	Messages      []llm.Message `json:"messages"`
	SessionID     string        `json:"session_id"`
	TaskID        string        `json:"task_id"`
	ClassID       string        `json:"class_id"`
	TeacherID     string        `json:"teacher_id"`
	StudentID     string        `json:"student_id"`
	Platform      string        `json:"platform,omitempty"`
	ThinkingLevel string        `json:"thinking_level,omitempty"`
	TenantID      string        `json:"-"`
}

// ChatResponse is the HTTP-independent result of a chat execution.
type ChatResponse struct {
	Reply             string                  `json:"reply"`
	SkillsUsed        []string                `json:"skills_used"`
	Steps             int                     `json:"steps"`
	Actions           []planner.AgentAction   `json:"actions,omitempty"`
	Plan              []planner.PlanStep      `json:"plan,omitempty"`
	ContextLayers     []string                `json:"context_layers,omitempty"`
	EmotionHint       *emotion.Result         `json:"emotion,omitempty"`
	StickerSuggestion any                     `json:"sticker_suggestion,omitempty"`
	StickerMulti      map[string]*emotion.StickerSuggestion `json:"sticker_suggestions,omitempty"`
	Sandbox           map[string]any          `json:"sandbox,omitempty"`
	Rich              any                     `json:"rich,omitempty"`
	BrowserRequired   bool                    `json:"-"`
	BrowserPayload    any                     `json:"-"`
	TraceID           string                  `json:"-"`
}

// ExecuteChatPipeline runs the full chat pipeline: validation → session
// management → planning → post-processing. Extracted from handleChat to
// separate business logic from HTTP transport, enabling reuse from IM
// channels and direct testing.
func (g *Gateway) ExecuteChatPipeline(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()
	ctx, traceSpan := observe.StartTrace(ctx, "gateway.ExecuteChatPipeline")
	traceSpan.Attrs["tenant_id"] = req.TenantID

	// ── 1. Quota check ──
	if !g.usage.CheckQuota(req.TenantID) {
		observe.EndSpan(traceSpan, fmt.Errorf("quota exceeded"))
		return nil, fmt.Errorf("quota exceeded")
	}

	// ── 2. Input guardrails ──
	if g.zhGuard != nil && len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1].Content
		guardResult := g.zhGuard.Run(ctx, lastMsg)
		if guardResult.Blocked {
			observe.EndSpan(traceSpan, fmt.Errorf("guardrail blocked"))
			return nil, fmt.Errorf("guardrail: %s", guardResult.Rule)
		}
		if guardResult.Redacted != "" {
			req.Messages[len(req.Messages)-1].Content = guardResult.Redacted
		}
	}

	// ── 2b. Unified sanitizer ──
	if g.sanitizer != nil && len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1].Content
		sr := g.sanitizer.Sanitize(ctx, guardrails.SanitizeRequest{
			Input:  lastMsg,
			Source: guardrails.SourceUserPrompt,
		})
		if sr.Blocked {
			observe.EndSpan(traceSpan, fmt.Errorf("sanitizer blocked"))
			return nil, fmt.Errorf("sanitizer: %s (%s)", sr.Rule, sr.ThreatType)
		}
		if sr.Sanitized != "" {
			req.Messages[len(req.Messages)-1].Content = sr.Sanitized
		}
	}

	// ── 3. Task thread binding ──
	var taskContext string
	if req.TaskID != "" && g.threadMgr != nil {
		req.SessionID = g.threadMgr.Ensure(req.TaskID, req.TenantID)
		if g.workMemMgr != nil {
			taskContext = g.workMemMgr.RenderForTask(req.TaskID)
			if len(req.Messages) > 0 {
				lastMsg := req.Messages[len(req.Messages)-1]
				if lastMsg.Role == "user" {
					g.workMemMgr.ExtractConfirmFromThread(req.TaskID, lastMsg.Content)
				}
			}
		}
	}

	// ── 4. Session history ──
	msgs := req.Messages
	if req.SessionID != "" {
		_ = g.convStore.GetOrCreate(req.SessionID, req.TenantID)
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

	msgs = g.augmentMessagesForIntent(msgs, req.TenantID)

	// ── 5. Intent detection & browser requirement check ──
	intent := requestIntent{}
	if len(req.Messages) > 0 {
		intent = g.detectRequestIntent(req.Messages[len(req.Messages)-1].Content, req.TenantID)
	}
	if intent.RequiresBrowser && !intent.BrowserConnected {
		reply := browserRequirementReply()
		if req.SessionID != "" {
			g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: reply})
		}
		observe.EndSpan(traceSpan, nil)
		return &ChatResponse{
			Reply:           reply,
			SkillsUsed:      []string{},
			Steps:           1,
			BrowserRequired: true,
			BrowserPayload:  browserRequirementPayload(),
			TraceID:         observe.TraceIDFromContext(ctx),
		}, nil
	}

	// ── 6. Memory ingestion (user messages) ──
	if g.orchestrator != nil && len(req.Messages) > 0 {
		for _, m := range req.Messages {
			if m.Role == "user" && m.Content != "" {
				_ = g.orchestrator.Ingest(ctx, req.TenantID, m.Content, "conversation", "user_input")
				g.metrics.Cognitive().MemoryIngest.Add(1)
			}
		}
	}

	// ── 7. Model routing ──
	routedTier, _ := g.resolveThinkingLevel(ctx, req.ThinkingLevel, msgs, traceSpan)

	// ── 8. Budget pre-check ──
	if g.costTracker != nil {
		totalChars := 0
		for _, m := range msgs {
			totalChars += len([]rune(m.Content))
			for _, p := range m.ContentParts {
				totalChars += len([]rune(p.Text))
			}
		}
		model := g.planner.LLMClientFor(routedTier).Model()
		if g.costTracker.WouldExceedBudget(model, totalChars/3+50, 500) {
			observe.EndSpan(traceSpan, fmt.Errorf("budget exceeded"))
			return nil, fmt.Errorf("cost budget would be exceeded")
		}
	}

	// ── 9. Emotion analysis ──
	emotionHint := g.analyzeEmotion(ctx, req.SessionID, msgs)

	// ── 10. Session provider override ──
	var sessionClient *llm.Client
	if req.SessionID != "" && g.providerReg != nil {
		if sp := g.providerReg.GetForSession(req.SessionID); sp != nil {
			sessionClient = sp.Client
		}
	}

	// ── 11. Build plan request and execute ──
	planReq := planner.PlanRequest{
		Messages:       msgs,
		ClassID:        req.ClassID,
		TeacherID:      req.TeacherID,
		StudentID:      req.StudentID,
		TenantID:       req.TenantID,
		ModelOverride:  routedTier,
		ClientOverride: sessionClient,
		EmotionHint:    emotionHint,
		TaskID:         req.TaskID,
		TaskContext:    taskContext,
		TraceID:        traceSpan.TraceID,
	}

	if slashResp, handled, slashErr := g.tryHandleSlashCommand(ctx, planReq); handled {
		if slashErr != nil {
			observe.EndSpan(traceSpan, slashErr)
			return nil, fmt.Errorf("slash command: %w", slashErr)
		}
		if req.SessionID != "" {
			g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: slashResp.Result.Reply})
		}
		observe.EndSpan(traceSpan, nil)
		return &ChatResponse{
			Reply:      slashResp.Result.Reply,
			SkillsUsed: slashResp.Result.SkillsUsed,
			Steps:      slashResp.Result.Steps,
			TraceID:    observe.TraceIDFromContext(ctx),
		}, nil
	}

	result, err := g.planner.Run(ctx, planReq)
	if err != nil {
		g.metrics.RecordRequest(time.Since(start), 0, 0, err)
		observe.EndSpan(traceSpan, err)
		g.triggerSelfHeal(ctx, req.Messages, err)
		return nil, fmt.Errorf("planner: %w", err)
	}

	// ── 12. Post-processing ──
	estTokensIn := int64(len(fmt.Sprint(msgs))/4 + 50)
	estTokensOut := int64(len(result.Reply)/4 + 50)
	g.usage.RecordChat(req.TenantID, estTokensIn+estTokensOut)
	g.metrics.RecordRequest(time.Since(start), estTokensIn, estTokensOut, nil)
	for _, sk := range result.SkillsUsed {
		g.metrics.RecordSkillCall(sk, 0, nil)
	}

	g.InvokeReplyHooks(ctx, channelpkg.Message{ChannelType: "webui", Content: lastUserMessage(req.Messages)}, channelpkg.Reply{Content: result.Reply})
	g.recordCost(ctx, req, routedTier, estTokensIn, estTokensOut, start, traceSpan)

	if g.zhGuard != nil && result.Reply != "" {
		outResult := g.zhGuard.Run(ctx, result.Reply)
		if outResult.Redacted != "" {
			result.Reply = outResult.Redacted
		}
	}

	g.persistChatResult(ctx, req, result)
	g.runPostChatHooks(ctx, req, result, emotionHint)

	observe.EndSpan(traceSpan, nil)

	// ── 13. Build response ──
	resp := &ChatResponse{
		Reply:         result.Reply,
		SkillsUsed:    result.SkillsUsed,
		Steps:         result.Steps,
		Actions:       result.Actions,
		ContextLayers: result.ContextLayers,
		EmotionHint:   emotionHint,
		TraceID:       observe.TraceIDFromContext(ctx),
	}
	if result.Plan != nil {
		resp.Plan = result.Plan
		resp.Sandbox = extractSandboxFromPlan(result.Plan)
	}
	roots := []string{".", filepath.Join(".", "data"), filepath.Join(".", "data", "output"), filepath.Join(".", "data", "tasks")}
	rich := RenderAgentActions(result.Actions)
	rich = AttachFilesToRich(rich, result, roots)
	if rich != nil && len(rich.Components) > 0 {
		resp.Rich = json.RawMessage(rich.ToJSON())
	}
	g.attachStickerSuggestions(resp, emotionHint, req.Platform)
	return resp, nil
}

// resolveThinkingLevel determines the model tier based on thinking level.
func (g *Gateway) resolveThinkingLevel(ctx context.Context, level string, msgs []llm.Message, span *observe.Span) (tier, modelID string) {
	if level == "" {
		level = os.Getenv("THINKING_LEVEL")
	}
	switch level {
	case "deep":
		span.Attrs["thinking_level"] = "deep"
		return "expert", ""
	case "none":
		span.Attrs["thinking_level"] = "none"
		return "fast", ""
	default:
		span.Attrs["thinking_level"] = "auto"
		if g.smartRouter != nil && len(msgs) > 0 {
			routedModel, t := g.smartRouter.Route(ctx, msgs[len(msgs)-1].Content, false)
			if routedModel != nil {
				modelID = routedModel.ModelID
			}
			return t.String(), modelID
		}
		return "", ""
	}
}

// analyzeEmotion runs async emotion analysis on the last user message.
func (g *Gateway) analyzeEmotion(ctx context.Context, sessionID string, msgs []llm.Message) *emotion.Result {
	featureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureEmotion)
	if g.emotionAnalyzer == nil || !g.emotionAnalyzer.Enabled() || !featureOK || len(msgs) == 0 {
		return nil
	}
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Role != "user" || lastMsg.Content == "" {
		return nil
	}

	emotionCh := make(chan *emotion.Result, 1)
	emotionCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	safego.Go("chat-emotion-analyze", func() {
		hint, _ := g.emotionAnalyzer.AnalyzeText(emotionCtx, lastMsg.Content)
		emotionCh <- hint
	})

	var hint *emotion.Result
	select {
	case hint = <-emotionCh:
	case <-emotionCtx.Done():
		slog.Debug("chat: emotion analysis timed out")
	}
	cancel()

	if hint == nil {
		return nil
	}

	if g.emotionHistory != nil {
		g.emotionHistory.Record(sessionID, hint.Emotion, hint.Confidence, hint.Source)
	}
	if g.emotionShift != nil {
		g.emotionShift.Observe(sessionID, string(hint.Emotion), hint.Confidence)
	}

	minConf := 0.5
	if g.personaChain != nil {
		minConf = g.personaChain.FloatFeature(persona.FeatureEmotionMinConfidence, 0.5)
	}
	if hint.Confidence < minConf {
		return nil
	}
	return hint
}

// triggerSelfHeal attempts to generate a plugin for unsupported tasks.
func (g *Gateway) triggerSelfHeal(ctx context.Context, messages []llm.Message, planErr error) {
	if g.healer == nil || len(messages) == 0 {
		return
	}
	lastMsg := messages[len(messages)-1].Content
	errMsg := planErr.Error()
	safego.Go("selfheal", func() {
		healCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if g.healer.ShouldHeal(lastMsg, errMsg) {
			generated, err := g.healer.GenerateAndInstall(healCtx, lastMsg+"\nError: "+errMsg)
			if err != nil {
				slog.Warn("selfheal: generation failed", "err", err)
			} else {
				slog.Info("selfheal: plugin generated and installed", "name", generated.Name)
			}
		}
	})
}

// recordCost tracks token cost for the request.
func (g *Gateway) recordCost(ctx context.Context, req *ChatRequest, tier string, tokensIn, tokensOut int64, start time.Time, span *observe.Span) {
	if g.costTracker == nil {
		return
	}
	model := g.planner.LLMClientFor(tier).Model()
	cost, alert := g.costTracker.Record(model, req.TenantID, "", req.SessionID, int(tokensIn), int(tokensOut), time.Since(start))
	span.Attrs["cost_usd"] = fmt.Sprintf("%.6f", cost)
	if alert != nil {
		slog.Warn("cost alert", "type", alert.Type, "message", alert.Message)
	}
}

// persistChatResult saves the assistant reply to the session and triggers auto-titling.
func (g *Gateway) persistChatResult(ctx context.Context, req *ChatRequest, result *planner.PlanResult) {
	if req.SessionID == "" {
		return
	}

	assistantContent := result.Reply
	if summary := result.ExecutionSummary(); summary != "" {
		assistantContent = summary + "\n\n" + assistantContent
	}
	var sandboxInfo map[string]any
	if result.Plan != nil {
		sandboxInfo = extractSandboxFromPlan(result.Plan)
	}
	assistantContent = embedSandboxMarker(assistantContent, sandboxInfo)
	g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: assistantContent})

	sess := g.convStore.GetSession(req.SessionID)
	if sess != nil && sess.Name == "" && len(req.Messages) > 0 {
		userMsg := req.Messages[len(req.Messages)-1].Content
		assistReply := result.Reply
		sessionID := req.SessionID
		safego.Go("auto-title", func() {
			titleCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			title := g.generateConversationTitle(titleCtx, userMsg, assistReply)
			if title != "" {
				g.convStore.Rename(sessionID, title)
			}
		})
	}
}

// runPostChatHooks triggers async post-processing: memory pipeline, learning loop,
// adaptive loop, skill growth, and skill suggestions.
func (g *Gateway) runPostChatHooks(ctx context.Context, req *ChatRequest, result *planner.PlanResult, emotionHint *emotion.Result) {
	if g.orchestrator != nil && result.Reply != "" {
		_ = g.orchestrator.Ingest(ctx, req.TenantID, result.Reply, "conversation", "assistant_reply")
		g.metrics.Cognitive().MemoryIngest.Add(1)
	}

	userMsg := lastUserMessage(req.Messages)

	// Memory pipeline
	if g.pipeline != nil && userMsg != "" {
		reply := result.Reply
		tid := req.TenantID
		safego.Go("memory-pipeline", func() {
			pCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			chatMsgs := []memory.ChatMessage{
				{Role: "user", Content: userMsg},
				{Role: "assistant", Content: reply},
			}
			pResult, err := g.pipeline.Process(pCtx, tid, chatMsgs)
			if err != nil {
				slog.Error("memory pipeline failed", "err", err, "tenant", tid)
			} else if pResult != nil && len(pResult.ExtractedFacts) > 0 {
				if g.factHook != nil {
					g.factHook.OnExtracted(pResult.ExtractedFacts)
				}
				g.ingestFactsToRAG(pCtx, pResult.ExtractedFacts)
			}
		})
	}

	// Learning loop
	if g.learning != nil && userMsg != "" {
		quality := 7
		if g.learning.Reflect() != nil {
			if eval, err := g.learning.Reflect().Evaluate(ctx, userMsg, result.Reply, nil); err == nil {
				quality = eval.Quality
			}
		}
		g.learning.AfterInteraction(ctx, userMsg, result.Reply, result.SkillsUsed, quality)
	}

	// Adaptive loop
	if g.adaptiveLoop != nil && userMsg != "" {
		reply := result.Reply
		safego.Go("adaptive-loop", func() {
			aCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			g.adaptiveLoop.ObserveInteraction(aCtx, userMsg, reply)
		})
		if emotionHint != nil && emotionHint.IsPositive() {
			g.adaptiveLoop.RecordFeedback(adaptive.Feedback{
				Type:        adaptive.FeedbackPreference,
				Dimension:   adaptive.DimEmoji,
				UserMessage: userMsg,
				Correction:  "with_emoji",
			})
		}
	}

	// Skill growth
	if g.skillGrow != nil && userMsg != "" {
		actions := result.SkillsUsed
		safego.Go("skill-grow", func() {
			gCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			g.skillGrow.Observe(gCtx, userMsg)
			if len(actions) > 0 {
				g.skillGrow.ObserveActions(gCtx, actions)
			}
		})
	}

	// Skill suggestion
	g.suggestCounter++
	if g.skillSuggester != nil && userMsg != "" && len(result.Reply) > 500 && g.suggestCounter%5 == 0 {
		reply := result.Reply
		skills := result.SkillsUsed
		sid := req.SessionID
		safego.Go("skill-suggest", func() {
			sCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			suggestions, err := g.skillSuggester.Analyze(sCtx, userMsg, reply, skills)
			if err == nil && len(suggestions.Suggestions) > 0 {
				g.storePendingSuggestions(sid, suggestions.Suggestions)
			}
		})
	}
}

// attachStickerSuggestions adds sticker suggestions to the response.
func (g *Gateway) attachStickerSuggestions(resp *ChatResponse, emotionHint *emotion.Result, platform string) {
	if emotionHint == nil || emotionHint.Emotion == emotion.EmotionNeutral || emotionHint.Emotion == emotion.EmotionUnknown {
		return
	}
	featureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureSticker)
	freq := 2.0
	if g.personaChain != nil {
		freq = g.personaChain.FloatFeature(persona.FeatureStickerFrequency, 2)
	}
	if g.stickerMap == nil || !featureOK || mathRandFloat64() >= stickerSendProb(freq) {
		return
	}
	if platform != "" {
		resp.StickerSuggestion = g.stickerMap.Suggest(emotionHint.Emotion, platform)
	} else {
		resp.StickerMulti = g.stickerMap.SuggestMulti(emotionHint.Emotion)
	}
}

func lastUserMessage(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return ""
}

