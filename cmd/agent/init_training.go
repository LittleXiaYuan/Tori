package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/appdir"
)

// initTrainingPipeline is Phase 9: sets up the DataCollector (writes training
// pairs to Ledger on every successful exchange) and the NightScheduler
// (batch export + lifecycle cleanup).
func initTrainingPipeline(app *agentrt.App) {
	if app.Ledger == nil {
		slog.Info("training pipeline: skipped (no Ledger)")
		return
	}

	// ── DataCollector: auto-collects successful conversation pairs ──
	enabled := os.Getenv("TRAINING_COLLECTOR") != "false"
	dc := planner.NewDataCollector(app.Ledger, planner.DataCollectorConfig{
		MinQueryLen: 10,
		MinReplyLen: 20,
		MinScore:    0.5,
		Enabled:     enabled,
	})
	app.Planner.SetDataCollector(dc)
	// Capture the persona + recalled-memory each collected turn was conditioned
	// on, so the self-distill exporter replays them as the training system prompt
	// (keeps an online loop from grinding a fine-tuned persona into a generic
	// assistant). Sources mirror the planner's own persona/memory wiring.
	if pc, ok := app.Get(agentrt.CompPersonaChain); ok {
		if chain, ok := pc.(*persona.PriorityChain); ok {
			dc.SetPersonaProvider(chain.SystemPromptFunc())
		}
	}
	if app.Orchestrator != nil {
		orch := app.Orchestrator
		dc.SetMemoryProvider(func(ctx context.Context, tenantID, query string) string {
			return orch.CompileContext(ctx, tenantID, query)
		})
	}
	app.Set("data_collector", dc)
	slog.Info("training data collector: initialized", "enabled", enabled)

	// ── NightScheduler: batch lifecycle + training export ──
	outputDir := os.Getenv("TRAINING_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = appdir.Sub("training")
	}

	ns := planner.NewNightScheduler(app.Ledger, planner.NightSchedulerConfig{
		OutputDir: outputDir,
		TenantIDs: func() []string {
			raw := os.Getenv("TRAINING_TENANT_IDS")
			if raw == "" {
				return []string{"default"}
			}
			var ids []string
			for _, id := range strings.Split(raw, ",") {
				id = strings.TrimSpace(id)
				if id != "" {
					ids = append(ids, id)
				}
			}
			return ids
		},
	})
	// Wire LoRA scheduler hook if available
	if loraRaw, ok := app.Get("lora_scheduler"); ok {
		if hook, ok := loraRaw.(planner.LoRAHook); ok {
			ns.SetLoRAHook(hook)
			slog.Info("night scheduler: lora hook attached")
		}
	}

	app.Set("night_scheduler", ns)
	slog.Info("night scheduler: initialized", "output_dir", outputDir)

	// Optional nightly self-distillation: after the night export, run the full
	// Collect→Score→Train→Eval→Deploy pipeline. Opt-in because training is
	// expensive and requires a configured trainer backend.
	var nightDistill func(ctx context.Context)
	if os.Getenv("SELF_DISTILL_NIGHTLY") == "true" {
		if pipeline := appSelfDistillPipeline(app); pipeline != nil {
			nightDistill = func(ctx context.Context) {
				cfg := localbrain.DefaultSelfDistillConfig()
				report := pipeline.Run(ctx, cfg)
				slog.Info("night scheduler: self-distill finished",
					"run_id", report.RunID,
					"success", report.Success,
					"qualified_samples", report.QualifiedSamples,
					"improvement", report.Improvement,
				)
			}
			slog.Info("night scheduler: nightly self-distill enabled")
		} else {
			slog.Warn("night scheduler: SELF_DISTILL_NIGHTLY=true but pipeline unavailable (requires LOCAL_LORA_ENABLED=true + LocalBrain)")
		}
	}

	// Register periodic night run (03:00 daily)
	app.Lifecycle.RegisterFunc("night_scheduler", func(ctx context.Context) error {
		go runNightLoop(ctx, ns, nightDistill)
		return nil
	}, func(ctx context.Context) error {
		return nil
	})
}

func runNightLoop(ctx context.Context, ns *planner.NightScheduler, distill func(ctx context.Context)) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 3, 0, 0, 0, now.Location())
		if now.Hour() < 3 {
			next = time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			slog.Info("night scheduler: running nightly pipeline")
			results := ns.Run(ctx)
			for _, r := range results {
				slog.Info("night scheduler: tenant result",
					"tenant", r.TenantID,
					"duration", r.Duration,
					"export_path", r.ExportPath,
					"error", r.Error,
				)
			}
			if distill != nil {
				distill(ctx)
			}
		}
	}
}
