/**
 * Lightweight WASM Plugin Pack SDK slice.
 *
 * This keeps WASM plugin registration, lifecycle control, dry-run execution,
 * Host ABI plan/gate previews, approval queue writeback persistence, and
 * evidence export usable without importing the full generated OpenAPI SDK:
 *
 *   import { createWASMPluginClient } from "yunque-client/wasm-plugin";
 */

export const WASM_PLUGIN_REMOTE_INSTALL_PLAN_ARTIFACTS = [
  "remote-install-plan.json",
  "approval-gate-plan.json",
  "approval-queue-entry.json",
  "approval-decision-plan.json",
  "approval-writeback-plan.json",
  "approval-queue-store.json",
  "approval-queue-record.json",
  "installer-continuation-plan.json",
  "installer-download-handoff-plan.json",
  "installer-download-record.json",
  "installer-package-cache.tgz",
  "installer-registration-handoff-plan.json",
  "installer-audit-handoff-plan.json",
  "signature-verification.json",
] as const;

export const WASM_PLUGIN_HOST_ABI_EXECUTION_GATE_CAPABILITY =
  "wasm.host_abi.execution_gate";

export const WASM_PLUGIN_HOST_ABI_BLOCKED_STATUS =
  "blocked_until_host_abi_enforcement";

export const WASM_PLUGIN_MODULE_INTEGRITY_GATE_CAPABILITY =
  "wasm.module.integrity_gate";

export const WASM_PLUGIN_MODULE_INTEGRITY_BLOCKED_STATUS =
  "blocked_module_sha256_mismatch";

export const WASM_PLUGIN_REMOTE_SIGNATURE_VERIFICATION_CAPABILITY =
  "wasm.remote_install.signature_verification_plan";

export const WASM_PLUGIN_REMOTE_SIGNATURE_BLOCKED_STATUS =
  "blocked_until_signature_verifier";

export const WASM_PLUGIN_APPROVAL_QUEUE_ENTRY_ARTIFACT =
  "approval-queue-entry.json";

export const WASM_PLUGIN_APPROVAL_QUEUE_BLOCKED_STATUS =
  "blocked_until_approval_queue";

export const WASM_PLUGIN_APPROVAL_DECISION_PLAN_ARTIFACT =
  "approval-decision-plan.json";

export const WASM_PLUGIN_APPROVAL_WRITEBACK_PLAN_ARTIFACT =
  "approval-writeback-plan.json";

export const WASM_PLUGIN_APPROVAL_QUEUE_STORE_ARTIFACT =
  "approval-queue-store.json";

export const WASM_PLUGIN_APPROVAL_QUEUE_WRITEBACK_CAPABILITY =
  "wasm.remote_install.approval_queue_writeback";

export const WASM_PLUGIN_INSTALLER_CONTINUATION_PLAN_CAPABILITY =
  "wasm.remote_install.installer_continuation_plan";

export const WASM_PLUGIN_INSTALLER_CONTINUATION_PLAN_ARTIFACT =
  "installer-continuation-plan.json";

export const WASM_PLUGIN_INSTALLER_DOWNLOAD_WRITEBACK_CAPABILITY =
  "wasm.remote_install.installer_download_writeback";

export const WASM_PLUGIN_INSTALLER_DOWNLOAD_RECORD_ARTIFACT =
  "installer-download-record.json";

export const WASM_PLUGIN_INSTALLER_PACKAGE_CACHE_ARTIFACT =
  "installer-package-cache.tgz";

export type WASMPluginPermissionPolicy = {
  ledger_kv: boolean;
  memory_search: boolean;
  http_fetch: boolean;
  allowed_hosts?: string[];
  env_allowlist?: string[];
  max_memory_mb: number;
  timeout_seconds: number;
};

export type WASMPluginSummary = {
  slug: string;
  name: string;
  version: string;
  description?: string;
  runtime: string;
  entrypoint: string;
  module_path: string;
  sha256?: string;
  status: string;
  loaded_at?: string;
  exec_count: number;
  last_exec_at?: string;
  permissions: WASMPluginPermissionPolicy;
  capabilities?: string[];
};

export type WASMPlugin = WASMPluginSummary & {
  tags?: string[];
};

