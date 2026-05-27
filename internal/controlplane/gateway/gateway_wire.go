package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/cognikernel"
	"yunque-agent/internal/execution/channel"
	reflectpkg "yunque-agent/internal/experimental/reflect"
)

// --- Wiring methods ---
// These methods connect Gateway subsystems together after all Set*() calls.
// Each Wire*() sets up cross-component callbacks and event bridges.

// imThinkingLevels stores per-user thinking level overrides for IM channels.
// Key: "channelType:userID", Value: "none"|"auto"|"deep"
var imThinkingLevels sync.Map

// GetIMThinkingLevel returns the thinking level for an IM user (default: "auto").
func GetIMThinkingLevel(channelType, userID string) string {
	key := channelType + ":" + userID
	if v, ok := imThinkingLevels.Load(key); ok {
		return v.(string)
	}
	return "auto"
}

// WireStickerCommands connects the sticker command system to the channel registry.
// This creates a CommandInterceptor with sticker commands (/add, /add-all, /sticker,
// /sticker-del, /cancel) and attaches it to the channel registry so all IM channels
// get universal sticker command support.
// Call this after SetStickerMap, SetStickerCollector, and SetChannelRegistry.
func (g *Gateway) WireStickerCommands() {
	if g.channelReg == nil {
		return
	}

	ci := channel.NewCommandInterceptor()
	sc := &channel.StickerCommands{}

	if g.stickerCollector != nil {
		sc.StartCollect = func(channelType, userID, emotionStr string) string {
			em, ok := emotion.ParseStickerCommand("/sticker " + emotionStr)
			if !ok {
				em = emotion.EmotionHappy
			}
			return g.stickerCollector.StartSession(channelType, userID, em)
		}
		sc.StartBulkAdd = func(channelType, userID string) string {
			return g.stickerCollector.StartAddSession(channelType, userID)
		}
		sc.ListStickers = func(platform string) string {
			return g.stickerCollector.ListStickers(platform)
		}
		sc.CancelSession = func(channelType, userID string) bool {
			return g.stickerCollector.CancelSession(channelType, userID)
		}
	}

	if g.stickerMap != nil {
		sc.DeleteStickers = func(platform, emotionStr string) string {
			em := emotion.Emotion(emotionStr)
			g.stickerMap.Clear(platform, em)
			return fmt.Sprintf("✅ 已删除 %s 平台的「%s」情绪贴图", platform, emotionStr)
		}
	}

	// Fetch-and-learn for channels that support StickerSetFetcher
	if g.channelReg != nil && g.stickerCollector != nil {
		sc.FetchAndLearnSet = func(channelType, setName string) (string, error) {
			ch, ok := g.channelReg.Get(channelType)
			if !ok {
				return "", fmt.Errorf("channel %s not found", channelType)
			}
			fetcher, ok := ch.(channel.StickerSetFetcher)
			if !ok {
				return "", fmt.Errorf("channel %s does not support fetching sticker sets", channelType)
			}
			stickers, err := fetcher.FetchStickerSet(setName)
			if err != nil {
				return "", err
			}
			result := g.stickerCollector.LearnStickerSet(channelType, stickers)
			return result, nil
		}
	}

	ci.Register(sc.Handler())
	g.channelReg.SetCommandInterceptor(ci)
}

