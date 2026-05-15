package main

import (
	"context"
	"testing"

	cognikernelpack "yunque-agent/internal/packs/cognikernel"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/packruntime"
)

func TestCogniModulePackStateControlsRuntimeLoops(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	ensureBuiltinPacks(registry)
	if pack, ok := registry.Get(cognikernelpack.PackID); !ok || pack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected enabled Cogni Kernel pack after builtin install, got %#v ok=%v", pack, ok)
	}

	module := &cogniModule{
		registry:     cogni.NewRegistry(),
		dir:          t.TempDir(),
		store:        cogni.NewInMemoryTraceStore(16),
		experiences:  make(map[string]*cogni.ExperienceStore),
		bus:          cogni.NewCogniBus(cogni.NewEvaluator(), cogni.DefaultBusConfig()),
		packRegistry: registry,
	}
	module.sentinel = cogni.NewSentinel(module.store, module.registry, cogni.SentinelPolicy{Interval: 0})
	module.hook = cogni.NewHook(module.registry)
	if err := module.registry.Add(&cogni.Declaration{
		ID:         "pack-gated-cogni",
		Activation: cogni.ActivationRules{AlwaysOn: true},
		Experience: cogni.ExperienceConfig{Enabled: true},
	}, "test"); err != nil {
		t.Fatalf("registry.Add: %v", err)
	}

	module.watchCogniKernelPackState(context.Background())
	module.syncCogniKernelPackRuntime(context.Background())
	if !module.cogniRuntimeActive() {
		t.Fatalf("expected runtime active while Cogni Kernel pack is enabled")
	}
	if module.scheduler == nil {
		t.Fatalf("expected scheduler to be initialized while pack is enabled")
	}
	if got := module.bus.ActiveCognis(); got != 1 {
		t.Fatalf("expected bus to register active cogni, got %d", got)
	}
	if _, ok := module.experiences["pack-gated-cogni"]; !ok {
		t.Fatalf("expected experience store to be initialized while pack is enabled")
	}

	if _, err := registry.Disable(cognikernelpack.PackID); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if module.cogniRuntimeActive() {
		t.Fatalf("expected runtime inactive after pack disable")
	}
	if got := module.bus.ActiveCognis(); got != 0 {
		t.Fatalf("expected bus to clear stale declarations after disable, got %d", got)
	}
	if len(module.experiences) != 0 {
		t.Fatalf("expected experience stores cleared after disable, got %#v", module.experiences)
	}

	if _, err := registry.Enable(cognikernelpack.PackID); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !module.cogniRuntimeActive() {
		t.Fatalf("expected runtime active after pack re-enable")
	}
	if got := module.bus.ActiveCognis(); got != 1 {
		t.Fatalf("expected bus to re-register declarations after re-enable, got %d", got)
	}
	if _, ok := module.experiences["pack-gated-cogni"]; !ok {
		t.Fatalf("expected experience store to be recreated after re-enable")
	}
}

func TestCogniModuleStatusFollowsCogniKernelPackState(t *testing.T) {
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	ensureBuiltinPacks(registry)
	module := &cogniModule{
		registry:     cogni.NewRegistry(),
		store:        cogni.NewInMemoryTraceStore(16),
		experiences:  make(map[string]*cogni.ExperienceStore),
		packRegistry: registry,
	}
	module.sentinel = cogni.NewSentinel(module.store, module.registry, cogni.SentinelPolicy{Interval: 0})

	module.syncCogniKernelPackRuntime(context.Background())
	if status := module.Status(); !status.Enabled || !status.Running {
		t.Fatalf("expected status enabled/running while pack enabled, got %#v", status)
	}

	if _, err := registry.Disable(cognikernelpack.PackID); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	module.syncCogniKernelPackRuntime(context.Background())
	if status := module.Status(); status.Enabled || status.Running {
		t.Fatalf("expected status disabled/stopped while pack disabled, got %#v", status)
	}
}
