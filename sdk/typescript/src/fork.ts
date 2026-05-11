/** Lightweight Fork SDK slice: conversation root forks, branches, and branch lists. */
export type ForkMessage = { role: "user" | "assistant" | "system" | string; content: string; timestamp?: string; [key: string]: unknown };
export type ConversationFork = { id: string; parent_id?: string; session_id: string; label?: string; messages: ForkMessage[]; created_at: string; children?: string[]; [key: string]: unknown };
export type ForkRootResponse = ConversationFork | { fork: null; [key: string]: unknown };
export type ForkCreateRequest = { session_id: string; messages?: ForkMessage[] };
export type ForkBranchRequest = { fork_id: string; at_index: number; label?: string };
export type ForkDeleteResponse = { deleted: boolean; [key: string]: unknown };
export type ForkListResponse = { forks: ConversationFork[]; [key: string]: unknown };
export type ForkClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class ForkClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Fork request failed with HTTP ${status}`); this.name = "ForkClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined && value !== "") url.searchParams.set(key, value); }

export class ForkClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: ForkClientOptions) { if (!options.baseUrl) throw new Error("ForkClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("ForkClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  root(sessionId: string): Promise<ForkRootResponse> { return this.request<ForkRootResponse>("GET", "/v1/fork", { query: { session_id: sessionId } }); }
  get(id: string): Promise<ConversationFork> { return this.request<ConversationFork>("GET", "/v1/fork", { query: { id } }); }
  create(request: ForkCreateRequest): Promise<ConversationFork> { return this.request<ConversationFork>("POST", "/v1/fork", { body: request }); }
  remove(id: string): Promise<ForkDeleteResponse> { return this.request<ForkDeleteResponse>("DELETE", "/v1/fork", { query: { id } }); }
  branch(request: ForkBranchRequest): Promise<ConversationFork> { return this.request<ConversationFork>("POST", "/v1/fork/branch", { body: request }); }
  list(sessionId: string): Promise<ForkListResponse> { return this.request<ForkListResponse>("GET", "/v1/fork/list", { query: { session_id: sessionId } }); }

  private async request<T>(method: "GET" | "POST" | "DELETE", path: string, options: { body?: unknown; query?: Record<string, string | undefined>; headers?: HeadersInit } = {}): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = mergeHeaders(this.headers, options.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new ForkClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createForkClient(options: ForkClientOptions): ForkClient { return new ForkClient(options); }