// WireAgentCommands connects universal slash commands (/think, /status, /mission, /help)
// to all IM channels via the CommandInterceptor.
// Call this after WireStickerCommands (it registers on the existing interceptor).
func (g *Gateway) WireAgentCommands() {
	if g.channelReg == nil {
		return
	}

	ac := &channel.AgentCommands{}

	// Wire /think — per-user thinking level control
	ac.GetThinkingLevel = func(channelType, userID string) string {
		return GetIMThinkingLevel(channelType, userID)
	}
	ac.SetThinkingLevel = func(channelType, userID, level string) string {
		key := channelType + ":" + userID
		imThinkingLevels.Store(key, level)
		labels := map[string]string{
			"none": "⚡ 快速模式 — 极速响应，跳过深度推理",
			"auto": "🤖 自动模式 — 智能路由，按需分配模型",
			"deep": "🧠 深度模式 — 专家模型，深度推理和分析",
		}
		return fmt.Sprintf("已切换: %s", labels[level])
	}

	// Wire /status — agent status summary
	ac.GetStatus = func() string {
		var b strings.Builder
		b.WriteString("*云鸢 Agent 状态*\n\n")

		// Skills count
		if g.registry != nil {
			all := g.registry.All()
			b.WriteString(fmt.Sprintf("🔧 技能: %d 个已加载\n", len(all)))
		}

		// Channel status
		if g.channelReg != nil {
			channels := g.channelReg.All()
			b.WriteString(fmt.Sprintf("📡 渠道: %d 个已连接\n", len(channels)))
		}

		// Memory
		if g.orchestrator != nil {
			b.WriteString("🧠 记忆系统: 在线\n")
		}

		// Model router
		if g.smartRouter != nil {
			b.WriteString("🔀 智能路由: 在线\n")
		}

		// Optimizer
		if g.planner != nil {
			b.WriteString("⚙️ 技能优化器: 在线\n")
		}

		return b.String()
	}

	// Wire /mission — NL mission creation
	ac.CreateMission = func(description string) (string, error) {
		if g.planner == nil {
			return "", fmt.Errorf("planner unavailable")
		}

		ctx := context.Background()
		result, err := g.planner.ParseMissionIntent(ctx, description)
		if err != nil {
			return "", err
		}

		// Auto-create based on parsed result
		var created string
		switch result.Type {
		case "cron":
			cronExpr, _ := result.Config["cron_expr"].(string)
			message, _ := result.Config["message"].(string)
			if cronExpr == "" {
				cronExpr = "0 9 * * *"
			}
			if message == "" {
				message = result.Description
			}
			if g.cronMgr != nil {
				_, err = g.cronMgr.Add(result.Name,
					cron.Schedule{Type: cron.ScheduleCron, CronExpr: cronExpr},
					cron.Payload{Kind: cron.PayloadAgentTurn, Message: message})
				if err != nil {
					return "", fmt.Errorf("cron create: %w", err)
				}
				created = fmt.Sprintf("✅ 已创建定时任务\n\n*%s*\n⏰ %s\n📝 %s",
					result.Name, cronExpr, message)
			}
		case "task":
			if g.taskStore != nil {
				_, err = g.taskStore.Create(task.CreateRequest{
					Title:       result.Name,
					Description: result.Description,
					TenantID:    "default",
				})
				if err != nil {
					return "", err
				}
				created = fmt.Sprintf("✅ 已创建任务\n\n*%s*\n📝 %s",
					result.Name, result.Description)
			}
		default:
			created = fmt.Sprintf("✅ 已识别意图\n\n类型: %s\n名称: %s\n📝 %s\n💡 %s",
				result.Type, result.Name, result.Description, result.Explanation)
		}

		if created == "" {
			created = fmt.Sprintf("已解析: %s (%s) — %s",
				result.Name, result.Type, result.Explanation)
		}
		return created, nil
	}

	// Register on existing interceptor, or create one if needed
	interceptor := g.channelReg.GetInterceptor()
	if interceptor == nil {
		interceptor = channel.NewCommandInterceptor()
		g.channelReg.SetCommandInterceptor(interceptor)
	}
	interceptor.Register(ac.Handler())
}

// WireProgressTracker attaches real-time progress tracking to IM channels.
// Channels with ProgressSender will show step-by-step execution trace
// by editing a "thinking..." message during planner execution.
func (g *Gateway) WireProgressTracker() {
	if g.channelReg == nil {
		return
	}

	tracker := &channel.ProgressTracker{
		Registry:    g.channelReg,
		MinInterval: 800 * time.Millisecond,
	}
	g.channelReg.SetProgressTracker(tracker)
}

