/**
 * Lightweight Cost and Usage SDK slice.
 *
 * This keeps cost dashboards, budget controls, usage and quota checks usable
 * without importing the full generated OpenAPI SDK:
 *
 *   import { createCostClient } from "yunque-client/cost";
 */

export type CostSummaryResponse = {
  summary?: unknown;
  today_cost?: number;
  month_cost?: number;
  status?: string;
  [key: string]: unknown;
};

export type CostBudget = {
  daily_limit_usd?: number;
  monthly_limit_usd?: number;
  per_task_limit_usd?: number;
  per_model_limit_usd?: Record<string, number>;
  [key: string]: unknown;
};

export type SetCostBudgetResponse = {
  ok?: boolean;
  [key: string]: unknown;
};

export type CostBreakdownResponse = {
  by_channel?: Record<string, number>;
  by_tier?: Record<string, number>;
  by_runner_type?: Record<string, number>;
  by_provider?: Record<string, number>;
  [key: string]: unknown;
};

export type CostHistoryQuery = {
  page?: number;
  limit?: number;
  task_id?: string;
  model?: string;
  channel?: string;
  runner_type?: string;
  provider_id?: string;
};

export type CostAlertsResponse = {
  alerts?: unknown[];
  today_cost?: number;
  month_cost?: number;
  [key: string]: unknown;
};

export type UsageRecord = {
  tenant_id: string;
  chat_calls?: number;
  stream_calls?: number;
  skill_execs?: number;
  tokens_used?: number;
  last_call?: string;
  [key: string]: unknown;
};

export type QuotaConfig = {
  max_chat_calls?: number;
  max_tokens_per_day?: number;
};

export type SetQuotaRequest = {
  tenant_id?: string;
  quota: QuotaConfig;
};

export type SetQuotaResponse = {
  status?: string;
  [key: string]: unknown;
};

export type CostClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class CostClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Cost request failed with HTTP ${status}`);
    this.name = "CostClientError";
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

function setOptionalQuery(url: URL, key: string, value: string | number | undefined): void {
  if (value === undefined || value === "") return;
  url.searchParams.set(key, String(value));
}

export class CostClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: CostClientOptions) {
    if (!options.baseUrl) throw new Error("CostClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("CostClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  summary(): Promise<CostSummaryResponse> {
    return this.request<CostSummaryResponse>("GET", "/v1/cost/summary");
  }

  setBudget(budget: CostBudget): Promise<SetCostBudgetResponse> {
    return this.request<SetCostBudgetResponse>("POST", "/v1/cost/budget", budget);
  }

  task(id: string): Promise<unknown> {
    return this.request<unknown>("GET", "/v1/cost/task", undefined, { id });
  }

  taskTimeline(id: string): Promise<unknown> {
    return this.request<unknown>("GET", "/v1/cost/task/timeline", undefined, { id });
  }

  breakdown(): Promise<CostBreakdownResponse> {
    return this.request<CostBreakdownResponse>("GET", "/v1/cost/breakdown");
  }

  history(query?: CostHistoryQuery): Promise<unknown> {
    return this.request<unknown>("GET", "/v1/cost/history", undefined, query);
  }

  alerts(): Promise<CostAlertsResponse> {
    return this.request<CostAlertsResponse>("GET", "/v1/cost/alerts");
  }

  usage(): Promise<UsageRecord> {
    return this.request<UsageRecord>("GET", "/v1/usage");
  }

  setQuota(body: SetQuotaRequest): Promise<SetQuotaResponse> {
    return this.request<SetQuotaResponse>("POST", "/v1/quota", body);
  }

  private async request<T>(
    method: "GET" | "POST",
    path: string,
    body?: unknown,
    query?: Record<string, string | number | undefined>,
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    for (const [key, value] of Object.entries(query ?? {})) setOptionalQuery(url, key, value);

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
    if (!response.ok) throw new CostClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createCostClient(options: CostClientOptions): CostClient {
  return new CostClient(options);
}
