/** Lightweight RBAC SDK slice: roles, assignments, and permission checks. */
export type RBACAction = "read" | "write" | "execute" | "delete" | "admin" | string;
export type RBACResource = "chat" | "memory" | "knowledge" | "tasks" | "workflows" | "plugins" | "settings" | "audit" | "trust" | "providers" | "users" | "billing" | string;

export type RBACPermission = { resource: RBACResource; action: RBACAction; conditions?: string[] };
export type RBACRole = { id: string; name: string; description?: string; permissions: RBACPermission[]; is_built_in?: boolean; created_at?: string; [key: string]: unknown };
export type RBACRolesResponse = { roles: RBACRole[]; total: number; [key: string]: unknown };
export type RBACDeletedResponse = { deleted: string; [key: string]: unknown };
export type RBACRoleBindingRequest = { subject_id: string; role_id: string; tenant_id?: string };
export type RBACRoleBindingResponse = { status: "assigned" | "revoked" | string; subject_id: string; role_id: string; [key: string]: unknown };
export type RBACCheckRequest = { subject_id?: string; tenant_id?: string; resource: RBACResource; action: RBACAction };
export type RBACCheckResponse = { allowed: boolean; subject_id: string; resource: RBACResource; action: RBACAction; [key: string]: unknown };
export type RBACMyRolesResponse = { subject_id: string; roles: RBACRole[]; total: number; [key: string]: unknown };
export type RBACClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class RBACClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `RBAC request failed with HTTP ${status}`); this.name = "RBACClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, value); }

export class RBACClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: RBACClientOptions) { if (!options.baseUrl) throw new Error("RBACClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("RBACClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  roles(): Promise<RBACRolesResponse> { return this.request<RBACRolesResponse>("GET", "/v1/rbac/roles"); }
  createRole(role: RBACRole): Promise<RBACRole> { return this.request<RBACRole>("POST", "/v1/rbac/roles", { body: role }); }
  deleteRole(id: string): Promise<RBACDeletedResponse> { return this.request<RBACDeletedResponse>("DELETE", "/v1/rbac/roles", { query: { id } }); }
  assignRole(request: RBACRoleBindingRequest): Promise<RBACRoleBindingResponse> { return this.request<RBACRoleBindingResponse>("POST", "/v1/rbac/assign", { body: request }); }
  revokeRole(request: RBACRoleBindingRequest): Promise<RBACRoleBindingResponse> { return this.request<RBACRoleBindingResponse>("POST", "/v1/rbac/revoke", { body: request }); }
  check(request: RBACCheckRequest): Promise<RBACCheckResponse> { return this.request<RBACCheckResponse>("POST", "/v1/rbac/check", { body: request }); }
  myRoles(): Promise<RBACMyRolesResponse> { return this.request<RBACMyRolesResponse>("GET", "/v1/rbac/my-roles"); }

  private async request<T>(method: "DELETE" | "GET" | "POST", path: string, options: { body?: unknown; query?: Record<string, string | undefined>; headers?: HeadersInit } = {}): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = mergeHeaders(this.headers, options.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new RBACClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createRBACClient(options: RBACClientOptions): RBACClient { return new RBACClient(options); }
