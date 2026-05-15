/**
 * Lightweight WASM Plugin Pack SDK slice.
 *
 * This keeps WASM plugin registration, lifecycle control, dry-run execution,
 * Host ABI plan previews, and evidence export usable without importing the full
 * generated OpenAPI SDK:
 *
 *   import { createWASMPluginClient } from "yunque-client/wasm-plugin";
 */

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

function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers {
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
    if (!fetchImpl) throw new Error("WASMPluginClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<WASMPluginStatusResponse> {
    return this.request<WASMPluginStatusResponse>("GET", "/v1/wasm-plugin/status");
  }

  plugins(): Promise<WASMPluginListResponse> {
    return this.request<WASMPluginListResponse>("GET", "/v1/wasm-plugin/plugins");
  }

  installPlugin(input: WASMPluginInstallRequest): Promise<WASMPluginInstallResponse> {
    return this.request<WASMPluginInstallResponse>("POST", "/v1/wasm-plugin/plugins", input);
  }

  plugin(slug: string): Promise<WASMPluginResponse> {
    return this.request<WASMPluginResponse>("GET", `/v1/wasm-plugin/plugins/${enc(slug)}`);
  }

  load(slug: string): Promise<WASMPluginLifecycleResponse> {
    return this.request<WASMPluginLifecycleResponse>("POST", "/v1/wasm-plugin/plugins/load", { slug });
  }

  unload(slug: string): Promise<WASMPluginLifecycleResponse> {
    return this.request<WASMPluginLifecycleResponse>("POST", "/v1/wasm-plugin/plugins/unload", { slug });
  }

  execute(input: WASMPluginExecuteRequest): Promise<WASMPluginExecuteResponse> {
    return this.request<WASMPluginExecuteResponse>("POST", "/v1/wasm-plugin/execute", input);
  }

  evidence(slug: string): Promise<WASMPluginEvidenceResponse> {
    return this.request<WASMPluginEvidenceResponse>("GET", `/v1/wasm-plugin/evidence/${enc(slug)}`);
  }

  private async request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new WASMPluginClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createWASMPluginClient(options: WASMPluginClientOptions): WASMPluginClient {
  return new WASMPluginClient(options);
}
