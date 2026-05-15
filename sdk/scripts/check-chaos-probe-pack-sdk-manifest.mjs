import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/chaos-probe-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.chaos-probe") fail(`unexpected Chaos Probe pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("Chaos Probe pack sdk.typescript must match chaos-probe-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("Chaos Probe pack frontend menu path must remain /packs/chaos-probe");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("Chaos Probe pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("Chaos Probe pack must be rollbackable");
if (pack.defaultState !== "disabled") fail("Chaos Probe pack should stay default disabled until scheduler and degrade write-back are wired");
if (pack.metadata?.stage !== "pack-shell-before-scheduler") fail("Chaos Probe pack should declare pack-shell-before-scheduler stage");
if (pack.metadata?.blueprint !== "doc/CHAOS-PROBE.md") fail("Chaos Probe pack should point to doc/CHAOS-PROBE.md");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`Chaos Probe pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createChaosProbePackClient",
  "/v1/chaos-probe/status",
  "/v1/chaos-probe/probes",
  "/v1/chaos-probe/run",
  "/v1/chaos-probe/reports",
  "/v1/chaos-probe/evidence/",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`chaos-probe-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createChaosProbePackClient") || page.includes('from "@/lib/api"') || page.includes("api.chaosProbe")) {
  fail("Chaos Probe pack page must use chaos-probe-pack-client instead of monolithic api.ts");
}
for (const token of ["Chaos Probe", "保存 Definitions", "运行探针", "导出证据包", "Pack shell"]) {
  if (!page.includes(token)) fail(`Chaos Probe pack page missing product token: ${token}`);
}

const frontendTest = readRepoFile("heroui-web/src/lib/__tests__/chaos-probe-pack-client.test.ts");
for (const token of ["/v1/chaos-probe/status", "/v1/chaos-probe/run", "/v1/chaos-probe/evidence/chaos-1"]) {
  if (!frontendTest.includes(token)) fail(`Chaos Probe frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/chaosprobe/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_chaos_probe_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.chaos-probe\"",
  "safe_probe_ready",
  "scheduler_ready",
  "degrade_engine_ready",
  "alert_writeback_ready",
  "json-chaos-probe-evidence",
  "cfg.DataPath(\"chaos-probe\")",
  "chaosprobepack.New",
  "packs/examples/chaos-probe-pack/pack.json",
  "ensureBuiltinPacks",
  "TestChaosProbePackGateReturnsNotFoundWhenDisabled",
  "StatusMethodNotAllowed",
]) {
  if (!backend.includes(token)) fail(`Chaos Probe backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("sdk/typescript/src/chaos-probe.ts") + "\n" + readRepoFile("sdk/typescript/src/chaos-probe.test.ts");
for (const token of [
  "createChaosProbeClient",
  "ChaosProbeClientError",
  "/v1/chaos-probe/status",
  "/v1/chaos-probe/run",
  "/v1/chaos-probe/evidence/",
  "Chaos Probe request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript Chaos Probe SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("sdk/typescript/package.json") || "{}");
if (pkg.exports?.["./chaos-probe"]?.import !== "./src/chaos-probe.ts") fail("yunque-client/chaos-probe subpath export is missing or drifted");

const monolithicApi = readRepoFile("heroui-web/src/lib/api.ts");
for (const token of ["chaosProbeStatus:", "chaosProbeRun:", "chaosProbeEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose Chaos Probe method: ${token}`);
}

if (failures.length) {
  console.error("Chaos Probe Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Chaos Probe Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