// WireStickerEnricher sets up automatic sticker sending based on detected emotion.
// After the planner replies, if the incoming message carries a recognized emotion
// above the confidence threshold, a sticker is appended to the reply.
// Call this after SetStickerMap, SetEmotionAnalyzer, and SetChannelRegistry.
func (g *Gateway) WireStickerEnricher() {
	if g.channelReg == nil || g.stickerMap == nil || g.emotionAnalyzer == nil {
		return
	}

	enricher := &channel.StickerEnricher{
		MinConfidence: 0.5,
		AnalyzeEmotion: func(text string) (string, float64) {
			if !g.emotionAnalyzer.Enabled() {
				return "", 0
			}
			featureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureEmotion)
			if !featureOK {
				return "", 0
			}
			res, err := g.emotionAnalyzer.AnalyzeText(context.Background(), text)
			if err != nil || res == nil {
				return "", 0
			}
			return string(res.Emotion), res.Confidence
		},
		SuggestSticker: func(emo, platform string) *channel.StickerComponent {
			stickerFeatureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureSticker)
			if !stickerFeatureOK {
				return nil
			}
			s := g.stickerMap.Suggest(emotion.Emotion(emo), platform)
			if s == nil {
				return nil
			}
			sc := channel.NewSticker(s.PackageID, s.StickerID)
			sc.Platform = s.Platform
			sc.FileID = s.FileID
			sc.SetName = s.SetName
			sc.Emoji = s.Emoji
			if s.CDNURL != "" {
				sc.URL = s.CDNURL
			}
			return sc
		},
		SendProbability: func() float64 {
			freq := 2.0
			if g.personaChain != nil {
				freq = g.personaChain.FloatFeature(persona.FeatureStickerFrequency, 2)
			}
			return stickerSendProb(freq)
		},
	}

	// Override MinConfidence from persona if available
	if g.personaChain != nil {
		minConf := g.personaChain.FloatFeature(persona.FeatureEmotionMinConfidence, 0.5)
		enricher.MinConfidence = minConf
	}

	g.channelReg.SetStickerEnricher(enricher)
}

// WireReverieActions connects Reverie's action callbacks to the actual subsystems:
//   - write_memory → Memory Orchestrator (ingest as "reverie_insight")
//   - create_task → Task Runner (create a new task)
//   - update_profile → Persona identity (update user profile key-value)
//
// This enables the "reverie → memory → reflection" feedback loop:
// Reverie thinks → writes to memory → memory informs future conversations
// → reflection learns from outcomes → strategies guide Reverie.
// Call this after SetReverie, SetOrchestrator, SetTaskRunner.
func (g *Gateway) WireReverieActions() {
	if g.reverie == nil {
		return
	}

	// write_memory → orchestrator
	if g.orchestrator != nil {
		g.reverie.SetWriteMemory(func(ctx context.Context, fact string) error {
			g.metrics.Cognitive().ReverieAction.Add(1)
			return g.orchestrator.Ingest(ctx, "default", fact, "reverie_insight", "reverie")
		})
	}

	// create_task → task store
	if g.taskStore != nil {
		g.reverie.SetCreateTask(func(ctx context.Context, title, desc string) error {
			g.metrics.Cognitive().ReverieAction.Add(1)
			_, err := g.taskStore.Create(task.CreateRequest{
				Title:       title,
				Description: desc,
				TenantID:    "default",
			})
			return err
		})
	}

	// update_profile → memory as persistent profile fact
	if g.orchestrator != nil {
		g.reverie.SetUpdateProfile(func(ctx context.Context, key, value string) error {
			g.metrics.Cognitive().ReverieAction.Add(1)
			fact := fmt.Sprintf("[用户画像] %s: %s", key, value)
			return g.orchestrator.Ingest(ctx, "default", fact, "profile_update", "reverie")
		})
	}
}

