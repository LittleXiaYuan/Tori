/** Lightweight Conversations SDK slice. */
export type ConversationMessageRole = "system" | "user" | "assistant" | "tool" | string;
export type ConversationContentPart = { type?: string; text?: string; image_url?: unknown; [key: string]: unknown };
export type ConversationMessage = { role: ConversationMessageRole; content?: string | ConversationContentPart[]; tool_call_id?: string; tool_calls?: unknown[]; reasoning_content?: string; [key: string]: unknown };
export type ConversationSession = { id: string; tenant_id?: string; name?: string; summary?: string; pinned?: boolean; archived_at?: string; messages?: ConversationMessage[]; created_at?: string; updated_at?: string; [key: string]: unknown };
export type ConversationsResponse = { sessions: ConversationSession[]; count: number; [key: string]: unknown };
export type ConversationMessagesResponse = { messages: ConversationMessage[]; count: number; [key: string]: unknown };
export type ConversationDeleteResponse = { status: "deleted" | string; [key: string]: unknown };
export type ManageConversationRequest = { session_id: string; name?: string; pinned?: boolean; archive?: boolean };
export type ManageConversationResponse = { status: "updated" | string; session?: ConversationSession; [key: string]: unknown };
export type ConversationPipelinePhase = { phase: string; duration_ms?: number; detail?: Record<string, unknown>; [key: string]: unknown };
export type ConversationReplayTurn = { turn: number; timestamp?: string; user_message?: string; assistant_reply?: string; trace_id?: string; pipeline?: ConversationPipelinePhase[]; trust_delta?: number; tokens_in?: number; tokens_out?: number; [key: string]: unknown };
export type ConversationReplayResponse = { session_id: string; raw?: boolean; turns: ConversationReplayTurn[]; total_turns: number; [key: string]: unknown };
export type ConversationReplayOptions = { raw?: boolean; limit?: number; offset?: number };

export type ConversationsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class ConversationsClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Conversations request failed with HTTP ${status}`); this.name = "ConversationsClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | number | boolean | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, String(value)); }

export class ConversationsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: ConversationsClientOptions) {
    if (!options.baseUrl) throw new Error("ConversationsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("ConversationsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  list(options: { archived?: boolean } = {}): Promise<ConversationsResponse> { return this.json<ConversationsResponse>("/v1/conversations", { query: { archived: options.archived ? true : undefined } }); }
  messages(sessionId: string): Promise<ConversationMessagesResponse> { return this.json<ConversationMessagesResponse>("/v1/conversations/messages", { query: { session_id: sessionId } }); }
  deleteMessages(sessionId: string): Promise<ConversationDeleteResponse> { return this.json<ConversationDeleteResponse>("/v1/conversations/messages", { method: "DELETE", query: { session_id: sessionId } }); }
  manage(request: ManageConversationRequest): Promise<ManageConversationResponse> { return this.json<ManageConversationResponse>("/v1/conversations/manage", { method: "PUT", body: request }); }
  replay(sessionId: string, options: ConversationReplayOptions = {}): Promise<ConversationReplayResponse> { return this.json<ConversationReplayResponse>("/v1/conversations/replay", { query: { session_id: sessionId, raw: options.raw ? true : undefined, limit: options.limit, offset: options.offset } }); }

  rename(sessionId: string, name: string): Promise<ManageConversationResponse> { return this.manage({ session_id: sessionId, name }); }
  pin(sessionId: string, pinned = true): Promise<ManageConversationResponse> { return this.manage({ session_id: sessionId, pinned }); }
  archive(sessionId: string, archive = true): Promise<ManageConversationResponse> { return this.manage({ session_id: sessionId, archive }); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(path: string, options: { method?: "DELETE" | "GET" | "PUT"; body?: unknown; query?: Record<string, string | number | boolean | undefined>; headers?: HeadersInit } = {}): Promise<T> { const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = this.authHeaders(options.headers); const init: RequestInit = { method: options.method ?? "GET", headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); } const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new ConversationsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createConversationsClient(options: ConversationsClientOptions): ConversationsClient { return new ConversationsClient(options); }
