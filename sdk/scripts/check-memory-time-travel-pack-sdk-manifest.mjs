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
for (const token of ["Approved rollback write-back plan", "buildApprovedRollbackPlan", "approved-rollback-plan.json", "rollback-writeback-plan.json", "approval-request-plan.json", "global_approval_enqueue_ready"]) {
  if (!page.includes(token)) fail(`Memory Time Travel pack page missing approved rollback token: ${token}`);
}
for (const token of ["KV audit proof-link schema", "previewAuditLinks", "audit-link-preview.json", "loadAuditLinks", "native kv_history", "buildRetentionPrunePlan", "生成审批计划"]) {
  if (!page.includes(token)) fail(`Memory Time Travel pack page missing KV audit link token: ${token}`);
}
for (const token of ["Native kv_history plan", "buildNativeKVHistoryPlan", "previewNativeKVHistoryMigration", "native-kv-history-plan.json", "kv-history-migration-plan.json", "kv-history-index-plan.json", "kv-history-migration-preview.json", "writes_native_kv_history"]) {
  if (!page.includes(token)) fail(`Memory Time Travel pack page missing native kv_history plan token: ${token}`);
}
for (const token of ["cutover plan", "buildKVHistoryCutoverPlan", "kv-history-cutover-plan.json", "kv-history-dual-read-plan.json", "dual_read_ready", "dual_write_ready", "switches_temporal_adapter"]) {
  if (!page.includes(token)) fail(`Memory Time Travel pack page missing kv_history cutover plan token: ${token}`);
}
for (const token of ["cutover readiness gate", "runKVHistoryCutoverReadiness", "kv-history-cutover-readiness.json", "passed_gate_count", "blocked_gate_count", "writes_ledger_kv"]) {
  if (!page.includes(token)) fail(`Memory Time Travel pack page missing kv_history cutover readiness token: ${token}`);
}
for (const token of ["dual-read parity gate", "runKVHistoryDualReadParity", "kv-history-dual-read-parity.json", "dual_read_parity_ready", "parity_passed", "switches_temporal_adapter"]) {
  if (!page.includes(token)) fail(`Memory Time Travel pack page missing kv_history dual-read parity token: ${token}`);
}

