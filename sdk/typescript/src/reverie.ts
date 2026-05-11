/** Lightweight Reverie SDK slice. */
export type ReverieThought = { id?: string; category?: string; significance?: number; delivered?: boolean; [key: string]: unknown };
export type ReverieJournalQuery = { category?: string; min_significance?: number; delivered?: boolean; limit?: number; offset?: number };
export type ReverieJournalResponse = { thoughts: ReverieThought[] | null; total: number; limit: number; offset: number };
export type ReverieConfig = { enabled?: boolean; interval_minutes?: number; min_significance?: number; quiet_start?: number; quiet_end?: number; [key: string]: unknown };
export type ReverieConfigResponse = { config: ReverieConfig | Record<string, unknown>; running?: boolean };
export type ReverieThinkRequest = { event_type?: string; trigger?: string };
export type ReverieThinkResponse = { thought: ReverieThought };
export type ReverieDeleteResponse = { deleted?: boolean; id?: string; [key: string]: unknown };
export type ReverieActionsResponse = { actions: unknown[]; total: number };
export type ReverieTargetsResponse = { targets: Array<{ channel?: string; targets?: string[]; env_var?: string; [key: string]: unknown }>; count: number; env_prefix?: string };
export type ReverieClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class ReverieClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Reverie request failed with HTTP ${status}`); this.name = "ReverieClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function setOptionalQuery(url: URL, key: string, value: string | number | boolean | undefined): void { if (value === undefined || value === "") return; url.searchParams.set(key, String(value)); }

export class ReverieClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: ReverieClientOptions) { if (!options.baseUrl) throw new Error("ReverieClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("ReverieClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  journal(query?: ReverieJournalQuery): Promise<ReverieJournalResponse> { return this.request<ReverieJournalResponse>("GET", "/v1/reverie/journal", undefined, query); }
  stats(): Promise<Record<string, unknown>> { return this.request<Record<string, unknown>>("GET", "/v1/reverie/stats"); }
  config(): Promise<ReverieConfigResponse> { return this.request<ReverieConfigResponse>("GET", "/v1/reverie/config"); }
  updateConfig(body: ReverieConfig): Promise<ReverieConfigResponse> { return this.request<ReverieConfigResponse>("PUT", "/v1/reverie/config", body); }
  think(body?: ReverieThinkRequest): Promise<ReverieThinkResponse> { return this.request<ReverieThinkResponse>("POST", "/v1/reverie/think", body ?? {}); }
  deleteThought(id: string): Promise<ReverieDeleteResponse> { return this.request<ReverieDeleteResponse>("DELETE", "/v1/reverie/thought", undefined, { id }); }
  actions(): Promise<ReverieActionsResponse> { return this.request<ReverieActionsResponse>("GET", "/v1/reverie/actions"); }
  targets(): Promise<ReverieTargetsResponse> { return this.request<ReverieTargetsResponse>("GET", "/v1/reverie/targets"); }
  private async request<T>(method: "GET" | "PUT" | "POST" | "DELETE", path: string, body?: unknown, query?: Record<string, string | number | boolean | undefined>): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); for (const [key, value] of Object.entries(query ?? {})) setOptionalQuery(url, key, value);
    const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new ReverieClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}
export function createReverieClient(options: ReverieClientOptions): ReverieClient { return new ReverieClient(options); }
