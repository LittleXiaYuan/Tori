/** Lightweight Settings/Backup SDK slice. */
export type SettingsSchemaField = { key?: string; name?: string; label?: string; type?: string; sensitive?: boolean; required?: boolean; [key: string]: unknown };
export type SettingsSchemaGroup = { id?: string; name?: string; title?: string; fields?: SettingsSchemaField[]; [key: string]: unknown };
export type SettingsSchemaResponse = { groups: SettingsSchemaGroup[] };
export type SettingsConfigResponse = { values: Record<string, string> };
export type SettingsUpdateResponse = { success: boolean; restart_required?: boolean; message?: string; error?: string; [key: string]: unknown };
export type SettingsCheckResponse = { env_exists?: boolean; has_llm_key?: boolean; has_llm_url?: boolean; has_llm_model?: boolean; api_ok?: boolean; setup_needed?: boolean; [key: string]: unknown };
export type SettingsReloadResponse = { success: boolean; reloaded?: string[]; message?: string; error?: string; [key: string]: unknown };
export type SettingsDetectDirsResponse = { dirs?: Record<string, string>; default_paths?: string[]; current_read?: string; current_write?: string; [key: string]: unknown };
export type BackupInfoResponse = { files: Record<string, number>; file_count: number; total_bytes: number; version?: string; [key: string]: unknown };
export type BackupImportResponse = { success?: boolean; restored?: number; skipped?: number; manifest?: unknown; message?: string; [key: string]: unknown };
export type BackupExportResponse = { blob: Blob; filename?: string; contentType?: string };
export type SettingsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class SettingsClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Settings request failed with HTTP ${status}`); this.name = "SettingsClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function filenameFromDisposition(disposition: string | null): string | undefined { if (!disposition) return undefined; const star = /filename\*=UTF-8''([^;]+)/i.exec(disposition); if (star?.[1]) return decodeURIComponent(star[1]); const normal = /filename="?([^";]+)"?/i.exec(disposition); return normal?.[1]; }

export class SettingsClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: SettingsClientOptions) { if (!options.baseUrl) throw new Error("SettingsClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("SettingsClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  schema(): Promise<SettingsSchemaResponse> { return this.json<SettingsSchemaResponse>("GET", "/api/settings/schema"); }
  config(): Promise<SettingsConfigResponse> { return this.json<SettingsConfigResponse>("GET", "/api/settings/config"); }
  updateConfig(values: Record<string, string>): Promise<SettingsUpdateResponse> { return this.json<SettingsUpdateResponse>("PUT", "/api/settings/config", { values }); }
  check(): Promise<SettingsCheckResponse> { return this.json<SettingsCheckResponse>("GET", "/api/settings/check"); }
  reload(): Promise<SettingsReloadResponse> { return this.json<SettingsReloadResponse>("POST", "/v1/config/reload", {}); }
  detectDirs(): Promise<SettingsDetectDirsResponse> { return this.json<SettingsDetectDirsResponse>("GET", "/api/settings/detect-dirs"); }
  backupInfo(): Promise<BackupInfoResponse> { return this.json<BackupInfoResponse>("GET", "/v1/backup/info"); }
  async exportBackup(): Promise<BackupExportResponse> {
    const response = await this.send("GET", "/v1/backup/export");
    if (!response.ok) { const parsed = await parseResponse(response); throw new SettingsClientError(response.status, parsed, messageFromErrorBody(parsed)); }
    return { blob: await response.blob(), filename: filenameFromDisposition(response.headers.get("content-disposition")), contentType: response.headers.get("content-type") ?? undefined };
  }
  importBackup(backup: Blob, filename = "yunque-backup.zip"): Promise<BackupImportResponse> {
    const form = new FormData(); form.append("backup", backup, filename);
    return this.json<BackupImportResponse>("POST", "/v1/backup/import", form);
  }
  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async send(method: "GET" | "PUT" | "POST", path: string, body?: unknown): Promise<Response> {
    const headers = this.authHeaders(); const init: RequestInit = { method, headers };
    if (body instanceof FormData) init.body = body; else if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); }
    return this.fetchImpl(new URL(`${this.baseUrl}${path}`), init);
  }
  private async json<T>(method: "GET" | "PUT" | "POST", path: string, body?: unknown): Promise<T> { const response = await this.send(method, path, body); const parsed = await parseResponse(response); if (!response.ok) throw new SettingsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}
export function createSettingsClient(options: SettingsClientOptions): SettingsClient { return new SettingsClient(options); }
