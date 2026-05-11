/**
 * Lightweight IDE SDK slice.
 *
 * This keeps IDE status and code-review integration usable without importing
 * the full generated OpenAPI SDK:
 *
 *   import { createIDEClient } from "yunque-client/ide";
 */

export type IDEReviewMode = "full" | "diff" | "quick" | string;

export type IDEStatusResponse = {
  version?: string;
  connected?: boolean;
  capabilities?: string[];
  skills_count?: number;
  server_time?: string;
  uptime_sec?: number;
  [key: string]: unknown;
};

export type IDEReviewRequest = {
  file_path?: string;
  content?: string;
  diff?: string;
  language?: string;
  mode?: IDEReviewMode;
};

export type IDEReviewIssue = {
  line?: number;
  severity?: "error" | "warning" | "info" | string;
  message?: string;
  suggestion?: string;
  [key: string]: unknown;
};

export type IDEReviewResponse = {
  summary?: string;
  issues?: IDEReviewIssue[];
  score?: number;
  improvements?: string[];
  [key: string]: unknown;
};

export type IDEClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class IDEClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `IDE request failed with HTTP ${status}`);
    this.name = "IDEClientError";
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

export class IDEClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: IDEClientOptions) {
    if (!options.baseUrl) throw new Error("IDEClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("IDEClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<IDEStatusResponse> {
    return this.request<IDEStatusResponse>("GET", "/v1/ide/status");
  }

  review(body: IDEReviewRequest): Promise<IDEReviewResponse> {
    return this.request<IDEReviewResponse>("POST", "/v1/ide/review", body);
  }

  reviewDiff(body: Omit<IDEReviewRequest, "mode" | "content"> & { diff: string }): Promise<IDEReviewResponse> {
    return this.review({ ...body, mode: "diff" });
  }

  reviewQuick(body: Omit<IDEReviewRequest, "mode" | "diff"> & { content: string }): Promise<IDEReviewResponse> {
    return this.review({ ...body, mode: "quick" });
  }

  reviewFull(body: Omit<IDEReviewRequest, "mode" | "diff"> & { content: string }): Promise<IDEReviewResponse> {
    return this.review({ ...body, mode: "full" });
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
    if (!response.ok) throw new IDEClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createIDEClient(options: IDEClientOptions): IDEClient {
  return new IDEClient(options);
}
