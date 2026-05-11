/**
 * Lightweight Trust and governance SDK slice.
 *
 * This keeps trust scores, review-gate status and skill-growth patterns usable
 * without importing the full generated OpenAPI SDK:
 *
 *   import { createTrustClient } from "yunque-client/trust";
 */

export type TrustScore = {
  slug?: string;
  score?: number;
  level?: string;
  [key: string]: unknown;
};

export type TrustScoresResponse = {
  scores: Record<string, TrustScore | number | unknown>;
  count: number;
};

export type TrustSlugRequest = {
  slug: string;
};

export type TrustResetResponse = {
  status?: string;
  slug?: string;
  [key: string]: unknown;
};

export type TrustGrantResponse = {
  status?: string;
  slug?: string;
  level?: string;
  upgraded?: number;
  [key: string]: unknown;
};

export type ReviewStatusResponse = {
  enabled: boolean;
  trust_enabled: boolean;
  distill_enabled: boolean;
  [key: string]: unknown;
};

export type SkillGrowPattern = {
  pattern?: string;
  count?: number;
  confidence?: number;
  [key: string]: unknown;
};

export type SkillGrowPatternsResponse = {
  patterns: SkillGrowPattern[];
  count: number;
};

export type TrustClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class TrustClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Trust request failed with HTTP ${status}`);
    this.name = "TrustClientError";
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

export class TrustClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: TrustClientOptions) {
    if (!options.baseUrl) throw new Error("TrustClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("TrustClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  scores(): Promise<TrustScoresResponse> {
    return this.request<TrustScoresResponse>("GET", "/api/trust/scores");
  }

  reset(body: TrustSlugRequest): Promise<TrustResetResponse> {
    return this.request<TrustResetResponse>("POST", "/api/trust/reset", body);
  }

  grant(body: TrustSlugRequest): Promise<TrustGrantResponse> {
    return this.request<TrustGrantResponse>("POST", "/api/trust/grant", body);
  }

  grantAll(): Promise<TrustGrantResponse> {
    return this.grant({ slug: "*" });
  }

  reviewStatus(): Promise<ReviewStatusResponse> {
    return this.request<ReviewStatusResponse>("GET", "/api/review/status");
  }

  skillGrowPatterns(): Promise<SkillGrowPatternsResponse> {
    return this.request<SkillGrowPatternsResponse>("GET", "/api/skillgrow/patterns");
  }

  private async request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
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
    if (!response.ok) throw new TrustClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createTrustClient(options: TrustClientOptions): TrustClient {
  return new TrustClient(options);
}
