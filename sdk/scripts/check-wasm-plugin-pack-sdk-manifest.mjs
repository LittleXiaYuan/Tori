import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dirname, "../..");
const manifest = JSON.parse(readFileSync(resolve(repoRoot, "sdk/manifest/wasm-plugin-pack-sdk.json"), "utf8"));
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

if (pack.id !== "yunque.pack.wasm-plugin") fail(`unexpected WASM Plugin pack id: ${pack.id}`);
if (pack.sdk?.typescript !== manifest.sdkImport) fail("WASM Plugin pack sdk.typescript must match wasm-plugin-pack-sdk.json sdkImport");
if (pack.frontend?.menus?.[0]?.path !== manifest.frontend.menuPath) fail("WASM Plugin pack frontend menu path must remain /packs/wasm-plugin");
if (pack.frontend?.routes?.[0]?.component !== manifest.frontend.component) fail("WASM Plugin pack frontend route component drifted");
if (pack.update?.rollback !== true) fail("WASM Plugin pack must be rollbackable");
if (pack.defaultState !== "disabled") fail("WASM Plugin pack should stay default disabled because it is a high-risk execution surface");
if (pack.metadata?.stage !== "pack-shell-before-runtime-hosts") fail("WASM Plugin pack should declare pack-shell-before-runtime-hosts stage");
if (pack.metadata?.risk !== "high") fail("WASM Plugin pack should keep high-risk metadata");

const routeSpecs = new Set((pack.backend?.routeSpecs ?? []).map((route) => `${route.method} ${route.path}`));
for (const route of manifest.routes ?? []) {
  if (!routeSpecs.has(route)) fail(`WASM Plugin pack manifest missing routeSpec: ${route}`);
}

const client = readRepoFile(manifest.frontend.client);
for (const token of [
  "createWASMPluginPackClient",
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
  "WASMPluginHostABIPlan",
  "WASMPluginHostABIExecutionGate",
  "WASMPluginModuleIntegrityGate",
  "WASMPluginRemoteInstallPlan",
  "WASMPluginRemoteInstallApprovalPlan",
  "WASMPluginRemoteInstallApprovalDecisionPlan",
  "WASMPluginRemoteInstallApprovalWritebackPlan",
  "WASMPluginApprovalDecisionPlan",
  "WASMPluginApprovalWritebackPlan",
  "host_abi_plan",
  "host_abi_gate",
  "module_integrity_gate",
  "HostABIExecutionGate",
  "host_abi_execution_gate_ready",
  "host_abi_enforcement_ready",
  "module_integrity_gate_ready",
  "wasm.host_abi.execution_gate",
  "wasm.module.integrity_gate",
  "module_integrity_gate",
  "ModuleIntegrityGate",
  "integrity_gate_ready",
  "blocked_module_sha256_mismatch",
  "execution_gate_ready",
  "allows_execution",
  "blocked_until_host_abi_enforcement",
  "blocked_module_sha256_mismatch",
  "integrity_gate_ready",
  "remote_install_plan",
  "approval_gate_plan",
  "module_integrity_gate_ready",
  "remote_install_plan_ready",
  "signature_verification_plan_ready",
  "signature_verify_ready",
  "signature_verification",
  "WASMPluginSignatureVerificationPlan",
  "wasm.remote_install.signature_verification_plan",
  "approval_gate_plan_ready",
  "approval_queue_plan_ready",
  "approval_queue_entry",
  "approval_decision_plan_ready",
  "approval_decision_ready",
  "applies_approval_decision",
  "approval_decision_plan",
  "approval_writeback_plan_ready",
  "approval_writeback_ready",
  "approval_writeback_plan",
  "installer_blocked_until_writeback",
  "WASMPluginApprovalQueueEntryPlan",
  "remote-install-plan.json",
  "approval-gate-plan.json",
  "approval-queue-entry.json",
  "approval-decision-plan.json",
  "approval-writeback-plan.json",
  "approval-queue-store.json",
  "approval-queue-record.json",
  "installer-continuation-plan.json",
  "installer-download-handoff-plan.json",
  "installer-registration-handoff-plan.json",
  "installer-audit-handoff-plan.json",
  "signature-verification.json",
  "downloads",
  "enforcement_ready",
  "writes_files",
  "host_abi_gate",
  "execution_gate_ready",
  "allows_execution",
  "remoteInstallPlan",
  "remoteInstallApprovalPlan",
  "remoteInstallApprovalDecisionPlan",
  "remoteInstallApprovalWritebackPlan",
  "method: \"POST\"",
]) {
  if (!client.includes(token)) fail(`wasm-plugin-pack-client missing token: ${token}`);
}

