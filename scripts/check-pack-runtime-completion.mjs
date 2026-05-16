#!/usr/bin/env node
import { existsSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const failures = [];
const evidence = [];

function fail(item, message) {
  failures.push(`${item}: ${message}`);
}

function ok(item, message) {
  evidence.push({ item, message });
}

function read(path) {
  const full = resolve(repoRoot, path);
  if (!existsSync(full)) {
    fail(path, "missing file");
    return "";
  }
  return readFileSync(full, "utf8");
}

function requireTokens(item, text, tokens) {
  const missing = tokens.filter((token) => !text.includes(token));
  if (missing.length > 0) fail(item, `missing tokens: ${missing.join(", ")}`);
  else ok(item, `${tokens.length} tokens present`);
}

function runCheck(item, command, args, expected = 0) {
  const result = spawnSync(command, args, { cwd: repoRoot, encoding: "utf8" });
  if (result.status !== expected) {
    fail(item, `${command} ${args.join(" ")} exited ${result.status}: ${result.stderr || result.stdout}`);
  } else {
    ok(item, `${command} ${args.join(" ")} passed`);
  }
}

const manifest = read("pkg/packruntime/manifest.go");
const registry = read("pkg/packruntime/registry.go");
const backend = read("pkg/packruntime/backend.go");
const gateway = [
  "internal/controlplane/gateway/handlers_packs.go",
  "internal/controlplane/gateway/gateway.go",
  "internal/controlplane/gateway/gateway_setters.go",
  "internal/controlplane/gateway/handlers_packs_test.go",
  "internal/controlplane/gateway/handlers_cogni.go",
  "internal/controlplane/gateway/handlers_browser_pack.go",
  "internal/controlplane/gateway/handlers_browser_pack_test.go",
  "internal/controlplane/gateway/handlers_chaos_probe_pack_test.go",
  "internal/controlplane/gateway/handlers_cognitive_canary_pack_test.go",
  "internal/controlplane/gateway/handlers_guardrail_fuzzer_pack_test.go",
  "internal/controlplane/gateway/handlers_memory_time_travel_pack_test.go",
  "internal/controlplane/gateway/handlers_rpa_replay_pack_test.go",
  "internal/controlplane/gateway/handlers_sbom_drift_pack_test.go",
  "internal/controlplane/gateway/handlers_skill_anomaly_pack_test.go",
  "internal/controlplane/gateway/handlers_wasm_plugin_pack_test.go",
  "internal/controlplane/gateway/handlers_cogni_experience_test.go",
  "cmd/agent/init_tasks.go",
].map(read).join("\n");
const backup = read("internal/packs/backup/handler.go");
const backupManifest = read("packs/examples/backup-pack/pack.json");
const loraPack = read("internal/packs/lora/handler.go");
const loraManifest = read("packs/examples/lora-pack/pack.json");
const cogniKernelPack = read("internal/packs/cognikernel/handler.go");
const cogniKernelManifest = read("packs/examples/cogni-kernel-pack/pack.json");
const browserIntentPack = read("internal/packs/browserintent/handler.go");
const browserIntentManifest = read("packs/examples/browser-intent-pack/pack.json");
const chaosProbePack = read("internal/packs/chaosprobe/handler.go");
const chaosProbeManifest = read("packs/examples/chaos-probe-pack/pack.json");
const cognitiveCanaryPack = read("internal/packs/cognitivecanary/handler.go");
const cognitiveCanaryManifest = read("packs/examples/cognitive-canary-pack/pack.json");
const guardrailFuzzerPack = read("internal/packs/guardrailfuzzer/handler.go");
const guardrailFuzzerManifest = read("packs/examples/guardrail-fuzzer-pack/pack.json");
const memoryTimeTravelPack = read("internal/packs/memorytimetravel/handler.go");
const memoryTimeTravelManifest = read("packs/examples/memory-time-travel-pack/pack.json");
const rpaReplayPack = read("internal/packs/rpareplay/handler.go");
const rpaReplayManifest = read("packs/examples/rpa-replay-pack/pack.json");
const sbomDriftPack = read("internal/packs/sbomdrift/handler.go");
const sbomDriftManifest = read("packs/examples/sbom-drift-pack/pack.json");
const skillAnomalyPack = read("internal/packs/skillanomaly/handler.go");
const skillAnomalyManifest = read("packs/examples/skill-anomaly-pack/pack.json");
const wasmPluginPack = read("internal/packs/wasmplugin/handler.go");
const wasmPluginManifest = read("packs/examples/wasm-plugin-pack/pack.json");
const scaffold = read("scripts/scaffold-pack.mjs") + "\n" + read("scripts/check-pack-scaffold.mjs");
const fullVerification = read("scripts/check-pack-runtime-all.mjs");
const frontend = [
  "heroui-web/src/lib/pack-sync.tsx",
  "heroui-web/src/lib/__tests__/pack-sync.test.ts",
  "heroui-web/src/lib/packs-client.ts",
  "heroui-web/src/lib/__tests__/packs-client.test.ts",
  "heroui-web/src/app/packs/page.tsx",
  "heroui-web/src/app/packs/[...slug]/page.tsx",
  "heroui-web/src/app/packs/backup/page.tsx",
  "heroui-web/src/app/packs/lora/page.tsx",
  "heroui-web/src/app/packs/cognis/page.tsx",
  "heroui-web/src/components/cherry/__tests__/settings-modal-pack-entry.test.tsx",
  "heroui-web/src/lib/backup-pack-client.ts",
  "heroui-web/src/lib/__tests__/backup-pack-client.test.ts",
  "heroui-web/src/lib/lora-pack-client.ts",
  "heroui-web/src/lib/__tests__/lora-pack-client.test.ts",
  "heroui-web/src/lib/cogni-kernel-pack-client.ts",
  "heroui-web/src/lib/__tests__/cogni-kernel-pack-client.test.ts",
  "heroui-web/src/app/packs/browser/page.tsx",
  "heroui-web/src/lib/browser-intent-pack-client.ts",
  "heroui-web/src/lib/__tests__/browser-intent-pack-client.test.ts",
  "heroui-web/src/app/packs/chaos-probe/page.tsx",
  "heroui-web/src/lib/chaos-probe-pack-client.ts",
  "heroui-web/src/lib/__tests__/chaos-probe-pack-client.test.ts",
  "heroui-web/src/app/packs/cognitive-canary/page.tsx",
  "heroui-web/src/lib/cognitive-canary-pack-client.ts",
  "heroui-web/src/lib/__tests__/cognitive-canary-pack-client.test.ts",
  "heroui-web/src/app/packs/guardrail-fuzzer/page.tsx",
  "heroui-web/src/lib/guardrail-fuzzer-pack-client.ts",
  "heroui-web/src/lib/__tests__/guardrail-fuzzer-pack-client.test.ts",
  "heroui-web/src/app/packs/memory-time-travel/page.tsx",
  "heroui-web/src/lib/memory-time-travel-pack-client.ts",
  "heroui-web/src/lib/__tests__/memory-time-travel-pack-client.test.ts",
  "heroui-web/src/app/packs/rpa-replay/page.tsx",
  "heroui-web/src/lib/rpa-replay-pack-client.ts",
  "heroui-web/src/lib/__tests__/rpa-replay-pack-client.test.ts",
  "heroui-web/src/app/packs/sbom-drift/page.tsx",
  "heroui-web/src/lib/sbom-drift-pack-client.ts",
  "heroui-web/src/lib/__tests__/sbom-drift-pack-client.test.ts",
  "heroui-web/src/app/packs/skill-anomaly/page.tsx",
  "heroui-web/src/lib/skill-anomaly-pack-client.ts",
  "heroui-web/src/lib/__tests__/skill-anomaly-pack-client.test.ts",
  "heroui-web/src/app/packs/wasm-plugin/page.tsx",
  "heroui-web/src/lib/wasm-plugin-pack-client.ts",
  "heroui-web/src/lib/__tests__/wasm-plugin-pack-client.test.ts",
  "heroui-web/src/lib/pack-types.ts",
  "heroui-web/src/lib/api.ts",
  "heroui-web/src/lib/api-types/skills.ts",
].map(read).join("\n");
const legacyBackupPage = read("heroui-web/src/app/backup/page.tsx");
const backupPackPage = read("heroui-web/src/app/packs/backup/page.tsx");
const legacyLoRAPage = read("heroui-web/src/app/lora/page.tsx");
const loraPackPage = read("heroui-web/src/app/packs/lora/page.tsx");
const legacyCogniPage = read("heroui-web/src/app/cognis/page.tsx");
const cogniPackPage = read("heroui-web/src/app/packs/cognis/page.tsx");
const legacyBrowserPage = read("heroui-web/src/app/browser/page.tsx");
const browserPackPage = read("heroui-web/src/app/packs/browser/page.tsx");
const chaosProbePackPage = read("heroui-web/src/app/packs/chaos-probe/page.tsx");
const cognitiveCanaryPackPage = read("heroui-web/src/app/packs/cognitive-canary/page.tsx");
const guardrailFuzzerPackPage = read("heroui-web/src/app/packs/guardrail-fuzzer/page.tsx");
const memoryTimeTravelPackPage = read("heroui-web/src/app/packs/memory-time-travel/page.tsx");
const rpaReplayPackPage = read("heroui-web/src/app/packs/rpa-replay/page.tsx");
const sbomDriftPackPage = read("heroui-web/src/app/packs/sbom-drift/page.tsx");
const skillAnomalyPackPage = read("heroui-web/src/app/packs/skill-anomaly/page.tsx");
const wasmPluginPackPage = read("heroui-web/src/app/packs/wasm-plugin/page.tsx");
const frontendShell = [
  "heroui-web/src/components/sidebar.tsx",
  "heroui-web/src/lib/nav-items.tsx",
  "heroui-web/src/components/command-palette.tsx",
].map(read).join("\n");
const sdk = [
  "sdk/typescript/src/packs.ts",
  "sdk/typescript/src/packs.test.ts",
  "sdk/manifest/packs-sdk.json",
  "sdk/manifest/lora-pack-sdk.json",
  "sdk/manifest/cogni-kernel-pack-sdk.json",
  "sdk/manifest/browser-intent-pack-sdk.json",
  "sdk/manifest/chaos-probe-pack-sdk.json",
  "sdk/manifest/cognitive-canary-pack-sdk.json",
  "sdk/manifest/guardrail-fuzzer-pack-sdk.json",
  "sdk/manifest/memory-time-travel-pack-sdk.json",
  "sdk/manifest/rpa-replay-pack-sdk.json",
  "sdk/manifest/sbom-drift-pack-sdk.json",
  "sdk/manifest/skill-anomaly-pack-sdk.json",
  "sdk/manifest/wasm-plugin-pack-sdk.json",
  "sdk/typescript/src/cognitive-canary.ts",
  "sdk/typescript/src/cognitive-canary.test.ts",
  "sdk/typescript/src/guardrail-fuzzer.ts",
  "sdk/typescript/src/guardrail-fuzzer.test.ts",
  "sdk/typescript/src/memory-time-travel.ts",
  "sdk/typescript/src/memory-time-travel.test.ts",
  "sdk/typescript/src/rpa-replay.ts",
  "sdk/typescript/src/rpa-replay.test.ts",
  "sdk/typescript/src/sbom-drift.ts",
  "sdk/typescript/src/sbom-drift.test.ts",
  "sdk/typescript/src/sbom-drift-ci.ts",
  "sdk/typescript/src/sbom-drift-ci.test.ts",
  "sdk/typescript/src/skill-anomaly.ts",
  "sdk/typescript/src/skill-anomaly.test.ts",
  "sdk/typescript/src/wasm-plugin.ts",
  "sdk/typescript/src/wasm-plugin.test.ts",
  "sdk/typescript/src/chaos-probe.ts",
  "sdk/typescript/src/chaos-probe.test.ts",
  "sdk/scripts/check-packs-sdk-manifest.mjs",
  "sdk/scripts/check-lora-pack-sdk-manifest.mjs",
  "sdk/scripts/check-cogni-kernel-pack-sdk-manifest.mjs",
  "sdk/scripts/check-browser-intent-pack-sdk-manifest.mjs",
  "sdk/scripts/check-chaos-probe-pack-sdk-manifest.mjs",
  "sdk/scripts/check-cognitive-canary-pack-sdk-manifest.mjs",
  "sdk/scripts/check-guardrail-fuzzer-pack-sdk-manifest.mjs",
  "sdk/scripts/check-memory-time-travel-pack-sdk-manifest.mjs",
  "sdk/scripts/check-rpa-replay-pack-sdk-manifest.mjs",
  "sdk/scripts/check-sbom-drift-pack-sdk-manifest.mjs",
  "sdk/scripts/check-skill-anomaly-pack-sdk-manifest.mjs",
  "sdk/scripts/check-wasm-plugin-pack-sdk-manifest.mjs",
].map(read).join("\n");
const docs = [
  "packs/AUTHORING.md",
  "doc/PACK-RUNTIME-BLUEPRINT.md",
  "docs/guide/pack-runtime.md",
  "docs/zh/guide/pack-runtime.md",
  "docs/guide/pack-runtime-state.md",
  "docs/zh/guide/pack-runtime-state.md",
].map(read).join("\n");

requireTokens("轻内核/manifest 协议", manifest + docs + backupManifest, [
  "type Manifest",
  "BackendManifest",
  "FrontendManifest",
  "SDKManifest",
  "DistributionManifest",
  "UpdateManifest",
  "BackendRouteSpec",
  "AllowsRoute",
  "frontend.menus",
  "frontend.routes",
  "sdk.typescript",
  "distribution.packageUrl",
  "update.rollback",
]);

requireTokens("本地 installed registry / install-enable-disable-rollback", registry + gateway, [
  "type Registry",
  "RegistryFileName",
  "InstallWithArtifacts",
  "Enable(id string)",
  "Disable(id string)",
  "Rollback(id string)",
  "PreviousVersion",
  "PreviousArtifacts",
  "PruneArtifacts",
  "/v1/packs/install",
  "/v1/packs/enable",
  "/v1/packs/disable",
  "/v1/packs/rollback",
  "/v1/packs/prune",
  "ensureBuiltinPacks",
  "loadBuiltinPackManifest",
  "packs/examples/lora-pack/pack.json",
  "packs/examples/cogni-kernel-pack/pack.json",
  "packs/examples/browser-intent-pack/pack.json",
  "packs/examples/chaos-probe-pack/pack.json",
  "packs/examples/cognitive-canary-pack/pack.json",
  "packs/examples/guardrail-fuzzer-pack/pack.json",
  "packs/examples/memory-time-travel-pack/pack.json",
  "packs/examples/rpa-replay-pack/pack.json",
  "packs/examples/sbom-drift-pack/pack.json",
  "packs/examples/skill-anomaly-pack/pack.json",
  "packs/examples/wasm-plugin-pack/pack.json",
]);

requireTokens("后端 backend pack module registry / route gates", backend + gateway, [
  "type BackendModule interface",
  "PackID() string",
  "Routes() []BackendRoute",
  "RegisterBackendPack",
  "BackendPacks",
  "backendPackRoutes",
  "backendPackRouteInfos",
  "backendPackAuth",
  "BackendRouteAuthPassthrough",
  "requirePackRoute",
  "packRouteEnabled",
  "normalizeBackendRouteMethods",
  "Methods: methods",
  "http.StatusMethodNotAllowed",
  "route conflict",
  "handlePackBackendModules",
  "backuppack.DefaultHandler()",
]);

requireTokens("backup-pack 示例包", backup + backupManifest, [
  "const PackID = \"yunque.pack.backup\"",
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/backup/info",
  "/v1/backup/export",
  "/v1/backup/import",
  "yunque-client/backup",
  "distribution",
  "rollback",
]);

requireTokens("lora-pack 蓝图能力包", loraPack + loraManifest + frontend, [
  "const PackID = \"yunque.pack.lora\"",
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/lora/status",
  "/v1/lora/trigger",
  "/v1/lora/config",
  "http.MethodPatch",
  "yunque-client/lora",
  "LoRA / LAA Evolution Pack",
  "createLoRAPackClient",
  "pack route is not enabled",
  "lora-pack-client",
  "/packs/lora",
  "distribution",
  "rollback",
]);

requireTokens("cogni-kernel 蓝图能力包", cogniKernelPack + cogniKernelManifest + frontend + gateway, [
  'const PackID = "yunque.pack.cogni-kernel"',
  "type CogniGateway interface",
  "HandleCogniKernelPack",
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/cognis",
  "/v1/cognis/",
  "http.MethodDelete",
  "yunque-client/cognis",
  "Cogni Kernel Pack",
  "createCogniKernelPackClient",
  "cogni-kernel-pack-client",
  "TestCogniKernelPackGateReturnsNotFoundWhenDisabled",
  "/packs/cognis",
  "distribution",
  "rollback",
]);

requireTokens("browser-intent 蓝图能力包", browserIntentPack + browserIntentManifest + frontend + gateway, [
  'const PackID = "yunque.pack.browser-intent"',
  "type BrowserGateway interface",
  "HandleBrowserIntentPack",
  "HandleBrowserIntentSession",
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/browser/status",
  "/v1/browser/intent/plan",
  "/api/browser/ext/session",
  "/api/browser/ext/scenarios/run",
  "BrowserActPlan",
  "browser_act_plan_ready",
  "browser_act_ready",
  "permission_gate_ready",
  "runtime_skill_gate_ready",
  "opp_gate_ready",
  "consumes_browser_session",
  "executes_browser_actions",
  "writes_browser_state",
  "network_access",
  "browser-act-plan.json",
  "browser-permission-gate.json",
  "runtime-skill-gate.json",
  "opp-gate-plan.json",
  "http.MethodPost",
  "yunque-client/browser",
  "Browser Intent Pack",
  "createBrowserIntentPackClient",
  "browserActPlan",
  "browser-intent-pack-client",
  "TestBrowserIntentPackGateReturnsNotFoundWhenDisabled",
  "/packs/browser",
  "distribution",
  "rollback",
]);

requireTokens("chaos-probe 蓝图能力包", chaosProbePack + chaosProbeManifest + frontend + gateway + sdk + docs, [
  'const PackID = "yunque.pack.chaos-probe"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/chaos-probe/status",
  "/v1/chaos-probe/probes",
  "/v1/chaos-probe/run",
  "/v1/chaos-probe/scheduler/plan",
  "/v1/chaos-probe/degrade-state/writeback",
  "/v1/chaos-probe/degrade-state/engine/plan",
  "/v1/chaos-probe/reports",
  "/v1/chaos-probe/evidence/",
  "safe_probe_ready",
  "scheduler_plan_ready",
  "scheduler_ready",
  "metrics_plan_ready",
  "prometheus_ready",
  "degrade_writeback_plan_ready",
  "degrade_writeback_ready",
  "degrade_state_store_ready",
  "writes_degrade_state_store",
  "degrade_engine_plan_ready",
  "audit_append_plan_ready",
  "merkle_append_ready",
  "consumes_degrade_state_store",
  "writes_runtime_degrade_state",
  "runtime_degrade_state_ready",
  "degrade_engine_ready",
  "alert_writeback_plan_ready",
  "alert_writeback_ready",
  "chaos.scheduler.plan",
  "chaos.metrics.plan",
  "chaos.degrade.plan",
  "chaos.degrade_state.writeback",
  "chaos.degrade_state.engine.plan",
  "chaos.audit.append.plan",
  "chaos.alert.writeback.plan",
  "scheduler-plan.json",
  "metrics-plan.json",
  "degrade-writeback-plan.json",
  "degrade-state-store.json",
  "degrade-state-record.json",
  "degrade-engine-plan.json",
  "runtime-degrade-handoff-plan.json",
  "audit-append-plan.json",
  "cfg.DataPath(\"chaos-probe\")",
  "json-chaos-probe-evidence",
  "http.MethodPost",
  "yunque-client/chaos-probe",
  "Chaos Probe Pack",
  "createChaosProbePackClient",
  "createChaosProbeClient",
  "schedulerPlan",
  "writeDegradeState",
  "degradeEnginePlan",
  "chaos-probe-pack-client",
  "TestChaosProbePackGateReturnsNotFoundWhenDisabled",
  "/packs/chaos-probe",
  "pack-shell-before-scheduler",
  "distribution",
  "rollback",
]);

requireTokens("cognitive-canary 蓝图能力包", cognitiveCanaryPack + cognitiveCanaryManifest + frontend + gateway + sdk + docs, [
  'const PackID = "yunque.pack.cognitive-canary"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/cognitive-canary/status",
  "/v1/cognitive-canary/scenarios",
  "/v1/cognitive-canary/evaluate",
  "/v1/cognitive-canary/shadow/plan",
  "/v1/cognitive-canary/response-collector/writeback",
  "/v1/cognitive-canary/response-collector/pipeline/plan",
  "/v1/cognitive-canary/reports",
  "/v1/cognitive-canary/evidence/",
  "shadow_plan_ready",
  "shadow_traffic_ready",
  "judge_plan_ready",
  "judge_pipeline_ready",
  "response_collector_plan_ready",
  "response_collector_store_ready",
  "response_collector_writeback_ready",
  "writes_response_collector_store",
  "response_collector_pipeline_plan_ready",
  "consumes_response_collector_store",
  "response_collector_pipeline_ready",
  "response_collector_ready",
  "metrics_plan_ready",
  "prometheus_ready",
  "quality_sli_ready",
  "auto_rollback_plan_ready",
  "auto_rollback_ready",
  "canary.shadow.plan",
  "canary.response_collector.plan",
  "canary.response_collector.writeback",
  "canary.response_collector.pipeline.plan",
  "canary.judge.plan",
  "canary.metrics.plan",
  "canary.rollback.plan",
  "shadow-plan.json",
  "response-collector-plan.json",
  "response-collector-store.json",
  "response-collector-record.json",
  "response-collector-pipeline-plan.json",
  "response-collector-handoff-plan.json",
  "response_collectors",
  "response_collector_summary",
  "response_collector_store",
  "response_collector_records",
  "response_collector_pipeline_plan",
  "artifact_sha256",
  "writes_files",
  "judge-plan.json",
  "metrics-plan.json",
  "rollback-plan.json",
  "cfg.DataPath(\"cognitive-canary\")",
  "json-cognitive-canary-evidence",
  "http.MethodPost",
  "yunque-client/cognitive-canary",
  "Cognitive Canary Pack",
  "createCognitiveCanaryPackClient",
  "createCognitiveCanaryClient",
  "shadowPlan",
  "responseCollectorWriteback",
  "responseCollectorPipelinePlan",
  "cognitive-canary-pack-client",
  "TestCognitiveCanaryPackGateReturnsNotFoundWhenDisabled",
  "/packs/cognitive-canary",
  "pack-shell-before-shadow-traffic",
  "distribution",
  "rollback",
]);

requireTokens("guardrail-fuzzer 蓝图能力包", guardrailFuzzerPack + guardrailFuzzerManifest + frontend + gateway + sdk + docs, [
  'const PackID = "yunque.pack.guardrail-fuzzer"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/guardrail-fuzzer/status",
  "/v1/guardrail-fuzzer/corpus",
  "/v1/guardrail-fuzzer/run",
  "/v1/guardrail-fuzzer/ci-gate/plan",
  "/v1/guardrail-fuzzer/native-corpus/plan",
  "/v1/guardrail-fuzzer/reports",
  "/v1/guardrail-fuzzer/evidence/",
  "fuzzer_ready",
  "ci_gate_plan_ready",
  "ci_gate_ready",
  "rule_writeback_plan_ready",
  "rule_writeback_ready",
  "alert_plan_ready",
  "alert_ready",
  "native_corpus_plan_ready",
  "native_corpus_sync_ready",
  "go_native_fuzz_plan_ready",
  "go_native_fuzz_ready",
  "corpus_manifest",
  "sync_summary",
  "content_sha256",
  "writes_files",
  "guardrail.ci_gate.plan",
  "guardrail.rule_writeback.plan",
  "guardrail.alert.plan",
  "guardrail.native_corpus.plan",
  "guardrail.go_native_fuzz.plan",
  "guardrail.native_corpus.manifest_preview",
  "ci-gate-plan.json",
  "rule-writeback-plan.json",
  "alert-plan.json",
  "native-corpus-plan.json",
  "go-native-fuzz-plan.json",
  "native-corpus-manifest.json",
  "native-corpus-sync-preview.json",
  "cfg.DataPath(\"guardrail-fuzzer\")",
  "json-guardrail-fuzzer-evidence",
  "http.MethodPost",
  "yunque-client/guardrail-fuzzer",
  "Guardrail Fuzzer Pack",
  "createGuardrailFuzzerPackClient",
  "createGuardrailFuzzerClient",
  "ciGatePlan",
  "nativeCorpusPlan",
  "guardrail-fuzzer-pack-client",
  "TestGuardrailFuzzerPackGateReturnsNotFoundWhenDisabled",
  "/packs/guardrail-fuzzer",
  "pack-shell-before-ci-fuzz",
  "distribution",
  "rollback",
]);


requireTokens("memory-time-travel 蓝图能力包", memoryTimeTravelPack + memoryTimeTravelManifest + frontend + gateway + sdk + docs, [
  'const PackID = "yunque.pack.memory-time-travel"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/memory-time-travel/status",
  "/v1/memory-time-travel/snapshots",
  "/v1/memory-time-travel/snapshot-at",
  "/v1/memory-time-travel/diff",
  "/v1/memory-time-travel/rollback-plan",
  "/v1/memory-time-travel/rollback/approved-plan",
  "/v1/memory-time-travel/retention/plan",
  "/v1/memory-time-travel/retention/prune-plan",
  "/v1/memory-time-travel/kv-history/native-plan",
  "/v1/memory-time-travel/kv-history/migration-preview",
  "/v1/memory-time-travel/kv-history/dual-read/parity",
  "/v1/memory-time-travel/kv-history/cutover/plan",
  "/v1/memory-time-travel/kv-history/cutover/readiness",
  "/v1/memory-time-travel/audit/links",
  "/v1/memory-time-travel/audit/links/preview",
  "/v1/memory-time-travel/audit/links/writeback-plan",
  "/v1/memory-time-travel/audit/links/writeback/store",
  "/v1/memory-time-travel/audit/links/writeback/executor/plan",
  "/v1/memory-time-travel/audit/verify",
  "/v1/memory-time-travel/evidence/",
  "snapshot_store_ready",
  "temporal_query_ready",
  "ledger_history_ready",
  "memory_persister_writeback_ready",
  "approved_rollback_plan_ready",
  "approval_request_plan_ready",
  "approval_manager_bridge_plan_ready",
  "global_approval_enqueue_ready",
  "rollback_writeback_plan_ready",
  "writes_ledger_kv",
  "writes_temporal_kv",
  "retention_plan_ready",
  "retention_prune_plan_ready",
  "retention_prune_ready",
  "native_kv_history_plan_ready",
  "kv_history_migration_plan_ready",
  "kv_history_index_plan_ready",
  "native_kv_history_preview_ready",
  "kv_history_cutover_plan_ready",
  "kv_history_cutover_readiness_ready",
  "cutover_readiness_check_ready",
  "dual_read_plan_ready",
  "dual_read_parity_check_ready",
  "dual_read_parity_ready",
  "parity_passed",
  "dual_write_plan_ready",
  "native_kv_history_ready",
  "writes_native_kv_history",
  "migrates_kv_history",
  "dual_read_ready",
  "dual_write_ready",
  "cutover_ready",
  "switches_temporal_adapter",
  "memory.retention.plan",
  "memory.retention.prune_plan",
  "memory.kv_history.native_plan",
  "memory.kv_history.migration_preview",
  "memory.kv_history.dual_read.parity",
  "memory.kv_history.cutover.plan",
  "memory.kv_history.cutover.readiness",
  "memory.rollback.approved_plan",
  "memory.rollback.writeback.plan",
  "kv_audit_link_schema_ready",
  "kv_audit_link_preview_ready",
  "kv_audit_link_writeback_plan_ready",
  "kv_audit_link_writeback_store_ready",
  "kv_audit_link_writeback_executor_plan_ready",
  "executor_input_contract_ready",
  "audit_proof_link_executor_ready",
  "consumes_audit_link_writeback_store",
  "audit_append_plan_ready",
  "writes_audit_chain",
  "writes_audit_link_writeback_store",
  "kv_audit_link_writeback_ready",
  "kv_audit_linkage_ready",
  "backfills_audit_seq",
  "backfills_audit_hash",
  "consumes_audit_link_preview",
  "memory.audit.links.schema",
  "memory.audit.links.preview",
  "memory.audit.links.writeback_plan",
  "memory.audit.links.writeback_store",
  "memory.audit.links.writeback_executor_plan",
  "retention-plan.json",
  "retention_plan",
  "approved-rollback-plan.json",
  "approved_rollback_plan",
  "rollback-writeback-plan.json",
  "rollback_writeback_plan",
  "approval-request-plan.json",
  "approval_request_plan",
  "retention-prune-plan.json",
  "retention_prune_plan",
  "native-kv-history-plan.json",
  "kv-history-migration-plan.json",
  "kv-history-index-plan.json",
  "kv-history-migration-preview.json",
  "kv-history-dual-read-parity.json",
  "kv-history-cutover-plan.json",
  "kv-history-cutover-readiness.json",
  "kv-history-dual-read-plan.json",
  "kv-history-dual-write-plan.json",
  "native_kv_history_plan",
  "kv_history_migration_plan",
  "kv_history_index_plan",
  "kv_history_migration_preview",
  "kv_history_dual_read_parity",
  "kv_history_cutover_plan",
  "kv_history_cutover_readiness",
  "kv_history_dual_read_plan",
  "kv_history_dual_write_plan",
  "audit-links.json",
  "audit-link-preview.json",
  "audit-link-writeback-plan.json",
  "audit-link-writeback-store.json",
  "audit-link-writeback-record.json",
  "audit-link-writeback-executor-plan.json",
  "audit-link-executor-handoff-plan.json",
  "audit-link-executor-audit-plan.json",
  "kv_audit_link_schema",
  "kv_audit_link_preview",
  "kv_audit_link_writeback_plan",
  "kv_audit_link_writeback_actions",
  "kv_audit_link_writeback_executor_plan",
  "audit_link_executor_handoff_plan",
  "audit_link_executor_audit_plan",
  "kv_audit_links",
  "max_snapshots_per_namespace",
  "memory.audit.verify",
  "MerkleVerifier",
  "VerifyMerkleAuditChain",
  "TemporalKVReader",
  "NativeKVHistoryPreviewer",
  "PreviewNativeKVHistoryRows",
  "KVHistoryDualReadParity",
  "TemporalWritebackReady",
  "WithLedgerPersisterTemporalKV",
  "NewTemporalKVStore",
  "ledger-kv-history",
  "merkle_verification_ready",
  "rollback_writeback_ready",
  "cfg.DataPath(\"memory-time-travel\")",
  "json-memory-time-travel-evidence",
  "audit-verification.json",
  "audit_verification",
  "http.MethodPost",
  "yunque-client/memory-time-travel",
  "Memory Time Travel Pack",
  "createMemoryTimeTravelPackClient",
  "createMemoryTimeTravelClient",
  "retentionPrunePlan",
  "nativeKVHistoryPlan",
  "nativeKVHistoryMigrationPreview",
  "kvHistoryDualReadParity",
  "kvHistoryCutoverPlan",
  "kvHistoryCutoverReadiness",
  "approvedRollbackPlan",
  "auditLinks",
  "auditLinksPreview",
  "auditLinksWritebackPlan",
  "auditLinksWritebackStore",
  "auditLinksWritebackExecutorPlan",
  "auditVerify",
  "memory-time-travel-pack-client",
  "TestMemoryTimeTravelPackGateReturnsNotFoundWhenDisabled",
  "/packs/memory-time-travel",
  "pack-shell-before-ledger-kv-history",
  "distribution",
  "rollback",
]);

requireTokens("rpa-replay 蓝图能力包", rpaReplayPack + rpaReplayManifest + frontend + gateway + sdk, [
  'const PackID = "yunque.pack.rpa-replay"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/rpa-replay/status",
  "/v1/rpa-replay/traces",
  "/v1/rpa-replay/recordings/start",
  "/v1/rpa-replay/recordings/stop",
  "/v1/rpa-replay/replay",
  "/v1/rpa-replay/executor/plan",
  "/v1/rpa-replay/evidence/",
  "executor_plan_ready",
  "executor_ready",
  "action_tracer_plan_ready",
  "action_tracer_ready",
  "browser_intent_gate_plan_ready",
  "browser_intent_ready",
  "consumes_browser_intent",
  "executes_browser_actions",
  "writes_browser_state",
  "writes_files",
  "network_access",
  "rpa.executor.plan",
  "rpa.browser_intent.gate_plan",
  "rpa.action_tracer.handoff_plan",
  "executor-handoff-plan.json",
  "browser-intent-gate-plan.json",
  "action-tracer-plan.json",
  "cfg.DataPath(\"rpa-replay\")",
  "rpa.replay.dry_run",
  "json-evidence-pack",
  "http.MethodPost",
  "yunque-client/rpa-replay",
  "RPA Replay Pack",
  "createRPAReplayPackClient",
  "createRPAReplayClient",
  "executorPlan",
  "rpa-replay-pack-client",
  "TestRPAReplayPackGateReturnsNotFoundWhenDisabled",
  "/packs/rpa-replay",
  "pack-shell-before-executor",
  "distribution",
  "rollback",
]);

requireTokens("sbom-drift 蓝图能力包", sbomDriftPack + sbomDriftManifest + frontend + gateway + sdk, [
  'const PackID = "yunque.pack.sbom-drift"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/sbom-drift/status",
  "/v1/sbom-drift/snapshots",
  "/v1/sbom-drift/diff",
  "/v1/sbom-drift/cyclonedx/",
  "/v1/sbom-drift/ci-gate/plan",
  "/v1/sbom-drift/baseline/artifact-source/plan",
  "/v1/sbom-drift/ci-gate/baseline/writeback",
  "/v1/sbom-drift/ci-gate/workflow/writeback/plan",
  "/v1/sbom-drift/evidence/",
  "scanner_ready",
  "cyclonedx_ready",
  "ci_gate_plan_ready",
  "ci_baseline_store_ready",
  "ci_baseline_writeback_ready",
  "artifact_source_plan_ready",
  "baseline_fetch_plan_ready",
  "fetches_artifact_baseline",
  "writes_baseline_snapshot",
  "ci_workflow_writeback_plan_ready",
  "consumes_ci_baseline_store",
  "writes_ci_baseline_store",
  "ci_gate_ready",
  "govulncheck_plan_ready",
  "govulncheck_ready",
  "sbom.cyclonedx.export",
  "sbom.ci_gate.plan",
  "sbom.baseline.artifact_source.plan",
  "sbom.baseline.fetch.plan",
  "sbom.ci_baseline.writeback",
  "sbom.ci_workflow.writeback_plan",
  "sbom.govulncheck.plan",
  "govulncheck-plan.json",
  "ci-baseline-store.json",
  "ci-baseline-record.json",
  "baseline-artifact-source-plan.json",
  "baseline-fetch-handoff-plan.json",
  "ci-workflow-writeback-plan.json",
  "ci-workflow-handoff-plan.json",
  "release-blocker-plan.json",
  "govulncheck-report.json",
  "writes_files",
  "cfg.DataPath(\"sbom-drift\")",
  "json-sbom-drift-evidence",
  "http.MethodPost",
  "yunque-client/sbom-drift",
  "SBOM Drift Pack",
  "createSBOMDriftPackClient",
  "createSBOMDriftClient",
  "sbom-drift-pack-client",
  "TestSBOMDriftPackGateReturnsNotFoundWhenDisabled",
  "/packs/sbom-drift",
  "pack-shell-before-ci",
  "distribution",
  "rollback",
]);

requireTokens("skill-anomaly 蓝图能力包", skillAnomalyPack + skillAnomalyManifest + frontend + gateway + sdk + docs, [
  'const PackID = "yunque.pack.skill-anomaly"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/skill-anomaly/status",
  "/v1/skill-anomaly/events",
  "/v1/skill-anomaly/profiles",
  "/v1/skill-anomaly/detect",
  "/v1/skill-anomaly/audit-hook/plan",
  "/v1/skill-anomaly/approval-queue/writeback",
  "/v1/skill-anomaly/approval-queue/bridge/plan",
  "/v1/skill-anomaly/evidence/",
  "detector_ready",
  "audit_hook_plan_ready",
  "audit_hook_ready",
  "trust_mutation_plan_ready",
  "trust_mutation_ready",
  "approval_writeback_ready",
  "approval_queue_store_ready",
  "approval_manager_bridge_plan_ready",
  "global_approval_enqueue_ready",
  "approval_queue_store",
  "approval_queue_record",
  "approval_manager_bridge_plan",
  "approval-queue-store.json",
  "approval-queue-record.json",
  "approval-manager-bridge-plan.json",
  "ApprovalQueueWritebackReport",
  "ApprovalManagerBridgePlanReport",
  "GlobalApprovalRequestPlan",
  "SkillAnomalyApprovalQueueWriteback",
  "SkillAnomalyApprovalManagerBridgePlan",
  "skill.approval_queue.writeback",
  "skill.approval_manager.bridge.plan",
  "skill_anomaly.approval_queue.writeback",
  "skill_anomaly.approval_manager.bridge.plan",
  "writes_approval_queue",
  "writes_approval_queue_file",
  "execution_blocked",
  "action_allowed",
  "cfg.DataPath(\"skill-anomaly\")",
  "json-skill-anomaly-evidence",
  "audit-hook-plan.json",
  "trust-mutation-plan.json",
  "http.MethodPost",
  "yunque-client/skill-anomaly",
  "Skill Anomaly Pack",
  "createSkillAnomalyPackClient",
  "createSkillAnomalyClient",
  "approvalQueueWriteback",
  "approvalManagerBridgePlan",
  "skill-anomaly-pack-client",
  "TestSkillAnomalyPackGateReturnsNotFoundWhenDisabled",
  "/packs/skill-anomaly",
  "pack-shell-before-audit-hook",
  "distribution",
  "rollback",
]);

requireTokens("wasm-plugin 蓝图能力包", wasmPluginPack + wasmPluginManifest + frontend + gateway + sdk + docs, [
  'const PackID = "yunque.pack.wasm-plugin"',
  "func (h *Handler) Routes() []packruntime.BackendRoute",
  "/v1/wasm-plugin/status",
  "/v1/wasm-plugin/plugins",
  "/v1/wasm-plugin/plugins/load",
  "/v1/wasm-plugin/plugins/unload",
  "/v1/wasm-plugin/execute",
  "/v1/wasm-plugin/remote-install/plan",
  "/v1/wasm-plugin/remote-install/approval/plan",
  "/v1/wasm-plugin/remote-install/approval/decision/plan",
  "/v1/wasm-plugin/remote-install/approval/writeback/plan",
  "/v1/wasm-plugin/evidence/",
  "runtime_ready",
  "abi_plan_ready",
  "abi_ready",
  "host_abi_execution_gate_ready",
  "host_abi_enforcement_ready",
  "module_integrity_gate_ready",
  "remote_install_plan_ready",
  "remote_install_ready",
  "signature_verification_plan_ready",
  "signature_verify_ready",
  "approval_gate_plan_ready",
  "approval_gate_ready",
  "approval_queue_plan_ready",
  "approval_queue_entry",
  "ApprovalQueueEntryPlan",
  "approval_decision_plan_ready",
  "approval_decision_ready",
  "applies_approval_decision",
  "approval_decision_plan",
  "ApprovalDecisionPlan",
  "RemoteInstallApprovalDecisionPlanReport",
  "approval_writeback_plan_ready",
  "approval_writeback_ready",
  "approval_writeback_plan",
  "ApprovalWritebackPlan",
  "RemoteInstallApprovalWritebackPlanReport",
  "installer_blocked_until_writeback",
  "blocked_until_approval_queue",
  "host_abi_plan",
  "host_abi_gate",
  "module_integrity_gate",
  "remote_install_plan",
  "signature_verification",
  "approval_gate_plan",
  "approval_queue_entry",
  "wasm.host_abi.plan",
  "wasm.host_abi.execution_gate",
  "wasm.module.integrity_gate",
  "module-integrity-gate.json",
  "integrity_gate_ready",
  "blocked_module_sha256_mismatch",
  "execution_gate_ready",
  "allows_execution",
  "blocked_until_host_abi_enforcement",
  "wasm.remote_install.plan",
  "wasm.remote_install.signature_verification_plan",
  "wasm.remote_install.approval_plan",
  "wasm.remote_install.approval_decision_plan",
  "wasm.remote_install.approval_writeback_plan",
  "wasm.remote_install.approval_queue_writeback",
  "host-abi-plan.json",
  "remote-install-plan.json",
  "approval-gate-plan.json",
  "approval-queue-entry.json",
  "approval-decision-plan.json",
  "approval-writeback-plan.json",
  "approval-queue-store.json",
  "approval-queue-record.json",
  "signature-verification.json",
  "WASMPluginSignatureVerificationPlan",
  "blocked_until_signature_verifier",
  "allows_install",
  "enforcement_ready",
  "writes_files",
  "downloads",
  "cfg.DataPath(\"wasm-plugin\")",
  "json-wasm-plugin-evidence",
  "http.MethodPost",
  "yunque-client/wasm-plugin",
  "WASM Plugin Pack",
  "createWASMPluginPackClient",
  "createWASMPluginClient",
  "remoteInstallApprovalPlan",
  "remoteInstallApprovalDecisionPlan",
  "remoteInstallApprovalWritebackPlan",
  "remoteInstallApprovalQueueWriteback",
  "WASMPluginRemoteInstallApprovalPlan",
  "WASMPluginRemoteInstallApprovalDecisionPlan",
  "WASMPluginRemoteInstallApprovalWritebackPlan",
  "WASMPluginRemoteInstallApprovalQueueWriteback",
  "WASMPluginApprovalQueueRecord",
  "WASMPluginApprovalQueueStoreSummary",
  "wasm-plugin-pack-client",
  "TestWASMPluginPackGateReturnsNotFoundWhenDisabled",
  "/packs/wasm-plugin",
  "pack-shell-before-runtime-hosts",
  "normalizeModulePath",
  "validateModulePath",
  "distribution",
  "rollback",
]);

requireTokens("前端同步菜单/路由/资源/控制台", frontend + fullVerification, [
  "fetchEnabledPacks",
  "buildPackNavItems",
  "buildPackRouteBindings",
  "findPackRouteBinding",
  "pack-sync frontend runtime",
  "createPacksClient",
  "packs-client",
  "pack-types",
  "createBackupPackClient",
  "backup-pack-client",
  "Frontend Pack sync tests",
  "Frontend packs client tests",
  "Frontend backup pack client tests",
  "Frontend LoRA pack client tests",
  "Frontend Cogni Kernel pack client tests",
  "Frontend Browser Intent pack client tests",
  "Frontend Chaos Probe pack client tests",
  "Frontend Cognitive Canary pack client tests",
  "Frontend Guardrail Fuzzer pack client tests",
  "Frontend RPA Replay pack client tests",
  "Frontend SBOM Drift pack client tests",
  "Frontend Skill Anomaly pack client tests",
  "Frontend WASM Plugin pack client tests",
  "Frontend shell pack entry tests",
  "PackRuntimeRoutePage",
  "enabled()",
  "frontend?.menus",
  "frontend?.routes",
  "routeSpecs",
  "buildPackBackendRouteBindings",
  "/v1/packs/enabled",
  "manifest.distribution",
  "backendModules()",
  "installFromURL",
  "downloadArtifact",
  "prune()",
  "previousArtifacts",
  "SDK 调用入口",
]);


if (frontendShell.includes('href: "/backup"') || frontendShell.includes('href: "/backup"') || frontendShell.includes('nav-backup')) {
  fail("前端同步菜单/路由/资源/控制台", "backup pack must not be exposed as a hard-coded main-shell nav item; use /v1/packs/enabled pack sync");
} else {
  ok("前端轻内核导航", "backup entry is not hard-coded in sidebar/nav-items/command-palette");
}

if (backupPackPage.includes("api.backupInfo") || backupPackPage.includes("api.backupExport") || backupPackPage.includes("api.backupImport") || backupPackPage.includes('from "@/lib/api"')) {
  fail("前端同步菜单/路由/资源/控制台", "backup pack page must use backup-pack-client instead of the monolithic api object");
} else {
  ok("前端 pack 客户端拆分", "backup page uses backup-pack-client instead of monolithic api backup methods");
}

if (!legacyLoRAPage.includes('redirect("/packs/lora")')) {
  fail("前端同步菜单/路由/资源/控制台", "legacy /lora page must redirect to the LoRA pack route");
} else {
  ok("前端 LoRA pack 路由兼容", "legacy /lora redirects to /packs/lora");
}

if (frontendShell.includes('href: "/lora"') || frontendShell.includes('nav-lora')) {
  fail("前端同步菜单/路由/资源/控制台", "LoRA must not be exposed as a hard-coded main-shell nav item; use enabled-pack sync");
} else {
  ok("前端轻内核导航", "LoRA entry is not hard-coded in sidebar/nav-items/command-palette");
}

if (loraPackPage.includes("api.getLoRA") || loraPackPage.includes("api.triggerLoRA") || loraPackPage.includes("api.rollbackLoRA") || loraPackPage.includes('from "@/lib/api"')) {
  fail("前端同步菜单/路由/资源/控制台", "LoRA pack page must use lora-pack-client instead of the monolithic api object");
} else {
  ok("前端 LoRA pack 客户端拆分", "LoRA page uses lora-pack-client instead of monolithic api LoRA methods");
}

if (!legacyCogniPage.includes('redirect("/packs/cognis")')) {
  fail("前端同步菜单/路由/资源/控制台", "legacy /cognis page must redirect to the Cogni Kernel pack route");
} else {
  ok("前端 Cogni Kernel pack 路由兼容", "legacy /cognis redirects to /packs/cognis");
}

if (frontendShell.includes('href: "/cognis"') || frontendShell.includes("nav-cognis")) {
  fail("前端同步菜单/路由/资源/控制台", "Cogni Kernel must not be exposed as a hard-coded main-shell nav item; use enabled-pack sync");
} else {
  ok("前端轻内核导航", "Cogni Kernel entry is not hard-coded in sidebar/nav-items/command-palette");
}

if (cogniPackPage.includes("api.listCognis") || cogniPackPage.includes("api.reloadCognis") || cogniPackPage.includes("api.triggerCogniEvolution") || cogniPackPage.includes('from "@/lib/api"')) {
  fail("前端同步菜单/路由/资源/控制台", "Cogni Kernel pack page must use cogni-kernel-pack-client instead of the monolithic api object");
} else {
  ok("前端 Cogni Kernel pack 客户端拆分", "Cogni Kernel page uses cogni-kernel-pack-client instead of monolithic api Cogni methods");
}

if (!legacyBrowserPage.includes('redirect("/packs/browser")')) {
  fail("前端同步菜单/路由/资源/控制台", "legacy /browser page must redirect to the Browser Intent pack route");
} else {
  ok("前端 Browser Intent pack 路由兼容", "legacy /browser redirects to /packs/browser");
}

if (frontendShell.includes('href: "/browser"') || frontendShell.includes("nav-browser")) {
  fail("前端同步菜单/路由/资源/控制台", "Browser Intent must not be exposed as a hard-coded main-shell nav item; use enabled-pack sync");
} else {
  ok("前端轻内核导航", "Browser Intent entry is not hard-coded in sidebar/nav-items/command-palette");
}

if (browserPackPage.includes("api.browser") || browserPackPage.includes('from "@/lib/api"') || !browserPackPage.includes("createBrowserIntentPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "Browser Intent pack page must use browser-intent-pack-client instead of the monolithic api object");
} else {
  ok("前端 Browser Intent pack 客户端拆分", "Browser Intent page uses browser-intent-pack-client instead of monolithic api browser methods");
}

if (chaosProbePackPage.includes("api.chaosProbe") || chaosProbePackPage.includes('from "@/lib/api"') || !chaosProbePackPage.includes("createChaosProbePackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "Chaos Probe pack page must use chaos-probe-pack-client instead of the monolithic api object");
} else {
  ok("前端 Chaos Probe pack 客户端拆分", "Chaos Probe page uses chaos-probe-pack-client instead of monolithic api chaos probe methods");
}

if (cognitiveCanaryPackPage.includes("api.cognitiveCanary") || cognitiveCanaryPackPage.includes('from "@/lib/api"') || !cognitiveCanaryPackPage.includes("createCognitiveCanaryPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "Cognitive Canary pack page must use cognitive-canary-pack-client instead of the monolithic api object");
} else {
  ok("前端 Cognitive Canary pack 客户端拆分", "Cognitive Canary page uses cognitive-canary-pack-client instead of monolithic api cognitive canary methods");
}
if (!cognitiveCanaryPackPage.includes("response collector") || !cognitiveCanaryPackPage.includes("writes_files")) {
  fail("前端同步菜单/路由/资源/控制台", "Cognitive Canary pack page should preview response collector plan boundaries");
} else {
  ok("前端 Cognitive Canary response collector 预览", "Cognitive Canary page shows response collector plan metadata and writes_files boundary");
}
if (!cognitiveCanaryPackPage.includes("Collector Pipeline 计划") || !cognitiveCanaryPackPage.includes("consumes_response_collector_store")) {
  fail("前端同步菜单/路由/资源/控制台", "Cognitive Canary pack page should preview response collector pipeline handoff boundaries");
} else {
  ok("前端 Cognitive Canary response collector pipeline 预览", "Cognitive Canary page shows plan-only collector pipeline handoff boundaries");
}

if (guardrailFuzzerPackPage.includes("api.guardrailFuzzer") || guardrailFuzzerPackPage.includes('from "@/lib/api"') || !guardrailFuzzerPackPage.includes("createGuardrailFuzzerPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "Guardrail Fuzzer pack page must use guardrail-fuzzer-pack-client instead of the monolithic api object");
} else {
  ok("前端 Guardrail Fuzzer pack 客户端拆分", "Guardrail Fuzzer page uses guardrail-fuzzer-pack-client instead of monolithic api guardrail fuzzer methods");
}

if (memoryTimeTravelPackPage.includes("api.memoryTimeTravel") || memoryTimeTravelPackPage.includes('from "@/lib/api"') || !memoryTimeTravelPackPage.includes("createMemoryTimeTravelPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "Memory Time Travel pack page must use memory-time-travel-pack-client instead of the monolithic api object");
} else {
  ok("前端 Memory Time Travel pack 客户端拆分", "Memory Time Travel page uses memory-time-travel-pack-client instead of monolithic api memory time travel methods");
}

if (rpaReplayPackPage.includes("api.rpa") || rpaReplayPackPage.includes('from "@/lib/api"') || !rpaReplayPackPage.includes("createRPAReplayPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "RPA Replay pack page must use rpa-replay-pack-client instead of the monolithic api object");
} else {
  ok("前端 RPA Replay pack 客户端拆分", "RPA Replay page uses rpa-replay-pack-client instead of monolithic api RPA methods");
}

if (sbomDriftPackPage.includes("api.sbom") || sbomDriftPackPage.includes('from "@/lib/api"') || !sbomDriftPackPage.includes("createSBOMDriftPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "SBOM Drift pack page must use sbom-drift-pack-client instead of the monolithic api object");
} else {
  ok("前端 SBOM Drift pack 客户端拆分", "SBOM Drift page uses sbom-drift-pack-client instead of monolithic api SBOM methods");
}

if (skillAnomalyPackPage.includes("api.skillAnomaly") || skillAnomalyPackPage.includes('from "@/lib/api"') || !skillAnomalyPackPage.includes("createSkillAnomalyPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "Skill Anomaly pack page must use skill-anomaly-pack-client instead of the monolithic api object");
} else {
  ok("前端 Skill Anomaly pack 客户端拆分", "Skill Anomaly page uses skill-anomaly-pack-client instead of monolithic api skill anomaly methods");
}

if (wasmPluginPackPage.includes("api.wasm") || wasmPluginPackPage.includes('from "@/lib/api"') || !wasmPluginPackPage.includes("createWASMPluginPackClient")) {
  fail("前端同步菜单/路由/资源/控制台", "WASM Plugin pack page must use wasm-plugin-pack-client instead of the monolithic api object");
} else {
  ok("前端 WASM Plugin pack 客户端拆分", "WASM Plugin page uses wasm-plugin-pack-client instead of monolithic api WASM methods");
}

const packsConsolePage = read("heroui-web/src/app/packs/page.tsx");
if (packsConsolePage.includes("api.packsInstalled") || packsConsolePage.includes("api.packBackendModules") || packsConsolePage.includes("api.packInstall") || packsConsolePage.includes("api.packEnable") || packsConsolePage.includes("api.packDisable") || packsConsolePage.includes("api.packRollback") || packsConsolePage.includes("api.packPrune")) {
  fail("前端同步菜单/路由/资源/控制台", "Pack console must use packs-client instead of monolithic api pack methods");
} else {
  ok("前端 Pack Runtime 客户端拆分", "Pack console uses packs-client instead of monolithic api pack methods");
}

const monolithicApi = read("heroui-web/src/lib/api.ts");
const cherrySettings = read("heroui-web/src/components/cherry/settings-modal.tsx");
const forbiddenMonolithicPackMethods = [
  "backupInfo:",
  "backupExport:",
  "backupImport:",
  "packsInstalled:",
  "packsEnabled:",
  "packBackendModules:",
  "packInstall:",
  "packInstallFromURL:",
  "packEnable:",
  "packDisable:",
  "packRollback:",
  "packPrune:",
  "getLoRAStatus:",
  "getLoRAHistory:",
  "getLoRASummary:",
  "previewLoRATrainingData:",
  "triggerLoRATraining:",
  "rollbackLoRA:",
  "getEvolutionState:",
  "getLoRAConfig:",
  "updateLoRAConfig:",
  "listCognis:",
  "getCogni:",
  "addCogni:",
  "removeCogni:",
  "setCogniEnabled:",
  "reloadCognis:",
  "getCogniHealth:",
  "getCogniAlerts:",
  "scanCogniAlerts:",
  "verifyCognis:",
  "getCogniTraces:",
  "getCogniTracesByID:",
  "generateCogni:",
  "getCogniWorkflows:",
  "runCogniWorkflow:",
  "getCogniExperience:",
  "confirmCogniExperiencePattern:",
  "triggerCogniEvolution:",
  "getCogniEvolution:",
  "getCogniFederation:",
  "exposeCogni:",
  "browserNavigate:",
  "browserScreenshot:",
  "browserOcr:",
  "browserStatus:",
  "browserConfig:",
  "browserScreenshotLatest:",
  "browserOPPPending:",
  "browserOPPDecide:",
  "browserExtStatus:",
  "browserExtAction:",
  "browserExtScenarios:",
  "browserExtRunScenario:",
  "chaosProbeStatus:",
  "chaosProbeRun:",
  "chaosProbeEvidence:",
  "cognitiveCanaryStatus:",
  "cognitiveCanaryEvaluate:",
  "cognitiveCanaryEvidence:",
  "guardrailFuzzerStatus:",
  "guardrailFuzzerRun:",
  "guardrailFuzzerEvidence:",
  "memoryTimeTravelStatus:",
  "memoryTimeTravelDiff:",
  "memoryTimeTravelEvidence:",
  "rpaReplayStatus:",
  "createRPAReplayTrace:",
  "rpaReplay:",
  "rpaReplayEvidence:",
  "sbomDriftStatus:",
  "createSBOMDriftSnapshot:",
  "sbomDriftDiff:",
  "sbomDriftEvidence:",
  "skillAnomalyStatus:",
  "createSkillAnomalyEvent:",
  "skillAnomalyDetect:",
  "skillAnomalyEvidence:",
  "wasmPluginStatus:",
  "createWASMPlugin:",
  "wasmPluginExecute:",
  "wasmPluginEvidence:",
];
const leakedMonolithicMethods = forbiddenMonolithicPackMethods.filter((token) => monolithicApi.includes(token));
if (leakedMonolithicMethods.length > 0) {
  fail("前端轻内核 API 拆分", `monolithic api.ts still exposes pack methods: ${leakedMonolithicMethods.join(", ")}`);
} else {
  ok("前端轻内核 API 拆分", "backup/pack/browser/chaos-probe/cognitive-canary/guardrail-fuzzer/memory-time-travel/rpa/sbom/skill-anomaly/wasm methods live in lightweight clients instead of monolithic api.ts");
}

if (cherrySettings.includes("createBackupPackClient") || cherrySettings.includes("backupPack.export") || cherrySettings.includes("api.backup")) {
  fail("前端轻内核设置入口", "Cherry settings must not execute backup pack actions directly; link to Pack Runtime instead");
} else if (!cherrySettings.includes("BACKUP_PACK_ID") || !cherrySettings.includes('"/packs/backup"') || !cherrySettings.includes("packsClient") || !cherrySettings.includes(".installed()")) {
  fail("前端轻内核设置入口", "Cherry settings must expose backup as a pack-registry-aware entrypoint");
} else {
  ok("前端轻内核设置入口", "Cherry settings links to backup pack through pack registry state");
}

requireTokens("TypeScript packs SDK", sdk, [
  "createPacksClient",
  "installed()",
  "enabled()",
  "backendModules()",
  "install(request",
  "enable(id",
  "disable(id",
  "rollback(id",
  "prune()",
  "frontendSync()",
  "routeBinding(path",
  "PackRouteBinding",
  "PackBackendRouteBinding",
  "PackDistributionManifest",
  "PackPruneResponse",
  "createChaosProbeClient",
  "ChaosProbeClientError",
  "createCognitiveCanaryClient",
  "CognitiveCanaryClientError",
  "createGuardrailFuzzerClient",
  "GuardrailFuzzerClientError",
  "createMemoryTimeTravelClient",
  "MemoryTimeTravelClientError",
  "createRPAReplayClient",
  "RPAReplayClientError",
  "createSBOMDriftClient",
  "createSBOMDriftCIClient",
  "SBOMDriftClientError",
  "createSkillAnomalyClient",
  "SkillAnomalyClientError",
  "createWASMPluginClient",
  "WASMPluginClientError",
  "download?: boolean",
  "distributions:",
  "routeBindings:",
]);

requireTokens("脚手架和可回滚工程化", scaffold + fullVerification + docs, [
  "check-pack-runtime-completion.mjs",
  "check-pack-runtime-all.mjs",
  "scaffold-pack.mjs",
  "--dry-run",
  "--json",
  "check-pack-scaffold.mjs",
  "-pack-client.ts",
  "-pack-client.test.ts",
  "frontend.menus[].path",
  "frontend.routes[].path",
  "distribution.packageUrl",
  "update.rollback=true",
  "GatewayConfig.BackendPacks",
  "RegisterBackendPack",
]);

runCheck("contract checker", process.execPath, ["scripts/check-pack-contract.mjs"]);
runCheck("scaffold checker", process.execPath, ["scripts/check-pack-scaffold.mjs"]);
runCheck("packs sdk checker", process.execPath, ["sdk/scripts/check-packs-sdk-manifest.mjs"]);
runCheck("lora pack sdk checker", process.execPath, ["sdk/scripts/check-lora-pack-sdk-manifest.mjs"]);
runCheck("cogni kernel pack sdk checker", process.execPath, ["sdk/scripts/check-cogni-kernel-pack-sdk-manifest.mjs"]);
runCheck("browser intent pack sdk checker", process.execPath, ["sdk/scripts/check-browser-intent-pack-sdk-manifest.mjs"]);
runCheck("chaos probe pack sdk checker", process.execPath, ["sdk/scripts/check-chaos-probe-pack-sdk-manifest.mjs"]);
runCheck("cognitive canary pack sdk checker", process.execPath, ["sdk/scripts/check-cognitive-canary-pack-sdk-manifest.mjs"]);
runCheck("guardrail fuzzer pack sdk checker", process.execPath, ["sdk/scripts/check-guardrail-fuzzer-pack-sdk-manifest.mjs"]);
runCheck("memory time travel pack sdk checker", process.execPath, ["sdk/scripts/check-memory-time-travel-pack-sdk-manifest.mjs"]);
runCheck("rpa replay pack sdk checker", process.execPath, ["sdk/scripts/check-rpa-replay-pack-sdk-manifest.mjs"]);
runCheck("sbom drift pack sdk checker", process.execPath, ["sdk/scripts/check-sbom-drift-pack-sdk-manifest.mjs"]);
runCheck("skill anomaly pack sdk checker", process.execPath, ["sdk/scripts/check-skill-anomaly-pack-sdk-manifest.mjs"]);
runCheck("wasm plugin pack sdk checker", process.execPath, ["sdk/scripts/check-wasm-plugin-pack-sdk-manifest.mjs"]);

if (failures.length > 0) {
  console.error("Pack Runtime completion audit failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("Pack Runtime completion audit ok:");
for (const item of evidence) console.log(`- ${item.item}: ${item.message}`);
