/** Lightweight Plugins SDK slice. */
export type PluginSkillManifest = { name?: string; description?: string; parameters?: Record<string, unknown>; [key: string]: unknown };
export type PluginManifest = { name: string; description?: string; language?: string; system_prompt?: string; skills?: PluginSkillManifest[]; enabled?: boolean; [key: string]: unknown };
export type PluginsListResponse = { plugins: PluginManifest[]; [key: string]: unknown };
export type PluginToggleResponse = { name: string; enabled: boolean; skills_count: number; [key: string]: unknown };
export type PluginCreateRequest = { name: string; description?: string; language?: string; template?: string; system_prompt?: string; skills?: PluginSkillManifest[] };
export type PluginCreateResponse = { status: string; name: string; dir: string; full_path?: string; [key: string]: unknown };
export type PluginDeleteResponse = { status: string; name: string; [key: string]: unknown };
export type PluginFile = { name: string; content: string; size: number; [key: string]: unknown };
export type PluginFilesResponse = { files: PluginFile[]; builtin?: boolean; [key: string]: unknown };
export type PluginFileSaveResponse = { status: string; [key: string]: unknown };
export type PluginUIResponse = { tabs: unknown[]; [key: string]: unknown };
export type PluginReloadResponse = { status: string; skills: number; [key: string]: unknown };
export type PluginOpenFolderResponse = { ok: boolean; path: string; [key: string]: unknown };
export type PluginsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class PluginsClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) {
    super(message || `Plugins request failed with HTTP ${status}`);
    this.name = "PluginsClientError";
    this.status = status;
    this.body = body;
  }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class PluginsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: PluginsClientOptions) {
    if (!options.baseUrl) throw new Error("PluginsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("PluginsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  list(): Promise<PluginsListResponse> { return this.json<PluginsListResponse>("/v1/plugins"); }
  toggle(name: string, enabled: boolean): Promise<PluginToggleResponse> { return this.post<PluginToggleResponse>("/v1/plugins/toggle", { name, enabled }); }
  create(request: PluginCreateRequest): Promise<PluginCreateResponse> { return this.post<PluginCreateResponse>("/v1/plugins/create", request); }
  delete(name: string): Promise<PluginDeleteResponse> { const url = new URL(`${this.baseUrl}/v1/plugins/delete`); url.searchParams.set("name", name); return this.json<PluginDeleteResponse>(url, { method: "DELETE" }); }
  files(name: string): Promise<PluginFilesResponse> { const url = new URL(`${this.baseUrl}/v1/plugins/files`); url.searchParams.set("name", name); return this.json<PluginFilesResponse>(url); }
  saveFile(name: string, file: string, content: string, plugin?: string): Promise<PluginFileSaveResponse> { const url = new URL(`${this.baseUrl}/v1/plugins/files`); url.searchParams.set("name", name); return this.json<PluginFileSaveResponse>(url, { method: "PUT", body: JSON.stringify({ plugin, file, content }) }); }
  ui(): Promise<PluginUIResponse> { return this.json<PluginUIResponse>("/v1/plugins/ui"); }
  reload(): Promise<PluginReloadResponse> { return this.json<PluginReloadResponse>("/v1/plugins/reload", { method: "POST" }); }
  openFolder(name?: string): Promise<PluginOpenFolderResponse> { const url = new URL(`${this.baseUrl}/v1/plugins/open-folder`); if (name) url.searchParams.set("name", name); return this.json<PluginOpenFolderResponse>(url); }

  private post<T>(path: string, body: unknown): Promise<T> { return this.json<T>(path, { method: "POST", body: JSON.stringify(body) }); }
  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> { const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl; const headers = this.authHeaders(init.headers); if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json"); const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new PluginsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createPluginsClient(options: PluginsClientOptions): PluginsClient { return new PluginsClient(options); }
