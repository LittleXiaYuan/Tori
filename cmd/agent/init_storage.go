package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	agentrt "yunque-agent/internal/agentcore/runtime"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/pkg/safego"
)

// initStorage initializes storage backends.
// Ledger (SQLite) is the primary persistence layer, initialized here in Phase 1
// so that all later phases (memory, planner, tasks) can access it via app.Get(agentrt.CompLedger).
func initStorage(app *agentrt.App) error {
	dbPath := os.Getenv("LEDGER_DB_PATH")
	if dbPath == "" {
		dbPath = app.Config.DataPath("ledger", "ledger.db")
	}

	ldg, err := iledger.InitLedgerAt(dbPath)
	if err != nil {
		if os.Getenv("YUNQUE_LEDGER_AUTO_RECOVER") == "true" {
			recovered, report, recoverErr := iledger.InitLedgerAtRecovering(dbPath, app.Config.DataPath("ledger", "quarantine"), nil)
			if recoverErr == nil {
				ldg = recovered
				slog.Error("ledger SQLite was unhealthy and has been quarantined; started with fresh Ledger. Restore latest backup-pack archive to recover prior state.",
					"reason", report.Reason,
					"quarantine_dir", report.QuarantineDir,
					"files", report.Files)
				err = nil
			} else {
				slog.Error("ledger auto-recovery failed", "err", recoverErr)
			}
		}
	}
	if err != nil {
		if os.Getenv("ALLOW_EPHEMERAL") == "true" {
			slog.Error("╔══════════════════════════════════════════════════════════╗")
			slog.Error("║ LEDGER INIT FAILED - running in EPHEMERAL mode          ║")
			slog.Error("║ All data will be LOST on restart!                        ║")
			slog.Error("╚══════════════════════════════════════════════════════════╝",
				"err", err)
			return nil
		}
		return fmt.Errorf("ledger init failed: %w (set ALLOW_EPHEMERAL=true to run without persistence)", err)
	}

	app.Set(agentrt.CompLedger, ldg)
	app.Ledger = ldg

	slog.Info("storage: Ledger (SQLite) initialized", "db", dbPath)

	// Periodic WAL checkpoint (every 5 minutes) + health check (every hour)
	app.Lifecycle.RegisterFunc("ledger_maintenance", func(ctx context.Context) error {
		safego.Go("ledger-checkpoint", func() {
			checkpointTick := time.NewTicker(5 * time.Minute)
			healthTick := time.NewTicker(1 * time.Hour)
			defer checkpointTick.Stop()
			defer healthTick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-checkpointTick.C:
					if err := ldg.Checkpoint(ctx); err != nil {
						slog.Warn("ledger: checkpoint failed", "err", err)
					}
				case <-healthTick.C:
					if err := ldg.HealthCheck(ctx); err != nil {
						slog.Error("ledger: INTEGRITY CHECK FAILED", "err", err)
					}
				}
			}
		})
		return nil
	}, func(_ context.Context) error {
		slog.Info("closing Ledger SQLite connection")
		return ldg.Close()
	})

	return nil
}
