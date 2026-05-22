package main

import (
	"context"
	"log/slog"
	"time"

	"yunque-agent/internal/agentcore/embeddings"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/task"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/pkg/safego"

	"yunque-agent/internal/ledgercore"
)

func initLedgerStateEngine(app *agentrt.App, typedLdg *ledger.Ledger, taskStore task.Store, taskRunner *task.Runner) {
	embedResRaw, _ := app.Get("embed_resolver")

	ledgerSync := iledger.NewLedgerSync(typedLdg, taskStore)
	taskRunner.OnTaskEvent(ledgerSync.OnEvent)
	app.Set("ledger_sync", ledgerSync)

	memBridge := iledger.NewMemoryBridge(typedLdg, taskStore)
	taskRunner.OnTaskEvent(memBridge.OnEvent)
	app.Set("ledger_memory_bridge", memBridge)

	if embedResRaw != nil {
		if embedRes, ok := embedResRaw.(*embeddings.Resolver); ok {
			if emb, ok := embedRes.Primary(); ok {
				typedLdg.Vector.SetEmbedFunc(func(ctx context.Context, text string) ([]float32, error) {
					return emb.Embed(ctx, text)
				})
				typedLdg.Vector.SetDimensions(emb.Dimensions())
				slog.Info("ledger vector index: embed function attached", "dims", emb.Dimensions())
			}
		}
	}
	if typedLdg.Vector.Dimensions() == 0 {
		typedLdg.Vector.SetDimensions(envInt("EMBED_DIMS", 0))
	}
	configureLedgerVectorANN(context.Background(), app, typedLdg, defaultTenantID())

	typedLdg.Recall.SetGraph(typedLdg.Graph)

	app.Lifecycle.RegisterFunc("ledger_lifecycle", func(ctx context.Context) error {
		safego.Go("ledger-lifecycle-ticker", func() {
			ticker := time.NewTicker(6 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					typedLdg.Lifecycle.RunDecay(ctx, "default")
					typedLdg.Lifecycle.RunGC(ctx, "default")
					typedLdg.Lifecycle.RunConsolidate(ctx, "default")
				}
			}
		})
		return nil
	}, nil)

	slog.Info("ledger state engine initialized",
		"sync", true, "memory_bridge", true,
		"vector", typedLdg.Vector.Enabled(), "graph", true, "lifecycle", true)
}