export type WASMPluginStatusResponse = {
  pack_id: string;
  stage: string;
  runtime_ready: boolean;
  abi_plan_ready: boolean;
  abi_ready: boolean;
  host_abi_execution_gate_ready: boolean;
  host_abi_enforcement_ready: boolean;
  module_integrity_gate_ready: boolean;
  remote_install_plan_ready: boolean;
  remote_install_ready: boolean;
  signature_verification_plan_ready: boolean;
  signature_verify_ready: boolean;
  approval_gate_plan_ready: boolean;
  approval_gate_ready: boolean;
  approval_queue_plan_ready: boolean;
  approval_queue_ready: boolean;
  approval_decision_plan_ready: boolean;
  approval_decision_ready: boolean;
  approval_writeback_plan_ready: boolean;
  approval_writeback_ready: boolean;
  approval_queue_store_ready: boolean;
  installer_continuation_plan_ready: boolean;
  installer_download_writeback_ready: boolean;
  installer_ready: boolean;
  installer_blocked_until_signature_verify: boolean;
  installer_blocked_until_installer_wiring: boolean;
  approval_queue_store?: WASMPluginApprovalQueueStoreSummary;
  plugin_count: number;
  loaded_count: number;
  plugin_dir?: string;
  store_dir?: string;
  sandbox?: Record<string, unknown>;
  capabilities: string[];
  notes?: string[];
};

export type WASMPluginListResponse = {
  plugins: WASMPluginSummary[];
  count: number;
};

export type WASMPluginResponse = {
  plugin: WASMPlugin;
};

export type WASMPluginInstallRequest = {
  slug: string;
  name: string;
  version?: string;
  description?: string;
  module_path?: string;
  entrypoint?: string;
  permissions?: Partial<WASMPluginPermissionPolicy>;
  capabilities?: string[];
  tags?: string[];
  dry_run?: boolean;
};

export type WASMPluginInstallResponse = WASMPluginResponse & {
  status: string;
  plan?: WASMPluginPermissionCheck[];
  host_abi_plan?: WASMPluginHostABIPlan;
};

export type WASMPluginLifecycleResponse = WASMPluginResponse & {
  status: string;
};

export type WASMPluginExecuteRequest = {
  slug: string;
  input?: string;
  entrypoint?: string;
  dry_run?: boolean;
};

export type WASMPluginPermissionCheck = {
  name: string;
  allowed: boolean;
  reason?: string;
};

export type WASMPluginHostABIFunctionPlan = {
  name: string;
  category: string;
  permission: string;
  enabled: boolean;
  enforcement_ready: boolean;
  writes_files: boolean;
  network_access: boolean;
  constraints?: string[];
  reason?: string;
};

export type WASMPluginHostABISummary = {
  function_count: number;
  enabled_count: number;
  ledger_kv: boolean;
  memory_search: boolean;
  http_fetch: boolean;
  env_get: boolean;
  allowed_host_count: number;
  env_allowlist_count: number;
};

export type WASMPluginHostABIResourceLimits = {
  max_memory_mb: number;
  timeout_seconds: number;
  allowed_hosts: string[];
  env_allowlist: string[];
};

export type WASMPluginHostABIPlan = {
  plan_ready: boolean;
  ready: boolean;
  status: string;
  enforcement_ready: boolean;
  writes_files: boolean;
  network_access: boolean;
  functions: WASMPluginHostABIFunctionPlan[];
  summary: WASMPluginHostABISummary;
  resource_limits: WASMPluginHostABIResourceLimits;
  labels: string[];
  notes?: string[];
};

export type WASMPluginHostABIExecutionGate = {
  execution_gate_ready: boolean;
  allows_execution: boolean;
  blocked: boolean;
  status: string;
  enforcement_ready: boolean;
  writes_files: boolean;
  network_access: boolean;
  requested_functions?: string[];
  allowed_functions?: string[];
  blocked_functions?: string[];
  reason?: string;
  labels: string[];
  notes?: string[];
};

export type WASMPluginModuleIntegrityGate = {
  integrity_gate_ready: boolean;
  allows_execution: boolean;
  blocked: boolean;
  status: string;
  expected_sha256?: string;
  actual_sha256?: string;
  module_path: string;
  writes_files: boolean;
  network_access: boolean;
  reason?: string;
  labels: string[];
  notes?: string[];
};

export type WASMPluginRemoteInstallPlanRequest = {
  slug?: string;
  name?: string;
  version?: string;
  package_url: string;
  manifest_url?: string;
  module_path?: string;
  sha256?: string;
  signature?: string;
  signature_algorithm?: string;
  public_key_id?: string;
  trust_root?: string;
  entrypoint?: string;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
  capabilities?: string[];
  tags?: string[];
};

