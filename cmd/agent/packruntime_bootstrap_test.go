package main

import (
	"testing"

	"yunque-agent/pkg/packruntime"
)

func TestEnsureBuiltinPacksInstallsBackupCogniKernelLoRAAndBrowserIntent(t *testing.T) {
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

	ensureBuiltinPacks(registry)
	if got := len(registry.List()); got != 4 {
		t.Fatalf("expected idempotent builtin install, got %d packs", got)
	}
}
