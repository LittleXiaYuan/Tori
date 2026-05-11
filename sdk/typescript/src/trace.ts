/**
 * Lightweight Trace SDK slice.
 *
 * This keeps execution/audit trace inspection usable without importing the
 * full generated OpenAPI SDK:
 *
 *   import { createTraceClient } from "yunque-client/trace";
 */

export type TraceEvent = {
  id?: string;
  trace_id?: string;
  task_id?: string;
  type?: string;
  message?: string;
  ts?: string;
  timestamp?: string;
  data?: Record<string, unknown>;
  [key: string]: unknown;
};

export type TraceQueryOptions = {
  raw?: boolean;
};

export type TraceRecentOptions = TraceQueryOptions & {
  limit?: number;
};

export type TraceEventsResponse = {
  count: number;
  raw?: boolean;
  events: TraceEvent[];
};

export type TraceByIDResponse = TraceEventsResponse & {
  trace_id: string;
};

export type TraceByTaskResponse = TraceEventsResponse & {
  task_id: string;
};

export type TraceClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class TraceClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Trace request failed with HTTP ${status}`);
    this.name = "TraceClientError";
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

function setOptionalQuery(url: URL, key: string, value: string | number | boolean | undefined): void {
  if (value === undefined || value === "") return;
  url.searchParams.set(key, String(value));
}

export class TraceClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: TraceClientOptions) {
    if (!options.baseUrl) throw new Error("TraceClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("TraceClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  recent(options?: TraceRecentOptions): Promise<TraceEventsResponse> {
    return this.request<TraceEventsResponse>("/v1/trace/recent", {
      limit: options?.limit,
      raw: options?.raw,
    });
  }

  byTraceId(traceId: string, options?: TraceQueryOptions): Promise<TraceByIDResponse> {
    return this.request<TraceByIDResponse>(`/v1/trace/${encodeURIComponent(traceId)}`, { raw: options?.raw });
  }

  byTaskId(taskId: string, options?: TraceQueryOptions): Promise<TraceByTaskResponse> {
    return this.request<TraceByTaskResponse>(`/v1/trace/task/${encodeURIComponent(taskId)}`, { raw: options?.raw });
  }

  private async request<T>(path: string, query?: Record<string, string | number | boolean | undefined>): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    if (query) {
      for (const [key, value] of Object.entries(query)) {
        setOptionalQuery(url, key, value);
      }
    }

    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const response = await this.fetchImpl(url, { method: "GET", headers });
    const parsed = await parseResponse(response);
    if (!response.ok) throw new TraceClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createTraceClient(options: TraceClientOptions): TraceClient {
  return new TraceClient(options);
}