const page = readRepoFile(manifest.frontend.page);
if (!page.includes("createWASMPluginPackClient") || page.includes('from "@/lib/api"') || page.includes("api.wasm")) {
  fail("WASM Plugin pack page must use wasm-plugin-pack-client instead of monolithic api.ts");
}
for (const token of ["WASM 插件引擎", "校验 / 注册插件", "Dry-run", "导出证据包", "Host ABI plan", "Host ABI execution gate", "module integrity gate", "module_integrity_gate_ready", "integrity_gate_ready", "sha256_blocked", "execution_gate_ready", "allows_execution", "blocked", "远程签名包安装计划", "远程安装审批 Gate 计划", "远程安装审批决策计划", "远程安装审批写回桥接计划", "remote_install_plan_ready", "remote_install_ready", "approval_gate_plan_ready", "approval_gate_ready", "approval_queue_plan_ready", "approval_queue_ready", "approval_decision_plan_ready", "approval_decision_ready", "applies_approval_decision", "approval_writeback_plan_ready", "approval_writeback_ready", "installer_blocked_until_writeback", "installer_continuation_plan_ready", "installer_ready", "installer-continuation-plan.json", "installer-download-handoff-plan.json", "installer-registration-handoff-plan.json", "would_allow_installer_continue", "blocks_installer", "decision_key", "queue_status", "blocked_until_approval_queue", "download_ready", "signature_verify_ready", "signature_verification_plan_ready", "signature_verification", "verifier_gate_ready", "allows_install", "remote-install-plan.json", "approval-gate-plan.json", "approval-queue-entry.json", "approval-decision-plan.json", "approval-writeback-plan.json", "signature-verification.json", "enforcement_ready", "writes_files", "pack-shell"]) {
  if (!page.includes(token)) fail(`WASM Plugin pack page missing product token: ${token}`);
}

const frontendTest = readRepoFile("heroui-web/src/lib/__tests__/wasm-plugin-pack-client.test.ts");
for (const token of ["/v1/wasm-plugin/status", "/v1/wasm-plugin/execute", "/v1/wasm-plugin/remote-install/plan", "/v1/wasm-plugin/remote-install/approval/plan", "/v1/wasm-plugin/remote-install/approval/decision/plan", "/v1/wasm-plugin/remote-install/approval/writeback/plan", "/v1/wasm-plugin/remote-install/approval/queue/writeback", "/v1/wasm-plugin/remote-install/installer/continuation/plan", "/v1/wasm-plugin/evidence/calculator", "host_abi_plan", "module_integrity_gate", "remote_install_plan", "signature_verification", "approval_gate_plan", "approval_queue_entry", "approval_decision_plan", "approval_writeback_plan", "host-abi-plan.json", "module-integrity-gate.json", "remote-install-plan.json", "signature-verification.json", "approval-gate-plan.json", "approval-queue-entry.json", "approval-decision-plan.json", "approval-writeback-plan.json", "approval-queue-store.json", "approval-queue-record.json", "installer-continuation-plan.json", "installer-download-handoff-plan.json", "installer-registration-handoff-plan.json", "installer-audit-handoff-plan.json"]) {
  if (!frontendTest.includes(token)) fail(`WASM Plugin frontend client test missing token: ${token}`);
}

const backend = readRepoFile("internal/packs/wasmplugin/handler.go")
  + "\n" + readRepoFile("internal/controlplane/gateway/handlers_wasm_plugin_pack_test.go")
  + "\n" + readRepoFile("cmd/agent/init_tasks.go")
  + "\n" + readRepoFile("cmd/agent/packruntime_bootstrap_test.go");
for (const token of [
  "const PackID = \"yunque.pack.wasm-plugin\"",
  "runtime_ready",
  "abi_plan_ready",
  "abi_ready",
  "module_integrity_gate_ready",
  "remote_install_plan_ready",
  "signature_verification_plan_ready",
  "signature_verify_ready",
  "signature_verification",
  "SignatureVerificationPlan",
  "wasm.remote_install.signature_verification_plan",
  "approval_gate_plan_ready",
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
  "wasm.remote_install.plan",
  "wasm.remote_install.approval_plan",
  "wasm.remote_install.approval_decision_plan",
  "wasm.remote_install.approval_writeback_plan",
  "host_abi_plan",
  "host_abi_gate",
  "module_integrity_gate",
  "HostABIExecutionGate",
  "host_abi_execution_gate_ready",
  "host_abi_enforcement_ready",
  "module_integrity_gate_ready",
  "wasm.host_abi.execution_gate",
  "wasm.module.integrity_gate",
  "module_integrity_gate",
  "ModuleIntegrityGate",
  "integrity_gate_ready",
  "blocked_module_sha256_mismatch",
  "execution_gate_ready",
  "allows_execution",
  "blocked_until_host_abi_enforcement",
  "blocked_module_sha256_mismatch",
  "integrity_gate_ready",
  "remote_install_plan",
  "approval_gate_plan",
  "wasm.host_abi.plan",
  "host-abi-plan.json",
  "module-integrity-gate.json",
  "remote-install-plan.json",
  "approval-gate-plan.json",
  "approval-decision-plan.json",
  "approval-writeback-plan.json",
  "approval-queue-store.json",
  "approval-queue-record.json",
  "installer-continuation-plan.json",
  "installer-download-handoff-plan.json",
  "installer-registration-handoff-plan.json",
  "installer-audit-handoff-plan.json",
  "signature-verification.json",
  "downloads",
  "json-wasm-plugin-evidence",
  "cfg.DataPath(\"wasm-plugin\")",
  "wasmpluginpack.New",
  "packs/examples/wasm-plugin-pack/pack.json",
  "ensureBuiltinPacks",
  "TestWASMPluginPackGateReturnsNotFoundWhenDisabled",
  "StatusMethodNotAllowed",
]) {
  if (!backend.includes(token)) fail(`WASM Plugin backend pack or gate missing token: ${token}`);
}

