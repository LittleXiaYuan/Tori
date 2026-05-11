/** Lightweight Speech/Upload SDK slice. */
export type SpeechTTSRequest = { text: string; voice?: string; format?: "mp3" | "wav" | "opus" | "flac" | string; emotion?: string };
export type SpeechAudioResponse = { blob: Blob; contentType?: string };
export type SpeechSTTOptions = { language?: string; detect_emotion?: boolean };
export type SpeechSTTResponse = { text: string; emotion?: unknown; [key: string]: unknown };
export type SpeechVoicesResponse = { voices: unknown[]; providers: unknown[]; [key: string]: unknown };
export type UploadResponse = { filename: string; size: number; path: string; parse?: unknown; analysis?: unknown; actions?: unknown[]; rich?: unknown; [key: string]: unknown };
export type SpeechClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };
export type AudioBody = Blob | ArrayBuffer | Uint8Array;

export class SpeechClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Speech request failed with HTTP ${status}`); this.name = "SpeechClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function setOptionalQuery(url: URL, key: string, value: string | boolean | undefined): void { if (value === undefined || value === "") return; url.searchParams.set(key, String(value)); }

export class SpeechClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: SpeechClientOptions) { if (!options.baseUrl) throw new Error("SpeechClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("SpeechClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  async tts(body: SpeechTTSRequest): Promise<SpeechAudioResponse> { const response = await this.send("POST", "/v1/speech/tts", body); if (!response.ok) { const parsed = await parseResponse(response); throw new SpeechClientError(response.status, parsed, messageFromErrorBody(parsed)); } return { blob: await response.blob(), contentType: response.headers.get("content-type") ?? undefined }; }
  stt(audio: AudioBody, options?: SpeechSTTOptions): Promise<SpeechSTTResponse> { const url = new URL(`${this.baseUrl}/v1/speech/stt`); setOptionalQuery(url, "language", options?.language); setOptionalQuery(url, "detect_emotion", options?.detect_emotion); return this.json<SpeechSTTResponse>("POST", url, audio, { "Content-Type": audio instanceof Blob && audio.type ? audio.type : "application/octet-stream" }); }
  voices(): Promise<SpeechVoicesResponse> { return this.json<SpeechVoicesResponse>("GET", new URL(`${this.baseUrl}/v1/speech/voices`)); }
  upload(file: Blob, filename = "upload.bin"): Promise<UploadResponse> { const form = new FormData(); form.append("file", file, filename); return this.json<UploadResponse>("POST", new URL(`${this.baseUrl}/v1/upload`), form); }
  sttStreamUrl(options?: SpeechSTTOptions): string { const url = new URL(`${this.baseUrl}/v1/speech/stt/stream`); url.protocol = url.protocol === "https:" ? "wss:" : "ws:"; setOptionalQuery(url, "language", options?.language); setOptionalQuery(url, "detect_emotion", options?.detect_emotion); return String(url); }
  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async send(method: "GET" | "POST", path: string, body?: unknown, extraHeaders?: HeadersInit): Promise<Response> { return this.fetchImpl(new URL(`${this.baseUrl}${path}`), this.init(method, body, extraHeaders)); }
  private init(method: "GET" | "POST", body?: unknown, extraHeaders?: HeadersInit): RequestInit { const headers = this.authHeaders(extraHeaders); const init: RequestInit = { method, headers }; if (body instanceof FormData) init.body = body; else if (body instanceof Blob || body instanceof ArrayBuffer) init.body = body; else if (body instanceof Uint8Array) init.body = body.buffer.slice(body.byteOffset, body.byteOffset + body.byteLength) as ArrayBuffer; else if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); } return init; }
  private async json<T>(method: "GET" | "POST", url: URL, body?: unknown, extraHeaders?: HeadersInit): Promise<T> { const response = await this.fetchImpl(url, this.init(method, body, extraHeaders)); const parsed = await parseResponse(response); if (!response.ok) throw new SpeechClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}
export function createSpeechClient(options: SpeechClientOptions): SpeechClient { return new SpeechClient(options); }
