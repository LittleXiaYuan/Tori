/** Lightweight Admin SDK slice. */
export type DesktopConsoleResponse = { console_hidden: boolean; error?: string; [key: string]: unknown };
export type DesktopAutostartResponse = { autostart_enabled?: boolean; error?: string; [key: string]: unknown };
export type TenantRecord = { id?: string; name?: string; api_key?: string; role?: string; [key: string]: unknown };
export type TenantListResponse = { tenants: TenantRecord[]; count: number };
export type NLConfigRequest = { text: string; execute?: boolean };
export type NLConfigResponse = { status: "ok" | "partial" | string; result?: unknown; executed: boolean; [key: string]: unknown };
export type AdminClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class AdminClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Admin request failed with HTTP ${status}`); this.name = "AdminClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class AdminClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: AdminClientOptions) { if (!options.baseUrl) throw new Error("AdminClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("AdminClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  consoleStatus(): Promise<DesktopConsoleResponse> { return this.request<DesktopConsoleResponse>("GET", "/v1/desktop/console"); }
  toggleConsole(): Promise<DesktopConsoleResponse> { return this.request<DesktopConsoleResponse>("POST", "/v1/desktop/console", {}); }
  autostartStatus(): Promise<DesktopAutostartResponse> { return this.request<DesktopAutostartResponse>("GET", "/v1/desktop/autostart"); }
  toggleAutostart(): Promise<DesktopAutostartResponse> { return this.request<DesktopAutostartResponse>("POST", "/v1/desktop/autostart", {}); }
  listTenants(): Promise<TenantListResponse> { return this.request<TenantListResponse>("GET", "/v1/tenants"); }
  createTenant(name: string): Promise<TenantRecord> { return this.request<TenantRecord>("POST", "/v1/tenants", { name }); }
  nlConfig(body: NLConfigRequest): Promise<NLConfigResponse> { return this.request<NLConfigResponse>("POST", "/v1/nl-config", body, { allowStatuses: [422] }); }
  nlConfigTranslate(text: string): Promise<NLConfigResponse> { return this.request<NLConfigResponse>("POST", "/v1/nl-config/translate", { text, execute: false }, { allowStatuses: [422] }); }
  private async request<T>(method: "GET" | "POST", path: string, body?: unknown, options?: { allowStatuses?: number[] }): Promise<T> {
    const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); }
    const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), init); const parsed = await parseResponse(response); const allowed = response.ok || options?.allowStatuses?.includes(response.status); if (!allowed) throw new AdminClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}
export function createAdminClient(options: AdminClientOptions): AdminClient { return new AdminClient(options); }
