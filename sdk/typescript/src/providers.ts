/**
 * Lightweight LLM Providers/Models SDK slice.
 *
 * This keeps model configuration, provider testing, and runtime provider
 * switching usable without importing the full generated OpenAPI SDK:
 *
 *   import { createProvidersClient } from "yunque-client/providers";
 */

export type ProviderMode = "local" | "tori" | "hybrid" | string;

export type ModelEntry = {
  id: string;
  model_id: string;
  name?: string;
  type?: string;
  client_type?: string;
  base_url?: string;
  supports_reasoning?: boolean;
  dimensions?: number;
  [key: string]: unknown;
};

export type ProviderConfig = {
  id?: string;
  preset_id?: string;
  base_url?: string;
  api_key?: string;
  model?: string;
  name?: string;
  tier?: string;
  enabled?: boolean;
  type?: string;
  source?: string;
  display_name?: string;
  [key: string]: unknown;
};

export type ProviderSummary = ProviderConfig & {
  id: string;
  model?: string;
  base_url?: string;
};

export type ProviderPreset = {
  id?: string;
  name?: string;
  base_url?: string;
  models?: Array<Record<string, unknown>>;
  [key: string]: unknown;
};

export type ModelsResponse = {
  models: ModelEntry[];
};

export type ProvidersResponse = {
  providers: ProviderSummary[];
  mode?: ProviderMode;
  warning?: string;
};

export type ProviderModeResponse = {
  mode: ProviderMode;
  bound?: boolean;
  ok?: boolean;
};

export type ProviderPresetsResponse = {
  presets: ProviderPreset[];
};

export type ProviderTestResponse = {
  success?: boolean;
  ok?: boolean;
  error?: string;
  provider?: Record<string, unknown>;
  [key: string]: unknown;
};

export type ProviderActionResponse = {
  ok?: boolean;
  status?: string;
  provider_id?: string;
  model?: string;
  action?: string;
  reset_count?: number;
  [key: string]: unknown;
};

export type ProviderSessionOverrideRequest = {
  session_id: string;
  provider_id?: string;
};

export type LocalDiscoverRequest = {
  base_url: string;
};

export type LocalRegisterRequest = {
  base_url: string;
  model?: string;
  tier?: string;
  backend?: string;
};

export type ToriDiscoverResponse = {
  ok?: boolean;
  models?: Array<Record<string, unknown>>;
  registered?: number;
  error?: string;
  [key: string]: unknown;
};

export type ExecProviderResponse = {
  exec_provider?: string;
  available_providers?: string[];
  ok?: boolean;
};

export type ProvidersClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class ProvidersClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Providers request failed with HTTP ${status}`);
    this.name = "ProvidersClientError";
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
  if (!isRecord(body)) return undefined;
  for (const key of ["message", "detail", "error", "reason"]) {
    const value = body[key];
    if (typeof value === "string" && value.trim()) return value;
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

function setOptionalQuery(url: URL, key: string, value: string | boolean | undefined): void {
  if (value === undefined || value === "") return;
  url.searchParams.set(key, String(value));
}

export class ProvidersClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: ProvidersClientOptions) {
    if (!options.baseUrl) throw new Error("ProvidersClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("ProvidersClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  listModels(): Promise<ModelsResponse> {
    return this.request<ModelsResponse>("GET", "/v1/models");
  }

  addModel(model: ModelEntry): Promise<ModelEntry> {
    return this.request<ModelEntry>("POST", "/v1/models", model);
  }

  deleteModel(id: string): Promise<ProviderActionResponse> {
    return this.request<ProviderActionResponse>("DELETE", "/v1/models", undefined, { id });
  }

  listProviders(): Promise<ProvidersResponse> {
    return this.request<ProvidersResponse>("GET", "/api/providers");
  }

  testProvider(id: string): Promise<ProviderTestResponse> {
    return this.request<ProviderTestResponse>("POST", "/api/providers/test", { id });
  }

  enableProvider(id: string): Promise<ProviderActionResponse> {
    return this.request<ProviderActionResponse>("POST", "/api/providers/enable", { id });
  }

  disableProvider(id: string): Promise<ProviderActionResponse> {
    return this.request<ProviderActionResponse>("POST", "/api/providers/disable", { id });
  }

  switchModel(id: string, model: string): Promise<ProviderActionResponse> {
    return this.request<ProviderActionResponse>("POST", "/api/providers/switch-model", { id, model });
  }

  setSessionProvider(body: ProviderSessionOverrideRequest): Promise<ProviderActionResponse> {
    return this.request<ProviderActionResponse>("POST", "/api/providers/session", body);
  }

  clearSessionProvider(sessionId: string): Promise<ProviderActionResponse> {
    return this.setSessionProvider({ session_id: sessionId, provider_id: "" });
  }

  getMode(): Promise<ProviderModeResponse> {
    return this.request<ProviderModeResponse>("GET", "/api/providers/mode");
  }

  setMode(mode: ProviderMode): Promise<ProviderModeResponse> {
    return this.request<ProviderModeResponse>("POST", "/api/providers/mode", { mode });
  }

  presets(): Promise<ProviderPresetsResponse> {
    return this.request<ProviderPresetsResponse>("GET", "/api/providers/presets");
  }

  registerProvider(body: ProviderConfig): Promise<ProviderActionResponse> {
    return this.request<ProviderActionResponse>("POST", "/api/providers/register", body);
  }

  deleteProvider(id: string): Promise<ProviderActionResponse> {
    return this.request<ProviderActionResponse>("POST", "/api/providers/delete", { id });
  }

  discoverLocal(body: LocalDiscoverRequest): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("POST", "/api/providers/local/discover", body);
  }

  registerLocal(body: LocalRegisterRequest): Promise<ProviderActionResponse> {
    return this.request<ProviderActionResponse>("POST", "/api/providers/local/register", body);
  }

  discoverTori(options?: { autoRegister?: boolean }): Promise<ToriDiscoverResponse> {
    return this.request<ToriDiscoverResponse>("POST", "/api/providers/tori/discover", undefined, {
      auto_register: options?.autoRegister,
    });
  }

  getExecProvider(): Promise<ExecProviderResponse> {
    return this.request<ExecProviderResponse>("GET", "/api/providers/exec");
  }

  setExecProvider(providerId: string): Promise<ExecProviderResponse> {
    return this.request<ExecProviderResponse>("POST", "/api/providers/exec", { provider_id: providerId });
  }

  resetBreakers(): Promise<ProviderActionResponse> {
    return this.request<ProviderActionResponse>("POST", "/api/breaker/reset");
  }

  private async request<T>(
    method: "DELETE" | "GET" | "POST",
    path: string,
    body?: unknown,
    query?: Record<string, string | boolean | undefined>,
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    if (query) {
      for (const [key, value] of Object.entries(query)) {
        setOptionalQuery(url, key, value);
      }
    }

    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(url, init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new ProvidersClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createProvidersClient(options: ProvidersClientOptions): ProvidersClient {
  return new ProvidersClient(options);
}
