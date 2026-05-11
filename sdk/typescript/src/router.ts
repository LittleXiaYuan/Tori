/** Lightweight Router SDK slice: smart-router slot and routing statistics. */
export type RouterStatsResponse = { status?: "not configured" | string; slots?: unknown; stats?: unknown; [key: string]: unknown };
export type RouterClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class RouterClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Router request failed with HTTP ${status}`); this.name = "RouterClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class RouterClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: RouterClientOptions) { if (!options.baseUrl) throw new Error("RouterClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("RouterClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  stats(): Promise<RouterStatsResponse> { return this.request<RouterStatsResponse>("GET", "/v1/router/stats"); }

  private async request<T>(method: "GET", path: string): Promise<T> {
    const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), { method, headers }); const parsed = await parseResponse(response); if (!response.ok) throw new RouterClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createRouterClient(options: RouterClientOptions): RouterClient { return new RouterClient(options); }
