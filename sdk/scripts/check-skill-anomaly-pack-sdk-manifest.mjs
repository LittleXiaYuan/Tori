import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/skill-anomaly-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.skill-anomaly") fail(`unexpected Skill Anomaly pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("Skill Anomaly pack sdk.typescript must match skill-anomaly-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("Skill Anomaly pack frontend menu path must remain /packs/skill-anomaly");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("Skill Anomaly pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("Skill Anomaly pack must be rollbackable");
if (pack.defaultState !== "disabled") fail("Skill Anomaly pack should stay default disabled until audit hook wiring is complete");
if (pack.metadata?.stage !== "pack-shell-before-audit-hook") fail("Skill Anomaly pack should declare pack-shell-before-audit-hook stage");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`Skill Anomaly pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createSkillAnomalyPackClient",
  "/v1/skill-anomaly/status",
  "/v1/skill-anomaly/events",
  "/v1/skill-anomaly/profiles",
  "/v1/skill-anomaly/detect",
  "/v1/skill-anomaly/evidence/",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`skill-anomaly-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createSkillAnomalyPackClient") || page.includes('from "@/lib/api"') || page.includes("api.skillAnomaly")) {
  fail("Skill Anomaly pack page must use skill-anomaly-pack-client instead of monolithic api.ts");
}
for (const token of ["Skill 行为异常", "写入基线事件", "Dry-run 检测", "导出证据包", "pack-shell"]) {
  if (!page.includes(token)) fail(`Skill Anomaly pack page missing product token: ${token}`);
}

const frontendTest = readRepoFile("heroui-web/src/lib/__tests__/skill-anomaly-pack-client.test.ts");
for (const token of ["/v1/skill-anomaly/status", "/v1/skill-anomaly/detect", "/v1/skill-anomaly/evidence/text_processing"]) {
  if (!frontendTest.includes(token)) fail(`Skill Anomaly frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/skillanomaly/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_skill_anomaly_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.skill-anomaly\"",
  "detector_ready",
  "audit_hook_ready",
  "json-skill-anomaly-evidence",
  "cfg.DataPath(\"skill-anomaly\")",
  "skillanomalypack.New",
  "packs/examples/skill-anomaly-pack/pack.json",
  "ensureBuiltinPacks",
  "TestSkillAnomalyPackGateReturnsNotFoundWhenDisabled",
  "StatusMethodNotAllowed",
]) {
  if (!backend.includes(token)) fail(`Skill Anomaly backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("sdk/typescript/src/skill-anomaly.ts") + "\n" + readRepoFile("sdk/typescript/src/skill-anomaly.test.ts");
for (const token of [
  "createSkillAnomalyClient",
  "SkillAnomalyClientError",
  "/v1/skill-anomaly/status",
  "/v1/skill-anomaly/detect",
  "/v1/skill-anomaly/evidence/",
  "Skill Anomaly request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript Skill Anomaly SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("sdk/typescript/package.json") || "{}");
if (pkg.exports?.["./skill-anomaly"]?.import !== "./src/skill-anomaly.ts") fail("yunque-client/skill-anomaly subpath export is missing or drifted");

const monolithicApi = readRepoFile("heroui-web/src/lib/api.ts");
for (const token of ["skillAnomalyStatus:", "createSkillAnomalyEvent:", "skillAnomalyDetect:", "skillAnomalyEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose Skill Anomaly method: ${token}`);
}

if (failures.length) {
  console.error("Skill Anomaly Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Skill Anomaly Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
