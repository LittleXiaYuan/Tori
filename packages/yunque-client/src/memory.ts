/**
 * Lightweight Memory SDK slice.
 *
 * Import this subpath when an external shell only needs memory read/write
 * operations and should not pull in the full generated OpenAPI client:
 *
 *   import { createMemoryClient } from "yunque-client/memory";
 */

export type MemoryLayer = "short" | "mid" | "long" | "all" | string;

export type MemoryItem = {
  key?: string;
  value?: string;
  content?: string;
  source?: string;
  layer?: MemoryLayer;
  score?: number;
  tags?: string[];
  [key: string]: unknown;
};

export type MemoryStats = {
  short?: number;
  mid?: number;
  long?: number;
  [key: string]: unknown;
};

export type MemorySearchRequest = {
  query: string;
  limit?: number;
  layer?: MemoryLayer;
};

export type MemorySearchResponse = {
  results: MemoryItem[];
  count?: number;
};

export type MemoryAddRequest = {
  key?: string;
  value?: string;
  content?: string;
  layer?: Exclude<MemoryLayer, "all">;
  source?: string;
  tags?: string[];
};

export type MemoryAddResponse = {
  status?: string;
  [key: string]: unknown;
};

export type MemoryCompactRequest = {
  target_count?: number;
  decay_days?: number;
};

export type MemoryCompactResponse = Record<string, unknown>;

export type MemoryClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class MemoryClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Memory request failed with HTTP ${status}`);
    this.name = "MemoryClientError";
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

export class MemoryClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: MemoryClientOptions) {
    if (!options.baseUrl) throw new Error("MemoryClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("MemoryClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  stats(): Promise<MemoryStats> {
    return this.request<MemoryStats>("GET", "/v1/memory/stats");
  }

  search(body: MemorySearchRequest): Promise<MemorySearchResponse> {
    return this.request<MemorySearchResponse>("POST", "/v1/memory/search", body);
  }

  add(body: MemoryAddRequest): Promise<MemoryAddResponse> {
    const normalized = { ...body, value: body.value ?? body.content };
    return this.request<MemoryAddResponse>("POST", "/v1/memory/add", normalized);
  }

  compact(body: MemoryCompactRequest = {}): Promise<MemoryCompactResponse> {
    return this.request<MemoryCompactResponse>("POST", "/v1/memory/compact", body);
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

    const response = await this.fetchImpl(`${this.baseUrl}${path}`, init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new MemoryClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createMemoryClient(options: MemoryClientOptions): MemoryClient {
  return new MemoryClient(options);
}
