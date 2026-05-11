/**
 * Lightweight Audit SDK slice.
 *
 * This keeps Merkle audit-chain and task audit-trail inspection usable without
 * importing the full generated OpenAPI SDK:
 *
 *   import { createAuditClient } from "yunque-client/audit";
 */

export type AuditTailQuery = {
  n?: number;
  type?: string;
  actor?: string;
};

export type AuditRecord = {
  id?: string;
  type?: string;
  actor?: string;
  action?: string;
  timestamp?: string;
  hash?: string;
  prev_hash?: string;
  [key: string]: unknown;
};

export type AuditTailResponse = {
  records: AuditRecord[];
  count: number;
  error?: string;
};

export type AuditVerifyResponse = {
  valid?: boolean;
  checked?: number;
  chain_length?: number;
  broken_at?: number;
  tampered_at?: number;
  error?: string;
  [key: string]: unknown;
};

export type AuditStatsResponse = {
  error?: string;
  [key: string]: unknown;
};

export type AuditTrailQuery = {
  date?: string;
  type?: string;
};

export type AuditTrailEntry = {
  operation?: string;
  result?: string;
  actor?: string;
  timestamp?: string;
  [key: string]: unknown;
};

export type AuditTrailResponse = {
  entries: AuditTrailEntry[];
  count: number;
};

export type AuditClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class AuditClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Audit request failed with HTTP ${status}`);
    this.name = "AuditClientError";
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

function setOptionalQuery(url: URL, key: string, value: string | number | undefined): void {
  if (value === undefined || value === "") return;
  url.searchParams.set(key, String(value));
}

export class AuditClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: AuditClientOptions) {
    if (!options.baseUrl) throw new Error("AuditClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("AuditClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  tail(query?: AuditTailQuery): Promise<AuditTailResponse> {
    return this.request<AuditTailResponse>("/v1/audit/tail", query);
  }

  verify(): Promise<AuditVerifyResponse> {
    return this.request<AuditVerifyResponse>("/v1/audit/verify");
  }

  stats(): Promise<AuditStatsResponse> {
    return this.request<AuditStatsResponse>("/v1/audit/stats");
  }

  trail(query?: AuditTrailQuery): Promise<AuditTrailResponse> {
    return this.request<AuditTrailResponse>("/api/audit/trail", query);
  }

  private async request<T>(path: string, query?: AuditTailQuery | AuditTrailQuery): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    for (const [key, value] of Object.entries(query ?? {})) setOptionalQuery(url, key, value);

    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const response = await this.fetchImpl(url, { method: "GET", headers });
    const parsed = await parseResponse(response);
    if (!response.ok) throw new AuditClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createAuditClient(options: AuditClientOptions): AuditClient {
  return new AuditClient(options);
}
