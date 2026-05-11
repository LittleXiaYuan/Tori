/** Lightweight MCP Dispatch SDK slice: worker registry, queue status, and enqueue helpers. */
export type DispatchWorker = {
  id?: string;
  name?: string;
  type?: string;
  capabilities?: string[];
  status?: string;
  last_seen?: string;
  [key: string]: unknown;
};
export type DispatchWorkersResponse = { workers: DispatchWorker[]; count: number; [key: string]: unknown };
export type DispatchQueueResponse = { message?: string; queues?: unknown; [key: string]: unknown };
export type DispatchEnqueueRequest = { task_id: string; capabilities?: string[]; priority?: number };
export type DispatchEnqueueResponse = { task_id: string; status: "enqueued" | string; [key: string]: unknown };
export type DispatchWorkerConfigResponse = { type: string; mcp_config: string; instructions: string; server_url: string; [key: string]: unknown };
export type DispatchStatusResponse = { status: string; [key: string]: unknown };
export type DispatchClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class DispatchClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Dispatch request failed with HTTP ${status}`); this.name = "DispatchClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | number | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined && value !== "") url.searchParams.set(key, String(value)); }

export class DispatchClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: DispatchClientOptions) { if (!options.baseUrl) throw new Error("DispatchClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("DispatchClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  workers(): Promise<DispatchWorkersResponse> { return this.request<DispatchWorkersResponse>("GET", "/v1/workers"); }
  worker(id: string): Promise<DispatchWorker> { return this.request<DispatchWorker>("GET", "/v1/workers/detail", { query: { id } }); }
  removeWorker(id: string): Promise<DispatchStatusResponse> { return this.request<DispatchStatusResponse>("POST", "/v1/workers/remove", { body: { id } }); }
  queue(): Promise<DispatchQueueResponse> { return this.request<DispatchQueueResponse>("GET", "/v1/dispatch/queue"); }
  enqueue(request: DispatchEnqueueRequest): Promise<DispatchEnqueueResponse> { return this.request<DispatchEnqueueResponse>("POST", "/v1/dispatch/enqueue", { body: request }); }
  workerConfig(type?: string): Promise<DispatchWorkerConfigResponse> { return this.request<DispatchWorkerConfigResponse>("GET", "/v1/workers/config", { query: { type } }); }

  private async request<T>(method: "GET" | "POST", path: string, options: { body?: unknown; query?: Record<string, string | number | undefined>; headers?: HeadersInit } = {}): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = mergeHeaders(this.headers, options.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new DispatchClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createDispatchClient(options: DispatchClientOptions): DispatchClient { return new DispatchClient(options); }
