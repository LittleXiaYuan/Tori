/** Lightweight Bots and Inbox SDK slice. */
export type BotStatus = "ready" | "stopped" | "creating" | string;
export type BotConfig = { model?: string; max_steps?: number; temperature?: number; max_context_msgs?: number; language?: string; reasoning_enabled?: boolean; reasoning_effort?: "low" | "medium" | "high" | string };
export type Bot = { id: string; name: string; description?: string; persona_dir?: string; is_active?: boolean; status?: BotStatus; config?: BotConfig; metadata?: Record<string, unknown>; created_at?: string; updated_at?: string; [key: string]: unknown };
export type BotsResponse = { bots: Bot[]; total: number; active: number; [key: string]: unknown };
export type CreateBotRequest = { name: string; description?: string; config?: BotConfig };
export type UpdateBotRequest = { name?: string; description?: string; config?: BotConfig; active?: boolean };
export type DeleteBotResponse = { status: "ok" | string; [key: string]: unknown };

export type InboxAction = "notify" | "trigger" | string;
export type InboxItem = { id: string; source?: string; header?: Record<string, unknown>; content: string; action?: InboxAction; is_read?: boolean; created_at?: string; read_at?: string; [key: string]: unknown };
export type InboxCount = { unread: number; total: number; [key: string]: unknown };
export type InboxResponse = { items: InboxItem[]; count: InboxCount; [key: string]: unknown };
export type PushInboxRequest = { source?: string; content: string; action?: InboxAction; header?: Record<string, unknown> };
export type InboxDeleteResponse = { status: "ok" | string; [key: string]: unknown };
export type InboxReadResponse = { marked: number; [key: string]: unknown };

export type ChannelGroup = { id: string; name?: string; channel_type?: string; chat_type?: string; member_count?: number; last_active?: string; [key: string]: unknown };
export type ChannelGroupsResponse = { groups: ChannelGroup[]; count: number; [key: string]: unknown };

export type BotsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class BotsClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Bots request failed with HTTP ${status}`); this.name = "BotsClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | boolean | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, String(value)); }

export class BotsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: BotsClientOptions) {
    if (!options.baseUrl) throw new Error("BotsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("BotsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  list(): Promise<BotsResponse> { return this.json<BotsResponse>("/v1/bots"); }
  create(request: CreateBotRequest): Promise<Bot> { return this.json<Bot>("/v1/bots", { method: "POST", body: { name: request.name, description: request.description ?? "", config: request.config ?? {} } }); }
  get(id: string): Promise<Bot> { return this.json<Bot>("/v1/bots/detail", { query: { id } }); }
  update(id: string, request: UpdateBotRequest): Promise<Bot> { return this.json<Bot>("/v1/bots/detail", { method: "PUT", query: { id }, body: request }); }
  setActive(id: string, active: boolean): Promise<Bot> { return this.update(id, { active }); }
  delete(id: string): Promise<DeleteBotResponse> { return this.json<DeleteBotResponse>("/v1/bots/detail", { method: "DELETE", query: { id } }); }

  inbox(unread?: boolean): Promise<InboxResponse> { return this.json<InboxResponse>("/v1/inbox", { query: { unread: unread ? true : undefined } }); }
  pushInbox(request: PushInboxRequest): Promise<InboxItem> { return this.json<InboxItem>("/v1/inbox", { method: "POST", body: { source: request.source ?? "", content: request.content, action: request.action ?? "notify", header: request.header ?? {} } }); }
  deleteInbox(id: string): Promise<InboxDeleteResponse> { return this.json<InboxDeleteResponse>("/v1/inbox", { method: "DELETE", body: { id } }); }
  markInboxRead(ids: string[]): Promise<InboxReadResponse> { return this.json<InboxReadResponse>("/v1/inbox/read", { method: "POST", body: { ids, all: false } }); }
  markAllInboxRead(): Promise<InboxReadResponse> { return this.json<InboxReadResponse>("/v1/inbox/read", { method: "POST", body: { all: true } }); }
  channelGroups(type?: string): Promise<ChannelGroupsResponse> { return this.json<ChannelGroupsResponse>("/v1/channels/groups", { query: { type } }); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(path: string, options: { method?: "DELETE" | "GET" | "POST" | "PUT"; body?: unknown; query?: Record<string, string | boolean | undefined>; headers?: HeadersInit } = {}): Promise<T> { const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = this.authHeaders(options.headers); const init: RequestInit = { method: options.method ?? "GET", headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); } const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new BotsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createBotsClient(options: BotsClientOptions): BotsClient { return new BotsClient(options); }
