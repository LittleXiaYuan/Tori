package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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

const hiddenAttachmentContextMarker = "[隐藏附件上下文]"

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
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Trace-ID", observe.TraceIDFromContext(ctx))

	flusher, ok := w.(http.Flusher)
	if !ok {
		apperror.WriteCode(w, apperror.CodeInternal, "streaming not supported")
		observe.EndSpan(traceSpan, nil)
		return
	}

	var streamMu sync.Mutex
	sendEvent := func(event, data string) {
		streamMu.Lock()
		defer streamMu.Unlock()
		sseEvent(w, flusher, event, data)
	}
	sendKeepAlive := func() {
		streamMu.Lock()
		defer streamMu.Unlock()
		// Some dev/proxy layers buffer tiny SSE frames. A padded comment is a
		// legal SSE heartbeat, ignored by our parser, but large enough to flush
		// through Next/Tauri dev plumbing and reset the browser read timer.
		_, _ = w.Write([]byte(": yunque-agent keepalive " + strings.Repeat(".", 2048) + "\n\n"))
		flusher.Flush()
	}

	// Send an immediate empty frame so the browser knows the stream is alive
	// before guardrails, memory, routing, or the first upstream LLM token runs.
	// Some providers can take >60s to produce a first token; without this ping
	// the frontend may show “响应超时（60s 无数据）” even though the request is still running.
	sendKeepAlive()
	sendEvent("ping", "")

	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendKeepAlive()
				sendEvent("ping", "")
			}
		}
	}()

	// Attachment is an inline user-uploaded file. Currently used by the Cherry
	// input-bar 📎 drawer. For now we stay in-process (no persistent storage)
	// and handle only small files — large binary uploads deserve their own
	// multipart endpoint, which is a separate task.
	type Attachment struct {
		Name    string `json:"name"`
		Mime    string `json:"mime"`
		DataB64 string `json:"data_b64"`
	}
	var req struct {
		Messages    []llm.Message `json:"messages"`
		SessionID   string        `json:"session_id"`
		TaskID      string        `json:"task_id"`
		Platform    string        `json:"platform,omitempty"`
		Thinking    *bool         `json:"thinking,omitempty"`
		Mode        string        `json:"mode,omitempty"`
		AiriMode    bool          `json:"airi_mode,omitempty"`
		WebSearch   bool          `json:"web_search,omitempty"`  // Cherry 🌐 drawer: force-enable web search
		ToolIDs     []string      `json:"tool_ids,omitempty"`    // Cherry 🔨 drawer: restrict to explicit skill subset
		Attachments []Attachment  `json:"attachments,omitempty"` // Cherry 📎 drawer: inline per-turn files
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendEvent("error", `{"code":"BAD_REQUEST","message":"invalid body"}`)
		observe.EndSpan(traceSpan, err)
		return
	}
	if len(req.Messages) == 0 {
		sendEvent("error", `{"code":"MESSAGES_REQUIRED","message":"messages array is required"}`)
		observe.EndSpan(traceSpan, nil)
		return
	}
	visibleMessages := cloneLLMMessages(req.Messages)
	hiddenAttachmentMessage := llm.Message{}

	// Inline attachments: for text-like MIMEs we decode and append the body
	// into the last user message. Binary / image attachments are only listed
	// by name+mime to keep the prompt from ballooning. Size-capped at 64KB
	// per file × 4 files to avoid breaking the context window.
	if len(req.Attachments) > 0 {
		const maxPerFile = 64 * 1024
		const maxFiles = 4
		var inlined strings.Builder
		hiddenAttachmentContext := ""
		attachmentSummaries := make([]string, 0, len(req.Attachments))
		for i, a := range req.Attachments {
			if i >= maxFiles {
				inlined.WriteString(fmt.Sprintf("\n[...%d more attachment(s) omitted]\n", len(req.Attachments)-maxFiles))
				attachmentSummaries = append(attachmentSummaries, fmt.Sprintf("- 还有 %d 个附件已省略；如需处理请分批发送。", len(req.Attachments)-maxFiles))
				break
			}
			decoded, err := base64.StdEncoding.DecodeString(a.DataB64)
			if err != nil {
				inlined.WriteString(fmt.Sprintf("\n[Attached: %s (%s) — decode failed]\n", a.Name, a.Mime))
				attachmentSummaries = append(attachmentSummaries, fmt.Sprintf("- %s：读取失败，可重新上传。", a.Name))
				continue
			}
			isAttachmentMetadata := strings.Contains(a.Mime, "x-yunque-attachment-metadata")
			isText := strings.HasPrefix(a.Mime, "text/") ||
				strings.Contains(a.Mime, "json") ||
				strings.Contains(a.Mime, "xml") ||
				strings.Contains(a.Mime, "yaml") ||
				strings.Contains(a.Mime, "javascript") ||
				strings.Contains(a.Mime, "typescript")
			if isText {
				body := string(decoded)
				body = truncateUTF8ByBytes(body, maxPerFile, "\n...[truncated]")
				inlined.WriteString(fmt.Sprintf("\n[Attached file: %s, %s, %d bytes]\n```\n%s\n```\n", a.Name, a.Mime, len(decoded), body))
				if isAttachmentMetadata {
					attachmentSummaries = append(attachmentSummaries, fmt.Sprintf("- %s（%s，%d bytes）：已记录文件信息，正文未直接展开。", a.Name, a.Mime, len(decoded)))
				} else {
					attachmentSummaries = append(attachmentSummaries, fmt.Sprintf("- %s（%s，%d bytes）：内容已作为隐藏上下文提供给模型。", a.Name, a.Mime, len(decoded)))
				}
			} else {
				inlined.WriteString(fmt.Sprintf("\n[Attached binary: %s, %s, %d bytes (content omitted from prompt)]\n", a.Name, a.Mime, len(decoded)))
				attachmentSummaries = append(attachmentSummaries, fmt.Sprintf("- %s（%s，%d bytes）：已记录文件信息，正文未直接展开。", a.Name, a.Mime, len(decoded)))
			}
		}
		if inlined.Len() > 0 && len(req.Messages) > 0 {
			lastIdx := len(req.Messages) - 1
			hiddenAttachmentContext = inlined.String()
			req.Messages[lastIdx].Content = req.Messages[lastIdx].Content + hiddenAttachmentContext
		}
		if len(attachmentSummaries) > 0 && len(visibleMessages) > 0 {
			lastIdx := len(visibleMessages) - 1
			if visibleMessages[lastIdx].Role == "user" {
				visibleMessages[lastIdx].Content = appendAttachmentSummaryForDisplay(visibleMessages[lastIdx].Content, attachmentSummaries)
			}
		}
		if hiddenAttachmentContext != "" && req.SessionID != "" {
			hiddenAttachmentMessage = llm.Message{
				Role:    "system",
				Content: buildHiddenAttachmentContextMessage(hiddenAttachmentContext),
			}
		}
	}

	// Web search toggle: prepend a soft instruction so the planner/LLM
	// knows to pick the web_search skill. We don't forcibly invoke it —
	// the model still decides when it's actually relevant, but the hint
	// makes it much more likely.
	if req.WebSearch && len(req.Messages) > 0 {
		req.Messages = append([]llm.Message{{
			Role:    "system",
			Content: "用户本次消息启用了『联网搜索』。当回答涉及事实、新闻、行情或需要最新信息时，请优先调用 web_search / search / searx 技能并在回复里引用来源。",
		}}, req.Messages...)
	}

	// Chinese guardrail pipeline
	if g.zhGuard != nil && len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1].Content
		guardResult := g.zhGuard.Run(ctx, lastMsg)
		if guardResult.Blocked {
			sendEvent("error", `{"code":"BLOCKED","message":"内容安全检查未通过"}`)
			observe.EndSpan(traceSpan, nil)
			return
		}
		if guardResult.Redacted != "" {
			req.Messages[len(req.Messages)-1].Content = guardResult.Redacted
			if len(visibleMessages) > 0 {
				lastVisibleIdx := len(visibleMessages) - 1
				if visibleMessages[lastVisibleIdx].Role == "user" && visibleMessages[lastVisibleIdx].Content != "" {
					visibleGuard := g.zhGuard.Run(ctx, visibleMessages[lastVisibleIdx].Content)
					if visibleGuard.Redacted != "" {
						visibleMessages[lastVisibleIdx].Content = visibleGuard.Redacted
					}
				}
			}
		}
	}

	// Session management
	msgs := req.Messages
	if req.SessionID != "" {
		_ = g.convStore.GetOrCreate(req.SessionID, tid)
		history := g.convStore.Get(req.SessionID)
		if len(req.Messages) <= 1 {
			if len(history) > 0 {
				msgs = append(history, req.Messages...)
			}
		} else if hiddenContexts := hiddenAttachmentContextMessages(history); len(hiddenContexts) > 0 {
			msgs = append(hiddenContexts, req.Messages...)
		}
		if len(req.Messages) > 0 {
			lastMsg := visibleMessages[len(visibleMessages)-1]
			if lastMsg.Role == "user" {
				g.convStore.Append(req.SessionID, lastMsg)
			}
			if hiddenAttachmentMessage.Content != "" {
				g.convStore.Append(req.SessionID, hiddenAttachmentMessage)
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
		runes := []rune(reply)
		chunkSize := 20
		for i := 0; i < len(runes); i += chunkSize {
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			chunk := string(runes[i:end])
			data, _ := json.Marshal(map[string]string{"content": chunk})
			sendEvent("delta", string(data))
		}
		doneBytes, _ := json.Marshal(map[string]any{
			"reply":               reply,
			"skills_used":         []string{},
			"steps":               1,
			"browser_requirement": browserRequirementPayload(),
			"suggestions": []map[string]string{
				{"type": "followup", "label": "Open browser setup"},
			},
		})
		sendEvent("done", string(doneBytes))
		if req.SessionID != "" {
			g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: reply})
		}
		observe.EndSpan(traceSpan, nil)
		return
	}
	// Memory: write user message(s) to short-term
	if g.orchestrator != nil && len(visibleMessages) > 0 {
		for _, m := range visibleMessages {
			if m.Role == "user" && m.Content != "" {
				_ = g.orchestrator.Ingest(ctx, tid, m.Content, "conversation", "user_input")
				g.metrics.Cognitive().MemoryIngest.Add(1)
			}
		}
	}

	// Smart router
	var routedTier string
	var routedModelID string
	if g.smartRouter != nil && len(msgs) > 0 {
		lastMsg := msgs[len(msgs)-1].Content
		routedModel, tier := g.smartRouter.Route(ctx, lastMsg, false)
		routedTier = tier.String()
		if routedModel != nil {
			routedModelID = routedModel.ModelID
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

	chatMode := req.Mode == "chat"
	fastMode := req.Mode == "fast"

	// Emotion analysis — run async so it doesn't block the planner
	type emotionResult struct {
		hint *emotion.Result
	}
	emotionCh := make(chan emotionResult, 1)
	emotionFeatureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureEmotion)
	if g.emotionAnalyzer != nil && g.emotionAnalyzer.Enabled() && emotionFeatureOK && !fastMode && !chatMode && len(msgs) > 0 {
		lastMsg := msgs[len(msgs)-1]
		if lastMsg.Role == "user" && lastMsg.Content != "" {
			go func() {
				hint, _ := g.emotionAnalyzer.AnalyzeText(ctx, lastMsg.Content)
				if hint != nil && g.emotionHistory != nil {
					g.emotionHistory.Record(req.SessionID, hint.Emotion, hint.Confidence, hint.Source)
				}
				emotionCh <- emotionResult{hint: hint}
			}()
		} else {
			emotionCh <- emotionResult{}
		}
	} else {
		emotionCh <- emotionResult{}
	}

	// Session-level provider override
	var sessionClient *llm.Client
	if g.providerReg != nil {
		if req.SessionID != "" {
			if sp := g.providerReg.GetForSession(req.SessionID); sp != nil {
				sessionClient = sp.Client
				slog.Info("agentic: using session provider override", "session", req.SessionID, "provider", sp.Config.ID)
			}
		}
		// The provider settings page writes the visible "execution provider" via
		// /api/providers/exec. Historically that only affected sub/exec agents,
		// while the main agentic planner still used the smart tier (usually the
		// env primary, e.g. Moonshot). Honor it here too so the model shown in
		// chat is the model that actually answers.
		if sessionClient == nil {
			if execProvider := strings.TrimSpace(g.ExecProvider()); execProvider != "" && execProvider != "smart" {
				if ep := g.providerReg.Get(execProvider); ep != nil && ep.Config.Enabled {
					sessionClient = ep.Client
					slog.Info("agentic: using exec provider override", "provider", ep.Config.ID, "model", ep.Config.Model)
				} else {
					slog.Warn("agentic: exec provider override not available, falling back", "provider", execProvider)
				}
			}
		}
	}

	// ── Run planner with StepCallback → SSE ──
	planReq := planner.PlanRequest{
		Messages:          msgs,
		TenantID:          tid,
		ModelOverride:     routedTier,
		ClientOverride:    sessionClient,
		TaskID:            req.TaskID,
		TaskContext:       taskContext,
		ThinkingEnabled:   req.Thinking,
		DisableTools:      chatMode,
		DisableDelegation: chatMode,
		AllowedSkills:     req.ToolIDs,
		StepCallback: func(event observe.AgentEvent) {
			streamEvent := friendlyAgentEventForStream(event)
			data, _ := json.Marshal(streamEvent)
			sendEvent(event.QualifiedType(), string(data))
			// Record to audit trail
			if g.eventTrail != nil {
				g.eventTrail.Record(event)
			}
		},
		TraceID: traceSpan.TraceID,
	}

	if slashResp, handled, slashErr := g.tryHandleSlashCommand(ctx, planReq); handled {
		if slashErr != nil {
			errData, _ := json.Marshal(map[string]string{"code": "SLASH_COMMAND_ERROR", "message": friendlyChatPipelineError(slashErr)})
			sendEvent("error", string(errData))
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
			sendEvent("delta", string(data))
		}

		doneBytes, _ := json.Marshal(slashResp.Raw)
		sendEvent("done", string(doneBytes))
		if req.SessionID != "" {
			g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: reply})
		}
		observe.EndSpan(traceSpan, nil)
		return
	}

	result, err := g.planner.Run(ctx, planReq)
	if err != nil {
		slog.Error("agentic planner error", "err", err, "tenant", tid)
		g.recordRouterOutcome(routedTier, routedModelID, start, err, "", traceSpan)
		errData, _ := json.Marshal(map[string]string{"code": "PLANNER_ERROR", "message": friendlyChatPipelineError(err)})
		sendEvent("error", string(errData))
		observe.EndSpan(traceSpan, err)
		return
	}
	g.recordRouterOutcome(routedTier, routedModelID, start, nil, result.Reply, traceSpan)

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
		sendEvent("actions", string(actJSON))
	}

	// Stream reasoning content (thinking) if present
	if result.ReasoningContent != "" {
		thinkData, _ := json.Marshal(map[string]string{"content": result.ReasoningContent})
		sendEvent("thinking", string(thinkData))
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
		sendEvent("delta", string(data))
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
	var sandboxInfo map[string]any
	if result.Plan != nil {
		doneData["plan"] = result.Plan
		sandboxInfo = extractSandboxFromPlan(result.Plan)
		if sandboxInfo != nil {
			doneData["sandbox"] = sandboxInfo
		}
	}
	if len(result.ContextLayers) > 0 {
		doneData["context_layers"] = result.ContextLayers
	}
	emotionHint := (<-emotionCh).hint
	if emotionHint != nil && emotionHint.Emotion != emotion.EmotionNeutral {
		doneData["emotion"] = emotionHint
	}
	// Suggestions: follow-up questions + skill save hint
	suggestions := buildSuggestions(result, req.Messages)
	if len(suggestions) > 0 {
		doneData["suggestions"] = suggestions
	}

	// Airi mode: push reply to desktop pet
	if req.AiriMode && reply != "" {
		type airiPusher interface{ PushToAiri(string) }
		if rawPlugin, ok := g.pluginReg.Get("airi"); ok {
			if ap, ok := rawPlugin.(airiPusher); ok {
				ap.PushToAiri(reply)
				doneData["airi_synced"] = true
			}
		}
	}

	doneBytes, _ := json.Marshal(doneData)
	sendEvent("done", string(doneBytes))

	// Save assistant reply to session
	if req.SessionID != "" {
		persistContent := embedSandboxMarker(reply, sandboxInfo)
		g.convStore.Append(req.SessionID, llm.Message{Role: "assistant", Content: persistContent})
	}

	// Memory: write assistant reply
	if g.orchestrator != nil && reply != "" {
		_ = g.orchestrator.Ingest(ctx, tid, reply, "conversation", "assistant_reply")
		g.metrics.Cognitive().MemoryIngest.Add(1)
	}

	// Memory pipeline (async) — skipped in fast/chat mode
	if g.pipeline != nil && !fastMode && !chatMode && len(visibleMessages) > 0 {
		lastUserMsg := ""
		for i := len(visibleMessages) - 1; i >= 0; i-- {
			if visibleMessages[i].Role == "user" {
				lastUserMsg = visibleMessages[i].Content
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
	for i := len(visibleMessages) - 1; i >= 0; i-- {
		if visibleMessages[i].Role == "user" {
			lastUserMsgForHook = visibleMessages[i].Content
			break
		}
	}
	if g.skillGrow != nil && !fastMode && !chatMode && lastUserMsgForHook != "" {
		growMsg := lastUserMsgForHook
		usedActions := result.SkillsUsed
		safego.Go("agentic-skill-grow", func() {
			growCtx, growCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer growCancel()
			g.skillGrow.Observe(growCtx, growMsg)
			if len(usedActions) > 0 {
				g.skillGrow.ObserveActions(growCtx, usedActions)
			}
		})
	}
	g.InvokeReplyHooks(ctx, channelpkg.Message{ChannelType: "webui", Content: lastUserMsgForHook}, channelpkg.Reply{Content: reply})

	// Usage tracking
	estTokens := int64(len(fmt.Sprint(msgs))/4 + len(reply)/4 + 100)
	g.usage.RecordChat(tid, estTokens)
	g.metrics.RecordRequest(time.Since(start), estTokens, estTokens, nil)

	observe.EndSpan(traceSpan, nil)
}

func friendlyAgentEventForStream(event observe.AgentEvent) observe.AgentEvent {
	streamEvent := event
	if friendly := plannerKnownFriendlyError(streamEvent.Summary); friendly != "" {
		streamEvent.Summary = friendly
	}
	streamEvent.Detail = friendlyAgentEventDetailForStream(streamEvent.Detail)
	return streamEvent
}

func cloneLLMMessages(in []llm.Message) []llm.Message {
	if len(in) == 0 {
		return nil
	}
	out := make([]llm.Message, len(in))
	copy(out, in)
	return out
}

func truncateUTF8ByBytes(s string, maxBytes int, suffix string) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	var b strings.Builder
	b.Grow(maxBytes + len(suffix))
	for _, r := range s {
		next := string(r)
		if b.Len()+len(next) > maxBytes {
			break
		}
		b.WriteString(next)
	}
	return b.String() + suffix
}

func appendAttachmentSummaryForDisplay(content string, summaries []string) string {
	if len(summaries) == 0 {
		return content
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(content))
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString("[附件已读取]\n")
	b.WriteString(strings.Join(summaries, "\n"))
	return b.String()
}

func buildHiddenAttachmentContextMessage(context string) string {
	context = strings.TrimSpace(context)
	if context == "" {
		return ""
	}
	return hiddenAttachmentContextMarker + "\n这段内容供后续追问时继续读取附件使用，不应直接展示给用户。\n\n" + context
}

func hiddenAttachmentContextMessages(msgs []llm.Message) []llm.Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]llm.Message, 0, 2)
	for _, msg := range msgs {
		if isHiddenAttachmentContextMessage(msg) {
			out = append(out, msg)
		}
	}
	const maxHiddenAttachmentContexts = 2
	if len(out) > maxHiddenAttachmentContexts {
		out = append([]llm.Message(nil), out[len(out)-maxHiddenAttachmentContexts:]...)
	}
	return out
}

