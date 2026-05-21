import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/rpa-replay-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.rpa-replay") fail(`unexpected RPA Replay pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("RPA Replay pack sdk.typescript must match rpa-replay-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("RPA Replay pack frontend menu path must remain /packs/rpa-replay");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("RPA Replay pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("RPA Replay pack must be rollbackable");
if (pack.defaultState !== "disabled") fail("RPA Replay pack should stay default disabled until executor readiness is wired");
if (pack.metadata?.stage !== "pack-shell-before-executor") fail("RPA Replay pack should declare pack-shell-before-executor stage");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`RPA Replay pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createRPAReplayPackClient",
  "/v1/rpa-replay/status",
  "/v1/rpa-replay/traces",
  "/v1/rpa-replay/recordings/start",
  "/v1/rpa-replay/recordings/stop",
  "/v1/rpa-replay/replay",
  "/v1/rpa-replay/executor/plan",
  "/v1/rpa-replay/evidence/",
  "executorPlan",
  "executor_plan_ready",
  "browser_intent_gate_plan_ready",
  "action_tracer_plan_ready",
  "executes_browser_actions",
  "writes_browser_state",
  "network_access",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`rpa-replay-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createRPAReplayPackClient") || page.includes('from "@/lib/api"') || page.includes("api.rpa")) {
  fail("RPA Replay pack page must use rpa-replay-pack-client instead of monolithic api.ts");
}
for (const token of ["RPA 录制回放", "Dry-run 回放计划", "导出证据包", "Executor handoff", "Browser Intent / ActionTracer handoff", "Pack shell"]) {
  if (!page.includes(token)) fail(`RPA Replay pack page missing product token: ${token}`);
}

const frontendTest = readRepoFile("apps/web/src/lib/__tests__/rpa-replay-pack-client.test.ts");
for (const token of ["/v1/rpa-replay/status", "/v1/rpa-replay/replay", "/v1/rpa-replay/executor/plan", "/v1/rpa-replay/evidence/export-report", "executes_browser_actions", "writes_browser_state", "network_access"]) {
  if (!frontendTest.includes(token)) fail(`RPA Replay frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/rpareplay/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_rpa_replay_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.rpa-replay\"",
  "rpa-replay",
  "executor_ready",
  "/v1/rpa-replay/executor/plan",
  "executor_plan_ready",
  "browser_intent_gate_plan_ready",
  "action_tracer_plan_ready",
  "consumes_browser_intent",
  "executes_browser_actions",
  "writes_browser_state",
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
  "Methods: []string",
  "rpareplaypack.New",
  "packs/official/rpa-replay-pack/pack.json",
  "ensureBuiltinPacks",
]) {
  if (!backend.includes(token)) fail(`RPA Replay backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("packages/yunque-client/src/rpa-replay.ts") + "\n" + readRepoFile("packages/yunque-client/src/rpa-replay.test.ts");
for (const token of [
  "createRPAReplayClient",
  "RPAReplayClientError",
  "/v1/rpa-replay/status",
  "/v1/rpa-replay/replay",
  "/v1/rpa-replay/executor/plan",
  "/v1/rpa-replay/evidence/",
  "executorPlan",
  "executor_plan_ready",
  "executor_ready",
  "browser_intent_gate_plan_ready",
  "executes_browser_actions",
  "writes_browser_state",
  "network_access",
  "RPA Replay request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript RPA Replay SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("packages/yunque-client/package.json") || "{}");
if (pkg.exports?.["./rpa-replay"]?.import !== "./src/rpa-replay.ts") fail("yunque-client/rpa-replay subpath export is missing or drifted");

const monolithicApi = readRepoFile("apps/web/src/lib/api.ts");
for (const token of ["rpaReplayStatus:", "createRPAReplayTrace:", "rpaReplay:", "rpaReplayEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose RPA Replay method: ${token}`);
}

if (failures.length) {
  console.error("RPA Replay Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`RPA Replay Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
