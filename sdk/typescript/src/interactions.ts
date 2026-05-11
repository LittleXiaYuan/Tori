/** Lightweight Interactions SDK slice: emotion, instructions, reactions and stickers. */
export type StatusResponse = { status: string; [key: string]: unknown };

export type EmotionHistoryEntry = { timestamp?: string; session_id?: string; emotion?: string; confidence?: number; source?: string; [key: string]: unknown };
export type EmotionHistoryResponse = { entries: EmotionHistoryEntry[]; summary?: Record<string, unknown>; total: number; [key: string]: unknown };
export type EmotionHistoryOptions = { sessionId?: string; limit?: number; from?: string; to?: string };

export type StickerSuggestion = { package_id: string; sticker_id: string; platform?: string; emotion?: string; file_id?: string; set_name?: string; cdnurl?: string; emoji?: string; [key: string]: unknown };
export type StickerMap = Record<string, Record<string, StickerSuggestion[]>>;
export type RegisterStickersRequest = { platform: string; emotion: string; stickers: Array<{ package_id: string; sticker_id: string }> };
export type ClearStickersRequest = { platform: string; emotion: string };
export type SendStickerRequest = { channel_type: string; target: string; package_id?: string; sticker_id?: string; file_id?: string; emoji?: string; platform?: string };
export type ReactRequest = { channel_type: string; target: string; message_id: string; emoji?: string };

export type UserInstruction = { instruction_id?: string; tenant_id?: string; category?: string; content: string; priority?: number; is_active?: boolean; created_at?: string; updated_at?: string; [key: string]: unknown };
export type InstructionsResponse = { instructions: UserInstruction[]; total: number; [key: string]: unknown };
export type InstructionStatusResponse = { status: "updated" | "deleted" | "reordered" | string; [key: string]: unknown };

export type InteractionsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class InteractionsClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Interactions request failed with HTTP ${status}`); this.name = "InteractionsClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | number | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, String(value)); }

export class InteractionsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: InteractionsClientOptions) {
    if (!options.baseUrl) throw new Error("InteractionsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("InteractionsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  emotionHistory(options: EmotionHistoryOptions = {}): Promise<EmotionHistoryResponse> { return this.json<EmotionHistoryResponse>("/v1/emotion/history", { query: { session_id: options.sessionId, limit: options.limit, from: options.from, to: options.to } }); }
  stickers(): Promise<StickerMap> { return this.json<StickerMap>("/v1/emotion/stickers"); }
  registerStickers(request: RegisterStickersRequest): Promise<StatusResponse> { return this.json<StatusResponse>("/v1/emotion/stickers", { method: "PUT", body: request }); }
  clearStickers(request: ClearStickersRequest): Promise<StatusResponse> { return this.json<StatusResponse>("/v1/emotion/stickers", { method: "DELETE", body: request }); }

  instructions(category?: string): Promise<InstructionsResponse> { return this.json<InstructionsResponse>("/v1/instructions", { query: { category } }); }
  createInstruction(instruction: UserInstruction): Promise<UserInstruction> { return this.json<UserInstruction>("/v1/instructions", { method: "POST", body: instruction }); }
  updateInstruction(instruction: UserInstruction): Promise<InstructionStatusResponse> { return this.json<InstructionStatusResponse>("/v1/instructions", { method: "PUT", body: instruction }); }
  deleteInstruction(id: string): Promise<InstructionStatusResponse> { return this.json<InstructionStatusResponse>("/v1/instructions", { method: "DELETE", query: { id } }); }
  reorderInstructions(ids: string[]): Promise<InstructionStatusResponse> { return this.json<InstructionStatusResponse>("/v1/instructions/reorder", { method: "POST", body: { ids } }); }

  react(request: ReactRequest): Promise<StatusResponse> { return this.json<StatusResponse>("/v1/react", { method: "POST", body: request }); }
  sendSticker(request: SendStickerRequest): Promise<StatusResponse> { return this.json<StatusResponse>("/v1/sticker/send", { method: "POST", body: request }); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(path: string, options: { method?: "DELETE" | "GET" | "POST" | "PUT"; body?: unknown; query?: Record<string, string | number | undefined>; headers?: HeadersInit } = {}): Promise<T> { const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = this.authHeaders(options.headers); const init: RequestInit = { method: options.method ?? "GET", headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); } const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new InteractionsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createInteractionsClient(options: InteractionsClientOptions): InteractionsClient { return new InteractionsClient(options); }
