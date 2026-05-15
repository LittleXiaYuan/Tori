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
  "/v1/cognitive-canary/reports",
  "/v1/cognitive-canary/evidence/",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`cognitive-canary-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createCognitiveCanaryPackClient") || page.includes('from "@/lib/api"') || page.includes("api.cognitiveCanary")) {
  fail("Cognitive Canary pack page must use cognitive-canary-pack-client instead of monolithic api.ts");
}
for (const token of ["Cognitive Canary", "保存 Scenarios", "运行评估", "导出证据包", "Pack shell"]) {
  if (!page.includes(token)) fail(`Cognitive Canary pack page missing product token: ${token}`);
}

const frontendTest = readRepoFile("heroui-web/src/lib/__tests__/cognitive-canary-pack-client.test.ts");
for (const token of ["/v1/cognitive-canary/status", "/v1/cognitive-canary/evaluate", "/v1/cognitive-canary/evidence/canary-1"]) {
  if (!frontendTest.includes(token)) fail(`Cognitive Canary frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/cognitivecanary/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_cognitive_canary_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.cognitive-canary\"",
  "shadow_traffic_ready",
  "judge_pipeline_ready",
  "quality_sli_ready",
  "auto_rollback_ready",
  "json-cognitive-canary-evidence",
  "cfg.DataPath(\"cognitive-canary\")",
  "cognitivecanarypack.New",
  "packs/examples/cognitive-canary-pack/pack.json",
  "ensureBuiltinPacks",
  "TestCognitiveCanaryPackGateReturnsNotFoundWhenDisabled",
  "StatusMethodNotAllowed",
]) {
  if (!backend.includes(token)) fail(`Cognitive Canary backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("sdk/typescript/src/cognitive-canary.ts") + "\n" + readRepoFile("sdk/typescript/src/cognitive-canary.test.ts");
for (const token of [
  "createCognitiveCanaryClient",
  "CognitiveCanaryClientError",
  "/v1/cognitive-canary/status",
  "/v1/cognitive-canary/evaluate",
  "/v1/cognitive-canary/evidence/",
  "Cognitive Canary request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript Cognitive Canary SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("sdk/typescript/package.json") || "{}");
if (pkg.exports?.["./cognitive-canary"]?.import !== "./src/cognitive-canary.ts") fail("yunque-client/cognitive-canary subpath export is missing or drifted");

const monolithicApi = readRepoFile("heroui-web/src/lib/api.ts");
for (const token of ["cognitiveCanaryStatus:", "cognitiveCanaryEvaluate:", "cognitiveCanaryEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose Cognitive Canary method: ${token}`);
}

if (failures.length) {
  console.error("Cognitive Canary Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Cognitive Canary Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
