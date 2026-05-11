/**
 * Lightweight Heartbeat SDK slice.
 *
 * This keeps proactive lifecycle heartbeat controls usable without importing
 * the full generated OpenAPI SDK:
 *
 *   import { createHeartbeatClient } from "yunque-client/heartbeat";
 */

export type HeartbeatStatusResponse = {
  running?: boolean;
  [key: string]: unknown;
};

export type UpdateHeartbeatRequest = {
  enabled?: boolean;
  interval_minutes?: number;
};

export type UpdateHeartbeatResponse = {
  status?: string;
  [key: string]: unknown;
};

export type HeartbeatLogEntry = {
  id?: string;
  timestamp?: string;
  summary?: string;
  status?: string;
  [key: string]: unknown;
};

export type HeartbeatLogsQuery = {
  limit?: number;
};

export type HeartbeatClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class HeartbeatClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Heartbeat request failed with HTTP ${status}`);
    this.name = "HeartbeatClientError";
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

function setOptionalQuery(url: URL, key: string, value: number | undefined): void {
  if (value === undefined) return;
  url.searchParams.set(key, String(value));
}

export class HeartbeatClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: HeartbeatClientOptions) {
    if (!options.baseUrl) throw new Error("HeartbeatClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("HeartbeatClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<HeartbeatStatusResponse> {
    return this.request<HeartbeatStatusResponse>("GET", "/v1/heartbeat");
  }

  update(body: UpdateHeartbeatRequest): Promise<UpdateHeartbeatResponse> {
    return this.request<UpdateHeartbeatResponse>("PUT", "/v1/heartbeat", body);
  }

  trigger(): Promise<HeartbeatLogEntry> {
    return this.request<HeartbeatLogEntry>("POST", "/v1/heartbeat/trigger", {});
  }

  logs(query?: HeartbeatLogsQuery): Promise<HeartbeatLogEntry[]> {
    return this.request<HeartbeatLogEntry[]>("GET", "/v1/heartbeat/logs", undefined, query);
  }

  private async request<T>(
    method: "GET" | "PUT" | "POST",
    path: string,
    body?: unknown,
    query?: HeartbeatLogsQuery,
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    setOptionalQuery(url, "limit", query?.limit);

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
    if (!response.ok) throw new HeartbeatClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createHeartbeatClient(options: HeartbeatClientOptions): HeartbeatClient {
  return new HeartbeatClient(options);
}
