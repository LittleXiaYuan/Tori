package main

import (
	"log/slog"
	"os"

	"yunque-agent/internal/ledgercore"

	"yunque-agent/internal/agentcore/localbrain"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/cognicore/recommend"
	"yunque-agent/internal/experimental/microagent"
	"yunque-agent/internal/experimental/rlsched"
	iledger "yunque-agent/internal/ledger"
)

// initIntelligence (Phase 6.8) registers the LocalBrain decision layer,
// AgenticThinking router, LoRA scheduler, microagent registry, RL scheduler,
// and recommend engine.
//
// MetaCog, World Model, Causal Engine, Curiosity and TaskDistiller are owned
// by initSoulLayer (Phase 7), which builds them with cost-aware LLM hooks,
// Reverie bus bridges, and tenant "default" alignment. They were previously
// re-created here, leading to duplicate event subscriptions and tenant
// inconsistency; that wiring has been removed in favor of the soul layer's.

func initIntelligence(app *agentrt.App) error {
	// ?? LocalBrain: local small model decision layer ??
	// Priority: "local" pool key ??"fast" key ??first available key.
	// LocalBrain pre-classifies intent cheaply; even when backed by the same
	// model, the fast-path (NeedTools=false for greetings) avoids injecting
	// 40+ tool definitions and cuts latency for trivial queries.
	var brain *localbrain.LocalBrain
	lbClient := app.LLMPool.Get("local")
	lbLabel := "local"
	if lbClient == nil {
		lbClient = app.LLMPool.Get("fast")
		lbLabel = "fast"
	}
	if lbClient == nil {
		lbClient = app.LLMPool.Get("smart")
		lbLabel = "smart (fallback)"
	}
	if lbClient == nil {
		lbClient = app.LLMPool.Get("primary")
		lbLabel = "primary (fallback)"
	}
	if lbClient != nil {
		brain = localbrain.New(lbClient, app.LLMPool)
		app.Planner.SetLocalBrain(brain)

		if routerRaw, ok := app.Get(agentrt.CompSmartRouter); ok {
			if r, ok := routerRaw.(interface {
				SetLocalBrain(*localbrain.LocalBrain)
			}); ok {
				r.SetLocalBrain(brain)
			}
		}

		slog.Info("localbrain: initialized", "backend", lbLabel)
	} else {
		slog.Warn("localbrain: no LLM client available, disabled")
	}
	app.Set("localbrain", brain)

	// ?? LoRA Scheduler: training lifecycle for small model evolution ??
	if brain != nil && app.Ledger != nil {
		loraCfg := localbrain.DefaultSchedulerConfig()
		loraCfg.BaseModel = os.Getenv("LOCALBRAIN_BASE_MODEL")
		if loraCfg.BaseModel == "" {
			loraCfg.BaseModel = "qwen-2.5-7b"
		}
		if dir := os.Getenv("LORA_ADAPTER_DIR"); dir != "" {
			loraCfg.AdapterDir = dir
		}
		if dir := os.Getenv("TRAINING_OUTPUT_DIR"); dir != "" {
			loraCfg.TrainingDataDir = dir
		}

		var loraAdapter *localbrain.LoRAAdapter
		if vllmURL := os.Getenv("VLLM_BASE_URL"); vllmURL != "" {
			loraAdapter = localbrain.NewLoRAAdapter(localbrain.LoRAAdapterConfig{
				BaseURL: vllmURL,
				APIKey:  os.Getenv("VLLM_API_KEY"),
			})
		}

		scheduler := localbrain.NewLoRAScheduler(app.Ledger, loraAdapter, brain, loraCfg)
		scheduler.LoadState()
		// Wire LoRAScheduler → Ledger KV
		if app.Ledger != nil {
			lmigrator := iledger.NewKVMigrator(app.Ledger)
			stateFile := loraCfg.AdapterDir
			if stateFile == "" {
				stateFile = "./data/adapters"
			}
			_ = lmigrator.MigrateFile("lora_scheduler", "state", stateFile+"/scheduler_state.json")
			scheduler.SetKVStore(iledger.NewKVConfigStore(app.Ledger, "lora_scheduler"))
			slog.Info("lora scheduler wired to Ledger KV")
		}

		// Wire LoRA Trainer (remote + local hybrid; auto mode always installs)
		trainerCfg := localbrain.TrainerConfigFromEnv()
		trainer := localbrain.NewLoRATrainer(trainerCfg)
		scheduler.SetTrainFunc(trainer.TrainFunc())
		slog.Info("lora trainer: initialized",
			"mode", trainerCfg.Mode,
			"remote_url", trainerCfg.RemoteURL,
			"script", trainerCfg.ScriptPath,
		)

		// Wire LoRA Evaluator (post-training quality gate)
		vllmURL := os.Getenv("VLLM_BASE_URL")
		evaluator := localbrain.NewLoRAEvaluator(localbrain.EvaluatorConfig{
			InferenceURL: vllmURL,
			APIKey:       os.Getenv("VLLM_API_KEY"),
			BaseModel:    loraCfg.BaseModel,
			MinScore:     loraCfg.EvalMinScore,
		})
		scheduler.SetEvalFunc(evaluator.EvalFunc())
		if vllmURL != "" {
			slog.Info("lora evaluator: initialized", "inference_url", vllmURL)
		} else {
			slog.Info("lora evaluator: initialized in passthrough mode (no vLLM)")
		}

		// Wire Training Metrics (observability)
		metricsDir := loraCfg.AdapterDir
		if metricsDir == "" {
			metricsDir = "./data/adapters"
		}
		metrics := localbrain.NewTrainingMetrics(metricsDir)
		scheduler.SetMetrics(metrics)
		app.Set("training_metrics", metrics)

		app.Set("lora_scheduler", scheduler)
		slog.Info("lora scheduler: initialized",
			"base_model", loraCfg.BaseModel,
			"min_samples", loraCfg.MinSamples,
			"has_vllm", loraAdapter != nil,
			"history_count", metrics.Count(),
		)

		// Wire Evolution Coordinator (multi-layer evolution orchestration)
		coordCfg := localbrain.DefaultCoordinatorConfig()
		coordCfg.StateDir = metricsDir
		coordinator := localbrain.NewEvolutionCoordinator(app.Ledger, brain, scheduler, metrics, coordCfg)
		app.Set("evolution_coordinator", coordinator)
		coordState := coordinator.State()
		slog.Info("evolution coordinator: initialized",
			"total_tasks", coordState.TotalTasks,
			"success_rate", coordState.RollingSuccessRate,
			"strategy_interval", coordCfg.StrategyInterval,
			"weight_threshold", coordCfg.WeightHitRateThreshold,
		)
	}

	// AgenticThinking: dynamic think depth
	var ldg *ledger.Ledger
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		ldg, _ = ldgRaw.(*ledger.Ledger)
	}

	agenticCfg := localbrain.DefaultAgenticConfig()
	at := localbrain.NewAgenticThinking(brain, app.LLMPool, ldg, agenticCfg)
	app.Planner.SetAgenticThinking(at)
	app.Set("agentic_thinking", at)
	slog.Info("agentic thinking: initialized", "default_level", agenticCfg.DefaultThinkLevel)

	// MetaCog, World Model, Causal Engine, Curiosity and TaskDistiller are
	// initialized by initSoulLayer with full wiring (cost-aware LLM, Reverie
	// bridge, tenant "default"). Re-creating bare versions here previously
	// caused duplicate goroutine subscriptions and tenant key drift.

	// ── MicroAgent Registry: domain-specific prompt enhancement ──
	maRegistry := microagent.NewRegistry()
	maDir := app.Config.DataPath("microagents")
	if _, err := os.Stat(maDir); err == nil {
		count, loadErr := microagent.LoadFromDirectory(maDir, microagent.ScopeGlobal, maRegistry)
		if loadErr != nil {
			slog.Warn("microagent: load error", "dir", maDir, "err", loadErr)
		} else if count > 0 {
			slog.Info("microagent: loaded from directory", "dir", maDir, "count", count)
		}
	}
	app.Set("microagent_registry", maRegistry)
	slog.Info("microagent registry: initialized", "agents", len(maRegistry.All()))

	// ?? RL Scheduler: Q-Learning task scheduling optimization ??
	rlActions := []string{"model_fast", "model_smart", "priority_high", "priority_normal", "priority_low"}
	rlCfg := rlsched.DefaultQLearnerConfig(rlActions)
	ql := rlsched.NewQLearner(rlCfg)
	app.Set("rl_scheduler", ql)
	slog.Info("rl scheduler: initialized", "actions", len(rlActions), "epsilon", rlCfg.Epsilon)

	// ?? Recommend Engine: personalized response/skill suggestions ??
	recEngine := recommend.NewEngine()
	app.Planner.SetSkillRecommendationEngine(recEngine)
	app.Set("recommend_engine", recEngine)
	slog.Info("recommend engine: initialized")

	return nil
}
