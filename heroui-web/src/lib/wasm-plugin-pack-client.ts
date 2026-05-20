import { fetcher } from "./api-core";

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

export interface WASMPluginPermissionPolicy {
  ledger_kv: boolean;
  memory_search: boolean;
  http_fetch: boolean;
  allowed_hosts?: string[];
  env_allowlist?: string[];
  max_memory_mb: number;
  timeout_seconds: number;
}

export interface WASMPluginSummary {
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
}

export interface WASMPlugin extends WASMPluginSummary {
  tags?: string[];
}

export interface WASMPluginStatus {
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
}

export interface WASMPluginPermissionCheck {
  name: string;
  allowed: boolean;
  reason?: string;
}

export interface WASMPluginHostABIFunctionPlan {
  name: string;
  category: string;
  permission: string;
  enabled: boolean;
  enforcement_ready: boolean;
  writes_files: boolean;
  network_access: boolean;
  constraints?: string[];
  reason?: string;
}

export interface WASMPluginHostABISummary {
  function_count: number;
  enabled_count: number;
  ledger_kv: boolean;
  memory_search: boolean;
  http_fetch: boolean;
  env_get: boolean;
  allowed_host_count: number;
  env_allowlist_count: number;
}

export interface WASMPluginHostABIResourceLimits {
  max_memory_mb: number;
  timeout_seconds: number;
  allowed_hosts: string[];
  env_allowlist: string[];
}

export interface WASMPluginHostABIPlan {
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
}

export interface WASMPluginHostABIExecutionGate {
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
}

export interface WASMPluginModuleIntegrityGate {
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
}

export interface WASMPluginRemoteInstallPlanInput {
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
}

export interface WASMPluginRemoteInstallApprovalPlanInput {
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
}

export interface WASMPluginRemoteInstallApprovalDecisionPlanInput extends WASMPluginRemoteInstallApprovalPlanInput {
  request_id?: string;
  request_key?: string;
  decision: "approved" | "denied" | "expired";
  decision_by?: string;
  decision_reason?: string;
}

export type WASMPluginRemoteInstallApprovalWritebackPlanInput =
  WASMPluginRemoteInstallApprovalDecisionPlanInput;

export type WASMPluginRemoteInstallApprovalQueueWritebackInput =
  WASMPluginRemoteInstallApprovalWritebackPlanInput;

export interface WASMPluginRemoteInstallInstallerContinuationPlanInput {
  request_id?: string;
  request_key?: string;
  slug?: string;
}

export interface WASMPluginRemoteInstallInstallerDownloadWritebackInput {
  request_id?: string;
  request_key?: string;
  slug?: string;
  approved: boolean;
  approved_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
}

export interface WASMPluginRemoteInstallPluginPlan {
  slug: string;
  name: string;
  version: string;
  runtime: string;
  entrypoint: string;
  module_path: string;
  capabilities?: string[];
  tags?: string[];
}

export interface WASMPluginRemoteInstallPackagePlan {
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
}

export interface WASMPluginSignatureVerificationPlan {
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
}

export interface WASMPluginRemoteInstallCheck {
  name: string;
  required: boolean;
  ready: boolean;
  reason?: string;
}

export interface WASMPluginRemoteInstallPlan {
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
}

export interface WASMPluginRemoteInstallApprovalPlan {
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
}

export interface WASMPluginApprovalQueueEntryPlan {
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
}

export interface WASMPluginApprovalDecisionPlan {
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
}

export interface WASMPluginApprovalWritebackPlan {
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
}

export interface WASMPluginRemoteInstallApprovalDecisionPlan {
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
}

export interface WASMPluginRemoteInstallApprovalWritebackPlan {
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
}

export interface WASMPluginApprovalQueueStoreSummary {
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
}

export interface WASMPluginApprovalQueueRecord {
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
}

export interface WASMPluginRemoteInstallApprovalQueueWriteback {
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
}

export interface WASMPluginInstallerContinuationPlan {
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
}

export interface WASMPluginRemoteInstallInstallerContinuationPlan {
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
}

export interface WASMPluginInstallerDownloadRecord {
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
}

export interface WASMPluginRemoteInstallInstallerDownloadWriteback {
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
}

export interface WASMPluginExecuteResult {
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
}

export interface WASMPluginInstallInput {
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
}

