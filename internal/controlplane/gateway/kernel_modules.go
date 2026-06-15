package gateway

// kernel_modules.go — Tier 0 microkernel, Phase 1: real lifecycle for v2 packs
// (see doc/MICROKERNEL-PACK-BLUEPRINT.md).
//
// The legacy RegisterBackendPack path mounts a v1 BackendModule's routes but has
// no lifecycle, so first-party packs can never truly Start/Stop on
// enable/disable. RegisterModule adds that: it wires the pack against the kernel
// Host, mounts its routes through the existing pack-route gate, starts it, and
// subscribes once to the pack registry so enable→Start / disable→Stop fire at
// runtime — mirroring how wasm packs already behave.
//
// Purely additive: a v2 Module is a superset of BackendModule, so existing v1
// packs are untouched (they simply don't get Start/Stop).

import (
	"context"
	"log/slog"
	"strings"

	"yunque-agent/pkg/packruntime"
)

// RegisterModule registers a v2 capability pack: Init(host) → mount routes →
// Start. It also ensures the registry lifecycle subscription is installed so the
// pack's Start/Stop track enable/disable. Errors from Init/Start are logged and
// returned; routes are still mounted so a failed Start can be retried by toggling
// the pack.
func (g *Gateway) RegisterModule(m packruntime.Module) error {
	if g == nil || m == nil {
		return nil
	}
	host := g.Host()
	if err := m.Init(host); err != nil {
		slog.Warn("kernel module: init failed", "pack", m.PackID(), "err", err)
		return err
	}
	// A Module satisfies BackendModule (PackID + Routes), so reuse the existing
	// mount path — routes go through the same Pack Runtime auth + enabled gate.
	g.RegisterBackendPack(m)

	g.wireKernelModuleLifecycle()

	if g.packModuleEnabled(m.PackID()) {
		if err := m.Start(context.Background()); err != nil {
			slog.Warn("kernel module: start failed", "pack", m.PackID(), "err", err)
			return err
		}
		g.registerPackSkills(m)
		slog.Info("kernel module: registered and started", "pack", m.PackID())
	} else {
		slog.Info("kernel module: registered (disabled, not started)", "pack", m.PackID())
	}
	return nil
}

// registerPackSkills adds an enabled pack's contributed tools (skills) into the
// skill registry, so the planner exposes them to the agent. No-op unless the
// pack implements SkillProvider and a registry is wired.
func (g *Gateway) registerPackSkills(m packruntime.Module) {
	sp, ok := m.(packruntime.SkillProvider)
	if !ok || g.registry == nil {
		return
	}
	for _, sk := range sp.Skills() {
		if sk == nil {
			continue
		}
		g.registry.Register(sk)
		slog.Info("kernel module: registered pack skill", "pack", m.PackID(), "skill", sk.Name())
	}
}

// unregisterPackSkills removes a disabled pack's contributed tools from the
// registry, so the agent loses that capability when the pack is turned off.
func (g *Gateway) unregisterPackSkills(m packruntime.Module) {
	sp, ok := m.(packruntime.SkillProvider)
	if !ok || g.registry == nil {
		return
	}
	for _, sk := range sp.Skills() {
		if sk == nil {
			continue
		}
		g.registry.Remove(sk.Name())
	}
}

// packModuleEnabled reports whether the pack is currently enabled in the
// registry. When no registry is wired (lean configs), modules default to
// enabled so they still start.
func (g *Gateway) packModuleEnabled(packID string) bool {
	if g.packRegistry == nil {
		return true
	}
	pack, ok := g.packRegistry.Get(packID)
	if !ok {
		// Not tracked by the registry (e.g. a built-in reference pack): treat as
		// enabled so RegisterModule still starts it.
		return true
	}
	return pack.Status == packruntime.PackStatusEnabled
}

// wireKernelModuleLifecycle installs (once) a registry OnChange subscription that
// drives v2 Module Start/Stop on enable/disable. Modules are discovered from the
// shared g.backendPacks slice (RegisterModule appends through RegisterBackendPack)
// by type-asserting to packruntime.Module, so this needs no extra bookkeeping.
func (g *Gateway) wireKernelModuleLifecycle() {
	if g.packRegistry == nil {
		return
	}
	g.kernelLifecycleOnce.Do(func() {
		g.packRegistry.OnChange(func(ev packruntime.ChangeEvent) {
			id := ev.Pack.Manifest.ID
			switch ev.Reason {
			case packruntime.ChangeReasonEnable:
				g.forEachKernelModule(id, func(m packruntime.Module) {
					if err := m.Start(context.Background()); err != nil {
						slog.Warn("kernel module: start on enable failed", "pack", id, "err", err)
					}
					g.registerPackSkills(m) // enabled pack's tools become callable
				})
			case packruntime.ChangeReasonDisable:
				g.forEachKernelModule(id, func(m packruntime.Module) {
					g.unregisterPackSkills(m) // remove tools before stopping
					if err := m.Stop(context.Background()); err != nil {
						slog.Warn("kernel module: stop on disable failed", "pack", id, "err", err)
					}
				})
			}
		})
	})
}

// PackContext aggregates context contributed by every ENABLED capability pack
// that implements packruntime.ContextProvider. Wired into the planner via
// SetPackContext so a Pack's enablement flows into the agent's reasoning (not
// just its HTTP routes). Disabled packs contribute nothing.
func (g *Gateway) PackContext(ctx context.Context, tenantID, query string) string {
	g.routesMu.RLock()
	packs := make([]packruntime.BackendModule, len(g.backendPacks))
	copy(packs, g.backendPacks)
	g.routesMu.RUnlock()

	var b strings.Builder
	for _, bp := range packs {
		if bp == nil {
			continue
		}
		provider, ok := bp.(packruntime.ContextProvider)
		if !ok {
			continue
		}
		if !g.packModuleEnabled(bp.PackID()) {
			continue
		}
		if section := strings.TrimSpace(provider.BuildContext(ctx, query, tenantID)); section != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(section)
		}
	}
	return b.String()
}

// forEachKernelModule runs fn for every registered v2 Module whose PackID matches.
func (g *Gateway) forEachKernelModule(packID string, fn func(packruntime.Module)) {
	g.routesMu.RLock()
	packs := make([]packruntime.BackendModule, len(g.backendPacks))
	copy(packs, g.backendPacks)
	g.routesMu.RUnlock()

	for _, bp := range packs {
		if bp == nil || bp.PackID() != packID {
			continue
		}
		if mod, ok := bp.(packruntime.Module); ok {
			fn(mod)
		}
	}
}
