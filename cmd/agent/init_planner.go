package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"yunque-agent/internal/ledgercore"

	"yunque-agent/internal/agentcore/adaptive"
	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/quality"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/trust"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	iledger "yunque-agent/internal/ledger"
)

// buildRecallReranker constructs a cross-encoder reranker for memory recall from
// env (Jina preferred, then Cohere). Returns nil when neither is configured, in
// which case recall keeps the Ledger engine's built-in multi-signal ordering.
func buildRecallReranker() knowledge.Reranker {
	if jinaKey := os.Getenv("JINA_API_KEY"); jinaKey != "" {
		return knowledge.NewJinaReranker(knowledge.JinaRerankerConfig{
			APIKey: jinaKey,
			Model:  os.Getenv("JINA_RERANK_MODEL"),
		})
	}
	if cohereKey := os.Getenv("COHERE_API_KEY"); cohereKey != "" {
		return knowledge.NewCohereReranker(knowledge.CohereRerankerConfig{
			APIKey: cohereKey,
			Model:  os.Getenv("COHERE_RERANK_MODEL"),
		})
	}
	return nil
}

// initPlanner initializes the Planner, context manager, skill optimizer,
// Reverie system, and learning loop.
// Extracted from main.go lines 532-618.
func initPlanner(app *agentrt.App) error {
	cfg := app.Config
	reflectEngine := app.MustGet(agentrt.CompReflectEngine).(*reflectpkg.Engine)
	adaptiveLoop := app.MustGet(agentrt.CompAdaptiveLoop).(*adaptive.Loop)
	personaChain := app.MustGet(agentrt.CompPersonaChain).(*persona.PriorityChain)

	// Create planner
	p := planner.NewPlanner(app.LLMClient, app.SkillRegistry, 30)
	p.SetLLMPool(app.LLMPool)
	p.SetProviderRegistry(app.Providers)
	p.SetPersonaPrompt(personaChain.SystemPromptFunc())
	p.SetWindowConfig(ctxwindow.DefaultConfig())

	// Native function calling mode
	nativeFC := os.Getenv("NATIVE_FC") != "false"
	p.SetNativeFC(nativeFC)

	// Ack mode
	ackEnabled := os.Getenv("ACK_ENABLED") != "false"
	p.SetAckEnabled(ackEnabled)

	// Locale
	if agentLocale := os.Getenv("AGENT_LOCALE"); agentLocale != "" {
		p.SetLocale(agentLocale)
	}

	// Context compression manager
	app.CtxManager = ctxwindow.NewManager(ctxwindow.ManagerConfig{
		MaxContextTokens: MaxContextTokens,
		EnforceMaxTurns:  MaxContextTurns,
		Compressor:       ctxwindow.NewTruncateCompressor(4, 0.82),
	})
	p.SetContextManager(app.CtxManager)
	p.SetDomainPrompt(app.PluginReg.CombinedPrompt())

	// Memory recall → planner
	orchestrator := app.Orchestrator
	p.SetMemory(func(ctx context.Context, tenantID, query string) string {
		app.Metrics.Cognitive().MemoryRecall.Add(1)
		compiled := orchestrator.CompileContext(ctx, tenantID, query)
		if compiled != "" {
			return compiled
		}
		profile := adaptiveLoop.Profile()
		return profile.Compile()
	})

	// ── Reflect Engine v2: intent-aware quality evaluation ──
	reflectEngine.SetMode(app.Config.ReflectMode)
	if app.Config.ReflectModel != "" && app.LLMPool != nil {
		reflectEngine.SetEvalModel(app.LLMPool, app.Config.ReflectModel)
	}
	slog.Info("reflect engine configured", "mode", app.Config.ReflectMode, "eval_model", app.Config.ReflectModel)

	qualityScorer := quality.NewScorer()

	if reflectEngine.Mode() != reflectpkg.ModeOff {
		p.SetReflect(func(ctx context.Context, intent, reply string) bool {
			app.Metrics.Cognitive().ReflectEval.Add(1)
			if classifyIntent(intent) == "relaxed" {
				return true
			}

			// Fast local quality check first → avoids LLM call for clearly good/bad replies
			qScore := qualityScorer.Evaluate(intent, reply)
			if qScore.Overall >= 0.6 {
				return true // clearly good, skip LLM evaluation
			}
			if qScore.Overall < 0.2 {
				slog.Info("reflect: local quality check failed", "score", qScore.Overall)
				return false // clearly bad, retry without LLM call
			}

			// Borderline (0.2-0.6): use LLM for deeper evaluation
			switch reflectEngine.Mode() {
			case reflectpkg.ModeStrict:
				eval, err := reflectEngine.Evaluate(ctx, intent, reply, nil)
				if err != nil {
					return true
				}
				return eval.Satisfied

			case reflectpkg.ModeLearning:
				go func() {
					aCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					eval, err := reflectEngine.Evaluate(aCtx, intent, reply, nil)
					if err != nil {
						return
					}
					slog.Info("reflect: async evaluation",
						"satisfied", eval.Satisfied, "quality", eval.Quality,
						"local_score", qScore.Overall)
				}()
				return true

			default:
				return true
			}
		})
	}

	p.SetSkillMetrics(app.Metrics.RecordSkillCall)

	// Skill optimizer
	app.SkillOptimizer = planner.NewSkillOptimizer(app.Metrics, cfg.DataPath("skill_performance.json"))
	p.SetSkillOptimizer(app.SkillOptimizer)
	slog.Info("skill optimizer initialized", "persist", cfg.DataPath("skill_performance.json"))

	checkpointPath := cfg.DataPath("planner", "checkpoints.jsonl")
	p.SetLongHorizonCheckpointStore(planner.NewFileLongHorizonCheckpointStore(checkpointPath))
	slog.Info("planner checkpoint store initialized", "persist", checkpointPath)

	// Ledger Recall → Planner graphContext
	// When planning, the Planner queries Ledger for historical experiences
	// using 7-factor scoring (keyword, goal, kind, recency, confidence, frequency, trust).
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			recallBridge := iledger.NewRecallBridge(ldg, defaultTenantID())
			if kr := buildRecallReranker(); kr != nil {
				recallBridge.SetReranker(func(ctx context.Context, query string, docs []string, topK int) ([]int, error) {
					res, err := kr.Rerank(ctx, query, docs, topK)
					if err != nil {
						return nil, err
					}
					idx := make([]int, 0, len(res))
					for _, r := range res {
						idx = append(idx, r.Index)
					}
					return idx, nil
				})
				slog.Info("ledger recall: cross-encoder reranker attached", "provider", kr.Name())
			}
			p.SetGraphContextForTenant(recallBridge.QueryTenant)
			slog.Info("ledger recall bridge attached to planner (tenant-aware, union with system)")

			// Wire Ledger instance → Planner for ReAct/Reasoning/Eval
			p.SetLedger(ldg)
			if app.Config.ReActEnabled {
				p.SetReActMode(true)
				slog.Info("planner: ReAct mode enabled (Ledger-powered)")
			}
			if app.Config.LongHorizonEnabled {
				p.SetLongHorizonMode(true)
				slog.Info("planner: Long-horizon DAG planner enabled")
			}
		}
	}

	// Reverie system
	reverieCfg := planner.DefaultReverieConfig()
	if os.Getenv("REVERIE_ENABLED") == "false" {
		reverieCfg.Enabled = false
	}
	if iv := os.Getenv("REVERIE_INTERVAL"); iv != "" {
		if d, err := time.ParseDuration(iv); err == nil {
			reverieCfg.Interval = d
		}
	}
	app.Reverie = planner.NewReverie(reverieCfg)
	app.Reverie.SetLLMCall(app.LLMBreaker.Call)
	app.Reverie.SetRecall(func(query string) string {
		return orchestrator.CompileContext(context.Background(), "system", query)
	})

	// Event-driven Reverie triggers
	reverieEventBus := planner.NewReverieEventBus(ReverieCooldownMinutes * time.Minute)
	app.Reverie.SetEventBus(reverieEventBus)
	emotionShiftDetector := planner.NewEmotionShiftDetector(reverieEventBus)
	taskFailureMonitor := planner.NewTaskFailureMonitor(reverieEventBus, 0.5, 10*time.Minute, 3)
	factEventHook := planner.NewFactEventHook(reverieEventBus, 3)
	p.SetTaskFailureMonitor(taskFailureMonitor)
	p.SetReverie(app.Reverie)

	app.Set("emotion_shift_detector", emotionShiftDetector)
	app.Set("fact_event_hook", factEventHook)
	app.Set("reverie_event_bus", reverieEventBus)

	slog.Info("reverie initialized", "enabled", reverieCfg.Enabled, "interval", reverieCfg.Interval, "event_triggers", true)

	app.Metrics.SetBreakerStatus(func() (string, int) {
		health := p.ModelRuntimeHealth()
		if !health.Configured {
			return "unconfigured", 0
		}
		return health.BreakerState, health.Failures
	})

	// Learning loop
	learningLoop := reflectpkg.NewLearningLoop(app.LLMClient, func(key, value string) {
		_ = app.MemManager.AddMid(context.Background(), "system", memory.Item{Key: key, Value: value, Source: "learning_loop"})
	})
	app.Set("learning_loop", learningLoop)

	// ── Trust Score System ──
	trustTracker := trust.NewTracker(cfg.DataPath("trust_scores.json"))

	// Pre-seed trust for built-in skills so they don't get blocked on first run.
	var seededCount int
	for _, sk := range app.SkillRegistry.All() {
		entry := trustTracker.Get(sk.Name())
		if entry.Score == 0 && entry.Executions == 0 {
			trustTracker.Seed(sk.Name(), 80)
			seededCount++
		}
	}
	if seededCount > 0 {
		slog.Info("trust: pre-seeded built-in skills", "count", seededCount, "score", 80)
	}

	app.Set("trust_tracker", trustTracker)

	// Record trust outcomes after each skill execution
	p.SetTrustRecord(func(skillName string, success bool) {
		if success {
			trustTracker.RecordSuccess(skillName)
		} else {
			trustTracker.RecordFailure(skillName, 1)
		}
	})

	// Trust gate: classify skills by permission requirement based on name/keywords.
	// Shell/exec skills need PermShell (score ≥80); network skills need PermNetwork (score ≥60).
	// New skills start at score 0 and must earn trust through successful executions.
	// Disable the gate with TRUST_GATE_DISABLED=true for development environments.
	if os.Getenv("TRUST_GATE_DISABLED") != "true" {
		p.SetTrustCheck(func(skillName string) error {
			name := strings.ToLower(skillName)
			required := trust.PermReadOnly
			switch {
			case containsAny(name, "shell", "exec", "cmd", "run_command", "terminal", "bash", "sh"):
				required = trust.PermShell
			case containsAny(name, "http", "fetch", "request", "browse", "web", "search", "download", "api"):
				required = trust.PermNetwork
			case containsAny(name, "write", "save", "create", "delete", "update", "modify", "upload"):
				required = trust.PermWrite
			}
			if required == trust.PermReadOnly {
				return nil // no restriction
			}
			entry := trustTracker.Get(skillName)
			if !trustTracker.CheckPermission(skillName, required) {
				return fmt.Errorf("skill %q requires %s trust (current score: %d/%d)",
					skillName, required, entry.Score, permThreshold(required))
			}
			return nil
		})
		slog.Info("trust gate enabled", "path", cfg.DataPath("trust_scores.json"))
	} else {
		slog.Info("trust gate disabled (TRUST_GATE_DISABLED=true)")
	}

	// Per-tool execution timeout. Default 60s is too short for slow generative
	// tools (e.g. deck_create asks an LLM to design a full HTML deck, which can
	// take 1-3 min on a reasoning model). Override via TOOL_TIMEOUT (Go duration,
	// e.g. "240s" or "4m").
	if v := os.Getenv("TOOL_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			p.SetToolTimeout(d)
			slog.Info("planner: tool timeout overridden", "timeout", d)
		}
	}

	// ── CognitivePlugin Context Injection ──
	// Wire CognitivePlugin.DynamicContext into every planning request so
	// plugins can inject domain-specific knowledge into the LLM's thinking.
	p.SetCognitiveContext(func(ctx context.Context, userMessage string) string {
		return app.PluginReg.CollectDynamicContext(ctx, userMessage)
	})
	cogCount := len(app.PluginReg.AllCognitive())
	if cogCount > 0 {
		slog.Info("cognitive plugins registered", "count", cogCount)
	}

	app.Planner = p
	slog.Info("planner initialized", "native_fc", nativeFC, "ack", ackEnabled)

	// ── Ledger KV Persistence ──
	// Migrate legacy JSON files into Ledger KV and wire KV stores to subsystems.
	// This centralizes all state into a single SQLite-backed database.
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			migrator := iledger.NewKVMigrator(ldg)
			_ = migrator.MigrateFile("trust", "scores", cfg.DataPath("trust_scores.json"))
			_ = migrator.MigrateFile("skill_optimizer", "history", cfg.DataPath("skill_performance.json"))
			_ = migrator.MigrateFile("reverie", "journal", cfg.DataPath("reverie.json"))

			trustTracker.SetKVStore(iledger.NewKVConfigStore(ldg, "trust"))
			app.SkillOptimizer.SetKVStore(iledger.NewKVConfigStore(ldg, "skill_optimizer"))
			app.Reverie.SetKVStore(iledger.NewKVConfigStore(ldg, "reverie"))
			slog.Info("ledger KV wired to trust/optimizer/reverie")
		}
	}

	return nil
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// permThreshold returns the minimum trust score required for a permission level.
func permThreshold(p trust.PermLevel) int {
	switch p {
	case trust.PermShell:
		return 80
	case trust.PermNetwork:
		return 60
	case trust.PermWrite:
		return 30
	default:
		return 0
	}
}

