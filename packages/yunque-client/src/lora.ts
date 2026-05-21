/**
 * Lightweight LoRA and evolution SDK slice.
 *
 * This keeps local-brain training lifecycle and evolution state usable without
 * importing the full generated OpenAPI SDK:
 *
 *   import { createLoRAClient } from "yunque-client/lora";
 */

export type LoRASchedulerState = {
  running?: boolean;
  last_run_at?: string;
  next_run_at?: string;
  status?: string;
  [key: string]: unknown;
};

export type LoRAStatusResponse = {
  scheduler?: LoRASchedulerState | Record<string, unknown>;
  active_model?: string;
  rolling_success_rate?: number;
  [key: string]: unknown;
};

export type LoRAHistoryResponse = {
  records: unknown[];
  count: number;
};

export type LoRASummaryResponse = {
  summary: unknown;
};

export type LoRAPreviewQuery = {
  tenant_id?: string;
};

export type LoRAPreviewResponse = {
  preview: {
    ready?: boolean;
    tenant_id?: string;
    sample_count?: number;
    min_samples?: number;
    reason?: string;
    [key: string]: unknown;
  };
};

export type TriggerLoRARequest = {
  tenant_id?: string;
};

export type TriggerLoRAResponse = {
  status?: string;
  tenant_id?: string;
  [key: string]: unknown;
};

export type LoRARollbackResponse = {
  status?: string;
  [key: string]: unknown;
};

export type LoRAEvolutionResponse = {
  state: unknown;
};

export type LoRAConfig = {
  min_samples?: number;
  min_interval?: string;
  eval_min_score?: number;
  max_adapters?: number;
  base_model?: string;
  training_data_dir?: string;
  adapter_dir?: string;
  ab_test_duration?: string;
  [key: string]: unknown;
};

export type LoRAConfigResponse = {
  config: LoRAConfig | Record<string, unknown>;
  status?: string;
};

export type LoRAClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class LoRAClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `LoRA request failed with HTTP ${status}`);
    this.name = "LoRAClientError";
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

function setOptionalQuery(url: URL, key: string, value: string | undefined): void {
  if (!value) return;
  url.searchParams.set(key, value);
}

export class LoRAClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: LoRAClientOptions) {
    if (!options.baseUrl) throw new Error("LoRAClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("LoRAClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<LoRAStatusResponse> {
    return this.request<LoRAStatusResponse>("GET", "/v1/lora/status");
  }

  history(): Promise<LoRAHistoryResponse> {
    return this.request<LoRAHistoryResponse>("GET", "/v1/lora/history");
  }

  summary(): Promise<LoRASummaryResponse> {
    return this.request<LoRASummaryResponse>("GET", "/v1/lora/summary");
  }

  preview(query?: LoRAPreviewQuery): Promise<LoRAPreviewResponse> {
    return this.request<LoRAPreviewResponse>("GET", "/v1/lora/preview", undefined, query);
  }

  trigger(body?: TriggerLoRARequest): Promise<TriggerLoRAResponse> {
    return this.request<TriggerLoRAResponse>("POST", "/v1/lora/trigger", body ?? {});
  }

  rollback(): Promise<LoRARollbackResponse> {
    return this.request<LoRARollbackResponse>("POST", "/v1/lora/rollback", {});
  }

  evolution(): Promise<LoRAEvolutionResponse> {
    return this.request<LoRAEvolutionResponse>("GET", "/v1/lora/evolution");
  }

  config(): Promise<LoRAConfigResponse> {
    return this.request<LoRAConfigResponse>("GET", "/v1/lora/config");
  }

  updateConfig(config: LoRAConfig, method: "PUT" | "PATCH" = "PUT"): Promise<LoRAConfigResponse> {
    return this.request<LoRAConfigResponse>(method, "/v1/lora/config", config);
  }

  private async request<T>(
    method: "GET" | "POST" | "PUT" | "PATCH",
    path: string,
    body?: unknown,
    query?: LoRAPreviewQuery,
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    setOptionalQuery(url, "tenant_id", query?.tenant_id);

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
    if (!response.ok) throw new LoRAClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createLoRAClient(options: LoRAClientOptions): LoRAClient {
  return new LoRAClient(options);
}