export type WASMPluginRemoteInstallApprovalPlanRequest = {
  slug?: string;
  name?: string;
  version?: string;
  package_url: string;
  manifest_url?: string;
  module_path?: string;
  sha256?: string;
  signature?: string;
  signature_algorithm?: string;
  public_key_id?: string;
  trust_root?: string;
  entrypoint?: string;
  requested_by?: string;
  reason?: string;
  risk_tier?: string;
  approvers?: string[];
  metadata?: Record<string, string>;
};

export type WASMPluginRemoteInstallApprovalDecisionPlanRequest =
  WASMPluginRemoteInstallApprovalPlanRequest & {
    request_id?: string;
    request_key?: string;
    decision: "approved" | "denied" | "expired";
    decision_by?: string;
    decision_reason?: string;
  };

export type WASMPluginRemoteInstallApprovalWritebackPlanRequest =
  WASMPluginRemoteInstallApprovalDecisionPlanRequest;

export type WASMPluginRemoteInstallApprovalQueueWritebackRequest =
  WASMPluginRemoteInstallApprovalWritebackPlanRequest;

export type WASMPluginRemoteInstallInstallerContinuationPlanRequest = {
  request_id?: string;
  request_key?: string;
  slug?: string;
};

export type WASMPluginRemoteInstallInstallerDownloadWritebackRequest = {
  request_id?: string;
  request_key?: string;
  slug?: string;
  approved: boolean;
  approved_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
};

export type WASMPluginRemoteInstallPluginPlan = {
  slug: string;
  name: string;
  version: string;
  runtime: string;
  entrypoint: string;
  module_path: string;
  capabilities?: string[];
  tags?: string[];
};

export type WASMPluginRemoteInstallPackagePlan = {
  manifest_url: string;
  package_url: string;
  expected_sha256?: string;
  signature?: string;
  signature_algorithm?: string;
  public_key_id?: string;
  trust_root?: string;
  manifest_artifact: string;
  package_artifact: string;
  cache_key: string;
};

export type WASMPluginSignatureVerificationPlan = {
  pack_id: string;
  generated_at: string;
  signature_verification_plan_ready: boolean;
  verification_gate_ready: boolean;
  signature_verify_ready: boolean;
  required: boolean;
  allows_install: boolean;
  blocked: boolean;
  status: string;
  algorithm: string;
  signature_provided: boolean;
  public_key_id_present: boolean;
  public_key_id?: string;
  trust_root?: string;
  expected_sha256?: string;
  expected_sha256_format_valid: boolean;
  canonical_payload_sha256: string;
  artifact: string;
  downloads: boolean;
  writes_files: boolean;
  network_access: boolean;
  checks: WASMPluginRemoteInstallCheck[];
  labels: string[];
  notes?: string[];
};

export type WASMPluginRemoteInstallCheck = {
  name: string;
  required: boolean;
  ready: boolean;
  reason?: string;
};

