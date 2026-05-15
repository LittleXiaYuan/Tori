import { fetcher } from "./api-core";

export const WASM_PLUGIN_REMOTE_INSTALL_PLAN_ARTIFACTS = [
  "remote-install-plan.json",
  "approval-gate-plan.json",
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
  approval_gate_plan_ready: boolean;
  approval_gate_ready: boolean;
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
  public_key_id?: string;
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
  public_key_id?: string;
  entrypoint?: string;
  requested_by?: string;
  reason?: string;
  risk_tier?: string;
  approvers?: string[];
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
  public_key_id?: string;
  manifest_artifact: string;
  package_artifact: string;
  cache_key: string;
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
  checks: WASMPluginRemoteInstallCheck[];
  approvers?: string[];
  artifacts: string[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  remote_install_plan_summary: WASMPluginRemoteInstallPlan;
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
  installPlugin(
    input: WASMPluginInstallInput,
  ): Promise<{
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
  evidence(
    slug: string,
  ): Promise<{
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
    approval_gate_plan: WASMPluginRemoteInstallApprovalPlan;
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
        approval_gate_plan: WASMPluginRemoteInstallApprovalPlan;
        sandbox?: Record<string, unknown>;
      }>(`/v1/wasm-plugin/evidence/${enc(slug)}`),
  };
}