const frontendTest = readRepoFile("heroui-web/src/lib/__tests__/memory-time-travel-pack-client.test.ts");
for (const token of ["/v1/memory-time-travel/status", "/v1/memory-time-travel/diff", "/v1/memory-time-travel/rollback/approved-plan", "/v1/memory-time-travel/retention/plan?namespace=memory_snapshot", "/v1/memory-time-travel/retention/prune-plan", "/v1/memory-time-travel/kv-history/native-plan?namespace=memory_snapshot", "/v1/memory-time-travel/kv-history/migration-preview?namespace=memory_snapshot&limit=50", "/v1/memory-time-travel/kv-history/dual-read/parity", "/v1/memory-time-travel/kv-history/cutover/plan", "/v1/memory-time-travel/kv-history/cutover/readiness", "/v1/memory-time-travel/audit/links/preview", "/v1/memory-time-travel/audit/links?namespace=memory_snapshot", "/v1/memory-time-travel/audit/verify?limit=3", "/v1/memory-time-travel/evidence/baseline"]) {
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
  "retention_prune_plan_ready",
  "retention_prune_ready",
  "memory.retention.plan",
  "memory.retention.prune_plan",
  "memory.kv_history.native_plan",
  "native-kv-history-plan.json",
  "kv-history-migration-plan.json",
  "kv-history-index-plan.json",
  "kv-history-migration-preview.json",
  "kv-history-dual-read-parity.json",
  "kv_history_migration_preview",
  "kv_history_dual_read_parity",
  "kv-history-cutover-plan.json",
  "kv-history-cutover-readiness.json",
  "kv-history-dual-read-plan.json",
  "kv-history-dual-write-plan.json",
  "kv_history_cutover_plan",
  "kv_history_cutover_readiness",
  "kv_history_dual_read_plan",
  "kv_history_dual_write_plan",
  "kv_history_cutover_plan_ready",
  "kv_history_cutover_readiness_ready",
  "cutover_readiness_check_ready",
  "dual_read_plan_ready",
  "dual_read_parity_check_ready",
  "dual_read_parity_ready",
  "parity_passed",
  "dual_write_plan_ready",
  "dual_read_ready",
  "dual_write_ready",
  "cutover_ready",
  "switches_temporal_adapter",
  "native_kv_history_plan_ready",
  "kv_history_migration_plan_ready",
  "kv_history_index_plan_ready",
  "native_kv_history_preview_ready",
  "writes_native_kv_history",
  "migrates_kv_history",
  "/v1/memory-time-travel/kv-history/native-plan",
  "/v1/memory-time-travel/kv-history/migration-preview",
  "/v1/memory-time-travel/kv-history/dual-read/parity",
  "/v1/memory-time-travel/kv-history/cutover/plan",
  "/v1/memory-time-travel/kv-history/cutover/readiness",
  "/v1/memory-time-travel/audit/links/preview",
  "approved-rollback-plan.json",
  "rollback-writeback-plan.json",
  "approval-request-plan.json",
  "approved_rollback_plan_ready",
  "approval_request_plan_ready",
  "approval_manager_bridge_plan_ready",
  "global_approval_enqueue_ready",
  "rollback_writeback_plan_ready",
  "writes_ledger_kv",
  "writes_temporal_kv",
  "memory.rollback.approved_plan",
  "memory.rollback.writeback.plan",
  "/v1/memory-time-travel/rollback/approved-plan",
  "retention-prune-plan.json",
  "retention_prune_plan",
  "audit-links.json",
  "kv_audit_link_schema",
  "kv_audit_links",
  "kv_audit_link_schema_ready",
  "kv_audit_link_preview_ready",
  "kv_audit_linkage_ready",
  "memory.audit.links.schema",
  "memory.audit.links.preview",
  "audit-link-preview.json",
  "kv_audit_link_preview",
  "MerkleVerifier",
  "VerifyMerkleAuditChain",
  "/v1/memory-time-travel/retention/plan",
  "/v1/memory-time-travel/retention/prune-plan",
  "/v1/memory-time-travel/kv-history/native-plan",
  "/v1/memory-time-travel/kv-history/migration-preview",
  "/v1/memory-time-travel/kv-history/cutover/plan",
  "/v1/memory-time-travel/kv-history/cutover/readiness",
  "/v1/memory-time-travel/audit/links",
  "/v1/memory-time-travel/audit/links/preview",
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
  "/v1/memory-time-travel/audit/verify",
  "/v1/memory-time-travel/evidence/",
  "retentionPlan",
  "retentionPrunePlan",
  "nativeKVHistoryPlan",
  "nativeKVHistoryMigrationPreview",
  "kvHistoryDualReadParity",
  "kvHistoryCutoverPlan",
  "kvHistoryCutoverReadiness",
  "auditLinksPreview",
  "native_kv_history_plan",
  "kv_history_migration_plan",
  "kv_history_index_plan",
  "kv_history_migration_preview",
  "kv_history_dual_read_parity",
  "native_kv_history_preview_ready",
  "kv_history_cutover_plan",
  "kv_history_cutover_readiness",
  "kv_history_dual_read_plan",
  "kv_history_dual_write_plan",
  "kv_history_cutover_plan_ready",
  "kv_history_cutover_readiness_ready",
  "cutover_readiness_check_ready",
  "dual_read_plan_ready",
  "dual_read_parity_check_ready",
  "dual_read_parity_ready",
  "parity_passed",
  "dual_write_plan_ready",
  "cutover_ready",
  "approvedRollbackPlan",
  "approved_rollback_plan",
  "rollback_writeback_plan",
  "approval_request_plan",
  "auditLinks",
  "auditLinksPreview",
  "auditVerify",
  "kv_audit_link_schema",
  "kv_audit_link_preview",
  "kv_audit_link_preview_ready",
  "retention_prune_plan",
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