export type WASMPluginRemoteInstallPlan = {
  pack_id: string;
  generated_at: string;
  status: string;
  remote_install_plan_ready: boolean;
  remote_install_ready: boolean;
  download_ready: boolean;
  signature_verify_ready: boolean;
  downloads: boolean;
  installs_plugin: boolean;
  writes_files: boolean;
  network_access: boolean;
  plugin: WASMPluginRemoteInstallPluginPlan;
  package: WASMPluginRemoteInstallPackagePlan;
  signature_verification: WASMPluginSignatureVerificationPlan;
  checks: WASMPluginRemoteInstallCheck[];
  artifacts: string[];
  actions: string[];
  labels: string[];
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginRemoteInstallPlanResponse = {
  plan: WASMPluginRemoteInstallPlan;
};

export type WASMPluginRemoteInstallApprovalPlan = {
  pack_id: string;
  generated_at: string;
  status: string;
  approval_gate_plan_ready: boolean;
  approval_gate_ready: boolean;
  requires_approval: boolean;
  approval_queue_plan_ready: boolean;
  approval_queue_ready: boolean;
  writes_approval_queue: boolean;
  writes_files: boolean;
  downloads: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  decision: string;
  risk_tier: string;
  requested_by?: string;
  reason?: string;
  plugin: WASMPluginRemoteInstallPluginPlan;
  package: WASMPluginRemoteInstallPackagePlan;
  signature_verification: WASMPluginSignatureVerificationPlan;
  approval_queue_entry: WASMPluginApprovalQueueEntryPlan;
  checks: WASMPluginRemoteInstallCheck[];
  approvers?: string[];
  artifacts: string[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  remote_install_plan_summary: WASMPluginRemoteInstallPlan;
  notes?: string[];
};

export type WASMPluginApprovalQueueEntryPlan = {
  pack_id: string;
  generated_at: string;
  approval_queue_plan_ready: boolean;
  approval_queue_ready: boolean;
  writes_approval_queue: boolean;
  requires_approval: boolean;
  status: string;
  queue_name: string;
  request_id: string;
  request_key: string;
  decision: string;
  decision_states: string[];
  risk_tier: string;
  requested_by?: string;
  reason?: string;
  approvers?: string[];
  required_fields: string[];
  plugin: WASMPluginRemoteInstallPluginPlan;
  package: WASMPluginRemoteInstallPackagePlan;
  signature_gate_status: string;
  canonical_payload_sha256: string;
  artifact: string;
  downloads: boolean;
  writes_files: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  checks: WASMPluginRemoteInstallCheck[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginApprovalDecisionPlan = {
  pack_id: string;
  generated_at: string;
  approval_decision_plan_ready: boolean;
  approval_decision_ready: boolean;
  applies_approval_decision: boolean;
  approval_queue_plan_ready: boolean;
  approval_queue_ready: boolean;
  writes_approval_queue: boolean;
  requires_approval: boolean;
  status: string;
  queue_name: string;
  request_id: string;
  request_key: string;
  decision_key: string;
  decision: "approved" | "denied" | "expired";
  decision_by: string;
  decision_reason?: string;
  would_allow_installer_continue: boolean;
  blocks_installer: boolean;
  required_fields: string[];
  plugin: WASMPluginRemoteInstallPluginPlan;
  package: WASMPluginRemoteInstallPackagePlan;
  signature_gate_status: string;
  canonical_payload_sha256: string;
  artifact: string;
  downloads: boolean;
  writes_files: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  checks: WASMPluginRemoteInstallCheck[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginApprovalWritebackPlan = {
  pack_id: string;
  generated_at: string;
  approval_writeback_plan_ready: boolean;
  approval_writeback_ready: boolean;
  approval_queue_plan_ready: boolean;
  approval_queue_ready: boolean;
  writes_approval_queue: boolean;
  approval_decision_plan_ready: boolean;
  approval_decision_ready: boolean;
  applies_approval_decision: boolean;
  requires_approval: boolean;
  status: string;
  queue_name: string;
  writeback_store: string;
  queue_operation: string;
  decision_operation: string;
  request_id: string;
  request_key: string;
  decision_key: string;
  decision: "approved" | "denied" | "expired";
  decision_by: string;
  decision_reason?: string;
  would_allow_installer_continue: boolean;
  blocks_installer: boolean;
  installer_blocked_until_writeback: boolean;
  required_fields: string[];
  plugin: WASMPluginRemoteInstallPluginPlan;
  package: WASMPluginRemoteInstallPackagePlan;
  signature_gate_status: string;
  canonical_payload_sha256: string;
  queue_artifact: string;
  decision_artifact: string;
  artifact: string;
  downloads: boolean;
  writes_files: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  checks: WASMPluginRemoteInstallCheck[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginRemoteInstallApprovalDecisionPlan = {
  pack_id: string;
  generated_at: string;
  status: string;
  approval_decision_plan_ready: boolean;
  approval_decision_ready: boolean;
  applies_approval_decision: boolean;
  approval_queue_plan_ready: boolean;
  approval_queue_ready: boolean;
  writes_approval_queue: boolean;
  writes_files: boolean;
  downloads: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  decision: "approved" | "denied" | "expired";
  decision_by: string;
  decision_reason?: string;
  request_id: string;
  request_key: string;
  would_allow_installer_continue: boolean;
  blocks_installer: boolean;
  plugin: WASMPluginRemoteInstallPluginPlan;
  package: WASMPluginRemoteInstallPackagePlan;
  signature_verification: WASMPluginSignatureVerificationPlan;
  approval_queue_entry: WASMPluginApprovalQueueEntryPlan;
  decision_plan: WASMPluginApprovalDecisionPlan;
  checks: WASMPluginRemoteInstallCheck[];
  artifacts: string[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  approval_gate_plan_summary: WASMPluginRemoteInstallApprovalPlan;
  notes?: string[];
};

export type WASMPluginRemoteInstallApprovalWritebackPlan = {
  pack_id: string;
  generated_at: string;
  status: string;
  approval_writeback_plan_ready: boolean;
  approval_writeback_ready: boolean;
  approval_queue_plan_ready: boolean;
  approval_queue_ready: boolean;
  writes_approval_queue: boolean;
  approval_decision_plan_ready: boolean;
  approval_decision_ready: boolean;
  applies_approval_decision: boolean;
  writes_files: boolean;
  downloads: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  decision: "approved" | "denied" | "expired";
  decision_by: string;
  decision_reason?: string;
  request_id: string;
  request_key: string;
  decision_key: string;
  would_allow_installer_continue: boolean;
  blocks_installer: boolean;
  installer_blocked_until_writeback: boolean;
  plugin: WASMPluginRemoteInstallPluginPlan;
  package: WASMPluginRemoteInstallPackagePlan;
  signature_verification: WASMPluginSignatureVerificationPlan;
  approval_queue_entry: WASMPluginApprovalQueueEntryPlan;
  decision_plan: WASMPluginApprovalDecisionPlan;
  writeback_plan: WASMPluginApprovalWritebackPlan;
  checks: WASMPluginRemoteInstallCheck[];
  artifacts: string[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  remote_install_plan_summary: WASMPluginRemoteInstallPlan;
  approval_gate_plan_summary: WASMPluginRemoteInstallApprovalPlan;
  notes?: string[];
};

export type WASMPluginApprovalQueueStoreSummary = {
  pack_id: string;
  queue_name: string;
  store: string;
  store_ready: boolean;
  record_count: number;
  artifact: string;
  writes_files: boolean;
  writes_approval_queue: boolean;
  writes_approval_queue_store: boolean;
  installer_writeback_ready: boolean;
  notes?: string[];
};

export type WASMPluginApprovalQueueRecord = {
  pack_id: string;
  queue_name: string;
  request_id: string;
  request_key: string;
  decision_key: string;
  decision: "approved" | "denied" | "expired";
  decision_by: string;
  decision_reason?: string;
  risk_tier: string;
  requested_by?: string;
  reason?: string;
  status: string;
  created_at: string;
  updated_at: string;
  approval_queue_store_ready: boolean;
  writes_approval_queue: boolean;
  writes_approval_queue_store: boolean;
  approval_writeback_ready: boolean;
  approval_queue_ready: boolean;
  approval_decision_ready: boolean;
  applies_approval_decision: boolean;
  installer_blocked_until_writeback: boolean;
  installer_blocked_until_installer_wiring: boolean;
  plugin: WASMPluginRemoteInstallPluginPlan;
  package: WASMPluginRemoteInstallPackagePlan;
  signature_gate_status: string;
  canonical_payload_sha256: string;
  approval_queue_entry: WASMPluginApprovalQueueEntryPlan;
  decision_plan: WASMPluginApprovalDecisionPlan;
  writeback_plan: WASMPluginApprovalWritebackPlan;
  store_artifact: string;
  downloads: boolean;
  writes_files: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  artifacts: string[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginRemoteInstallApprovalQueueWriteback = {
  pack_id: string;
  generated_at: string;
  status: string;
  approval_queue_store_ready: boolean;
  approval_writeback_plan_ready: boolean;
  approval_writeback_ready: boolean;
  approval_queue_ready: boolean;
  writes_approval_queue: boolean;
  writes_approval_queue_store: boolean;
  approval_decision_ready: boolean;
  applies_approval_decision: boolean;
  writes_files: boolean;
  downloads: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  decision: "approved" | "denied" | "expired";
  decision_by: string;
  decision_reason?: string;
  request_id: string;
  request_key: string;
  decision_key: string;
  installer_blocked_until_writeback: boolean;
  installer_blocked_until_installer_wiring: boolean;
  approval_queue_record: WASMPluginApprovalQueueRecord;
  approval_queue_store: WASMPluginApprovalQueueStoreSummary;
  plan_summary: WASMPluginRemoteInstallApprovalWritebackPlan;
  checks: WASMPluginRemoteInstallCheck[];
  artifacts: string[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginInstallerContinuationPlan = {
  pack_id: string;
  generated_at: string;
  installer_continuation_plan_ready: boolean;
  consumes_approval_queue_store: boolean;
  approval_queue_store_ready: boolean;
  approval_queue_record_found: boolean;
  approval_queue_ready: boolean;
  approval_decision_ready: boolean;
  approval_approved: boolean;
  would_allow_installer_continue: boolean;
  blocks_installer: boolean;
  installer_ready: boolean;
  installer_blocked_until_installer_wiring: boolean;
  status: string;
  queue_name: string;
  request_id?: string;
  request_key?: string;
  decision_key?: string;
  decision?: "approved" | "denied" | "expired";
  required_fields: string[];
  plugin?: WASMPluginRemoteInstallPluginPlan;
  package?: WASMPluginRemoteInstallPackagePlan;
  signature_gate_status?: string;
  canonical_payload_sha256?: string;
  queue_store_artifact: string;
  queue_record_artifact: string;
  download_handoff_artifact: string;
  registration_handoff_artifact: string;
  audit_handoff_artifact: string;
  artifact: string;
  remote_install_ready: boolean;
  download_ready: boolean;
  signature_verify_ready: boolean;
  downloads: boolean;
  writes_files: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  checks: WASMPluginRemoteInstallCheck[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginRemoteInstallInstallerContinuationPlan = {
  pack_id: string;
  generated_at: string;
  status: string;
  installer_continuation_plan_ready: boolean;
  consumes_approval_queue_store: boolean;
  approval_queue_store_ready: boolean;
  approval_queue_record_found: boolean;
  approval_queue_ready: boolean;
  approval_decision_ready: boolean;
  approval_writeback_ready: boolean;
  applies_approval_decision: boolean;
  approval_approved: boolean;
  would_allow_installer_continue: boolean;
  blocks_installer: boolean;
  installer_ready: boolean;
  installer_blocked_until_installer_wiring: boolean;
  remote_install_ready: boolean;
  download_ready: boolean;
  signature_verify_ready: boolean;
  downloads: boolean;
  writes_files: boolean;
  network_access: boolean;
  installs_plugin: boolean;
  decision?: "approved" | "denied" | "expired";
  decision_by?: string;
  decision_reason?: string;
  request_id?: string;
  request_key?: string;
  decision_key?: string;
  plugin?: WASMPluginRemoteInstallPluginPlan;
  package?: WASMPluginRemoteInstallPackagePlan;
  signature_gate_status?: string;
  canonical_payload_sha256?: string;
  approval_queue_record?: WASMPluginApprovalQueueRecord;
  approval_queue_store: WASMPluginApprovalQueueStoreSummary;
  installer_plan: WASMPluginInstallerContinuationPlan;
  checks: WASMPluginRemoteInstallCheck[];
  artifacts: string[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginInstallerDownloadRecord = {
  pack_id: string;
  generated_at: string;
  status: string;
  installer_download_writeback_ready: boolean;
  approval_queue_store_ready: boolean;
  approval_queue_record_found: boolean;
  approval_approved: boolean;
  download_ready: boolean;
  downloads: boolean;
  network_access: boolean;
  writes_files: boolean;
  writes_package_cache: boolean;
  signature_verify_ready: boolean;
  remote_install_ready: boolean;
  installs_plugin: boolean;
  installer_ready: boolean;
  installer_blocked_until_signature_verify: boolean;
  installer_blocked_until_registration: boolean;
  queue_name?: string;
  request_id?: string;
  request_key?: string;
  decision_key?: string;
  package_url?: string;
  artifact: string;
  cache_artifact: string;
  cache_path?: string;
  expected_sha256?: string;
  actual_sha256?: string;
  sha256_match: boolean;
  size_bytes: number;
  plugin?: WASMPluginRemoteInstallPluginPlan;
  package?: WASMPluginRemoteInstallPackagePlan;
  checks: WASMPluginRemoteInstallCheck[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginRemoteInstallInstallerDownloadWriteback = {
  pack_id: string;
  generated_at: string;
  status: string;
  installer_download_writeback_ready: boolean;
  consumes_approval_queue_store: boolean;
  consumes_installer_continuation_plan: boolean;
  approval_queue_store_ready: boolean;
  approval_queue_record_found: boolean;
  approval_approved: boolean;
  would_allow_installer_continue: boolean;
  approval_required: boolean;
  download_ready: boolean;
  downloads: boolean;
  network_access: boolean;
  writes_files: boolean;
  writes_package_cache: boolean;
  signature_verify_ready: boolean;
  remote_install_ready: boolean;
  installs_plugin: boolean;
  installer_ready: boolean;
  installer_blocked_until_signature_verify: boolean;
  installer_blocked_until_registration: boolean;
  request_id?: string;
  request_key?: string;
  decision_key?: string;
  decision?: "approved" | "denied" | "expired";
  approved_by?: string;
  reason?: string;
  plugin?: WASMPluginRemoteInstallPluginPlan;
  package?: WASMPluginRemoteInstallPackagePlan;
  approval_queue_record?: WASMPluginApprovalQueueRecord;
  approval_queue_store: WASMPluginApprovalQueueStoreSummary;
  installer_continuation_plan: WASMPluginInstallerContinuationPlan;
  download_record: WASMPluginInstallerDownloadRecord;
  checks: WASMPluginRemoteInstallCheck[];
  artifacts: string[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type WASMPluginRemoteInstallApprovalDecisionPlanResponse = {
  plan: WASMPluginRemoteInstallApprovalDecisionPlan;
};

export type WASMPluginRemoteInstallApprovalWritebackPlanResponse = {
  plan: WASMPluginRemoteInstallApprovalWritebackPlan;
};

export type WASMPluginRemoteInstallApprovalQueueWritebackResponse = {
  writeback: WASMPluginRemoteInstallApprovalQueueWriteback;
};

export type WASMPluginRemoteInstallInstallerContinuationPlanResponse = {
  plan: WASMPluginRemoteInstallInstallerContinuationPlan;
};

export type WASMPluginRemoteInstallInstallerDownloadWritebackResponse = {
  writeback: WASMPluginRemoteInstallInstallerDownloadWriteback;
};

export type WASMPluginRemoteInstallApprovalPlanResponse = {
  plan: WASMPluginRemoteInstallApprovalPlan;
};

export type WASMPluginExecuteResult = {
  slug: string;
  dry_run: boolean;
  entrypoint: string;
  success: boolean;
  exit_code: number;
  stdout?: string;
  stderr?: string;
  duration?: string;
  mem_used_bytes?: number;
  exports?: string[];
  kv_writes?: Record<string, string>;
  plan?: WASMPluginPermissionCheck[];
  host_abi_plan: WASMPluginHostABIPlan;
  host_abi_gate: WASMPluginHostABIExecutionGate;
  module_integrity_gate: WASMPluginModuleIntegrityGate;
  notes?: string[];
};

export type WASMPluginExecuteResponse = {
  result: WASMPluginExecuteResult;
};

export type WASMPluginEvidenceResponse = {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  plugin: WASMPlugin;
  plan: WASMPluginPermissionCheck[];
  host_abi_plan: WASMPluginHostABIPlan;
  host_abi_gate: WASMPluginHostABIExecutionGate;
  module_integrity_gate: WASMPluginModuleIntegrityGate;
  remote_install_plan: WASMPluginRemoteInstallPlan;
  signature_verification: WASMPluginSignatureVerificationPlan;
  approval_gate_plan: WASMPluginRemoteInstallApprovalPlan;
  approval_decision_plan: WASMPluginRemoteInstallApprovalDecisionPlan;
  approval_writeback_plan: WASMPluginRemoteInstallApprovalWritebackPlan;
  approval_queue_store: WASMPluginApprovalQueueStoreSummary;
  approval_queue_record: WASMPluginApprovalQueueRecord;
  installer_continuation_plan: WASMPluginInstallerContinuationPlan;
  installer_download_record: WASMPluginInstallerDownloadRecord;
  sandbox?: Record<string, unknown>;
};

export type WASMPluginClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class WASMPluginClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `WASM Plugin request failed with HTTP ${status}`);
    this.name = "WASMPluginClientError";
    this.status = status;
    this.body = body;
  }
}

function trimBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

function mergeHeaders(
  base: HeadersInit | undefined,
  extra?: HeadersInit,
): Headers {
  const headers = new Headers(base);
  if (!extra) return headers;
  new Headers(extra).forEach((value, key) => headers.set(key, value));
  return headers;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function messageFromErrorBody(body: unknown): string | undefined {
  if (typeof body === "string" && body.trim()) return body.trim();
  if (!isRecord(body)) return undefined;
  for (const key of ["message", "detail", "error", "reason"]) {
    const value = body[key];
    if (typeof value === "string" && value.trim()) return value;
    if (key === "error" && isRecord(value)) {
      const nested = messageFromErrorBody(value);
      if (nested) return nested;
    }
  }
  return undefined;
}

async function parseResponse(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

export class WASMPluginClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: WASMPluginClientOptions) {
    if (!options.baseUrl) throw new Error("WASMPluginClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl)
      throw new Error("WASMPluginClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<WASMPluginStatusResponse> {
    return this.request<WASMPluginStatusResponse>(
      "GET",
      "/v1/wasm-plugin/status",
    );
  }

  plugins(): Promise<WASMPluginListResponse> {
    return this.request<WASMPluginListResponse>(
      "GET",
      "/v1/wasm-plugin/plugins",
    );
  }

  installPlugin(
    input: WASMPluginInstallRequest,
  ): Promise<WASMPluginInstallResponse> {
    return this.request<WASMPluginInstallResponse>(
      "POST",
      "/v1/wasm-plugin/plugins",
      input,
    );
  }

  plugin(slug: string): Promise<WASMPluginResponse> {
    return this.request<WASMPluginResponse>(
      "GET",
      `/v1/wasm-plugin/plugins/${enc(slug)}`,
    );
  }

  load(slug: string): Promise<WASMPluginLifecycleResponse> {
    return this.request<WASMPluginLifecycleResponse>(
      "POST",
      "/v1/wasm-plugin/plugins/load",
      { slug },
    );
  }

  unload(slug: string): Promise<WASMPluginLifecycleResponse> {
    return this.request<WASMPluginLifecycleResponse>(
      "POST",
      "/v1/wasm-plugin/plugins/unload",
      { slug },
    );
  }

  execute(input: WASMPluginExecuteRequest): Promise<WASMPluginExecuteResponse> {
    return this.request<WASMPluginExecuteResponse>(
      "POST",
      "/v1/wasm-plugin/execute",
      input,
    );
  }

  remoteInstallPlan(
    input: WASMPluginRemoteInstallPlanRequest,
  ): Promise<WASMPluginRemoteInstallPlanResponse> {
    return this.request<WASMPluginRemoteInstallPlanResponse>(
      "POST",
      "/v1/wasm-plugin/remote-install/plan",
      input,
    );
  }

  remoteInstallApprovalPlan(
    input: WASMPluginRemoteInstallApprovalPlanRequest,
  ): Promise<WASMPluginRemoteInstallApprovalPlanResponse> {
    return this.request<WASMPluginRemoteInstallApprovalPlanResponse>(
      "POST",
      "/v1/wasm-plugin/remote-install/approval/plan",
      input,
    );
  }

  remoteInstallApprovalDecisionPlan(
    input: WASMPluginRemoteInstallApprovalDecisionPlanRequest,
  ): Promise<WASMPluginRemoteInstallApprovalDecisionPlanResponse> {
    return this.request<WASMPluginRemoteInstallApprovalDecisionPlanResponse>(
      "POST",
      "/v1/wasm-plugin/remote-install/approval/decision/plan",
      input,
    );
  }

  remoteInstallApprovalWritebackPlan(
    input: WASMPluginRemoteInstallApprovalWritebackPlanRequest,
  ): Promise<WASMPluginRemoteInstallApprovalWritebackPlanResponse> {
    return this.request<WASMPluginRemoteInstallApprovalWritebackPlanResponse>(
      "POST",
      "/v1/wasm-plugin/remote-install/approval/writeback/plan",
      input,
    );
  }

  remoteInstallApprovalQueueWriteback(
    input: WASMPluginRemoteInstallApprovalQueueWritebackRequest,
  ): Promise<WASMPluginRemoteInstallApprovalQueueWritebackResponse> {
    return this.request<WASMPluginRemoteInstallApprovalQueueWritebackResponse>(
      "POST",
      "/v1/wasm-plugin/remote-install/approval/queue/writeback",
      input,
    );
  }

  remoteInstallInstallerContinuationPlan(
    input: WASMPluginRemoteInstallInstallerContinuationPlanRequest,
  ): Promise<WASMPluginRemoteInstallInstallerContinuationPlanResponse> {
    return this.request<WASMPluginRemoteInstallInstallerContinuationPlanResponse>(
      "POST",
      "/v1/wasm-plugin/remote-install/installer/continuation/plan",
      input,
    );
  }

  remoteInstallInstallerDownloadWriteback(
    input: WASMPluginRemoteInstallInstallerDownloadWritebackRequest,
  ): Promise<WASMPluginRemoteInstallInstallerDownloadWritebackResponse> {
    return this.request<WASMPluginRemoteInstallInstallerDownloadWritebackResponse>(
      "POST",
      "/v1/wasm-plugin/remote-install/installer/download/writeback",
      input,
    );
  }

  evidence(slug: string): Promise<WASMPluginEvidenceResponse> {
    return this.request<WASMPluginEvidenceResponse>(
      "GET",
      `/v1/wasm-plugin/evidence/${enc(slug)}`,
    );
  }

  private async request<T>(
    method: "GET" | "POST",
    path: string,
    body?: unknown,
  ): Promise<T> {
    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization"))
      headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key"))
      headers.set("X-API-Key", this.apiKey);

    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(
      new URL(`${this.baseUrl}${path}`),
      init,
    );
    const parsed = await parseResponse(response);
    if (!response.ok)
      throw new WASMPluginClientError(
        response.status,
        parsed,
        messageFromErrorBody(parsed),
      );
    return parsed as T;
  }
}

export function createWASMPluginClient(
  options: WASMPluginClientOptions,
): WASMPluginClient {
  return new WASMPluginClient(options);
}
