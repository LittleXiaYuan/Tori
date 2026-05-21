/** Lightweight Skills SDK slice: runtime skills catalog, dynamic skill review, scan, and suggestions. */
export type SkillInfo = { name: string; description: string; parameters?: Record<string, unknown>; category?: string; usage_total?: number; success_rate?: number; [key: string]: unknown };
export type SkillCategory = { id: string; name: string; description?: string; [key: string]: unknown };
export type SkillsResponse = { skills: SkillInfo[]; count: number; categories?: SkillCategory[]; [key: string]: unknown };
export type DynamicSkill = { name: string; instruction?: string; description?: string; approval_status?: string; [key: string]: unknown };
export type DynamicSkillsResponse = { skills: DynamicSkill[]; [key: string]: unknown };
export type SkillApproveRequest = { name: string; instruction?: string };
export type SkillRejectRequest = { name: string };
export type SkillMutationResponse = { status: "ok" | string; [key: string]: unknown };
export type SkillScanResponse = { status: "scanned" | string; skills_loaded?: number; total_skills?: number; [key: string]: unknown };
export type SkillSuggestion = { name?: string; description?: string; instruction?: string; [key: string]: unknown };
export type SkillSuggestionsResponse = { suggestions: SkillSuggestion[]; [key: string]: unknown };
export type SkillsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class SkillsClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Skills request failed with HTTP ${status}`); this.name = "SkillsClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function setOptionalQuery(url: URL, key: string, value: string | undefined): void { if (value === undefined || value === "") return; url.searchParams.set(key, value); }

export class SkillsClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: SkillsClientOptions) { if (!options.baseUrl) throw new Error("SkillsClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("SkillsClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  list(): Promise<SkillsResponse> { return this.request<SkillsResponse>("GET", "/v1/skills"); }
  scan(): Promise<SkillScanResponse> { return this.request<SkillScanResponse>("POST", "/v1/skills/scan"); }
  dynamic(): Promise<DynamicSkillsResponse> { return this.request<DynamicSkillsResponse>("GET", "/v1/skills/dynamic"); }
  approve(request: SkillApproveRequest): Promise<SkillMutationResponse> { return this.request<SkillMutationResponse>("POST", "/v1/skills/approve", request); }
  reject(nameOrRequest: string | SkillRejectRequest): Promise<SkillMutationResponse> { const body = typeof nameOrRequest === "string" ? { name: nameOrRequest } : nameOrRequest; return this.request<SkillMutationResponse>("POST", "/v1/skills/reject", body); }
  suggestions(sessionId?: string): Promise<SkillSuggestionsResponse> { return this.request<SkillSuggestionsResponse>("GET", "/v1/skill-suggestions", undefined, { session_id: sessionId }); }

  private async request<T>(method: "GET" | "POST", path: string, body?: unknown, query?: Record<string, string | undefined>): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); if (query) for (const [key, value] of Object.entries(query)) setOptionalQuery(url, key, value);
    const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new SkillsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createSkillsClient(options: SkillsClientOptions): SkillsClient { return new SkillsClient(options); }