// WireReflectionLoop connects the reflection experience store to the planner
// so that compiled strategies are injected into conversation context.
// This closes the feedback loop: tasks run → experiences recorded → strategies compiled
// → strategies guide future conversations → better outcomes → better experiences.
func (g *Gateway) WireReflectionLoop() {
	if g.experienceStore == nil {
		return
	}
	if g.reflectiveLoop == nil {
		rl := cognikernel.NewReflectiveLoop()
		rl.SetExperienceRecord(func(source, category, outcome, lesson, lctx string, tags []string) {
			g.experienceStore.Add(reflectpkg.Experience{
				Source:   source,
				Category: category,
				Outcome:  outcome,
				Lesson:   lesson,
				Context:  lctx,
				Tags:     tags,
			})
		})
		g.reflectiveLoop = rl
	}
	if g.planner == nil {
		return
	}
	g.planner.SetStrategyContext(func() string {
		strategies := g.experienceStore.CompileStrategies(20)
		if strategies != "" {
			g.metrics.Cognitive().StrategyInject.Add(1)
		}
		return strategies
	})
}

// WireTaskSSE bridges task runner events to the SSE broker.
// Call this AFTER SetTaskRunner and SetSSEBroker.
func (g *Gateway) WireTaskSSE() {
	if g.taskRunner == nil || g.sseBroker == nil {
		return
	}
	g.taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		g.sseBroker.Broadcast(SSEEvent{
			Type: "task." + event,
			Data: map[string]string{
				"task_id": taskID,
				"detail":  detail,
			},
		})
	})
}

// WireFeishuCardActions registers card button handlers on the Feishu channel
// so users can control tasks (pause/cancel/retry) and approve/deny from IM cards.
// Call this AFTER SetChannelRegistry, SetTaskRunner, and SetApprovalManager.
func (g *Gateway) WireFeishuCardActions() {
	if g.channelReg == nil {
		return
	}
	ch, ok := g.channelReg.Get("feishu")
	if !ok {
		return
	}
	feishu, ok := ch.(*channel.Feishu)
	if !ok {
		return
	}

	feishu.SetCardActionHandler(func(openID string, action map[string]string) *channel.Card {
		act := action["action"]
		taskID := action["task_id"]

		switch act {
		case "retry_task":
			if g.taskRunner != nil && taskID != "" {
				go g.taskRunner.Restart(context.Background(), taskID)
				return channel.NewCard("✅ 已重新启动", channel.CardGreen).
					AddMarkdown("任务 `" + taskID + "` 正在重新执行...")
			}

		case "pause_task":
			if g.taskRunner != nil && taskID != "" {
				g.taskRunner.Pause(taskID)
				return channel.NewCard("⏸ 已暂停", channel.CardOrange).
					AddMarkdown("任务 `"+taskID+"` 已暂停").
					AddButton("恢复", channel.BtnPrimary, map[string]string{"action": "resume_task", "task_id": taskID})
			}

		case "resume_task":
			if g.taskRunner != nil && taskID != "" {
				go g.taskRunner.Resume(context.Background(), taskID)
				return channel.NewCard("▶️ 已恢复", channel.CardGreen).
					AddMarkdown("任务 `" + taskID + "` 继续执行...")
			}

		case "cancel_task":
			if g.taskRunner != nil && taskID != "" {
				g.taskRunner.Cancel(taskID)
				return channel.NewCard("🚫 已取消", channel.CardRed).
					AddMarkdown("任务 `" + taskID + "` 已取消")
			}

		case "approve":
			if g.approvalMgr != nil {
				approvalID := action["approval_id"]
				if approvalID == "" {
					approvalID = taskID // fallback
				}
				if err := g.approvalMgr.Approve(approvalID, openID); err != nil {
					return channel.ErrorCard("审批失败: " + err.Error())
				}
				return channel.NewCard("✅ 已批准", channel.CardGreen).
					AddMarkdown("审批已通过")
			}

		case "deny":
			if g.approvalMgr != nil {
				approvalID := action["approval_id"]
				if approvalID == "" {
					approvalID = taskID
				}
				if err := g.approvalMgr.Deny(approvalID, openID, "飞书卡片拒绝"); err != nil {
					return channel.ErrorCard("拒绝失败: " + err.Error())
				}
				return channel.NewCard("❌ 已拒绝", channel.CardRed).
					AddMarkdown("审批已拒绝")
			}
		}

		return nil
	})
}