func isHiddenAttachmentContextMessage(msg llm.Message) bool {
	return msg.Role == "system" && strings.Contains(msg.Content, hiddenAttachmentContextMarker)
}

func friendlyAgentEventDetailForStream(detail any) any {
	switch d := detail.(type) {
	case observe.HandoffDetail:
		if friendly := plannerKnownFriendlyError(d.Error); friendly != "" {
			d.Error = friendly
		}
		return d
	case *observe.HandoffDetail:
		if d == nil {
			return detail
		}
		clone := *d
		if friendly := plannerKnownFriendlyError(clone.Error); friendly != "" {
			clone.Error = friendly
		}
		return &clone
	case observe.ToolResultDetail:
		if friendly := plannerKnownFriendlyError(d.Error); friendly != "" {
			d.Error = friendly
		}
		if friendly := plannerKnownFriendlyError(d.Result); friendly != "" {
			d.Result = friendly
		}
		return d
	case *observe.ToolResultDetail:
		if d == nil {
			return detail
		}
		clone := *d
		if friendly := plannerKnownFriendlyError(clone.Error); friendly != "" {
			clone.Error = friendly
		}
		if friendly := plannerKnownFriendlyError(clone.Result); friendly != "" {
			clone.Result = friendly
		}
		return &clone
	case planner.ModelFallbackDetail:
		if friendly := plannerKnownFriendlyError(d.Reason); friendly != "" {
			d.Reason = friendly
		}
		return d
	case *planner.ModelFallbackDetail:
		if d == nil {
			return detail
		}
		clone := *d
		if friendly := plannerKnownFriendlyError(clone.Reason); friendly != "" {
			clone.Reason = friendly
		}
		return &clone
	default:
		return sanitizeAgentEventDetailValue(detail)
	}
}

