package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/runtime/heartbeat"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/config"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/scheduler"
	iledger "yunque-agent/internal/ledger"

	"yunque-agent/internal/ledgercore"
)

type sessionAuthResult struct {
	sched         *scheduler.Scheduler
	convStore     *session.Store
	feishuAPI     *channel.FeishuAPI
	jwtCfg        *gateway.JWTConfig
	botMgr        *bots.Manager
	inboxStore    *inbox.Store
	hbService     *heartbeat.Service
	forkTree      *session.ForkTree
	forkPersister *session.ForkPersister
}

func initSessionAuth(app *agentrt.App) (*sessionAuthResult, error) {
	cfg := app.Config
	p := app.Planner
	r := &sessionAuthResult{}

	// ── Scheduler ──
	r.sched = scheduler.New(func(ctx context.Context, job scheduler.Job) {
		result, err := p.Run(ctx, planner.PlanRequest{
			Messages: []llm.Message{{Role: "user", Content: job.Prompt}},
			TenantID: job.TenantID,
		})
		if err != nil {
			slog.Error("scheduler job failed", "job", job.Name, "err", err)
			return
		}
		slog.Info("scheduler job done", "job", job.Name, "reply_len", len(result.Reply))
	})
	go r.sched.Start(context.Background())
	app.Set(agentrt.CompScheduler, r.sched)

	// ── Session Store ──
	r.convStore = session.NewStore(DefaultSessionCapacity)
	fileRepo, err := session.NewFileRepo(cfg.DataPath("sessions"))
	if err != nil {
		slog.Warn("session file repo init failed", "err", err)
	} else {
		r.convStore.SetRepo(fileRepo)
		loaded := r.convStore.LoadFromRepo("")
		slog.Info("session store: file backend attached", "dir", cfg.DataPath("sessions"), "restored", loaded)
	}
	app.Set(agentrt.CompSessionStore, r.convStore)

	// ── Feishu API ──
	if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
		r.feishuAPI = channel.NewFeishuAPI(cfg.FeishuAppID, cfg.FeishuAppSecret)
	}

	// ── JWT ──
	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		generated, err := config.GenerateSecureKey(32)
		if err != nil {
			return nil, fmt.Errorf("initSessionAuth: failed to auto-generate JWT secret: %w", err)
		}
		jwtSecret = generated
		slog.Warn("JWT_SECRET not set, using auto-generated secure secret")
	}
	r.jwtCfg = &gateway.JWTConfig{
		Secret:     jwtSecret,
		Issuer:     "tori",
		Expiration: 24 * time.Hour,
	}
	slog.Info("jwt initialized", "issuer", r.jwtCfg.Issuer)

	// ── Bot Manager / Inbox ──
	r.botMgr = bots.NewManager()
	r.inboxStore = inbox.NewStore(DefaultInboxCapacity)
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			r.botMgr.SetKVStore(iledger.NewKVConfigStore(ldg, "bots"))
			r.inboxStore.SetKVStore(iledger.NewKVConfigStore(ldg, "inbox"))
		}
	}
	app.Set(agentrt.CompBotManager, r.botMgr)
	app.Set(agentrt.CompInbox, r.inboxStore)

	// ── Heartbeat ──
	// When HEARTBEAT_ENABLED=true AND profile < full (module not loaded), create inline.
	// Otherwise the heartbeatModule handles it via the module registry.
	hbEnabled := os.Getenv("HEARTBEAT_ENABLED") == "true"
	if hbEnabled && cfg.IsModuleDisabled("heartbeat") {
		hbEnabled = false
	}
	if hbEnabled && !cfg.ProfileAtLeast("full") {
		hbInterval := 30
		if v := os.Getenv("HEARTBEAT_INTERVAL"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				hbInterval = n
			}
		}
		r.hbService = heartbeat.New(heartbeat.Config{
			Enabled:  true,
			Interval: time.Duration(hbInterval) * time.Minute,
			MaxLogs:  200,
		}, func(ctx context.Context) (string, error) {
			summary := r.inboxStore.Summary(5)
			prompt := "这是你的心跳时间。请审视你的状态、回顾收件箱并做任何需要的维护工作。"
			if summary != "" {
				prompt += "\n\n" + summary
			}
			result, err := p.Run(ctx, planner.PlanRequest{
				Messages: []llm.Message{{Role: "user", Content: prompt}},
				TenantID: "system",
			})
			if err != nil {
				return "", err
			}
			return result.Reply, nil
		})
		r.hbService.SetOnResult(func(log *heartbeat.Log, policy *heartbeat.DeliveryPolicy) {
			app.Metrics.Cognitive().HeartbeatRun.Add(1)
			// policy is nil when no delivery policy is configured (the default).
			// Dereferencing policy.Targets here previously panicked the whole
			// sidecar on the first heartbeat (e.g. when the LLM call failed),
			// and the supervisor then refused to restart it.
			targets := 0
			if policy != nil {
				targets = len(policy.Targets)
			}
			status := ""
			if log != nil {
				status = log.Status
			}
			slog.Info("heartbeat delivered", "status", status, "targets", targets)
		})
		r.hbService.Start(context.Background())
		slog.Info("heartbeat started (inline)", "interval_min", hbInterval)
		app.Set(agentrt.CompHeartbeat, r.hbService)
	}

	// ── Fork tree ──
	r.forkTree = session.NewForkTree()
	r.forkPersister = session.NewForkPersister(cfg.DataPath("forks.json"))
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			migrator := iledger.NewKVMigrator(ldg)
			_ = migrator.MigrateFile("fork_tree", "forks", cfg.DataPath("forks.json"))
			r.forkPersister.SetKVStore(iledger.NewKVConfigStore(ldg, "fork_tree"))
			slog.Info("fork persister wired to Ledger KV")
		}
	}
	if err := r.forkPersister.Load(r.forkTree); err != nil {
		slog.Warn("fork tree load failed (starting fresh)", "err", err)
	}

	return r, nil
}