// WireTelegramCallbackActions registers InlineKeyboard button handlers on the Telegram channel
// so users can control tasks (pause/cancel/retry) and approve/deny from Telegram buttons.
// Call this AFTER SetChannelRegistry, SetTaskRunner, and SetApprovalManager.
func (g *Gateway) WireTelegramCallbackActions() {
	if g.channelReg == nil {
		return
	}
	ch, ok := g.channelReg.Get("telegram")
	if !ok {
		return
	}
	tg, ok := ch.(*channel.Telegram)
	if !ok {
		return
	}

	tg.SetCallbackActionHandler(func(chatID, messageID, userID, data string) *channel.Reply {
		// data format: "action:task_id"
		act, taskID := parseCallbackData(data)
		if act == "" || taskID == "" {
			return nil // not a task control callback, fall through
		}

		switch act {
		case "retry_task":
			if g.taskRunner != nil {
				go g.taskRunner.Restart(context.Background(), taskID)
				reply := channel.Reply{Content: "✅ 任务 `" + taskID + "` 正在重新执行...", Format: "markdown"}
				return &reply
			}

		case "pause_task":
			if g.taskRunner != nil {
				g.taskRunner.Pause(taskID)
				r := channel.TaskPausedReplyTG(taskID)
				return &r
			}

		case "resume_task":
			if g.taskRunner != nil {
				go g.taskRunner.Resume(context.Background(), taskID)
				reply := channel.Reply{Content: "▶️ 任务 `" + taskID + "` 继续执行...", Format: "markdown"}
				return &reply
			}

		case "cancel_task":
			if g.taskRunner != nil {
				g.taskRunner.Cancel(taskID)
				reply := channel.Reply{Content: "🚫 任务 `" + taskID + "` 已取消", Format: "markdown"}
				return &reply
			}

		case "approve":
			if g.approvalMgr != nil {
				if err := g.approvalMgr.Approve(taskID, userID); err != nil {
					reply := channel.Reply{Content: "❌ 审批失败: " + err.Error()}
					return &reply
				}
				reply := channel.Reply{Content: "✅ 审批已通过", Format: "markdown"}
				return &reply
			}

		case "deny":
			if g.approvalMgr != nil {
				if err := g.approvalMgr.Deny(taskID, userID, "Telegram按钮拒绝"); err != nil {
					reply := channel.Reply{Content: "❌ 拒绝失败: " + err.Error()}
					return &reply
				}
				reply := channel.Reply{Content: "❌ 审批已拒绝", Format: "markdown"}
				return &reply
			}
		}

		return nil
	})
}

// parseCallbackData splits "action:taskID" into parts.
func parseCallbackData(data string) (action, taskID string) {
	idx := -1
	for i, c := range data {
		if c == ':' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", ""
	}
	return data[:idx], data[idx+1:]
}

// WireRBAC creates the RBAC enforcer and middleware, configures the subject
// extractor using the gateway's internal auth context, and assigns a default
// "owner" policy for the default tenant.
// Call this AFTER requireAuth is configured (which populates ctxTenantKey + ctxRoleKey).
func (g *Gateway) WireRBAC() {
	enforcer := rbac.NewEnforcer()
	// Default policy: "default" tenant gets owner role.
	enforcer.AssignRole("default", "owner", "default")
	g.rbacEnforcer = enforcer

	g.rbacMiddleware = rbac.NewMiddleware(enforcer, func(r *http.Request) (string, string) {
		ctx := r.Context()
		return roleFromCtx(ctx), tenantFromCtx(ctx)
	})
	slog.Info("rbac: enforcer + middleware wired", "default_roles", len(enforcer.ListRoles()))
}
