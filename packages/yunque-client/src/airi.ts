/** Lightweight Airi bridge SDK slice. */
export type AiriClientOptions = { baseUrl: string; apiKey?: string; token?: string; headers?: HeadersInit; fetch?: typeof fetch };
export type AiriStatusResponse = { plugin: "airi" | string; connected: boolean; url?: string; module_name?: string; messages_sent?: number; messages_received?: number; [key: string]: unknown };
export type AiriModel = { id: string; object?: string; created?: number; owned_by?: string; [key: string]: unknown };
export type AiriModelsResponse = { object?: string; data: AiriModel[]; [key: string]: unknown };
export type AiriChatMessage = { role: "system" | "user" | "assistant" | string; content: string; [key: string]: unknown };
export type AiriChatCompletionRequest = { model?: string; messages: AiriChatMessage[]; stream?: boolean; [key: string]: unknown };
export type AiriChatCompletionResponse = { id?: string; object?: string; created?: number; model?: string; choices?: Array<{ index?: number; message?: AiriChatMessage; delta?: Partial<AiriChatMessage>; finish_reason?: string | null; [key: string]: unknown }>; [key: string]: unknown };
export type AiriStreamItem = { kind: "chunk"; chunk: AiriChatCompletionResponse } | { kind: "done" };

export class AiriClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Airi request failed with HTTP ${status}`); this.name = "AiriClientError"; this.status = status; this.body = body; } }

type SSEFrame = { data: string };
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
async function* readSSEFrames(body: ReadableStream<Uint8Array>): AsyncGenerator<SSEFrame> { const reader = body.getReader(); const decoder = new TextDecoder(); let buffer = ""; try { while (true) { const { value, done } = await reader.read(); if (done) break; buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, "\n"); let splitAt = buffer.indexOf("\n\n"); while (splitAt !== -1) { const raw = buffer.slice(0, splitAt); buffer = buffer.slice(splitAt + 2); const data = raw.split("\n").filter((line) => line.startsWith("data:")).map((line) => line.slice(5).replace(/^ /, "")).join("\n"); if (data) yield { data }; splitAt = buffer.indexOf("\n\n"); } } buffer += decoder.decode(); const data = buffer.trimEnd().split("\n").filter((line) => line.startsWith("data:")).map((line) => line.slice(5).replace(/^ /, "")).join("\n"); if (data) yield { data }; } finally { reader.releaseLock(); } }

export class AiriClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly apiKey: string | undefined; private readonly token: string | undefined;
  constructor(options: AiriClientOptions) { if (!options.baseUrl) throw new Error("AiriClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("AiriClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.apiKey = options.apiKey; this.token = options.token; }
  status(): Promise<AiriStatusResponse> { return this.json<AiriStatusResponse>("GET", "/v1/ext/airi/status"); }
  models(): Promise<AiriModelsResponse> { return this.json<AiriModelsResponse>("GET", "/v1/ext/airi/models"); }
  chatCompletions(request: AiriChatCompletionRequest): Promise<AiriChatCompletionResponse> { return this.json<AiriChatCompletionResponse>("POST", "/v1/ext/airi/chat/completions", { ...request, stream: false }); }
  async *streamChatCompletions(request: AiriChatCompletionRequest): AsyncGenerator<AiriStreamItem> { const response = await this.send("POST", "/v1/ext/airi/chat/completions", { ...request, stream: true }); if (!response.ok) { const parsed = await parseResponse(response); throw new AiriClientError(response.status, parsed, messageFromErrorBody(parsed)); } if (!response.body) throw new AiriClientError(response.status, undefined, "Airi stream response has no body"); yield* this.parseStream(response.body); }
  async *parseStream(body: ReadableStream<Uint8Array>): AsyncGenerator<AiriStreamItem> { for await (const frame of readSSEFrames(body)) { if (frame.data === "[DONE]") { yield { kind: "done" }; continue; } yield { kind: "chunk", chunk: JSON.parse(frame.data) as AiriChatCompletionResponse }; } }
  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); const token = this.apiKey ?? this.token; if (token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${token}`); return headers; }
  private async send(method: "GET" | "POST", path: string, body?: unknown): Promise<Response> { const headers = this.authHeaders(); const init: RequestInit = { method, headers }; if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); } return this.fetchImpl(new URL(`${this.baseUrl}${path}`), init); }
  private async json<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> { const response = await this.send(method, path, body); const parsed = await parseResponse(response); if (!response.ok) throw new AiriClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}
export function createAiriClient(options: AiriClientOptions): AiriClient { return new AiriClient(options); }
