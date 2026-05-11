/**
 * Lightweight self-iteration SDK slice.
 *
 * This keeps proposal review, approval and manual self-improvement cycles
 * usable without importing the full generated OpenAPI SDK:
 *
 *   import { createIterateClient } from "yunque-client/iterate";
 */

export type IterateProposalType = "install_skill" | "adjust_persona" | "add_memory" | "fix_behavior" | string;
export type IterateProposalStatus = "pending" | "approved" | "rejected" | "executed" | "failed" | string;
export type IterateRiskLevel = 0 | 1 | 2 | number;

export type IterateProposal = {
  id: string;
  type?: IterateProposalType;
  title?: string;
  description?: string;
  risk?: IterateRiskLevel;
  est_tokens?: number;
  status?: IterateProposalStatus;
  review_summary?: string;
  result?: string;
  created_at?: string;
  resolved_at?: string;
  [key: string]: unknown;
};

export type IterateProposalsQuery = {
  status?: "pending" | string;
};

export type IterateProposalsResponse = {
  proposals: IterateProposal[];
  count: number;
};

export type IterateDecisionRequest = {
  id: string;
};

export type IterateDecisionResponse = {
  status?: "approved" | "rejected" | string;
  id?: string;
  [key: string]: unknown;
};

export type IterateCycleLog = {
  id?: string;
  started_at?: string;
  ended_at?: string;
  tokens_used?: number;
  rounds?: number;
  proposals?: IterateProposal[];
  stopped_by?: string;
  [key: string]: unknown;
};

export type IterateTriggerResponse = {
  status?: string;
  cycle?: IterateCycleLog;
  error?: string;
  [key: string]: unknown;
};

export type IterateStatusResponse = {
  enabled: boolean;
  pending_proposals?: number;
  [key: string]: unknown;
};

export type IterateClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class IterateClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Iterate request failed with HTTP ${status}`);
    this.name = "IterateClientError";
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

function setOptionalQuery(url: URL, key: string, value: string | undefined): void {
  if (!value) return;
  url.searchParams.set(key, value);
}

export class IterateClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: IterateClientOptions) {
    if (!options.baseUrl) throw new Error("IterateClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("IterateClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  proposals(query?: IterateProposalsQuery): Promise<IterateProposalsResponse> {
    return this.request<IterateProposalsResponse>("GET", "/api/iterate/proposals", undefined, query);
  }

  pendingProposals(): Promise<IterateProposalsResponse> {
    return this.proposals({ status: "pending" });
  }

  approve(body: IterateDecisionRequest): Promise<IterateDecisionResponse> {
    return this.request<IterateDecisionResponse>("POST", "/api/iterate/approve", body);
  }

  reject(body: IterateDecisionRequest): Promise<IterateDecisionResponse> {
    return this.request<IterateDecisionResponse>("POST", "/api/iterate/reject", body);
  }

  trigger(): Promise<IterateTriggerResponse> {
    return this.request<IterateTriggerResponse>("POST", "/api/iterate/trigger", {});
  }

  status(): Promise<IterateStatusResponse> {
    return this.request<IterateStatusResponse>("GET", "/api/iterate/status");
  }

  private async request<T>(
    method: "GET" | "POST",
    path: string,
    body?: unknown,
    query?: IterateProposalsQuery,
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    setOptionalQuery(url, "status", query?.status);

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
    if (!response.ok) throw new IterateClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createIterateClient(options: IterateClientOptions): IterateClient {
  return new IterateClient(options);
}