export interface WASMPluginPackClient {
  status(): Promise<WASMPluginStatus>;
  plugins(): Promise<{ plugins: WASMPluginSummary[]; count: number }>;
  installPlugin(input: WASMPluginInstallInput): Promise<{
    plugin: WASMPlugin;
    status: string;
    plan?: WASMPluginPermissionCheck[];
    host_abi_plan?: WASMPluginHostABIPlan;
  }>;
  plugin(slug: string): Promise<{ plugin: WASMPlugin }>;
  load(slug: string): Promise<{ plugin: WASMPlugin; status: string }>;
  unload(slug: string): Promise<{ plugin: WASMPlugin; status: string }>;
  execute(input: {
    slug: string;
    input?: string;
    entrypoint?: string;
    dry_run?: boolean;
  }): Promise<{ result: WASMPluginExecuteResult }>;
  remoteInstallPlan(
    input: WASMPluginRemoteInstallPlanInput,
  ): Promise<{ plan: WASMPluginRemoteInstallPlan }>;
  remoteInstallApprovalPlan(
    input: WASMPluginRemoteInstallApprovalPlanInput,
  ): Promise<{ plan: WASMPluginRemoteInstallApprovalPlan }>;
  remoteInstallApprovalDecisionPlan(
    input: WASMPluginRemoteInstallApprovalDecisionPlanInput,
  ): Promise<{ plan: WASMPluginRemoteInstallApprovalDecisionPlan }>;
  remoteInstallApprovalWritebackPlan(
    input: WASMPluginRemoteInstallApprovalWritebackPlanInput,
  ): Promise<{ plan: WASMPluginRemoteInstallApprovalWritebackPlan }>;
  remoteInstallApprovalQueueWriteback(
    input: WASMPluginRemoteInstallApprovalQueueWritebackInput,
  ): Promise<{ writeback: WASMPluginRemoteInstallApprovalQueueWriteback }>;
  remoteInstallInstallerContinuationPlan(
    input: WASMPluginRemoteInstallInstallerContinuationPlanInput,
  ): Promise<{ plan: WASMPluginRemoteInstallInstallerContinuationPlan }>;
  remoteInstallInstallerDownloadWriteback(
    input: WASMPluginRemoteInstallInstallerDownloadWritebackInput,
  ): Promise<{ writeback: WASMPluginRemoteInstallInstallerDownloadWriteback }>;
  evidence(slug: string): Promise<{
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
  }>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

export function createWASMPluginPackClient(): WASMPluginPackClient {
  return {
    status: () => fetcher<WASMPluginStatus>("/v1/wasm-plugin/status"),
    plugins: () =>
      fetcher<{ plugins: WASMPluginSummary[]; count: number }>(
        "/v1/wasm-plugin/plugins",
      ),
    installPlugin: (input) =>
      fetcher<{
        plugin: WASMPlugin;
        status: string;
        plan?: WASMPluginPermissionCheck[];
        host_abi_plan?: WASMPluginHostABIPlan;
      }>("/v1/wasm-plugin/plugins", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    plugin: (slug) =>
      fetcher<{ plugin: WASMPlugin }>(`/v1/wasm-plugin/plugins/${enc(slug)}`),
    load: (slug) =>
      fetcher<{ plugin: WASMPlugin; status: string }>(
        "/v1/wasm-plugin/plugins/load",
        {
          method: "POST",
          body: JSON.stringify({ slug }),
        },
      ),
    unload: (slug) =>
      fetcher<{ plugin: WASMPlugin; status: string }>(
        "/v1/wasm-plugin/plugins/unload",
        {
          method: "POST",
          body: JSON.stringify({ slug }),
        },
      ),
    execute: (input) =>
      fetcher<{ result: WASMPluginExecuteResult }>("/v1/wasm-plugin/execute", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    remoteInstallPlan: (input) =>
      fetcher<{ plan: WASMPluginRemoteInstallPlan }>(
        "/v1/wasm-plugin/remote-install/plan",
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      ),
    remoteInstallApprovalPlan: (input) =>
      fetcher<{ plan: WASMPluginRemoteInstallApprovalPlan }>(
        "/v1/wasm-plugin/remote-install/approval/plan",
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      ),
    remoteInstallApprovalDecisionPlan: (input) =>
      fetcher<{ plan: WASMPluginRemoteInstallApprovalDecisionPlan }>(
        "/v1/wasm-plugin/remote-install/approval/decision/plan",
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      ),
    remoteInstallApprovalWritebackPlan: (input) =>
      fetcher<{ plan: WASMPluginRemoteInstallApprovalWritebackPlan }>(
        "/v1/wasm-plugin/remote-install/approval/writeback/plan",
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      ),
    remoteInstallApprovalQueueWriteback: (input) =>
      fetcher<{ writeback: WASMPluginRemoteInstallApprovalQueueWriteback }>(
        "/v1/wasm-plugin/remote-install/approval/queue/writeback",
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      ),
    remoteInstallInstallerContinuationPlan: (input) =>
      fetcher<{ plan: WASMPluginRemoteInstallInstallerContinuationPlan }>(
        "/v1/wasm-plugin/remote-install/installer/continuation/plan",
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      ),
    remoteInstallInstallerDownloadWriteback: (input) =>
      fetcher<{ writeback: WASMPluginRemoteInstallInstallerDownloadWriteback }>(
        "/v1/wasm-plugin/remote-install/installer/download/writeback",
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      ),
    evidence: (slug) =>
      fetcher<{
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
      }>(`/v1/wasm-plugin/evidence/${enc(slug)}`),
  };
}
