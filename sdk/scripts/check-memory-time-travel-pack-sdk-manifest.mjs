import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/memory-time-travel-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.memory-time-travel") fail(`unexpected Memory Time Travel pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("Memory Time Travel pack sdk.typescript must match memory-time-travel-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("Memory Time Travel pack frontend menu path must remain /packs/memory-time-travel");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("Memory Time Travel pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("Memory Time Travel pack must be rollbackable");
if (pack.defaultState !== "disabled") fail("Memory Time Travel pack should stay default disabled until Ledger KV history and write-back are wired");
if (pack.metadata?.stage !== "pack-shell-before-ledger-kv-history") fail("Memory Time Travel pack should declare pack-shell-before-ledger-kv-history stage");
if (pack.metadata?.blueprint !== "doc/MEMORY-TIME-TRAVEL.md") fail("Memory Time Travel pack should point to doc/MEMORY-TIME-TRAVEL.md");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`Memory Time Travel pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createMemoryTimeTravelPackClient",
  "/v1/memory-time-travel/status",
  "/v1/memory-time-travel/snapshots",
  "/v1/memory-time-travel/snapshot-at",
  "/v1/memory-time-travel/diff",
  "/v1/memory-time-travel/rollback-plan",
  "/v1/memory-time-travel/retention/plan",
  "/v1/memory-time-travel/audit/verify",
  "/v1/memory-time-travel/evidence/",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`memory-time-travel-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createMemoryTimeTravelPackClient") || page.includes('from "@/lib/api"') || page.includes("api.memoryTimeTravel")) {
  fail("Memory Time Travel pack page must use memory-time-travel-pack-client instead of monolithic api.ts");
}
for (const token of ["Memory Time Travel", "保存快照", "生成 diff", "导出证据包", "Retention dry-run plan", "Merkle 审计链验证", "Pack shell"]) {
  if (!page.includes(token)) fail(`Memory Time Travel pack page missing product token: ${token}`);
}

const frontendTest = readRepoFile("heroui-web/src/lib/__tests__/memory-time-travel-pack-client.test.ts");
for (const token of ["/v1/memory-time-travel/status", "/v1/memory-time-travel/diff", "/v1/memory-time-travel/retention/plan?namespace=memory_snapshot", "/v1/memory-time-travel/audit/verify?limit=3", "/v1/memory-time-travel/evidence/baseline"]) {
  if (!frontendTest.includes(token)) fail(`Memory Time Travel frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/memorytimetravel/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_memory_time_travel_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.memory-time-travel\"",
  "snapshot_store_ready",
  "temporal_query_ready",
  "ledger_history_ready",
  "merkle_verification_ready",
  "audit-verification.json",
  "audit_verification",
  "memory.audit.verify",
  "retention-plan.json",
  "retention_plan",
  "retention_plan_ready",
  "retention_prune_ready",
  "memory.retention.plan",
  "MerkleVerifier",
  "VerifyMerkleAuditChain",
  "/v1/memory-time-travel/retention/plan",
  "/v1/memory-time-travel/audit/verify",
  "rollback_writeback_ready",
  "json-memory-time-travel-evidence",
  "cfg.DataPath(\"memory-time-travel\")",
  "memorytimetravelpack.New",
  "packs/examples/memory-time-travel-pack/pack.json",
  "ensureBuiltinPacks",
  "TestMemoryTimeTravelPackGateReturnsNotFoundWhenDisabled",
  "StatusMethodNotAllowed",
]) {
  if (!backend.includes(token)) fail(`Memory Time Travel backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("sdk/typescript/src/memory-time-travel.ts") + "\n" + readRepoFile("sdk/typescript/src/memory-time-travel.test.ts");
for (const token of [
  "createMemoryTimeTravelClient",
  "MemoryTimeTravelClientError",
  "/v1/memory-time-travel/status",
  "/v1/memory-time-travel/diff",
  "/v1/memory-time-travel/retention/plan",
  "/v1/memory-time-travel/audit/verify",
  "/v1/memory-time-travel/evidence/",
  "retentionPlan",
  "auditVerify",
  "Memory Time Travel request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript Memory Time Travel SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("sdk/typescript/package.json") || "{}");
if (pkg.exports?.["./memory-time-travel"]?.import !== "./src/memory-time-travel.ts") fail("yunque-client/memory-time-travel subpath export is missing or drifted");

const monolithicApi = readRepoFile("heroui-web/src/lib/api.ts");
for (const token of ["memoryTimeTravelStatus:", "memoryTimeTravelDiff:", "memoryTimeTravelEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose Memory Time Travel method: ${token}`);
}

if (failures.length) {
  console.error("Memory Time Travel Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Memory Time Travel Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
