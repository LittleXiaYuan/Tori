/** Lightweight Scheduler SDK slice: prompt-based recurring job list/add/remove. */
export type SchedulerJob = { id: string; name: string; tenant_id?: string; interval?: number | string; prompt?: string; [key: string]: unknown };
export type SchedulerJobsResponse = { jobs: SchedulerJob[]; count: number; [key: string]: unknown };
export type SchedulerAddRequest = { name: string; prompt: string; interval: string };
export type SchedulerRemoveResponse = { status: "removed" | string; [key: string]: unknown };
export type SchedulerClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class SchedulerClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Scheduler request failed with HTTP ${status}`); this.name = "SchedulerClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class SchedulerClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: SchedulerClientOptions) { if (!options.baseUrl) throw new Error("SchedulerClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("SchedulerClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  jobs(): Promise<SchedulerJobsResponse> { return this.request<SchedulerJobsResponse>("GET", "/v1/scheduler/jobs"); }
  add(request: SchedulerAddRequest): Promise<SchedulerJob> { return this.request<SchedulerJob>("POST", "/v1/scheduler/add", { body: request }); }
  remove(id: string): Promise<SchedulerRemoveResponse> { return this.request<SchedulerRemoveResponse>("POST", "/v1/scheduler/remove", { body: { id } }); }

  private async request<T>(method: "GET" | "POST", path: string, options: { body?: unknown; headers?: HeadersInit } = {}): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); const headers = mergeHeaders(this.headers, options.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new SchedulerClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createSchedulerClient(options: SchedulerClientOptions): SchedulerClient { return new SchedulerClient(options); }
