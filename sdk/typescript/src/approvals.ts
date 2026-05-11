/**
 * Lightweight Approval SDK slice.
 *
 * This keeps human-in-the-loop approval queues and approval rules usable
 * without importing the full generated OpenAPI SDK:
 *
 *   import { createApprovalsClient } from "yunque-client/approvals";
 */

export type ApprovalStatus = "pending" | "approved" | "denied" | string;
export type ApprovalDecision = "allow_once" | "allow_always" | "deny_always" | string;

export type ApprovalRequest = {
  id: string;
  status?: ApprovalStatus;
  title?: string;
  action?: string;
  reason?: string;
  tenant_id?: string;
  created_at?: string;
  updated_at?: string;
  payload?: Record<string, unknown>;
  [key: string]: unknown;
};

export type ApprovalRule = {
  id?: string;
  tenant_id?: string;
  action?: string;
  tool?: string;
  pattern?: string;
  decision?: ApprovalDecision;
  reason?: string;
  [key: string]: unknown;
};

export type ListApprovalsOptions = {
  status?: ApprovalStatus | "";
  history?: boolean;
};

export type ListApprovalsResponse = {
  approvals: ApprovalRequest[];
  total: number;
};

export type ApprovalActionResponse = {
  status?: string;
  id?: string;
  decision?: string;
  deleted?: boolean;
  [key: string]: unknown;
};

export type ApprovalRulesResponse = {
  rules: ApprovalRule[];
  total: number;
};

export type ApprovalsClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class ApprovalsClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Approvals request failed with HTTP ${status}`);
    this.name = "ApprovalsClientError";
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

function setOptionalQuery(url: URL, key: string, value: string | boolean | undefined): void {
  if (value === undefined || value === "") return;
  url.searchParams.set(key, String(value));
}

export class ApprovalsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: ApprovalsClientOptions) {
    if (!options.baseUrl) throw new Error("ApprovalsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("ApprovalsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  list(options?: ListApprovalsOptions): Promise<ListApprovalsResponse> {
    return this.request<ListApprovalsResponse>("GET", "/v1/approvals", undefined, {
      status: options?.status,
      history: options?.history,
    });
  }

  pending(): Promise<ListApprovalsResponse> {
    return this.list({ status: "pending" });
  }

  history(): Promise<ListApprovalsResponse> {
    return this.list({ history: true });
  }

  approve(id: string): Promise<ApprovalActionResponse> {
    return this.request<ApprovalActionResponse>("POST", "/v1/approvals/approve", { id });
  }

  deny(id: string, reason?: string): Promise<ApprovalActionResponse> {
    return this.request<ApprovalActionResponse>("POST", "/v1/approvals/deny", { id, reason });
  }

  decide(id: string, decision: ApprovalDecision): Promise<ApprovalActionResponse> {
    return this.request<ApprovalActionResponse>("POST", "/v1/approvals/decide", { id, decision });
  }

  rules(): Promise<ApprovalRulesResponse> {
    return this.request<ApprovalRulesResponse>("GET", "/v1/approvals/rules");
  }

  addRule(rule: ApprovalRule): Promise<ApprovalActionResponse> {
    return this.request<ApprovalActionResponse>("POST", "/v1/approvals/rules", rule);
  }

  deleteRule(id: string): Promise<ApprovalActionResponse> {
    return this.request<ApprovalActionResponse>("DELETE", "/v1/approvals/rules", undefined, { id });
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
    if (!response.ok) throw new ApprovalsClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createApprovalsClient(options: ApprovalsClientOptions): ApprovalsClient {
  return new ApprovalsClient(options);
}
