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
	if !hasRouteSpec(cogni.Manifest.Backend.RouteSpecs, "POST", "/v1/cognis/generate") {
		t.Fatal("expected Cogni Kernel generate routeSpec")
	}
	if !hasRouteSpec(cogni.Manifest.Backend.RouteSpecs, "GET", "/v1/cognis/traces") {
		t.Fatal("expected Cogni Kernel traces routeSpec")
	}
	if !hasRouteSpec(cogni.Manifest.Backend.RouteSpecs, "GET", "/v1/cognis/runtime/pack-state") {
		t.Fatal("expected Cogni Kernel runtime pack-state routeSpec")
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

	channels, ok := registry.Get("yunque.pack.channels")
	if !ok {
		t.Fatal("expected Channels builtin pack to be installed")
	}
	if channels.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Channels default enabled, got %s", channels.Status)
	}
	if channels.Manifest.SDK.TypeScript != "yunque-client/channels" {
		t.Fatalf("unexpected Channels SDK import: %s", channels.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(channels.Manifest.Backend.RouteSpecs, "POST", "/v1/react") {
		t.Fatal("expected Channels react routeSpec")
	}
	if !hasRouteSpec(channels.Manifest.Backend.RouteSpecs, "POST", "/v1/sticker/send") {
		t.Fatal("expected Channels sticker send routeSpec")
	}
	if !hasRouteSpec(channels.Manifest.Backend.RouteSpecs, "GET", "/v1/channels/groups") {
		t.Fatal("expected Channels groups routeSpec")
	}

	market, ok := registry.Get("yunque.pack.market")
	if !ok {
		t.Fatal("expected Skill Market builtin pack to be installed")
	}
	if market.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Skill Market default enabled, got %s", market.Status)
	}
	if market.Manifest.SDK.TypeScript != "yunque-client/market" {
		t.Fatalf("unexpected Skill Market SDK import: %s", market.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(market.Manifest.Backend.RouteSpecs, "GET", "/v1/market/search") {
		t.Fatal("expected Skill Market search routeSpec")
	}
	if !hasRouteSpec(market.Manifest.Backend.RouteSpecs, "GET", "/v1/market/top") {
		t.Fatal("expected Skill Market top routeSpec")
	}
	if !hasRouteSpec(market.Manifest.Backend.RouteSpecs, "GET", "/v1/market/stats") {
		t.Fatal("expected Skill Market stats routeSpec")
	}

	skillhub, ok := registry.Get("yunque.pack.skillhub")
	if !ok {
		t.Fatal("expected SkillHub builtin pack to be installed")
	}
	if skillhub.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected SkillHub default enabled, got %s", skillhub.Status)
	}
	if skillhub.Manifest.SDK.TypeScript != "yunque-client/skillhub" {
		t.Fatalf("unexpected SkillHub SDK import: %s", skillhub.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(skillhub.Manifest.Backend.RouteSpecs, "GET", "/api/skillhub/search") {
		t.Fatal("expected SkillHub search routeSpec")
	}
	if !hasRouteSpec(skillhub.Manifest.Backend.RouteSpecs, "POST", "/api/skillhub/install") {
		t.Fatal("expected SkillHub install routeSpec")
	}
	if !hasRouteSpec(skillhub.Manifest.Backend.RouteSpecs, "GET", "/api/skillhub/policy/check") {
		t.Fatal("expected SkillHub policy check routeSpec")
	}

	connectors, ok := registry.Get("yunque.pack.connectors")
	if !ok {
		t.Fatal("expected Connectors builtin pack to be installed")
	}
	if connectors.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Connectors default enabled, got %s", connectors.Status)
	}
	if connectors.Manifest.SDK.TypeScript != "yunque-client/connectors" {
		t.Fatalf("unexpected Connectors SDK import: %s", connectors.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(connectors.Manifest.Backend.RouteSpecs, "GET", "/api/connectors") {
		t.Fatal("expected Connectors list routeSpec")
	}
	if !hasRouteSpec(connectors.Manifest.Backend.RouteSpecs, "POST", "/api/connectors/execute") {
		t.Fatal("expected Connectors execute routeSpec")
	}

	notifications, ok := registry.Get("yunque.pack.notifications")
	if !ok {
		t.Fatal("expected Notifications builtin pack to be installed")
	}
	if notifications.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Notifications default enabled, got %s", notifications.Status)
	}
	if notifications.Manifest.SDK.TypeScript != "yunque-client/notifications" {
		t.Fatalf("unexpected Notifications SDK import: %s", notifications.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(notifications.Manifest.Backend.RouteSpecs, "GET", "/api/notify/channels") {
		t.Fatal("expected Notifications channels routeSpec")
	}
	if !hasRouteSpec(notifications.Manifest.Backend.RouteSpecs, "POST", "/api/notify/share") {
		t.Fatal("expected Notifications share routeSpec")
	}

	scheduler, ok := registry.Get("yunque.pack.scheduler")
	if !ok {
		t.Fatal("expected Scheduler builtin pack to be installed")
	}
	if scheduler.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Scheduler default enabled, got %s", scheduler.Status)
	}
	if scheduler.Manifest.SDK.TypeScript != "yunque-client/scheduler" {
		t.Fatalf("unexpected Scheduler SDK import: %s", scheduler.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(scheduler.Manifest.Backend.RouteSpecs, "GET", "/v1/scheduler/jobs") {
		t.Fatal("expected Scheduler jobs routeSpec")
	}
	if !hasRouteSpec(scheduler.Manifest.Backend.RouteSpecs, "POST", "/v1/scheduler/add") {
		t.Fatal("expected Scheduler add routeSpec")
	}

	sessionQueue, ok := registry.Get("yunque.pack.session-queue")
	if !ok {
		t.Fatal("expected Session Queue builtin pack to be installed")
	}
	if sessionQueue.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Session Queue default enabled, got %s", sessionQueue.Status)
	}
	if sessionQueue.Manifest.SDK.TypeScript != "yunque-client/session-queue" {
		t.Fatalf("unexpected Session Queue SDK import: %s", sessionQueue.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(sessionQueue.Manifest.Backend.RouteSpecs, "GET", "/v1/sessions/queue") {
		t.Fatal("expected Session Queue list routeSpec")
	}
	if !hasRouteSpec(sessionQueue.Manifest.Backend.RouteSpecs, "POST", "/v1/sessions/queue/cancel") {
		t.Fatal("expected Session Queue cancel routeSpec")
	}

	heartbeat, ok := registry.Get("yunque.pack.heartbeat")
	if !ok {
		t.Fatal("expected Heartbeat builtin pack to be installed")
	}
	if heartbeat.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Heartbeat default enabled, got %s", heartbeat.Status)
	}
	if heartbeat.Manifest.SDK.TypeScript != "yunque-client/heartbeat" {
		t.Fatalf("unexpected Heartbeat SDK import: %s", heartbeat.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(heartbeat.Manifest.Backend.RouteSpecs, "GET", "/v1/heartbeat") {
		t.Fatal("expected Heartbeat status routeSpec")
	}
	if !hasRouteSpec(heartbeat.Manifest.Backend.RouteSpecs, "POST", "/v1/heartbeat/trigger") {
		t.Fatal("expected Heartbeat trigger routeSpec")
	}

	federationPack, ok := registry.Get("yunque.pack.federation")
	if !ok {
		t.Fatal("expected Federation builtin pack to be installed")
	}
	if federationPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Federation default enabled, got %s", federationPack.Status)
	}
	if federationPack.Manifest.SDK.TypeScript != "yunque-client/federation" {
		t.Fatalf("unexpected Federation SDK import: %s", federationPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(federationPack.Manifest.Backend.RouteSpecs, "POST", "/v1/federation/receive") {
		t.Fatal("expected Federation receive routeSpec")
	}

	speechPack, ok := registry.Get("yunque.pack.speech")
	if !ok {
		t.Fatal("expected Speech builtin pack to be installed")
	}
	if speechPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Speech default enabled, got %s", speechPack.Status)
	}
	if speechPack.Manifest.SDK.TypeScript != "yunque-client/speech" {
		t.Fatalf("unexpected Speech SDK import: %s", speechPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(speechPack.Manifest.Backend.RouteSpecs, "POST", "/v1/speech/tts") {
		t.Fatal("expected Speech TTS routeSpec")
	}
	if !hasRouteSpec(speechPack.Manifest.Backend.RouteSpecs, "GET", "/v1/speech/stt/stream") {
		t.Fatal("expected Speech STT stream routeSpec")
	}

	modulesPack, ok := registry.Get("yunque.pack.modules")
	if !ok {
		t.Fatal("expected Runtime Modules builtin pack to be installed")
	}
	if modulesPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Runtime Modules default enabled, got %s", modulesPack.Status)
	}
	if modulesPack.Manifest.SDK.TypeScript != "yunque-client/modules" {
		t.Fatalf("unexpected Runtime Modules SDK import: %s", modulesPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(modulesPack.Manifest.Backend.RouteSpecs, "GET", "/v1/modules") {
		t.Fatal("expected Runtime Modules list routeSpec")
	}

	identityPack, ok := registry.Get("yunque.pack.identity")
	if !ok {
		t.Fatal("expected Identity builtin pack to be installed")
	}
	if identityPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Identity default enabled, got %s", identityPack.Status)
	}
	if identityPack.Manifest.SDK.TypeScript != "yunque-client/identity" {
		t.Fatalf("unexpected Identity SDK import: %s", identityPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(identityPack.Manifest.Backend.RouteSpecs, "POST", "/v1/identity/resolve") {
		t.Fatal("expected Identity resolve routeSpec")
	}
	if !hasRouteSpec(identityPack.Manifest.Backend.RouteSpecs, "GET", "/v1/identity/profiles") {
		t.Fatal("expected Identity profiles routeSpec")
	}

	personaPack, ok := registry.Get("yunque.pack.persona")
	if !ok {
		t.Fatal("expected Persona builtin pack to be installed")
	}
	if personaPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Persona default enabled, got %s", personaPack.Status)
	}
	if personaPack.Manifest.SDK.TypeScript != "yunque-client/persona" {
		t.Fatalf("unexpected Persona SDK import: %s", personaPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(personaPack.Manifest.Backend.RouteSpecs, "GET", "/v1/persona") {
		t.Fatal("expected Persona read routeSpec")
	}
	if !hasRouteSpec(personaPack.Manifest.Backend.RouteSpecs, "POST", "/v1/persona/presets/custom") {
		t.Fatal("expected Persona custom preset routeSpec")
	}

	retrievalPack, ok := registry.Get("yunque.pack.retrieval")
	if !ok {
		t.Fatal("expected Retrieval builtin pack to be installed")
	}
	if retrievalPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Retrieval default enabled, got %s", retrievalPack.Status)
	}
	if retrievalPack.Manifest.SDK.TypeScript != "yunque-client/retrieval" {
		t.Fatalf("unexpected Retrieval SDK import: %s", retrievalPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(retrievalPack.Manifest.Backend.RouteSpecs, "POST", "/v1/embeddings") {
		t.Fatal("expected Retrieval embeddings routeSpec")
	}
	if !hasRouteSpec(retrievalPack.Manifest.Backend.RouteSpecs, "GET", "/v1/search/providers") {
		t.Fatal("expected Retrieval search providers routeSpec")
	}

	subagentsPack, ok := registry.Get("yunque.pack.subagents")
	if !ok {
		t.Fatal("expected Subagents builtin pack to be installed")
	}
	if subagentsPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Subagents default enabled, got %s", subagentsPack.Status)
	}
	if subagentsPack.Manifest.SDK.TypeScript != "yunque-client/subagents" {
		t.Fatalf("unexpected Subagents SDK import: %s", subagentsPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(subagentsPack.Manifest.Backend.RouteSpecs, "POST", "/v1/subagent") {
		t.Fatal("expected Subagents spawn routeSpec")
	}
	if !hasRouteSpec(subagentsPack.Manifest.Backend.RouteSpecs, "POST", "/v1/subagent/message") {
		t.Fatal("expected Subagents message routeSpec")
	}

	tracePack, ok := registry.Get("yunque.pack.trace")
	if !ok {
		t.Fatal("expected Trace builtin pack to be installed")
	}
	if tracePack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Trace default enabled, got %s", tracePack.Status)
	}
	if tracePack.Manifest.SDK.TypeScript != "yunque-client/trace" {
		t.Fatalf("unexpected Trace SDK import: %s", tracePack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(tracePack.Manifest.Backend.RouteSpecs, "GET", "/v1/trace/recent") {
		t.Fatal("expected Trace recent routeSpec")
	}
	if !hasBackendRoute(tracePack.Manifest, "/v1/trace/task/") {
		t.Fatal("expected Trace task route")
	}

	forks, ok := registry.Get("yunque.pack.forks")
	if !ok {
		t.Fatal("expected Conversation Forks builtin pack to be installed")
	}
	if forks.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Conversation Forks default enabled, got %s", forks.Status)
	}
	if forks.Manifest.SDK.TypeScript != "yunque-client/forks" {
		t.Fatalf("unexpected Conversation Forks SDK import: %s", forks.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(forks.Manifest.Backend.RouteSpecs, "GET", "/v1/fork") {
		t.Fatal("expected Conversation Forks read routeSpec")
	}
	if !hasRouteSpec(forks.Manifest.Backend.RouteSpecs, "POST", "/v1/fork/branch") {
		t.Fatal("expected Conversation Forks branch routeSpec")
	}

	orchestrator, ok := registry.Get("yunque.pack.orchestrator")
	if !ok {
		t.Fatal("expected IDE Work Orchestrator builtin pack to be installed")
	}
	if orchestrator.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected IDE Work Orchestrator default enabled, got %s", orchestrator.Status)
	}
	if orchestrator.Manifest.SDK.TypeScript != "yunque-client/orchestrator" {
		t.Fatalf("unexpected IDE Work Orchestrator SDK import: %s", orchestrator.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(orchestrator.Manifest.Backend.RouteSpecs, "GET", "/v1/orchestrator/status") {
		t.Fatal("expected IDE Work Orchestrator status routeSpec")
	}
	if !hasRouteSpec(orchestrator.Manifest.Backend.RouteSpecs, "PUT", "/v1/orchestrator/policy") {
		t.Fatal("expected IDE Work Orchestrator policy update routeSpec")
	}

	reflection, ok := registry.Get("yunque.pack.reflection")
	if !ok {
		t.Fatal("expected Reflection builtin pack to be installed")
	}
	if reflection.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Reflection default enabled, got %s", reflection.Status)
	}
	if reflection.Manifest.SDK.TypeScript != "yunque-client/reflection" {
		t.Fatalf("unexpected Reflection SDK import: %s", reflection.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(reflection.Manifest.Backend.RouteSpecs, "GET", "/v1/reflect/experiences") {
		t.Fatal("expected Reflection experiences read routeSpec")
	}
	if !hasRouteSpec(reflection.Manifest.Backend.RouteSpecs, "POST", "/v1/reflect/experiences") {
		t.Fatal("expected Reflection experiences write routeSpec")
	}
	if !hasRouteSpec(reflection.Manifest.Backend.RouteSpecs, "GET", "/v1/reflect/strategies") {
		t.Fatal("expected Reflection strategies routeSpec")
	}

	reverie, ok := registry.Get("yunque.pack.reverie")
	if !ok {
		t.Fatal("expected Reverie builtin pack to be installed")
	}
	if reverie.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Reverie default enabled, got %s", reverie.Status)
	}
	if reverie.Manifest.SDK.TypeScript != "yunque-client/reverie" {
		t.Fatalf("unexpected Reverie SDK import: %s", reverie.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(reverie.Manifest.Backend.RouteSpecs, "GET", "/v1/reverie/journal") {
		t.Fatal("expected Reverie journal routeSpec")
	}
	if !hasRouteSpec(reverie.Manifest.Backend.RouteSpecs, "GET", "/v1/reverie/dream/status") {
		t.Fatal("expected Reverie dream status routeSpec")
	}

	mcpDispatch, ok := registry.Get("yunque.pack.mcp-dispatch")
	if !ok {
		t.Fatal("expected MCP Dispatch builtin pack to be installed")
	}
	if mcpDispatch.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected MCP Dispatch default enabled, got %s", mcpDispatch.Status)
	}
	if mcpDispatch.Manifest.SDK.TypeScript != "yunque-client/mcp-dispatch" {
		t.Fatalf("unexpected MCP Dispatch SDK import: %s", mcpDispatch.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(mcpDispatch.Manifest.Backend.RouteSpecs, "POST", "/mcp/v1") {
		t.Fatal("expected MCP Dispatch JSON-RPC routeSpec")
	}
	if !hasRouteSpec(mcpDispatch.Manifest.Backend.RouteSpecs, "GET", "/v1/workers") {
		t.Fatal("expected MCP Dispatch worker list routeSpec")
	}

	cost, ok := registry.Get("yunque.pack.cost")
	if !ok {
		t.Fatal("expected Cost Governance builtin pack to be installed")
	}
	if cost.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Cost Governance default enabled, got %s", cost.Status)
	}
	if cost.Manifest.SDK.TypeScript != "yunque-client/cost" {
		t.Fatalf("unexpected Cost Governance SDK import: %s", cost.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(cost.Manifest.Backend.RouteSpecs, "GET", "/v1/cost/summary") {
		t.Fatal("expected Cost Governance summary routeSpec")
	}
	if !hasRouteSpec(cost.Manifest.Backend.RouteSpecs, "POST", "/v1/cost/budget") {
		t.Fatal("expected Cost Governance budget routeSpec")
	}

	desktopPack, ok := registry.Get("yunque.pack.desktop")
	if !ok {
		t.Fatal("expected Desktop Shell builtin pack to be installed")
	}
	if desktopPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Desktop Shell default enabled, got %s", desktopPack.Status)
	}
	if desktopPack.Manifest.SDK.TypeScript != "yunque-client/desktop" {
		t.Fatalf("unexpected Desktop Shell SDK import: %s", desktopPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(desktopPack.Manifest.Backend.RouteSpecs, "POST", "/v1/desktop/console") {
		t.Fatal("expected Desktop Shell console routeSpec")
	}
	if !hasRouteSpec(desktopPack.Manifest.Backend.RouteSpecs, "GET", "/v1/desktop/autostart") {
		t.Fatal("expected Desktop Shell autostart routeSpec")
	}

	cognitiveLayerPack, ok := registry.Get("yunque.pack.cognitive-layer")
	if !ok {
		t.Fatal("expected Cognitive Layer builtin pack to be installed")
	}
	if cognitiveLayerPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Cognitive Layer default enabled, got %s", cognitiveLayerPack.Status)
	}
	if cognitiveLayerPack.Manifest.SDK.TypeScript != "yunque-client/cognitive-layer" {
		t.Fatalf("unexpected Cognitive Layer SDK import: %s", cognitiveLayerPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(cognitiveLayerPack.Manifest.Backend.RouteSpecs, "POST", "/v1/cognitive-layer") {
		t.Fatal("expected Cognitive Layer toggle routeSpec")
	}

	pluginAPI, ok := registry.Get("yunque.pack.plugin-api")
	if !ok {
		t.Fatal("expected Plugin API Bridge builtin pack to be installed")
	}
	if pluginAPI.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Plugin API Bridge default enabled, got %s", pluginAPI.Status)
	}
	if pluginAPI.Manifest.SDK.TypeScript != "yunque-client/plugin-api" {
		t.Fatalf("unexpected Plugin API Bridge SDK import: %s", pluginAPI.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(pluginAPI.Manifest.Backend.RouteSpecs, "POST", "/v1/plugin-api/llm") {
		t.Fatal("expected Plugin API Bridge llm routeSpec")
	}
	if !hasRouteSpec(pluginAPI.Manifest.Backend.RouteSpecs, "GET", "/v1/plugin-api/extensions") {
		t.Fatal("expected Plugin API Bridge extensions routeSpec")
	}
	if !hasRouteSpec(pluginAPI.Manifest.Backend.RouteSpecs, "GET", "/v1/plugin-api/cron/list") {
		t.Fatal("expected Plugin API Bridge cron list routeSpec")
	}

	sandboxPack, ok := registry.Get("yunque.pack.sandbox")
	if !ok {
		t.Fatal("expected Sandbox builtin pack to be installed")
	}
	if sandboxPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Sandbox default enabled, got %s", sandboxPack.Status)
	}
	if sandboxPack.Manifest.SDK.TypeScript != "yunque-client/sandbox" {
		t.Fatalf("unexpected Sandbox SDK import: %s", sandboxPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(sandboxPack.Manifest.Backend.RouteSpecs, "POST", "/v1/sandbox/exec") {
		t.Fatal("expected Sandbox exec routeSpec")
	}
	if !hasRouteSpec(sandboxPack.Manifest.Backend.RouteSpecs, "GET", "/v1/sandbox/desktop/status") {
		t.Fatal("expected Sandbox desktop status routeSpec")
	}

	cronPack, ok := registry.Get("yunque.pack.cron")
	if !ok {
		t.Fatal("expected Cron builtin pack to be installed")
	}
	if cronPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Cron default enabled, got %s", cronPack.Status)
	}
	if cronPack.Manifest.SDK.TypeScript != "yunque-client/cron" {
		t.Fatalf("unexpected Cron SDK import: %s", cronPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(cronPack.Manifest.Backend.RouteSpecs, "POST", "/v1/cron/add") {
		t.Fatal("expected Cron add routeSpec")
	}

	filesPack, ok := registry.Get("yunque.pack.files")
	if !ok {
		t.Fatal("expected Files builtin pack to be installed")
	}
	if filesPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Files default enabled, got %s", filesPack.Status)
	}
	if filesPack.Manifest.SDK.TypeScript != "yunque-client/files" {
		t.Fatalf("unexpected Files SDK import: %s", filesPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(filesPack.Manifest.Backend.RouteSpecs, "GET", "/api/files/preview") {
		t.Fatal("expected Files preview routeSpec")
	}

	personaModesPack, ok := registry.Get("yunque.pack.persona-modes")
	if !ok {
		t.Fatal("expected Persona Modes builtin pack to be installed")
	}
	if personaModesPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Persona Modes default enabled, got %s", personaModesPack.Status)
	}
	if personaModesPack.Manifest.SDK.TypeScript != "yunque-client/persona-modes" {
		t.Fatalf("unexpected Persona Modes SDK import: %s", personaModesPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(personaModesPack.Manifest.Backend.RouteSpecs, "GET", "/v1/persona/mode/current") {
		t.Fatal("expected Persona Modes current routeSpec")
	}

	instructionsPack, ok := registry.Get("yunque.pack.instructions")
	if !ok {
		t.Fatal("expected Instructions builtin pack to be installed")
	}
	if instructionsPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Instructions default enabled, got %s", instructionsPack.Status)
	}
	if instructionsPack.Manifest.SDK.TypeScript != "yunque-client/instructions" {
		t.Fatalf("unexpected Instructions SDK import: %s", instructionsPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(instructionsPack.Manifest.Backend.RouteSpecs, "POST", "/v1/instructions/reorder") {
		t.Fatal("expected Instructions reorder routeSpec")
	}

	graphPack, ok := registry.Get("yunque.pack.graph")
	if !ok {
		t.Fatal("expected Graph builtin pack to be installed")
	}
	if graphPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Graph default enabled, got %s", graphPack.Status)
	}
	if graphPack.Manifest.SDK.TypeScript != "yunque-client/graph" {
		t.Fatalf("unexpected Graph SDK import: %s", graphPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(graphPack.Manifest.Backend.RouteSpecs, "GET", "/v1/graph/context") {
		t.Fatal("expected Graph context routeSpec")
	}

	computerUse, ok := registry.Get("yunque.pack.computer-use")
	if !ok {
		t.Fatal("expected Computer Use builtin pack to be installed")
	}
	if computerUse.Status != packruntime.PackStatusDisabled {
		t.Fatalf("expected Computer Use default disabled, got %s", computerUse.Status)
	}
	if computerUse.Manifest.SDK.TypeScript != "yunque-client/computer-use" {
		t.Fatalf("unexpected Computer Use SDK import: %s", computerUse.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(computerUse.Manifest.Backend.RouteSpecs, "POST", "/v1/computer/intent/plan") {
		t.Fatal("expected Computer Use intent plan routeSpec")
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

	statePack, ok := registry.Get("yunque.pack.state")
	if !ok {
		t.Fatal("expected State Kernel builtin pack to be installed")
	}
	if statePack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected State Kernel default enabled, got %s", statePack.Status)
	}
	if !hasBackendRoute(statePack.Manifest, "/v1/state") {
		t.Fatal("expected State Kernel snapshot route to be installed from manifest")
	}
	if !hasBackendRoute(statePack.Manifest, "/v1/state/resources") {
		t.Fatal("expected State Kernel resources route to be installed from manifest")
	}

	rbacPack, ok := registry.Get("yunque.pack.rbac")
	if !ok {
		t.Fatal("expected RBAC builtin pack to be installed")
	}
	if rbacPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected RBAC default enabled, got %s", rbacPack.Status)
	}
	if rbacPack.Manifest.SDK.TypeScript != "yunque-client/rbac" {
		t.Fatalf("unexpected RBAC SDK import: %s", rbacPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(rbacPack.Manifest.Backend.RouteSpecs, "POST", "/v1/rbac/check") {
		t.Fatal("expected RBAC check routeSpec")
	}
	if !hasRouteSpec(rbacPack.Manifest.Backend.RouteSpecs, "POST", "/v1/rbac/assign") {
		t.Fatal("expected RBAC assign routeSpec")
	}

	plannerRecoveryPack, ok := registry.Get("yunque.pack.planner-recovery")
	if !ok {
		t.Fatal("expected Planner Recovery builtin pack to be installed")
	}
	if plannerRecoveryPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Planner Recovery default enabled, got %s", plannerRecoveryPack.Status)
	}
	if plannerRecoveryPack.Manifest.SDK.TypeScript != "yunque-client/planner-recovery" {
		t.Fatalf("unexpected Planner Recovery SDK import: %s", plannerRecoveryPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(plannerRecoveryPack.Manifest.Backend.RouteSpecs, "GET", "/v1/planner/checkpoints") {
		t.Fatal("expected Planner Recovery checkpoints routeSpec")
	}
	if !hasRouteSpec(plannerRecoveryPack.Manifest.Backend.RouteSpecs, "POST", "/v1/planner/checkpoints/resume-plan") {
		t.Fatal("expected Planner Recovery resume-plan routeSpec")
	}

	toriPack, ok := registry.Get("yunque.pack.tori")
	if !ok {
		t.Fatal("expected Tori builtin pack to be installed")
	}
	if toriPack.Status != packruntime.PackStatusEnabled {
		t.Fatalf("expected Tori default enabled, got %s", toriPack.Status)
	}
	if toriPack.Manifest.SDK.TypeScript != "yunque-client/tori" {
		t.Fatalf("unexpected Tori SDK import: %s", toriPack.Manifest.SDK.TypeScript)
	}
	if !hasRouteSpec(toriPack.Manifest.Backend.RouteSpecs, "POST", "/v1/tori/bind") {
		t.Fatal("expected Tori bind routeSpec")
	}
	if !hasRouteSpec(toriPack.Manifest.Backend.RouteSpecs, "GET", "/v1/tori/status") {
		t.Fatal("expected Tori status routeSpec")
	}

	ensureBuiltinPacks(registry)
	// Count reflects the current auto-seeded packs/official/ set. Adjust if the
	// builtin pack set changes (dlc-demo stays excluded by design).
	if got := len(registry.List()); got != 65 {
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
