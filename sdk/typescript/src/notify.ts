/** Lightweight Notify SDK slice: notification channels, tests, and chat/share dispatch. */
export type NotifyChannelType = "webhook" | "dingtalk" | "feishu" | "wechat_work" | "email_smtp" | string;
export type NotifyChannel = { id: string; type: NotifyChannelType; name: string; url?: string; secret?: string; enabled?: boolean; smtp_host?: string; smtp_port?: number; smtp_user?: string; smtp_pass?: string; email_to?: string; [key: string]: unknown };
export type NotifyChannelsResponse = { channels: NotifyChannel[]; [key: string]: unknown };
export type NotifyOkResponse = { ok: boolean; [key: string]: unknown };
export type NotifyToggleRequest = { id: string; enabled: boolean };
export type NotifyShareFile = { name: string; path: string; size?: number };
export type NotifyShareRequest = { channel_id: string; title?: string; message?: string; session_id?: string; task_id?: string; url?: string; files?: NotifyShareFile[] };
export type NotifyShareResponse = { ok: boolean; sent_at: string; share?: { code: string; session_id: string; created_at: string; [key: string]: unknown }; channel?: { id: string; type: string; name: string; [key: string]: unknown }; [key: string]: unknown };
export type NotifyClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class NotifyClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Notify request failed with HTTP ${status}`); this.name = "NotifyClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, value); }

export class NotifyClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: NotifyClientOptions) { if (!options.baseUrl) throw new Error("NotifyClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("NotifyClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  channels(): Promise<NotifyChannelsResponse> { return this.request<NotifyChannelsResponse>("GET", "/api/notify/channels"); }
  addChannel(channel: NotifyChannel): Promise<NotifyOkResponse> { return this.request<NotifyOkResponse>("POST", "/api/notify/add", { body: channel }); }
  removeChannel(id: string): Promise<NotifyOkResponse> { return this.request<NotifyOkResponse>("POST", "/api/notify/remove", { query: { id } }); }
  toggleChannel(request: NotifyToggleRequest): Promise<NotifyOkResponse> { return this.request<NotifyOkResponse>("POST", "/api/notify/toggle", { body: request }); }
  testChannel(id: string): Promise<NotifyOkResponse> { return this.request<NotifyOkResponse>("POST", "/api/notify/test", { body: { id } }); }
  share(request: NotifyShareRequest): Promise<NotifyShareResponse> { return this.request<NotifyShareResponse>("POST", "/api/notify/share", { body: request }); }

  private async request<T>(method: "GET" | "POST", path: string, options: { body?: unknown; query?: Record<string, string | undefined>; headers?: HeadersInit } = {}): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = mergeHeaders(this.headers, options.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new NotifyClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createNotifyClient(options: NotifyClientOptions): NotifyClient { return new NotifyClient(options); }
