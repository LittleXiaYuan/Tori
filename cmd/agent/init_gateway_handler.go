package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	rdebug "runtime/debug"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/appdir"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/cognikernel"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/observe"
	pluginpkg "yunque-agent/pkg/plugin"
	"yunque-agent/pkg/safego"
)

// buildChannelHandler creates the message handler function for all channels.
func buildChannelHandler(
	p *planner.Planner,
	convStore *session.Store,
	channelReg *channel.Registry,
	orchestrator *memory.Orchestrator,
	reverie *planner.Reverie,
	emotionHistory *emotion.History,
	stickerCollector *emotion.StickerCollector,
	emotionShiftDetector *planner.EmotionShiftDetector,
	learningLoop *reflectpkg.LearningLoop,
	skillOptimizer *planner.SkillOptimizer,
	tenantMgr *tenant.Manager,
	engagementProfile channel.EngagementProfile,
	gw *gateway.Gateway,
	channelCtx context.Context,
	hookMgr *pluginpkg.HookManager,
	app *agentrt.App,
) func(msg channel.Message) channel.Reply {
	idResolver := gw.GetIdentityResolver()
	emotionAnalyzer := gw.GetEmotionAnalyzer()
	stickerMap := gw.GetStickerMap()

	return func(msg channel.Message) channel.Reply {
		// Recover from panics to prevent channel handler from crashing the process
		defer func() {
			if r := recover(); r != nil {
				stack := string(rdebug.Stack())
				slog.Error("channel handler panic recovered",
					"channel", msg.ChannelType, "user", msg.UserID,
					"panic", r, "stack", stack)
				if f, err := os.OpenFile(appdir.File("panic.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
					fmt.Fprintf(f, "=== HANDLER PANIC at %s ===\nchannel: %s\nuser: %s\npanic: %v\n%s\n\n",
						time.Now().Format(time.RFC3339), msg.ChannelType, msg.UserID, r, stack)
					f.Close()
				}
			}
		}()

		// Pre-ack reaction
		if msgID := msg.Extra["message_id"]; msgID != "" {
			safego.Go("PreAckReact", func() { gw.PreAckReact(channelCtx, msg.ChannelType, msg.ChannelID, msgID) })
		}

		// Sticker commands
		if msg.Content == "/sticker" || strings.HasPrefix(msg.Content, "/stickers") {
			return channel.Reply{Content: stickerCollector.ListStickers(msg.ChannelType), Format: "markdown"}
		}
		if msg.Content == "/add" {
			return channel.Reply{Content: stickerCollector.StartSession(msg.ChannelType, msg.UserID, emotion.Emotion("auto")), Format: "text"}
		}
		if msg.Content == "/add-all" {
			return channel.Reply{Content: stickerCollector.StartAddSession(msg.ChannelType, msg.UserID), Format: "text"}
		}
		if msg.Content == "/cancel" && stickerCollector.HasActiveSession(msg.ChannelType, msg.UserID) {
			stickerCollector.CancelSession(msg.ChannelType, msg.UserID)
			return channel.Reply{Content: "🚫 贴图收集已取消", Format: "text"}
		}

		// Batch sticker learning
		if msg.Rich != nil && msg.Rich.HasType(channel.ComponentSticker) &&
			stickerCollector.IsAddSession(msg.ChannelType, msg.UserID) {
			comp := msg.Rich.GetFirst(channel.ComponentSticker)
			sticker, _ := comp.(*channel.StickerComponent)
			if sticker != nil && sticker.SetName != "" {
				ch, chOk := channelReg.Get(msg.ChannelType)
				if chOk {
					if fetcher, ok := ch.(channel.StickerSetFetcher); ok {
						stickerCollector.ConsumeAddSession(msg.ChannelType, msg.UserID)
						items, err := fetcher.GetStickerSet(channelCtx, sticker.SetName)
						if err == nil && len(items) > 0 {
							return channel.Reply{Content: stickerCollector.LearnStickerSet(msg.ChannelType, items), Format: "markdown"}
						}
					}
				}
			}
		}

		if collected, reply := stickerCollector.TryCollect(msg); collected {
			return channel.Reply{Content: reply, Format: "text"}
		}

		// Identity resolution
		tenantID := "default"
		var identityProfile map[string]string
		if idResolver != nil && msg.ChannelType != "" {
			profile := idResolver.Resolve(msg.ChannelType, msg.UserID, msg.UserName)
			tenantID = profile.UnifiedID
			identityProfile = profile.Metadata
		}

		// /resume command — show cross-channel roaming session status
		if msg.Content == "/resume" {
			if tenantID != "default" {
				roamID := fmt.Sprintf("sess_roam_%s", tenantID)
				history := convStore.Get(roamID)
				if len(history) > 0 {
					lastCh := ""
					if identityProfile != nil {
						lastCh = identityProfile["last_channel"]
					}
					hint := ""
					if lastCh != "" && lastCh != msg.ChannelType {
						hint = fmt.Sprintf("（上次在 %s 聊的）", lastCh)
					}
					return channel.Reply{
						Content: fmt.Sprintf("💫 已接续漫游会话%s，共 %d 条对话记录。继续聊吧~", hint, len(history)),
						Format:  "text",
					}
				}
				return channel.Reply{Content: "💫 暂无跨渠道对话记录，发消息开始新对话吧~", Format: "text"}
			}
			return channel.Reply{Content: "⚠️ 跨渠道漫游需要先绑定身份（IdentityResolver 未配置）", Format: "text"}
		}

		// Session ID — cross-channel roaming for DMs
		chatType := msg.Extra["chat_type"]
		if tracker := channelReg.GetGroupTracker(); tracker != nil {
			switch chatType {
			case "group", "guild", "supergroup", "room", "GROUP":
				tracker.Track(channel.GroupInfo{
					ID: msg.ChannelID, ChannelType: msg.ChannelType, ChatType: chatType,
				})
			}
		}

		var sessionID string
		isGroup := false
		var channelSwitchNotice string
		switch chatType {
		case "group", "guild", "supergroup", "room", "GROUP":
			sessionID = fmt.Sprintf("sess_%s_%s", msg.ChannelType, msg.ChannelID)
			isGroup = true
		default:
			if tenantID != "default" {
				sessionID = fmt.Sprintf("sess_roam_%s", tenantID)
				if identityProfile != nil {
					prevCh := identityProfile["last_channel"]
					if prevCh != "" && prevCh != msg.ChannelType {
						channelSwitchNotice = fmt.Sprintf("💫 你从 %s 切换到了 %s，上下文已无缝衔接~\n\n", prevCh, msg.ChannelType)
					}
				}
				if idResolver != nil {
					idResolver.SetMeta(tenantID, "last_channel", msg.ChannelType)
				}
			} else {
				sessionID = fmt.Sprintf("sess_%s_%s", msg.ChannelType, msg.UserID)
			}
		}

		displayName := msg.UserName
		if displayName == "" {
			displayName = msg.UserID
		}

		_ = convStore.GetOrCreate(sessionID, tenantID)
		history := convStore.Get(sessionID)

		curProfile := channelReg.CurrentEngagement()
		var userContent string
		if curProfile.YAMLHeaders {
			userContent = channel.FormatUserMessageYAML(displayName, msg.ChannelType, chatType, msg.ChannelID, msg.Content)
		} else {
			userContent = fmt.Sprintf("[%s]: %s", displayName, msg.Content)
		}
		userMsg := llm.Message{Role: "user", Content: userContent}
		msgs := append(history, userMsg)

		// Auto-learn sticker
		if msg.Rich != nil && msg.Rich.HasType(channel.ComponentSticker) {
			var recentText string
			start := 0
			if len(history) > 5 {
				start = len(history) - 5
			}
			for _, m := range history[start:] {
				if m.Content != "" {
					recentText += m.Content + "\n"
				}
			}
			safego.Go("AutoLearn", func() { stickerCollector.AutoLearn(channelCtx, msg, recentText) })
			if isGroup {
				convStore.Append(sessionID, userMsg)
				return channel.Reply{}
			}
		}

		convStore.Append(sessionID, userMsg)

		// Memory ingestion
		if orchestrator != nil {
			memContent := fmt.Sprintf("[%s]: %s", displayName, msg.Content)
			if err := orchestrator.Ingest(channelCtx, tenantID, memContent, "conversation", "channel_input"); err != nil {
				slog.Warn("memory ingest failed", "err", err)
			}
		}

		// Emotion analysis
		var channelEmotionHint *emotion.Result
		if emotionAnalyzer != nil && emotionAnalyzer.Enabled() && msg.Content != "" {
			if emRes, err := emotionAnalyzer.AnalyzeText(channelCtx, msg.Content); err == nil && emRes != nil {
				emotionHistory.Record(sessionID, emRes.Emotion, emRes.Confidence, emRes.Source)
				if emotionShiftDetector != nil {
					emotionShiftDetector.Observe(sessionID, string(emRes.Emotion), emRes.Confidence)
				}
				if emRes.Confidence >= EmotionConfidenceThreshold && emRes.Emotion != emotion.EmotionNeutral && emRes.Emotion != emotion.EmotionUnknown {
					channelEmotionHint = emRes
				}
			}
		}

		// Group context
		var groupSystemPrompt, inboxContext string
		if isGroup {
			groupSystemPrompt = curProfile.GroupSystemPrompt
			if channelReg.Inbox() != nil {
				inboxContext = channelReg.Inbox().FormatForContext()
				channelReg.Inbox().Drain()
			}
		}

		// ── Multi-step progress for IM channels ──
		var stepCB planner.StepCallback
		if imCh, chOk := channelReg.Get(msg.ChannelType); chOk {
			if ps, psOk := imCh.(channel.ProgressSender); psOk {
				statusMsgID, sendErr := ps.SendAndGetID(channelCtx, msg.ChannelID,
					channel.Reply{Content: "🤔 思考中...", Format: "text"})
				if sendErr == nil && statusMsgID != "" {
					var statusLines []string
					stepCB = func(evt observe.AgentEvent) {
						statusLines = append(statusLines, evt.Summary)
						start := 0
						if len(statusLines) > 5 {
							start = len(statusLines) - 5
						}
						text := ""
						for _, l := range statusLines[start:] {
							text += l + "\n"
						}
						_ = ps.EditMessage(channelCtx, msg.ChannelID, statusMsgID, text)
					}
				}
			}
		}

		// Fire chat.before hook — plugins can observe/modify the incoming message
		if hookMgr != nil {
			hookMgr.EmitAll(channelCtx, pluginpkg.HookChatBefore, map[string]any{
				"channel": msg.ChannelType, "user_id": msg.UserID, "content": msg.Content,
			})
		}

		// CognitivePlugin routing: highest ShouldHandle score >= 0.7 claims the message.
		var result *planner.PlanResult
		var err error
		cogPlugin, cogScore := app.PluginReg.RouteMessage(channelCtx, msg.Content)
		if cogPlugin != nil {
			slog.Info("cognitive plugin claiming message",
				"plugin", cogPlugin.Name(), "score", cogScore, "channel", msg.ChannelType)
			cogEnv := &pluginpkg.CognitiveEnv{
				LLMCall: app.LLMBreaker.Call,
				MemorySearch: func(ctx context.Context, query string) string {
					return app.Orchestrator.CompileContext(ctx, tenantID, query)
				},
				TenantID:    tenantID,
				UserID:      msg.UserID,
				ChannelType: msg.ChannelType,
				SessionID:   sessionID,
			}
			reply, handleErr := cogPlugin.Handle(channelCtx, msg.Content, cogEnv)
			if handleErr != nil {
				slog.Warn("cognitive plugin handle failed, falling back to Planner",
					"plugin", cogPlugin.Name(), "err", handleErr)
				cogPlugin = nil // fall through to normal Planner
			} else {
				result = &planner.PlanResult{Reply: reply}
				err = nil
			}
		}

		if cogPlugin == nil {
			result, err = p.Run(channelCtx, planner.PlanRequest{
				Messages: msgs, TenantID: tenantID, EmotionHint: channelEmotionHint,
				IsGroup: isGroup, ChannelType: msg.ChannelType, ChatType: chatType,
				GroupSystemPrompt: groupSystemPrompt, InboxContext: inboxContext,
				StepCallback: stepCB,
			})
		}
		if err != nil {
			return channel.Reply{Content: "Error: " + err.Error(), Format: "text"}
		}

		safego.Go("SkillOptimizer", func() { skillOptimizer.Analyze() })

		assistantContent := result.Reply
		if summary := result.ExecutionSummary(); summary != "" {
			assistantContent = summary + "\n\n" + assistantContent
		}
		convStore.Append(sessionID, llm.Message{Role: "assistant", Content: assistantContent})

		// Fire chat.after hook — plugins can observe the completed reply
		if hookMgr != nil {
			hookMgr.EmitAll(channelCtx, pluginpkg.HookChatAfter, map[string]any{
				"channel": msg.ChannelType, "user_id": msg.UserID,
				"user_content": msg.Content, "reply": result.Reply,
				"skills_used": result.SkillsUsed,
			})
		}

		// Learning loop + experience
		safego.Go("LearningLoop", func() {
			learnCtx, learnCancel := context.WithTimeout(context.Background(), LearningLoopTimeout)
			defer learnCancel()
			quality := 7
			if learningLoop.Reflect() != nil {
				if eval, err := learningLoop.Reflect().Evaluate(learnCtx, msg.Content, result.Reply, nil); err == nil {
					quality = eval.Quality
				}
			}
			learningLoop.AfterInteraction(learnCtx, msg.Content, result.Reply, result.SkillsUsed, quality)
			outcome := "success"
			if quality < 5 {
				outcome = "failure"
			} else if quality < 7 {
				outcome = "partial"
			}
			experienceStore, _ := gw.GetExperienceStore()
			if experienceStore != nil {
				experienceStore.Add(reflectpkg.Experience{
					Source: "interaction", SourceID: sessionID, Category: "conversation",
					Outcome: outcome,
					Lesson:  fmt.Sprintf("[%s] Q: %s | Skills: %v | Quality: %d/10", displayName, truncChannel(msg.Content, 80), result.SkillsUsed, quality),
					Context: truncChannel(result.Reply, 120),
				})
			}
			// B-full: drive canonical ReflectiveLoop.Run so reflection.completed
			// ledger events fire for every interaction (Inner-life Pack timeline).
			if rl := gw.ReflectiveLoop(); rl != nil {
				_, _ = rl.Run(learnCtx, cognikernel.ConversationEndData{
					TenantID:   tenantID,
					SessionID:  sessionID,
					UserIntent: msg.Content,
					AgentReply: result.Reply,
					SkillsUsed: result.SkillsUsed,
				})
			}
		})

		replyContent := result.Reply
		if channelSwitchNotice != "" {
			replyContent = channelSwitchNotice + replyContent
		}
		reply := channel.Reply{Content: replyContent, Format: "markdown"}

		if rich := gateway.RenderAgentActions(result.Actions); rich != nil && len(rich.Components) > 0 {
			reply.Rich = rich
		}

		// Sticker reaction
		safego.Go("StickerReaction", func() {
			if emotionAnalyzer == nil || stickerMap == nil {
				return
			}
			emResult, err := emotionAnalyzer.AnalyzeText(channelCtx, result.Reply)
			if err != nil || emResult == nil {
				return
			}
			if emResult.Emotion == emotion.EmotionNeutral || emResult.Emotion == emotion.EmotionUnknown {
				return
			}
			if emResult.Confidence < EmotionConfidenceThreshold {
				return
			}
			suggestion := stickerMap.Suggest(emResult.Emotion, msg.ChannelType)
			if suggestion == nil || suggestion.FileID == "" || suggestion.SetName == "" {
				return
			}
			sc := channel.NewSticker("", suggestion.FileID)
			sc.Platform = msg.ChannelType
			sc.SetName = suggestion.SetName
			sc.Emoji = suggestion.Emoji
			ch, chOk := channelReg.Get(msg.ChannelType)
			if !chOk {
				return
			}
			if sender, ok := ch.(channel.StickerSender); ok {
				if err := sender.SendSticker(channelCtx, msg.ChannelID, sc); err != nil {
					slog.Warn("sticker send failed", "err", err)
				}
			}
		})

		gw.InvokeReplyHooks(channelCtx, msg, reply)

		return reply
	}
}

// wireReverieDelivery sets up Reverie thought delivery to configured channels.
// Falls back to Inbox when no REVERIE_TARGET_<CHANNEL> env vars are configured.
func wireReverieDelivery(reverie *planner.Reverie, channelReg *channel.Registry, inboxStore *inbox.Store, channelCtx context.Context) {
	reverie.SetDeliver(func(thought planner.Thought) {
		prefix := map[string]string{
			"insight": "💡", "question": "❓", "observation": "👀",
			"idea": "💡", "concern": "⚠️",
		}
		emoji := prefix[thought.Category]
		if emoji == "" {
			emoji = "💭"
		}

		content := thought.Content
		contentRunes := []rune(content)
		if len(contentRunes) > ReverieThoughtMaxRunes {
			content = string(contentRunes[:ReverieThoughtMaxRunes]) + "..."
		}
		text := fmt.Sprintf("%s %s", emoji, content)

		deliveredCount := 0
		for _, ch := range channelReg.All() {
			envKey := "REVERIE_TARGET_" + strings.ToUpper(ch.Type())
			raw := os.Getenv(envKey)
			if raw == "" {
				continue
			}
			targets := strings.Split(raw, ",")
			for _, t := range targets {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				reply := channel.Reply{Content: text, Format: "markdown"}
				if ch.Type() == "email" || ch.Type() == "wecom" {
					reply = channel.Reply{Content: text, Format: "text"}
				}
				if err := ch.Send(channelCtx, t, reply); err != nil {
					slog.Warn("reverie: deliver failed, retrying", "channel", ch.Type(), "target", t, "err", err)
					time.Sleep(2 * time.Second)
					if err2 := ch.Send(channelCtx, t, reply); err2 != nil {
						slog.Error("reverie: deliver retry failed", "channel", ch.Type(), "target", t, "err", err2)
						continue
					}
				}
				deliveredCount++
			}
		}
		if deliveredCount > 0 {
			slog.Info("reverie: thought delivered", "channel_count", deliveredCount,
				"category", thought.Category, "significance", thought.Significance)
			return
		}

		// Fallback: no channel targets configured — write to Inbox so the thought is not silently lost.
		if inboxStore != nil {
			_, err := inboxStore.Push("reverie", text, inbox.ActionNotify, map[string]any{
				"thought_id":   thought.ID,
				"category":     thought.Category,
				"significance": thought.Significance,
				"trigger":      thought.Trigger,
			})
			if err != nil {
				slog.Warn("reverie: inbox fallback failed", "err", err)
			} else {
				slog.Info("reverie: thought saved to inbox (no channel targets configured)",
					"category", thought.Category, "significance", thought.Significance)
			}
		}
	})
}
