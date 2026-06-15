package gateway

// wasm_pack_tools.go — Tier 0 microkernel "tool line" for WASM packs (see
// doc/MICROKERNEL-PACK-BLUEPRINT.md). When an enabled wasm pack declares
// backend.toolSpecs, the host builds a sandboxed WasmSkill per spec and registers
// it into the skill registry, so a *downloaded* WASM pack gives the agent a
// callable tool; disabling the pack removes those tools. Purely additive: packs
// without toolSpecs are unaffected.

import (
	"log/slog"
	"path/filepath"
	"strings"

	"yunque-agent/internal/packs/wasmroute"
	"yunque-agent/pkg/packruntime"
)

// registerWasmPackTools registers a wasm pack's declared agent tools into the
// skill registry. No-op for non-wasm packs, packs without toolSpecs, or when no
// registry is wired.
func (g *Gateway) registerWasmPackTools(pack packruntime.InstalledPack, installedDir string) {
	rt := pack.Manifest.Backend.Runtime
	if rt == nil || rt.Type != packruntime.RuntimeTypeWasm {
		return
	}
	if !rt.ABICompatible() {
		return // ABI unsupported by host; mountWasmPack already logged it.
	}
	if g.registry == nil || len(pack.Manifest.Backend.ToolSpecs) == 0 {
		return
	}
	modulePath := filepath.Join(installedDir, filepath.FromSlash(rt.Module))
	hostFuncs := g.buildWasmHostFuncs(pack.Manifest.ID, pack.Manifest.Backend.Permissions)
	sb := g.wasmSandbox()
	for _, ts := range pack.Manifest.Backend.ToolSpecs {
		name := strings.TrimSpace(ts.Name)
		if name == "" {
			continue
		}
		skill := wasmroute.NewSkill(name, ts.Description, ts.Parameters, modulePath, rt.SHA256, ts.Entrypoint, sb, hostFuncs...)
		g.registry.Register(skill)
		slog.Info("wasm pack: registered agent tool", "pack", pack.Manifest.ID, "tool", name)
	}
}

// unregisterWasmPackTools removes a wasm pack's contributed tools from the
// registry (on disable/uninstall), so the agent loses that capability.
func (g *Gateway) unregisterWasmPackTools(pack packruntime.InstalledPack) {
	if g.registry == nil {
		return
	}
	for _, ts := range pack.Manifest.Backend.ToolSpecs {
		if name := strings.TrimSpace(ts.Name); name != "" {
			g.registry.Remove(name)
		}
	}
}
