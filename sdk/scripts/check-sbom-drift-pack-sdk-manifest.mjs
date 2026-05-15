import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/sbom-drift-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.sbom-drift") fail(`unexpected SBOM Drift pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("SBOM Drift pack sdk.typescript must match sbom-drift-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("SBOM Drift pack frontend menu path must remain /packs/sbom-drift");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("SBOM Drift pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("SBOM Drift pack must be rollbackable");
if (pack.defaultState !== "disabled") fail("SBOM Drift pack should stay default disabled until CI scanner wiring is complete");
if (pack.metadata?.stage !== "pack-shell-before-ci") fail("SBOM Drift pack should declare pack-shell-before-ci stage");
for (const capability of ["sbom.cyclonedx.export", "sbom.ci_gate.plan", "sbom.govulncheck.plan"]) {
  if (!pack.backend?.capabilities?.includes(capability)) fail(`SBOM Drift pack capability missing: ${capability}`);
}

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`SBOM Drift pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createSBOMDriftPackClient",
  "/v1/sbom-drift/status",
  "/v1/sbom-drift/snapshots",
  "/v1/sbom-drift/diff",
  "/v1/sbom-drift/cyclonedx/",
  "/v1/sbom-drift/ci-gate/plan",
  "/v1/sbom-drift/evidence/",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`sbom-drift-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createSBOMDriftPackClient") || page.includes('from "@/lib/api"') || page.includes("api.sbom")) {
  fail("SBOM Drift pack page must use sbom-drift-pack-client instead of monolithic api.ts");
}
for (const token of ["SBOM 依赖漂移", "生成漂移报告", "CycloneDX", "CI Gate Plan", "导出证据包", "pack-shell"]) {
  if (!page.includes(token)) fail(`SBOM Drift pack page missing product token: ${token}`);
}

const frontendTest = readRepoFile("heroui-web/src/lib/__tests__/sbom-drift-pack-client.test.ts");
for (const token of ["/v1/sbom-drift/status", "/v1/sbom-drift/diff", "/v1/sbom-drift/cyclonedx/baseline", "/v1/sbom-drift/ci-gate/plan", "/v1/sbom-drift/evidence/baseline"]) {
  if (!frontendTest.includes(token)) fail(`SBOM Drift frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/sbomdrift/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_sbom_drift_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.sbom-drift\"",
  "scanner_ready",
  "cyclonedx_ready",
  "ci_gate_plan_ready",
  "ci_gate_ready",
  "govulncheck_plan_ready",
  "govulncheck_ready",
  "GovulncheckPlan",
  "govulncheck-report.json",
  "govulncheck-plan.json",
  "writes_files",
  "CycloneDX",
  "CIGatePlan",
  "json-sbom-drift-evidence",
  "cfg.DataPath(\"sbom-drift\")",
  "sbomdriftpack.New",
  "packs/examples/sbom-drift-pack/pack.json",
  "ensureBuiltinPacks",
  "TestSBOMDriftPackGateReturnsNotFoundWhenDisabled",
  "StatusMethodNotAllowed",
]) {
  if (!backend.includes(token)) fail(`SBOM Drift backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("sdk/typescript/src/sbom-drift.ts") + "\n" + readRepoFile("sdk/typescript/src/sbom-drift.test.ts");
for (const token of [
  "createSBOMDriftClient",
  "SBOMDriftClientError",
  "/v1/sbom-drift/status",
  "/v1/sbom-drift/diff",
  "/v1/sbom-drift/cyclonedx/",
  "/v1/sbom-drift/ci-gate/plan",
  "/v1/sbom-drift/evidence/",
  "SBOMDriftCycloneDXDocument",
  "SBOMDriftCIGatePlan",
  "SBOMDriftGovulncheckPlan",
  "govulncheck_plan_ready",
  "govulncheck_plan",
  "govulncheck-report.json",
  "writes_files",
  "SBOM Drift request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript SBOM Drift SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("sdk/typescript/package.json") || "{}");
if (pkg.exports?.["./sbom-drift"]?.import !== "./src/sbom-drift.ts") fail("yunque-client/sbom-drift subpath export is missing or drifted");

const monolithicApi = readRepoFile("heroui-web/src/lib/api.ts");
for (const token of ["sbomDriftStatus:", "createSBOMDriftSnapshot:", "sbomDriftDiff:", "sbomDriftEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose SBOM Drift method: ${token}`);
}

if (failures.length) {
  console.error("SBOM Drift Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`SBOM Drift Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
