package main

import (
	"testing"

	"yunque-agent/pkg/packruntime"
)

func TestEnsureBuiltinPacksInstallsBackupCogniKernelLoRABrowserIntentChaosProbeCognitiveCanaryGuardrailFuzzerMemoryTimeTravelRPAReplaySBOMDriftSkillAnomalyAndWASMPlugin(t *testing.T) {
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

	chaosProbe, ok := registry.Get("yunque.pack.chaos-probe")
	if !ok {
		t.Fatal("expected Chaos Probe builtin pack to be installed")
	}
	if chaosProbe.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected Chaos Probe default disabled, got %s", chaosProbe.Status)
	}
	if chaosProbe.Manifest.SDK.TypeScript != "yunque-client/chaos-probe" {
		t.Fatalf("unexpected Chaos Probe SDK import: %s", chaosProbe.Manifest.SDK.TypeScript)
	}

	cognitiveCanary, ok := registry.Get("yunque.pack.cognitive-canary")
	if !ok {
		t.Fatal("expected Cognitive Canary builtin pack to be installed")
	}
	if cognitiveCanary.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected Cognitive Canary default disabled, got %s", cognitiveCanary.Status)
	}
	if cognitiveCanary.Manifest.SDK.TypeScript != "yunque-client/cognitive-canary" {
		t.Fatalf("unexpected Cognitive Canary SDK import: %s", cognitiveCanary.Manifest.SDK.TypeScript)
	}

	guardrailFuzzer, ok := registry.Get("yunque.pack.guardrail-fuzzer")
	if !ok {
		t.Fatal("expected Guardrail Fuzzer builtin pack to be installed")
	}
	if guardrailFuzzer.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected Guardrail Fuzzer default disabled, got %s", guardrailFuzzer.Status)
	}
	if guardrailFuzzer.Manifest.SDK.TypeScript != "yunque-client/guardrail-fuzzer" {
		t.Fatalf("unexpected Guardrail Fuzzer SDK import: %s", guardrailFuzzer.Manifest.SDK.TypeScript)
	}

	memoryTimeTravel, ok := registry.Get("yunque.pack.memory-time-travel")
	if !ok {
		t.Fatal("expected Memory Time Travel builtin pack to be installed")
	}
	if memoryTimeTravel.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected Memory Time Travel default disabled, got %s", memoryTimeTravel.Status)
	}
	if memoryTimeTravel.Manifest.SDK.TypeScript != "yunque-client/memory-time-travel" {
		t.Fatalf("unexpected Memory Time Travel SDK import: %s", memoryTimeTravel.Manifest.SDK.TypeScript)
	}
	if !hasBackendRoute(memoryTimeTravel.Manifest, "/v1/memory-time-travel/kv-history/migration-preview") {
		t.Fatal("expected Memory Time Travel native kv_history migration preview route to be installed from manifest")
	}
	if !hasBackendRoute(memoryTimeTravel.Manifest, "/v1/memory-time-travel/kv-history/dual-read/parity") {
		t.Fatal("expected Memory Time Travel dual-read parity gate route to be installed from manifest")
	}
	if !hasBackendRoute(memoryTimeTravel.Manifest, "/v1/memory-time-travel/kv-history/cutover/readiness") {
		t.Fatal("expected Memory Time Travel cutover readiness gate route to be installed from manifest")
	}
	if !hasBackendRoute(memoryTimeTravel.Manifest, "/v1/memory-time-travel/audit/links/preview") {
		t.Fatal("expected Memory Time Travel audit proof-link preview route to be installed from manifest")
	}
	if !hasBackendRoute(memoryTimeTravel.Manifest, "/v1/memory-time-travel/audit/links/writeback-plan") {
		t.Fatal("expected Memory Time Travel audit proof-link writeback plan route to be installed from manifest")
	}
	if !hasBackendRoute(memoryTimeTravel.Manifest, "/v1/memory-time-travel/audit/links/writeback/store") {
		t.Fatal("expected Memory Time Travel audit proof-link writeback store route to be installed from manifest")
	}
	if !hasBackendRoute(memoryTimeTravel.Manifest, "/v1/memory-time-travel/audit/links/writeback/executor/plan") {
		t.Fatal("expected Memory Time Travel audit proof-link writeback executor plan route to be installed from manifest")
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
	if !hasRouteSpec(rpaReplay.Manifest.Backend.RouteSpecs, "POST", "/v1/rpa-replay/executor/plan") {
		t.Fatal("expected RPA Replay executor plan routeSpec")
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
	if !hasRouteSpec(sbomDrift.Manifest.Backend.RouteSpecs, "POST", "/v1/sbom-drift/ci-gate/baseline/writeback") {
		t.Fatal("expected SBOM Drift CI baseline writeback routeSpec")
	}
	if !hasRouteSpec(sbomDrift.Manifest.Backend.RouteSpecs, "POST", "/v1/sbom-drift/baseline/artifact-source/plan") {
		t.Fatal("expected SBOM Drift baseline artifact source plan routeSpec")
	}
	if !hasRouteSpec(sbomDrift.Manifest.Backend.RouteSpecs, "POST", "/v1/sbom-drift/ci-gate/workflow/writeback/plan") {
		t.Fatal("expected SBOM Drift CI workflow writeback plan routeSpec")
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
	if !hasRouteSpec(wasmPlugin.Manifest.Backend.RouteSpecs, "POST", "/v1/wasm-plugin/remote-install/signature-verification/writeback") {
		t.Fatal("expected WASM Plugin signature verification writeback routeSpec")
	}
	if !hasRouteSpec(wasmPlugin.Manifest.Backend.RouteSpecs, "POST", "/v1/wasm-plugin/remote-install/package/inspect/writeback") {
		t.Fatal("expected WASM Plugin package inspect writeback routeSpec")
	}
	if !hasRouteSpec(wasmPlugin.Manifest.Backend.RouteSpecs, "POST", "/v1/wasm-plugin/remote-install/installer/registration/plan") {
		t.Fatal("expected WASM Plugin installer registration plan routeSpec")
	}

	ensureBuiltinPacks(registry)
	if got := len(registry.List()); got != 17 {
		t.Fatalf("expected idempotent builtin install, got %d packs", got)
	}
}

func hasBackendRoute(manifest packruntime.Manifest, path string) bool {
	for _, route := range manifest.Backend.Routes {
		if route == path {
			return true
		}
	}
	return false
}

func hasRouteSpec(routes []packruntime.BackendRouteSpec, method string, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