// classifyIntent returns "strict" for work/task messages that need quality
// evaluation, and "relaxed" for casual/emotional messages that don't.
// This is a cheap rule-based classifier → zero LLM cost.
func classifyIntent(msg string) string {
	msg = strings.TrimSpace(msg)
	// Strip channel YAML header (e.g. "[username]: hi")
	if idx := strings.Index(msg, "]: "); idx >= 0 && idx < 60 {
		msg = strings.TrimSpace(msg[idx+3:])
	}

	r := []rune(msg)
	// Very short messages are always casual
	if len(r) <= 10 {
		return "relaxed"
	}

	lower := strings.ToLower(msg)

	// Strict: work/task keywords → need quality check
	strictWords := []string{
		"代码", "编程", "编写", "写个", "帮我写", "帮我找",
		"分析", "搜索", "查找", "查询", "计算", "统计",
		"数据", "api", "报告", "调试", "debug", "bug",
		"执行", "部署", "配置", "安装", "运行",
		"文件", "数据库", "服务器", "脚本",
		"function", "class", "import", "def ", "func ",
		"翻译", "总结", "提取", "转换", "格式化",
	}
	for _, w := range strictWords {
		if strings.Contains(lower, w) {
			return "strict"
		}
	}

	// Relaxed: emotional/creative/casual keywords → skip quality check
	relaxedWords := []string{
		"聊聊", "聊天", "陪我", "陪伴",
		"心情", "难过", "开心", "伤心", "焦虑", "嗨",
		"故事", "小说", "写诗", "散文",
		"角色扮演", "cosplay", "扮演", "roleplay",
		"倾诉", "安慰", "鼓励",
		"情感", "恋爱", "感情",
		"你好", "嗨", "哈喽", "在吗",
		"早上好", "晚上好", "晚安",
		"谢谢", "感谢", "ok", "好的",
		"哈哈", "嘿嘿", "呵呵", "lol",
	}
	for _, w := range relaxedWords {
		if strings.Contains(lower, w) {
			return "relaxed"
		}
	}

	// Default: relaxed → prefer not to block
	return "relaxed"
}
