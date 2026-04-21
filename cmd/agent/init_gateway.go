package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"yunque-agent/internal/backup"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/internal/desktop"
	"yunque-agent/internal/tori"
	"yunque-agent/internal/updater"
	pluginpkg "yunque-agent/pkg/plugin"
	"yunque-agent/pkg/safego"
)

// initGateway initializes the HTTP server, starts channels, and sets up
// the shutdown handler. This is the final initialization phase.
// Extracted from main.go lines 1937-2397.
func initGateway(app *agentrt.App) error {
	cfg := app.Config
	p := app.Planner
	gw := app.MustGet(agentrt.CompGateway).(*gateway.Gateway)
	channelReg := app.MustGet(agentrt.CompChannelReg).(*channel.Registry)
	tenantMgr := app.MustGet(agentrt.CompTenantMgr).(*tenant.Manager)
	sched := app.MustGet(agentrt.CompScheduler).(*scheduler.Scheduler)
	convStore := app.MustGet(agentrt.CompSessionStore).(*session.Store)
	emotionHistory := app.MustGet("emotion_history").(*emotion.History)
	stickerCollector := app.MustGet("sticker_collector").(*emotion.StickerCollector)
	emotionShiftDetector := app.MustGet("emotion_shift_detector").(*planner.EmotionShiftDetector)
	learningLoop := app.MustGet("learning_loop").(*reflectpkg.LearningLoop)
	skillOptimizer := app.SkillOptimizer
	engagementProfile := app.MustGet("engagement_profile").(channel.EngagementProfile)

	// ── Runtime Pool ──
	defTID := os.Getenv("DEFAULT_TENANT_ID")
	if defTID == "" {
		defTID = "default"
	}
	defaultRT := &agentrt.AgentRuntime{
		Config: agentrt.AgentConfig{
			ID: defTID, Name: "tori", Description: "Default Yunque Agent",
		},
		Planner: p, Memory: app.MemManager, Orchestrator: app.Orchestrator, Sessions: convStore,
	}
	rtPool := agentrt.NewPool()
	rtPool.Register(defaultRT)
	bindingRouter := agentrt.NewRouter(rtPool)
	gw.SetRuntimePool(rtPool)
	gw.SetBindingRouter(bindingRouter)
	app.RuntimePool = rtPool
	slog.Info("runtime pool + binding router initialized", "agents", rtPool.Count())

	gw.WireGraphToPlanner()
	gw.WireStickerCommands()
	gw.WireStickerEnricher()
	gw.WireTaskSSE()
	gw.WireFeishuCardActions()
	gw.WireTelegramCallbackActions()

	// ── RBAC middleware ──
	gw.WireRBAC()

	// ── Reflection loop ──
	// initTasks already created the ExperienceStore (wired to Ledger KV +
	// planner.SetStrategyContext). Reuse it so lessons captured by the
	// learning loop here flow into the same store the planner reads from.
	// Falling back to a fresh store keeps the gateway functional in lean
	// configurations that skip initSoulLayer.
	experienceStore, ok := gw.GetExperienceStore()
	if !ok || experienceStore == nil {
		experienceStore = reflectpkg.NewExperienceStore(cfg.DataPath("experience.json"))
		gw.SetExperienceStore(experienceStore)
		slog.Info("reflection: gateway-owned ExperienceStore created (initTasks did not)")
	}
	gw.WireReflectionLoop()
	// Wire LearningLoop → ExperienceStore so lessons become actionable strategies
	learningLoop.SetOnLesson(func(category, outcome, lesson, lctx string, tags []string) {
		app.Metrics.Cognitive().LessonLearned.Add(1)
		experienceStore.Add(reflectpkg.Experience{
			Source:   "learning_loop",
			Category: category,
			Outcome:  outcome,
			Lesson:   lesson,
			Context:  lctx,
			Tags:     tags,
		})
	})
	slog.Info("reflection loop wired (experience→strategies→planner)")

	// ── Reverie actions (write_memory, create_task, update_profile) ──
	gw.WireReverieActions()

	// ── Browser Extension Hub (replaces headless engine) ──
	browserHub := gateway.NewBrowserHub()
	gw.SetBrowserHub(browserHub)
	slog.Info("browser extension hub initialized")

	// ── Work Orchestration (IDE dispatch daemon) ──
	initWorkOrchestrator(app, gw)

	// ── Start Channels ──
	channelCtx, channelCancel := context.WithCancel(context.Background())

	// Wire Reverie delivery — with Inbox fallback when no channel targets are configured
	inboxStoreRaw, _ := app.Get(agentrt.CompInbox)
	inboxStoreForReverie, _ := inboxStoreRaw.(*inbox.Store)
	wireReverieDelivery(app.Reverie, channelReg, inboxStoreForReverie, channelCtx)
	app.Reverie.Start(channelCtx)

	// Start all channels with message handler
	hookMgrRaw, _ := app.Get("hook_manager")
	hookMgr, _ := hookMgrRaw.(*pluginpkg.HookManager)
	channelReg.StartAll(channelCtx, buildChannelHandler(
		p, convStore, channelReg, app.Orchestrator, app.Reverie,
		emotionHistory, stickerCollector, emotionShiftDetector,
		learningLoop, skillOptimizer, tenantMgr, engagementProfile, gw,
		channelCtx, hookMgr, app,
	))

	// ── Group Heartbeat ──
	if engagementProfile.HeartbeatEnabled && channelReg.Inbox() != nil {
		hb := channel.NewGroupHeartbeat(
			channelReg.Inbox(), &engagementProfile, channelReg,
			func(ctx context.Context, inboxContext string) (string, string, string) {
				hbProfile := channelReg.CurrentEngagement()
				if !hbProfile.HeartbeatEnabled {
					return "", "", ""
				}
				heartbeatPrompt := `你收到了一个心跳检查。以下是群组中你没有被直接@的近期消息。浏览这些消息，决定是否要主动参与讨论。
- 如果有值得回应的内容，直接给出你的回复
- 如果没什么需要回应的，回复空字符串 ""
- 不要重复问候或闲聊

` + inboxContext
				result, err := p.Run(ctx, planner.PlanRequest{
					Messages:          []llm.Message{{Role: "user", Content: heartbeatPrompt}},
					TenantID:          "default",
					IsGroup:           true,
					GroupSystemPrompt: hbProfile.GroupSystemPrompt,
				})
				if err != nil {
					slog.Warn("heartbeat planner error", "err", err)
					return "", "", ""
				}
				reply := strings.TrimSpace(result.Reply)
				if reply == "" || reply == `""` || strings.EqualFold(reply, "heartbeat_ok") {
					return "", "", ""
				}
				items := channelReg.Inbox().Peek()
				if len(items) > 0 {
					last := items[len(items)-1]
					return reply, last.ChannelID, last.ChannelType
				}
				return "", "", ""
			},
		)
		go hb.Start(channelCtx)
		slog.Info("group heartbeat started", "mode", engagementProfile.Mode, "interval", engagementProfile.HeartbeatInterval)
	}

	// ── Start Lifecycle (memory GC, promoter, night scheduler, etc.) ──
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	if err := app.Lifecycle.Start(lifecycleCtx); err != nil {
		lifecycleCancel()
		channelCancel()
		return fmt.Errorf("lifecycle start: %w", err)
	}

	// Auto-backup scheduler
	backupCfg := backup.DefaultConfig()
	if os.Getenv("AUTO_BACKUP") == "false" {
		backupCfg.Enabled = false
	}
	safego.Go("auto-backup", func() { backup.StartScheduler(lifecycleCtx, backupCfg) })

	// Auto-update checker
	updateCfg := updater.DefaultConfig()
	if os.Getenv("AUTO_UPDATE_CHECK") == "false" {
		updateCfg.Enabled = false
	}
	updateChecker := updater.NewChecker(updateCfg)
	app.Set("update_checker", updateChecker)
	gw.SetUpdateChecker(func() (string, string, bool) {
		rel, hasNew := updateChecker.Latest()
		if rel == nil {
			return "", "", false
		}
		return rel.TagName, rel.HTMLURL, hasNew
	})
	safego.Go("auto-update-checker", func() { updateChecker.Start(lifecycleCtx) })

	// ── Tori Integration ──
	toriCfg := tori.DefaultOAuthConfig()
	if toriURL := os.Getenv("TORI_URL"); toriURL != "" {
		toriCfg.ToriBaseURL = toriURL
	}
	toriTokenStore := tori.NewTokenStore(toriCfg)
	gw.SetToriTokenStore(toriTokenStore)
	if t := toriTokenStore.Get(); t != nil {
		tori.ApplyLLMConfig(t.ToriBaseURL, t.APIKey)
		safego.Go("tori-token-refresh", func() { toriTokenStore.StartAutoRefresh(lifecycleCtx) })
	}

	// ── HTTP Server ──
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      gw,
		ReadTimeout:  DefaultHTTPReadTimeout,
		WriteTimeout: DefaultHTTPWriteTimeout,
		IdleTimeout:  DefaultHTTPIdleTimeout,
	}
	app.Set(agentrt.CompHTTPServer, srv)

	// Auto-find an available port if the configured one is busy
	actualAddr := cfg.Addr
	ln, err := net.Listen("tcp", actualAddr)
	if err != nil {
		slog.Warn("configured port busy, auto-finding alternative", "addr", actualAddr, "err", err)
		ln, err = net.Listen("tcp", ":0")
		if err != nil {
			slog.Error("cannot bind any port", "err", err)
			fmt.Fprintf(os.Stderr, "\n[FATAL] Cannot bind any port: %v\n", err)
			os.Exit(2)
		}
		actualAddr = ln.Addr().String()
		slog.Info("auto-selected alternative port", "addr", actualAddr)
	}
	srv.Addr = actualAddr

	go func() {
		slog.Info("yunque-agent listening", "addr", actualAddr)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	browserURL := "http://localhost" + actualAddr
	if os.Getenv("OPEN_BROWSER") != "false" {
		go openBrowser(browserURL)
	}

	// Hide console window if requested (Windows only)
	if os.Getenv("HIDE_CONSOLE") == "true" {
		desktop.HideConsole()
		slog.Info("console window hidden (set HIDE_CONSOLE=false to show)")
	}

	// Start system tray icon (Windows only, non-blocking on other platforms)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go desktop.StartTray(desktop.TrayConfig{
		BrowserURL: browserURL,
		OnQuit: func() {
			quit <- syscall.SIGTERM
		},
	})

	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), GracefulShutdownTimeout)
	defer shutdownCancel()

	slog.Info("shutting down: cancelling background goroutines...")
	lifecycleCancel()

	slog.Info("shutting down: stopping channels...")
	channelCancel()

	// Stop services in reverse dependency order
	cronMgr, _ := app.Get(agentrt.CompCronMgr)
	triggerRT, _ := app.Get("trigger_rt")

	if cm, ok := cronMgr.(interface{ Stop() }); ok {
		cm.Stop()
	}
	if tr, ok := triggerRT.(interface{ Stop() }); ok {
		tr.Stop()
	}
	sched.Stop()
	convStore.StopGC()
	app.MemManager.StopPersister()

	// Save persistent state BEFORE closing Ledger
	slog.Info("shutting down: flushing persistent state...")
	if orchPersist, ok := app.Get(agentrt.CompOrchPersist); ok {
		if op, ok := orchPersist.(interface{ Save() error }); ok {
			if err := op.Save(); err != nil {
				slog.Error("orchestrator save failed", "err", err)
			}
		}
	}
	if al, ok := app.Get(agentrt.CompAdaptiveLoop); ok {
		if loop, ok := al.(interface{ SaveTo(string) error }); ok {
			if err := loop.SaveTo(cfg.DataPath("adaptive.json")); err != nil {
				slog.Error("adaptive save failed", "err", err)
			}
		}
	}
	emotionHistory.Flush()
	if ib, ok := app.Get(agentrt.CompInbox); ok {
		if flush, ok := ib.(interface{ FlushToKV() }); ok {
			flush.FlushToKV()
		}
	}
	if ks, ok := app.Get(agentrt.CompKnowledgeStore); ok {
		if flush, ok := ks.(interface{ FlushToKV() }); ok {
			flush.FlushToKV()
		}
	}
	if sk, ok := app.Get(agentrt.CompStateKernel); ok {
		if saver, ok := sk.(interface{ Save() error }); ok {
			if err := saver.Save(); err != nil {
				slog.Error("state kernel save failed", "err", err)
			}
		}
	}
	if idr, ok := app.Get(agentrt.CompIdentityRes); ok {
		if flush, ok := idr.(interface{ FlushToKV() }); ok {
			flush.FlushToKV()
		}
	}
	gw.FlushUsageKV()
	app.PluginReg.ShutdownAll(shutdownCtx)

	// Close Ledger/lifecycle LAST — after all state has been flushed
	slog.Info("shutting down: closing storage...")
	app.Lifecycle.Stop(shutdownCtx)

	desktop.StopTray()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "err", err)
	}
	slog.Info("yunque-agent stopped gracefully")
	return nil
}

// truncChannel truncates a string to maxLen runes (for channel handler logging).
func truncChannel(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}

func openBrowser(url string) {
	time.Sleep(500 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		slog.Debug("auto-open browser failed", "err", err)
	}
}

