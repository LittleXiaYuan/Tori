package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/guardrails"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	channelpkg "yunque-agent/internal/execution/channel"
	"yunque-agent/internal/observe"
)

// ChatRequest is the HTTP-independent representation of a chat request.
type ChatRequest struct {
	Messages       []llm.Message `json:"messages"`
	SessionID      string        `json:"session_id"`
	TaskID         string        `json:"task_id"`
	ClassID        string        `json:"class_id"`
	TeacherID      string        `json:"teacher_id"`
	StudentID      string        `json:"student_id"`
	Platform       string        `json:"platform,omitempty"`
	ThinkingLevel  string        `json:"thinking_level,omitempty"`
	WorkspacePaths []string      `json:"workspace_paths,omitempty"`
	TenantID       string        `json:"-"`
}

// ChatResponse is the HTTP-independent result of a chat execution.
type ChatResponse struct {
	Reply             string                                `json:"reply"`
	SkillsUsed        []string                              `json:"skills_used"`
	Steps             int                                   `json:"steps"`
	Actions           []planner.AgentAction                 `json:"actions,omitempty"`
	Plan              []planner.PlanStep                    `json:"plan,omitempty"`
	ContextLayers     []string                              `json:"context_layers,omitempty"`
	EmotionHint       *emotion.Result                       `json:"emotion,omitempty"`
	StickerSuggestion any                                   `json:"sticker_suggestion,omitempty"`
	StickerMulti      map[string]*emotion.StickerSuggestion `json:"sticker_suggestions,omitempty"`
	Sandbox           map[string]any                        `json:"sandbox,omitempty"`
	Rich              any                                   `json:"rich,omitempty"`
	BrowserRequired   bool                                  `json:"-"`
	BrowserPayload    any                                   `json:"-"`
	TraceID           string                                `json:"-"`
}

// ExecuteChatPipeline runs the full chat pipeline: validation → session
// management → planning → post-processing. Extracted from handleChat to
// separate business logic from HTTP transport, enabling reuse from IM
// channels and direct testing.
func (g *Gateway) ExecuteChatPipeline(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("chat pipeline: request is nil")
	}
	start := time.Now()
	ctx, traceSpan := observe.StartTrace(ctx, "gateway.ExecuteChatPipeline")
	traceSpan.Attrs["tenant_id"] = req.TenantID
	if err := g.ensureChatPipelineReady(req); err != nil {
		observe.EndSpan(traceSpan, err)
		return nil, err
	}
	req.WorkspacePaths = inferWorkspacePathsFromMessages(req.WorkspacePaths, req.Messages)

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
	routedTier, routedModelID := g.resolveThinkingLevel(ctx, req.ThinkingLevel, msgs, traceSpan)

	// ── 8. Budget pre-check ──
	if g.costTracker != nil {
		totalChars := 0
		for _, m := range msgs {
			totalChars += len([]rune(m.Content))
			for _, p := range m.ContentParts {
				totalChars += len([]rune(p.Text))
			}
		}
		model := g.planner.ModelIDForTier(routedTier)
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
		RoutedTier:     routedTier,
		ClientOverride: sessionClient,
		EmotionHint:    emotionHint,
		TaskID:         req.TaskID,
		TaskContext:    taskContext,
		TraceID:        traceSpan.TraceID,
		WorkspacePaths: req.WorkspacePaths,
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
		g.recordRouterOutcome(routedTier, routedModelID, start, err, "", traceSpan)
		g.metrics.RecordRequest(time.Since(start), 0, 0, err)
		observe.EndSpan(traceSpan, err)
		g.triggerSelfHeal(ctx, req.Messages, err)
		return nil, fmt.Errorf("planner: %w", err)
	}
	g.recordRouterOutcome(routedTier, routedModelID, start, nil, result.Reply, traceSpan)

	// ── 12. Post-processing ──
	estTokensIn := estimateMsgTokens(msgs)
	estTokensOut := estimateTokens(result.Reply)
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

	// Ignite the reflective loop: learn from this turn (async, non-blocking).
	g.fireReflection(req.TenantID, req.SessionID, lastUserMessage(req.Messages), result.Reply, result.SkillsUsed, routedTier)

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

func (g *Gateway) ensureChatPipelineReady(req *ChatRequest) error {
	if g.usage == nil {
		g.usage = NewUsageTracker()
	}
	if g.metrics == nil {
		g.metrics = observe.New()
	}
	if g.planner == nil {
		return fmt.Errorf("chat pipeline: planner not configured")
	}
	if req.SessionID != "" && g.convStore == nil {
		return fmt.Errorf("chat pipeline: conversation store not configured")
	}
	return nil
}