func sanitizeAgentEventDetailValue(value any) any {
	switch v := value.(type) {
	case string:
		if friendly := plannerKnownFriendlyError(v); friendly != "" {
			return friendly
		}
		return value
	case map[string]any:
		clone := make(map[string]any, len(v))
		changed := false
		for key, raw := range v {
			sanitized := sanitizeAgentEventDetailValue(raw)
			clone[key] = sanitized
			if !reflect.DeepEqual(sanitized, raw) {
				changed = true
			}
		}
		if changed {
			return clone
		}
		return value
	case []any:
		clone := make([]any, len(v))
		changed := false
		for i, raw := range v {
			sanitized := sanitizeAgentEventDetailValue(raw)
			clone[i] = sanitized
			if !reflect.DeepEqual(sanitized, raw) {
				changed = true
			}
		}
		if changed {
			return clone
		}
		return value
	default:
		return value
	}
}

// buildSuggestions generates follow-up suggestions.
// Prefers LLM-generated suggestions from PlanResult.Suggestions;
// falls back to rule-based hints if the model didn't provide any.
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

	// Use LLM-generated suggestions if available
	if len(result.Suggestions) > 0 {
		for _, s := range result.Suggestions {
			out = append(out, map[string]string{
				"type": "followup", "label": s,
			})
		}
		return out
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
