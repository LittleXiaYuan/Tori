package planner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	ldg "yunque-agent/internal/ledgercore"
)

// LoRAHook is called after training data export to trigger LoRA training.
// Implemented by localbrain.LoRAScheduler — avoids circular import.
type LoRAHook interface {
	CheckAndTrigger(ctx context.Context, tenantID string) error
}

// NightScheduler runs memory lifecycle operations and training data export
// during off-peak hours. Designed to be triggered by the agent's task scheduler
// or a cron job.
//
// Pipeline:
//  1. Memory lifecycle (decay → expire stale → GC → consolidation)
//  2. Event log compaction
//  3. Training data export to JSONL
//  4. LoRA training trigger (if hook is set)
//
// DreamHook runs cross-conversation pattern recognition during night cycle.
type DreamHook interface {
	RunDreams(ctx context.Context, tenantID string) error
}

type NightScheduler struct {
	ledger    *ldg.Ledger
	outputDir string // directory for exported JSONL files
	tenantIDs func() []string
	loraHook  LoRAHook  // optional: trigger LoRA training after export
	dreamHook DreamHook // optional: cross-conversation pattern recognition
}

// SetLoRAHook attaches a LoRA training trigger (called after JSONL export).
func (ns *NightScheduler) SetLoRAHook(hook LoRAHook) { ns.loraHook = hook }

// SetDreamHook attaches a dream consolidation hook for cross-conversation pattern recognition.
func (ns *NightScheduler) SetDreamHook(hook DreamHook) { ns.dreamHook = hook }

// NightSchedulerConfig configures the nighttime batch processor.
type NightSchedulerConfig struct {
	OutputDir string          // directory for JSONL output (default: ./data/training)
	TenantIDs func() []string // callback to list active tenant IDs
}

func NewNightScheduler(l *ldg.Ledger, cfg NightSchedulerConfig) *NightScheduler {
	if cfg.OutputDir == "" {
		cfg.OutputDir = "./data/training"
	}
	return &NightScheduler{
		ledger:    l,
		outputDir: cfg.OutputDir,
		tenantIDs: cfg.TenantIDs,
	}
}

// NightResult summarizes one nightly run.
type NightResult struct {
	TenantID        string               `json:"tenant_id"`
	LifecycleResult *ldg.LifecycleResult `json:"lifecycle"`
	CompactResult   *ldg.CompactResult   `json:"compact"`
	ExportResult    *ldg.ExportResult    `json:"export"`
	ExportPath      string               `json:"export_path"`
	Duration        time.Duration        `json:"duration"`
	Error           string               `json:"error,omitempty"`
}

// Run executes the full nightly pipeline for all tenants.
func (ns *NightScheduler) Run(ctx context.Context) []NightResult {
	tenants := ns.tenantIDs()
	if len(tenants) == 0 {
		slog.Info("night_scheduler: no tenants, skipping")
		return nil
	}

	os.MkdirAll(ns.outputDir, 0755)

	var results []NightResult
	for _, tid := range tenants {
		if ctx.Err() != nil {
			break
		}
		r := ns.runForTenant(ctx, tid)
		results = append(results, r)
	}

	slog.Info("night_scheduler: complete", "tenants", len(results))
	return results
}

func (ns *NightScheduler) runForTenant(ctx context.Context, tenantID string) NightResult {
	start := time.Now()
	result := NightResult{TenantID: tenantID}

	slog.Info("night_scheduler: starting", "tenant", tenantID)

	// Phase 1: Memory lifecycle
	lcResult, err := ns.ledger.Lifecycle.RunAll(ctx, tenantID)
	result.LifecycleResult = lcResult
	if err != nil {
		slog.Warn("night_scheduler: lifecycle failed", "tenant", tenantID, "err", err)
		result.Error = fmt.Sprintf("lifecycle: %v", err)
	}

	// Phase 2: Event log compaction
	compactCfg := ldg.DefaultCompactConfig()
	compResult, err := ns.ledger.CompactEvents(ctx, tenantID, compactCfg)
	result.CompactResult = compResult
	if err != nil {
		slog.Warn("night_scheduler: compaction failed", "tenant", tenantID, "err", err)
		if result.Error != "" {
			result.Error += "; "
		}
		result.Error += fmt.Sprintf("compact: %v", err)
	}

	// Phase 3: Export training data
	exportPath := filepath.Join(ns.outputDir, fmt.Sprintf("%s_%s.jsonl", tenantID, time.Now().Format("20060102")))
	f, err := os.Create(exportPath)
	if err != nil {
		slog.Warn("night_scheduler: create export file failed", "path", exportPath, "err", err)
		if result.Error != "" {
			result.Error += "; "
		}
		result.Error += fmt.Sprintf("export file: %v", err)
	} else {
		defer f.Close()

		sevenDaysAgo := time.Now().AddDate(0, 0, -7)
		exportCfg := ldg.ExportConfig{
			TenantID: tenantID,
			Format:   ldg.FormatOpenAIChatML,
			MinScore: 0.5,
			After:    &sevenDaysAgo,
		}
		expResult, err := ns.ledger.ExportTrainingData(ctx, f, exportCfg)
		result.ExportResult = expResult
		result.ExportPath = exportPath
		if err != nil {
			slog.Warn("night_scheduler: export failed", "tenant", tenantID, "err", err)
			if result.Error != "" {
				result.Error += "; "
			}
			result.Error += fmt.Sprintf("export: %v", err)
		}
	}

	// Phase 4: Trigger LoRA training if hook is configured
	if ns.loraHook != nil {
		if err := ns.loraHook.CheckAndTrigger(ctx, tenantID); err != nil {
			slog.Warn("night_scheduler: lora trigger failed", "tenant", tenantID, "err", err)
			if result.Error != "" {
				result.Error += "; "
			}
			result.Error += fmt.Sprintf("lora: %v", err)
		}
	}

	// Phase 5: Dream consolidation (cross-conversation pattern recognition)
	if ns.dreamHook != nil {
		if err := ns.dreamHook.RunDreams(ctx, tenantID); err != nil {
			slog.Warn("night_scheduler: dreams failed", "tenant", tenantID, "err", err)
			if result.Error != "" {
				result.Error += "; "
			}
			result.Error += fmt.Sprintf("dreams: %v", err)
		}
	}

	result.Duration = time.Since(start)
	slog.Info("night_scheduler: tenant complete",
		"tenant", tenantID,
		"duration", result.Duration,
		"error", result.Error,
	)
	return result
}

// RunSingle runs the nightly pipeline for a single tenant (useful for manual triggers).
func (ns *NightScheduler) RunSingle(ctx context.Context, tenantID string) NightResult {
	os.MkdirAll(ns.outputDir, 0755)
	return ns.runForTenant(ctx, tenantID)
}
