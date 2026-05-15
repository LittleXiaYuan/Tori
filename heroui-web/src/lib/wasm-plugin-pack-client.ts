import { fetcher } from "./api-core";

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
  installPlugin(input: WASMPluginInstallInput): Promise<{ plugin: WASMPlugin; status: string; plan?: WASMPluginPermissionCheck[]; host_abi_plan?: WASMPluginHostABIPlan }>;
  plugin(slug: string): Promise<{ plugin: WASMPlugin }>;
  load(slug: string): Promise<{ plugin: WASMPlugin; status: string }>;
  unload(slug: string): Promise<{ plugin: WASMPlugin; status: string }>;
  execute(input: { slug: string; input?: string; entrypoint?: string; dry_run?: boolean }): Promise<{ result: WASMPluginExecuteResult }>;
  evidence(slug: string): Promise<{ pack_id: string; exported_at: string; format: string; files: string[]; plugin: WASMPlugin; plan: WASMPluginPermissionCheck[]; host_abi_plan: WASMPluginHostABIPlan; sandbox?: Record<string, unknown> }>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

export function createWASMPluginPackClient(): WASMPluginPackClient {
  return {
    status: () => fetcher<WASMPluginStatus>("/v1/wasm-plugin/status"),
    plugins: () => fetcher<{ plugins: WASMPluginSummary[]; count: number }>("/v1/wasm-plugin/plugins"),
    installPlugin: (input) =>
      fetcher<{ plugin: WASMPlugin; status: string; plan?: WASMPluginPermissionCheck[]; host_abi_plan?: WASMPluginHostABIPlan }>("/v1/wasm-plugin/plugins", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    plugin: (slug) => fetcher<{ plugin: WASMPlugin }>(`/v1/wasm-plugin/plugins/${enc(slug)}`),
    load: (slug) =>
      fetcher<{ plugin: WASMPlugin; status: string }>("/v1/wasm-plugin/plugins/load", {
        method: "POST",
        body: JSON.stringify({ slug }),
      }),
    unload: (slug) =>
      fetcher<{ plugin: WASMPlugin; status: string }>("/v1/wasm-plugin/plugins/unload", {
        method: "POST",
        body: JSON.stringify({ slug }),
      }),
    execute: (input) =>
      fetcher<{ result: WASMPluginExecuteResult }>("/v1/wasm-plugin/execute", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    evidence: (slug) =>
      fetcher<{ pack_id: string; exported_at: string; format: string; files: string[]; plugin: WASMPlugin; plan: WASMPluginPermissionCheck[]; host_abi_plan: WASMPluginHostABIPlan; sandbox?: Record<string, unknown> }>(`/v1/wasm-plugin/evidence/${enc(slug)}`),
  };
}
