/** Lightweight Pack Runtime SDK slice for frontend shell sync and automation scripts. */
export type PackStatus = "installed" | "enabled" | "disabled" | string;
export type PackBackendManifest = { capabilities?: string[]; routes?: string[]; permissions?: string[]; [key: string]: unknown };
export type PackFrontendMenu = { key: string; label: string; path: string; icon?: string; order?: number; [key: string]: unknown };
export type PackFrontendRoute = { path: string; component: string; title?: string; [key: string]: unknown };
export type PackFrontendAssets = { type?: string; entry?: string; [key: string]: unknown };
export type PackFrontendManifest = { menus?: PackFrontendMenu[]; routes?: PackFrontendRoute[]; assets?: PackFrontendAssets; [key: string]: unknown };
export type PackSdkManifest = { typescript?: string; go?: string; python?: string; [key: string]: unknown };
export type PackUpdateManifest = { channel?: string; rollback?: boolean; [key: string]: unknown };
export type PackManifest = { id: string; name: string; version: string; description?: string; requiresCore?: string; optional?: boolean; defaultState?: string; backend?: PackBackendManifest; frontend?: PackFrontendManifest; sdk?: PackSdkManifest; update?: PackUpdateManifest; metadata?: Record<string, string>; [key: string]: unknown };
export type InstalledPack = { manifest: PackManifest; status: PackStatus; source?: string; installedAt?: string; updatedAt?: string; previousVersion?: string; [key: string]: unknown };
export type PacksListResponse = { packs: InstalledPack[]; enabled?: InstalledPack[]; count: number; [key: string]: unknown };
export type PackMutationResponse = { pack: InstalledPack; status: PackStatus; [key: string]: unknown };
export type PackInstallRequest = { manifestPath: string; source?: string };
export type PacksClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class PacksClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Packs request failed with HTTP ${status}`); this.name = "PacksClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class PacksClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: PacksClientOptions) { if (!options.baseUrl) throw new Error("PacksClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("PacksClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  installed(): Promise<PacksListResponse> { return this.json<PacksListResponse>("GET", "/v1/packs/installed"); }
  list(): Promise<PacksListResponse> { return this.json<PacksListResponse>("GET", "/v1/packs"); }
  enabled(): Promise<PacksListResponse> { return this.json<PacksListResponse>("GET", "/v1/packs/enabled"); }
  install(request: PackInstallRequest): Promise<PackMutationResponse> { return this.json<PackMutationResponse>("POST", "/v1/packs/install", { manifest_path: request.manifestPath, source: request.source }); }
  enable(id: string): Promise<PackMutationResponse> { return this.mutate("/v1/packs/enable", id); }
  disable(id: string): Promise<PackMutationResponse> { return this.mutate("/v1/packs/disable", id); }
  rollback(id: string): Promise<PackMutationResponse> { return this.mutate("/v1/packs/rollback", id); }
  async frontendSync(): Promise<{ menus: PackFrontendMenu[]; routes: PackFrontendRoute[]; packs: InstalledPack[] }> { const response = await this.enabled(); const packs = response.packs ?? []; return { packs, menus: packs.flatMap((pack) => pack.manifest.frontend?.menus ?? []).sort((a, b) => (a.order ?? 0) - (b.order ?? 0)), routes: packs.flatMap((pack) => pack.manifest.frontend?.routes ?? []) }; }
  private mutate(path: string, id: string): Promise<PackMutationResponse> { return this.json<PackMutationResponse>("POST", path, { id }); }
  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> { const headers = this.authHeaders(); const init: RequestInit = { method, headers }; if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); } const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), init); const parsed = await parseResponse(response); if (!response.ok) throw new PacksClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}
export function createPacksClient(options: PacksClientOptions): PacksClient { return new PacksClient(options); }
