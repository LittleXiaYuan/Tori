/** Lightweight Files SDK slice. */
export type FileEntry = { name: string; path: string; size: number; is_dir: boolean };
export type FileListResponse = { files: FileEntry[] };
export type FilePreviewResponse = { name: string; path: string; size: number; ext?: string; kind?: string; content_type?: string; preview?: string; truncated?: boolean; editable?: boolean; parse?: unknown; [key: string]: unknown };
export type FileDownloadResponse = { blob: Blob; filename?: string; contentType?: string };
export type FilesClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class FilesClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Files request failed with HTTP ${status}`); this.name = "FilesClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function setOptionalQuery(url: URL, key: string, value: string | undefined): void { if (value === undefined || value === "") return; url.searchParams.set(key, value); }
function filenameFromDisposition(disposition: string | null): string | undefined { if (!disposition) return undefined; const star = /filename\*=UTF-8''([^;]+)/i.exec(disposition); if (star?.[1]) return decodeURIComponent(star[1]); const normal = /filename="?([^";]+)"?/i.exec(disposition); return normal?.[1]; }

export class FilesClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: FilesClientOptions) { if (!options.baseUrl) throw new Error("FilesClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("FilesClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  list(path?: string): Promise<FileListResponse> { const url = new URL(`${this.baseUrl}/api/files`); setOptionalQuery(url, "path", path); return this.json<FileListResponse>(url); }
  preview(path: string): Promise<FilePreviewResponse> { const url = new URL(`${this.baseUrl}/api/files/preview`); url.searchParams.set("path", path); return this.json<FilePreviewResponse>(url); }
  async download(path: string): Promise<FileDownloadResponse> { const url = new URL(`${this.baseUrl}/api/files/download`); url.searchParams.set("path", path); const response = await this.fetchImpl(url, { method: "GET", headers: this.authHeaders() }); if (!response.ok) { const parsed = await parseResponse(response); throw new FilesClientError(response.status, parsed, messageFromErrorBody(parsed)); } return { blob: await response.blob(), filename: filenameFromDisposition(response.headers.get("content-disposition")), contentType: response.headers.get("content-type") ?? undefined }; }
  private authHeaders(): Headers { const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(url: URL): Promise<T> { const response = await this.fetchImpl(url, { method: "GET", headers: this.authHeaders() }); const parsed = await parseResponse(response); if (!response.ok) throw new FilesClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}
export function createFilesClient(options: FilesClientOptions): FilesClient { return new FilesClient(options); }
