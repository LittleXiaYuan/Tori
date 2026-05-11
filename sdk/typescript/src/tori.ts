/** Lightweight Tori SDK slice. */
export type ToriBindRequest = { tori_url?: string };
export type ToriBindResponse = { status: "pending" | string; authorize_url?: string; message?: string; [key: string]: unknown };
export type ToriBindingStatus = { bound: boolean; username?: string; email?: string; tori_url?: string; api_key_set?: boolean; expires_at?: string; [key: string]: unknown };
export type ToriUnbindResponse = { status: "unbound" | "not_bound" | string; [key: string]: unknown };
export type ToriHealthResponse = { status?: string; error?: string; [key: string]: unknown };
export type ToriUsageResponse = { error?: string; [key: string]: unknown };
export type ToriClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class ToriClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Tori request failed with HTTP ${status}`); this.name = "ToriClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class ToriClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: ToriClientOptions) { if (!options.baseUrl) throw new Error("ToriClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("ToriClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  bind(body: ToriBindRequest = {}): Promise<ToriBindResponse> { return this.request<ToriBindResponse>("POST", "/v1/tori/bind", body); }
  status(): Promise<ToriBindingStatus> { return this.request<ToriBindingStatus>("GET", "/v1/tori/status"); }
  unbind(): Promise<ToriUnbindResponse> { return this.request<ToriUnbindResponse>("POST", "/v1/tori/unbind", {}); }
  health(): Promise<ToriHealthResponse> { return this.request<ToriHealthResponse>("GET", "/v1/tori/health"); }
  usage(): Promise<ToriUsageResponse> { return this.request<ToriUsageResponse>("GET", "/v1/tori/usage"); }
  private async request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
    const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); }
    const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), init); const parsed = await parseResponse(response); if (!response.ok) throw new ToriClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}
export function createToriClient(options: ToriClientOptions): ToriClient { return new ToriClient(options); }
