import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/cognitive-canary-pack-sdk.json"), "utf8"));
const pack = JSON.parse(readFileSync(resolve(repoRoot, manifest.packManifest), "utf8"));
const failures = [];

function fail(message) {
  failures.push(message);
}

function readRepoFile(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) {
    fail(`missing file: ${path}`);
    return "";
  }
  return readFileSync(fullPath, "utf8");
}

if (pack.id !== "yunque.pack.cognitive-canary") fail(`unexpected Cognitive Canary pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("Cognitive Canary pack sdk.typescript must match cognitive-canary-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("Cognitive Canary pack frontend menu path must remain /packs/cognitive-canary");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("Cognitive Canary pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("Cognitive Canary pack must be rollbackable");
if (pack.defaultState !== "disabled") fail("Cognitive Canary pack should stay default disabled until shadow traffic and automatic rollback are wired");
if (pack.metadata?.stage !== "pack-shell-before-shadow-traffic") fail("Cognitive Canary pack should declare pack-shell-before-shadow-traffic stage");
if (pack.metadata?.blueprint !== "doc/COGNITIVE-CANARY.md") fail("Cognitive Canary pack should point to doc/COGNITIVE-CANARY.md");
if (!(pack.backend?.capabilities ?? []).includes("cognitive_canary.response_collector.plan")) fail("Cognitive Canary pack should expose response collector plan capability");
if (!(pack.backend?.capabilities ?? []).includes("cognitive_canary.response_collector.writeback")) fail("Cognitive Canary pack should expose response collector writeback capability");
if (!(pack.backend?.capabilities ?? []).includes("cognitive_canary.response_collector.pipeline.plan")) fail("Cognitive Canary pack should expose response collector pipeline plan capability");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`Cognitive Canary pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createCognitiveCanaryPackClient",
  "/v1/cognitive-canary/status",
  "/v1/cognitive-canary/scenarios",
  "/v1/cognitive-canary/evaluate",
  "/v1/cognitive-canary/shadow/plan",
  "/v1/cognitive-canary/response-collector/writeback",
  "/v1/cognitive-canary/response-collector/pipeline/plan",
  "/v1/cognitive-canary/reports",
  "/v1/cognitive-canary/evidence/",
  "shadowPlan",
  "responseCollectorWriteback",
  "responseCollectorPipelinePlan",
  "shadow_plan_ready",
  "response_collector_plan_ready",
  "response_collector_store_ready",
  "response_collector_writeback_ready",
  "writes_response_collector_store",
  "response_collector_pipeline_plan_ready",
  "consumes_response_collector_store",
  "response_collector_pipeline_ready",
  "response_collector_store",
  "response_collector_summary",
  "response_collectors",
  "response_collector_records",
  "artifact_sha256",
  "writes_files",
  "judge_plan_ready",
  "metrics_plan_ready",
  "prometheus_ready",
  "auto_rollback_plan_ready",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`cognitive-canary-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createCognitiveCanaryPackClient") || page.includes('from "@/lib/api"') || page.includes("api.cognitiveCanary")) {
  fail("Cognitive Canary pack page must use cognitive-canary-pack-client instead of monolithic api.ts");
}
for (const token of ["Cognitive Canary", "保存 Scenarios", "运行评估", "导出证据包", "Plan shell", "Shadow 计划", "response collector", "writes_files"]) {
  if (!page.includes(token)) fail(`Cognitive Canary pack page missing product token: ${token}`);
}
for (const token of ["写入 Collector Store", "responseCollectorWriteback", "collectorWriteback", "response_collector_store", "response-collector-store.json", "response-collector-record.json"]) {
  if (!page.includes(token)) fail(`Cognitive Canary pack page missing response collector writeback token: ${token}`);
}
for (const token of ["Collector Pipeline 计划", "responseCollectorPipelinePlan", "pipelinePlan", "response_collector_pipeline_plan", "response-collector-pipeline-plan.json", "response-collector-handoff-plan.json", "consumes_response_collector_store"]) {
  if (!page.includes(token)) fail(`Cognitive Canary pack page missing response collector pipeline plan token: ${token}`);
}

const frontendTest = readRepoFile("apps/web/src/lib/__tests__/cognitive-canary-pack-client.test.ts");
for (const token of ["/v1/cognitive-canary/status", "/v1/cognitive-canary/evaluate", "/v1/cognitive-canary/shadow/plan", "/v1/cognitive-canary/response-collector/writeback", "/v1/cognitive-canary/response-collector/pipeline/plan", "/v1/cognitive-canary/evidence/canary-1", "response_collector_summary", "artifact_sha256", "response-collector-plan.json", "response-collector-store.json", "response-collector-pipeline-plan.json", "consumes_response_collector_store", "writes_response_collector_store"]) {
  if (!frontendTest.includes(token)) fail(`Cognitive Canary frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/cognitivecanary/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_cognitive_canary_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.cognitive-canary\"",
  "/v1/cognitive-canary/shadow/plan",
  "/v1/cognitive-canary/response-collector/writeback",
  "/v1/cognitive-canary/response-collector/pipeline/plan",
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
  "canary.response_collector.pipeline.plan",
  "canary.response_collector.writeback",
  "canary.response_collector.plan",
  "response_collectors",
  "response_collector_summary",
  "response_collector_store",
  "response_collector_records",
  "response_collector_pipeline_plan",
  "artifact_sha256",
  "writes_files",
  "metrics_plan_ready",
  "prometheus_ready",
  "quality_sli_ready",
  "auto_rollback_plan_ready",
  "auto_rollback_ready",
  "canary.shadow.plan",
  "canary.judge.plan",
  "canary.metrics.plan",
  "canary.rollback.plan",
  "json-cognitive-canary-evidence",
  "shadow-plan.json",
  "response-collector-plan.json",
  "response-collector-store.json",
  "response-collector-record.json",
  "response-collector-pipeline-plan.json",
  "response-collector-handoff-plan.json",
  "judge-plan.json",
  "metrics-plan.json",
  "rollback-plan.json",
  "cfg.DataPath(\"cognitive-canary\")",
  "cognitivecanarypack.New",
  "packs/examples/cognitive-canary-pack/pack.json",
  "ensureBuiltinPacks",
  "TestCognitiveCanaryPackGateReturnsNotFoundWhenDisabled",
  "StatusMethodNotAllowed",
]) {
  if (!backend.includes(token)) fail(`Cognitive Canary backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("packages/yunque-client/src/cognitive-canary.ts") + "\n" + readRepoFile("packages/yunque-client/src/cognitive-canary.test.ts");
for (const token of [
  "createCognitiveCanaryClient",
  "CognitiveCanaryClientError",
  "/v1/cognitive-canary/status",
  "/v1/cognitive-canary/evaluate",
  "/v1/cognitive-canary/shadow/plan",
  "/v1/cognitive-canary/response-collector/writeback",
  "/v1/cognitive-canary/response-collector/pipeline/plan",
  "/v1/cognitive-canary/evidence/",
  "shadowPlan",
  "responseCollectorWriteback",
  "responseCollectorPipelinePlan",
  "CognitiveCanaryShadowPlan",
  "CognitiveCanaryResponseCollectorPlan",
  "CognitiveCanaryResponseCollectorWritebackReport",
  "CognitiveCanaryResponseCollectorPipelinePlan",
  "response_collector_plan_ready",
  "response_collector_writeback_ready",
  "writes_response_collector_store",
  "response_collector_pipeline_plan_ready",
  "consumes_response_collector_store",
  "response_collectors",
  "response_collector_store",
  "artifact_sha256",
  "writes_files",
  "Cognitive Canary request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript Cognitive Canary SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("packages/yunque-client/package.json") || "{}");
if (pkg.exports?.["./cognitive-canary"]?.import !== "./src/cognitive-canary.ts") fail("yunque-client/cognitive-canary subpath export is missing or drifted");

const monolithicApi = readRepoFile("apps/web/src/lib/api.ts");
for (const token of ["cognitiveCanaryStatus:", "cognitiveCanaryEvaluate:", "cognitiveCanaryEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose Cognitive Canary method: ${token}`);
}

if (failures.length) {
  console.error("Cognitive Canary Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Cognitive Canary Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
