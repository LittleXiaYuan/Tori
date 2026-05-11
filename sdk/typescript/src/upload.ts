/** Lightweight Upload SDK slice: authenticated multipart upload and parsed-file metadata. */
export type UploadParseMetadata = {
  status?: "parsed" | "needs_document_parser" | "unsupported" | "empty" | string;
  parser?: "local" | "document" | string;
  preview?: string;
  note?: string;
  chars?: number;
  markdown_chars?: number;
  [key: string]: unknown;
};

export type UploadAction = { type?: string; label?: string; path?: string; [key: string]: unknown };
export type UploadResponse = {
  filename: string;
  size: number;
  path: string;
  parse?: UploadParseMetadata;
  analysis?: unknown;
  actions?: UploadAction[];
  rich?: unknown;
  [key: string]: unknown;
};

export type UploadRequest = { file: Blob; filename?: string };
export type UploadClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class UploadClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Upload request failed with HTTP ${status}`); this.name = "UploadClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class UploadClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: UploadClientOptions) { if (!options.baseUrl) throw new Error("UploadClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("UploadClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  file(file: Blob, filename?: string): Promise<UploadResponse> { return this.upload({ file, filename }); }
  upload(request: UploadRequest): Promise<UploadResponse> { const form = new FormData(); if (request.filename) form.append("file", request.file, request.filename); else form.append("file", request.file); return this.request<UploadResponse>(form); }

  private async request<T>(body: FormData): Promise<T> {
    const url = new URL(`${this.baseUrl}/v1/upload`); const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const response = await this.fetchImpl(url, { method: "POST", headers, body }); const parsed = await parseResponse(response); if (!response.ok) throw new UploadClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createUploadClient(options: UploadClientOptions): UploadClient { return new UploadClient(options); }
