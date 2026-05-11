/** Lightweight Skill Market SDK slice: marketplace search, ranking, and stats. */
export type SkillMarketCategory = "general" | "education" | "coding" | "data" | "media" | "search" | "language" | "productivity" | "custom" | (string & {});
export type SkillMarketSkill = {
  name: string;
  version: string;
  description?: string;
  author?: string;
  category?: SkillMarketCategory;
  tags?: string[];
  license?: string;
  homepage?: string;
  deprecated?: boolean;
  installs?: number;
  rating?: number;
  rating_count?: number;
  created_at?: string;
  updated_at?: string;
  min_version?: string;
  dependencies?: string[];
  [key: string]: unknown;
};
export type SkillMarketSearchResponse = { skills: SkillMarketSkill[]; count?: number; [key: string]: unknown };
export type SkillMarketTopOptions = { n?: number; by?: "rating" | "popular" | (string & {}) };
export type SkillMarketStatsResponse = { total?: number; deprecated?: number; total_installs?: number; categories?: Record<string, number>; [key: string]: unknown };
export type SkillMarketClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class SkillMarketClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Skill market request failed with HTTP ${status}`); this.name = "SkillMarketClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | number | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined && value !== "") url.searchParams.set(key, String(value)); }

export class SkillMarketClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: SkillMarketClientOptions) { if (!options.baseUrl) throw new Error("SkillMarketClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("SkillMarketClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  search(q?: string): Promise<SkillMarketSearchResponse> { return this.request<SkillMarketSearchResponse>("/v1/market/search", { q }); }
  top(options: SkillMarketTopOptions = {}): Promise<SkillMarketSearchResponse> { return this.request<SkillMarketSearchResponse>("/v1/market/top", { n: options.n, by: options.by }); }
  stats(): Promise<SkillMarketStatsResponse> { return this.request<SkillMarketStatsResponse>("/v1/market/stats"); }

  private async request<T>(path: string, query?: Record<string, string | number | undefined>): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, query); const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const response = await this.fetchImpl(url, { method: "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new SkillMarketClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createSkillMarketClient(options: SkillMarketClientOptions): SkillMarketClient { return new SkillMarketClient(options); }
