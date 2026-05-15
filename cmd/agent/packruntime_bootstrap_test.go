package main

import (
	"testing"

	"yunque-agent/pkg/packruntime"
)

func TestEnsureBuiltinPacksInstallsBackupCogniKernelLoRABrowserIntentRPAReplaySBOMDriftSkillAnomalyAndWASMPlugin(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	ensureBuiltinPacks(registry)

	backup, ok := registry.Get("yunque.pack.backup")
	if !ok {
		t.Fatal("expected backup builtin pack to be installed")
	}
	if backup.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected backup default enabled, got %s", backup.Status)
	}
	cogni, ok := registry.Get("yunque.pack.cogni-kernel")
	if !ok {
		t.Fatal("expected Cogni Kernel builtin pack to be installed")
	}
	if cogni.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Cogni Kernel default enabled, got %s", cogni.Status)
	}
	if cogni.Manifest.SDK.TypeScript != "yunque-client/cognis" {
		t.Fatalf("unexpected Cogni Kernel SDK import: %s", cogni.Manifest.SDK.TypeScript)
	}
	lora, ok := registry.Get("yunque.pack.lora")
	if !ok {
		t.Fatal("expected LoRA builtin pack to be installed")
	}
	if lora.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected LoRA default disabled, got %s", lora.Status)
	}
	if lora.Manifest.SDK.TypeScript != "yunque-client/lora" {
		t.Fatalf("unexpected LoRA SDK import: %s", lora.Manifest.SDK.TypeScript)
	}
	browserIntent, ok := registry.Get("yunque.pack.browser-intent")
	if !ok {
		t.Fatal("expected Browser Intent builtin pack to be installed")
	}
	if browserIntent.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Browser Intent default enabled, got %s", browserIntent.Status)
	}
	if browserIntent.Manifest.SDK.TypeScript != "yunque-client/browser" {
		t.Fatalf("unexpected Browser Intent SDK import: %s", browserIntent.Manifest.SDK.TypeScript)
	}

	rpaReplay, ok := registry.Get("yunque.pack.rpa-replay")
	if !ok {
		t.Fatal("expected RPA Replay builtin pack to be installed")
	}
	if rpaReplay.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected RPA Replay default disabled, got %s", rpaReplay.Status)
	}
	if rpaReplay.Manifest.SDK.TypeScript != "yunque-client/rpa-replay" {
		t.Fatalf("unexpected RPA Replay SDK import: %s", rpaReplay.Manifest.SDK.TypeScript)
	}

	sbomDrift, ok := registry.Get("yunque.pack.sbom-drift")
	if !ok {
		t.Fatal("expected SBOM Drift builtin pack to be installed")
	}
	if sbomDrift.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected SBOM Drift default disabled, got %s", sbomDrift.Status)
	}
	if sbomDrift.Manifest.SDK.TypeScript != "yunque-client/sbom-drift" {
		t.Fatalf("unexpected SBOM Drift SDK import: %s", sbomDrift.Manifest.SDK.TypeScript)
	}

	skillAnomaly, ok := registry.Get("yunque.pack.skill-anomaly")
	if !ok {
		t.Fatal("expected Skill Anomaly builtin pack to be installed")
	}
	if skillAnomaly.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected Skill Anomaly default disabled, got %s", skillAnomaly.Status)
	}
	if skillAnomaly.Manifest.SDK.TypeScript != "yunque-client/skill-anomaly" {
		t.Fatalf("unexpected Skill Anomaly SDK import: %s", skillAnomaly.Manifest.SDK.TypeScript)
	}

	wasmPlugin, ok := registry.Get("yunque.pack.wasm-plugin")
	if !ok {
		t.Fatal("expected WASM Plugin builtin pack to be installed")
	}
	if wasmPlugin.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected WASM Plugin default disabled, got %s", wasmPlugin.Status)
	}
	if wasmPlugin.Manifest.SDK.TypeScript != "yunque-client/wasm-plugin" {
		t.Fatalf("unexpected WASM Plugin SDK import: %s", wasmPlugin.Manifest.SDK.TypeScript)
	}

	ensureBuiltinPacks(registry)
	if got := len(registry.List()); got != 8 {
		t.Fatalf("expected idempotent builtin install, got %d packs", got)
	}
}