const sdk = readRepoFile("sdk/typescript/src/wasm-plugin.ts") + "\n" + readRepoFile("sdk/typescript/src/wasm-plugin.test.ts");
for (const token of [
  "createWASMPluginClient",
  "WASMPluginClientError",
  "/v1/wasm-plugin/status",
  "/v1/wasm-plugin/execute",
  "/v1/wasm-plugin/remote-install/plan",
  "/v1/wasm-plugin/remote-install/approval/plan",
  "/v1/wasm-plugin/remote-install/approval/decision/plan",
  "/v1/wasm-plugin/remote-install/approval/writeback/plan",
  "/v1/wasm-plugin/evidence/",
  "WASMPluginHostABIPlan",
  "WASMPluginHostABIExecutionGate",
  "WASMPluginModuleIntegrityGate",
  "WASMPluginRemoteInstallPlan",
  "WASMPluginRemoteInstallApprovalPlan",
  "WASMPluginRemoteInstallApprovalDecisionPlan",
  "WASMPluginRemoteInstallApprovalWritebackPlan",
  "WASMPluginApprovalDecisionPlan",
  "WASMPluginApprovalWritebackPlan",
  "host_abi_plan",
  "host_abi_gate",
  "module_integrity_gate",
  "HostABIExecutionGate",
  "host_abi_execution_gate_ready",
  "host_abi_enforcement_ready",
  "module_integrity_gate_ready",
  "wasm.host_abi.execution_gate",
  "wasm.module.integrity_gate",
  "module_integrity_gate",
  "ModuleIntegrityGate",
  "integrity_gate_ready",
  "blocked_module_sha256_mismatch",
  "execution_gate_ready",
  "allows_execution",
  "blocked_until_host_abi_enforcement",
  "blocked_module_sha256_mismatch",
  "integrity_gate_ready",
  "remote_install_plan",
  "approval_gate_plan",
  "module_integrity_gate_ready",
  "remote_install_plan_ready",
  "signature_verification_plan_ready",
  "signature_verify_ready",
  "signature_verification",
  "WASMPluginSignatureVerificationPlan",
  "wasm.remote_install.signature_verification_plan",
  "approval_gate_plan_ready",
  "approval_queue_plan_ready",
  "approval_queue_entry",
  "approval_decision_plan_ready",
  "approval_decision_ready",
  "applies_approval_decision",
  "approval_decision_plan",
  "approval_writeback_plan_ready",
  "approval_writeback_ready",
  "approval_writeback_plan",
  "installer_blocked_until_writeback",
  "WASMPluginApprovalQueueEntryPlan",
  "remote-install-plan.json",
  "approval-gate-plan.json",
  "approval-queue-entry.json",
  "approval-decision-plan.json",
  "approval-writeback-plan.json",
  "approval-queue-store.json",
  "approval-queue-record.json",
  "installer-continuation-plan.json",
  "installer-download-handoff-plan.json",
  "installer-registration-handoff-plan.json",
  "installer-audit-handoff-plan.json",
  "signature-verification.json",
  "downloads",
  "enforcement_ready",
  "writes_files",
  "host_abi_gate",
  "execution_gate_ready",
  "allows_execution",
  "remoteInstallPlan",
  "remoteInstallApprovalPlan",
  "remoteInstallApprovalDecisionPlan",
  "remoteInstallApprovalWritebackPlan",
  "WASM Plugin request failed",
]) {
  if (!sdk.includes(token)) fail(`TypeScript WASM Plugin SDK slice missing token: ${token}`);
}

const pkg = JSON.parse(readRepoFile("sdk/typescript/package.json") || "{}");
if (pkg.exports?.["./wasm-plugin"]?.import !== "./src/wasm-plugin.ts") fail("yunque-client/wasm-plugin subpath export is missing or drifted");

const monolithicApi = readRepoFile("heroui-web/src/lib/api.ts");
for (const token of ["wasmPluginStatus:", "createWASMPlugin:", "wasmPluginExecute:", "wasmPluginEvidence:"]) {
  if (monolithicApi.includes(token)) fail(`monolithic api.ts should not expose WASM Plugin method: ${token}`);
}

if (failures.length) {
  console.error("WASM Plugin Pack SDK manifest check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`WASM Plugin Pack SDK manifest ok: ${routeSpecs.size} route specs, ${manifest.sdkImport} import`);
