/** Lightweight Missions and Reflection SDK slice. */
export type MissionParseResult = { type: "task" | "workflow" | "cron" | "trigger" | (string & {}); name: string; description: string; config: Record<string, unknown>; confidence: number; explanation: string; [key: string]: unknown };
export type ExperienceSource = "task" | "interaction" | "reverie" | (string & {});
export type ExperienceOutcome = "success" | "failure" | "partial" | (string & {});
export type ExperienceListOptions = { q?: string; source?: ExperienceSource; category?: string; outcome?: ExperienceOutcome; limit?: number };
export type ReflectExperience = {
  id?: string;
  source?: ExperienceSource;
  source_id?: string;
  category?: string;
  outcome?: ExperienceOutcome;
  lesson?: string;
  context?: string;
  tags?: string[];
  created_at?: string;
  [key: string]: unknown;
};
export type ExperiencesResponse = { experiences: ReflectExperience[]; total: number; [key: string]: unknown };
export type ExperienceStatsResponse = { total?: number; by_source?: Record<string, number>; by_category?: Record<string, number>; by_outcome?: Record<string, number>; recent_7d?: number; [key: string]: unknown };
export type StrategiesOptions = { limit?: number };
export type StrategiesResponse = { strategies: string; [key: string]: unknown };
export type MissionsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class MissionsClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Missions request failed with HTTP ${status}`); this.name = "MissionsClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function setOptionalQuery(url: URL, key: string, value: string | undefined): void { if (value === undefined || value === "") return; url.searchParams.set(key, value); }
function setOptionalNumberQuery(url: URL, key: string, value: number | undefined): void { if (value === undefined || !Number.isFinite(value) || value <= 0) return; url.searchParams.set(key, String(Math.trunc(value))); }

export class MissionsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: MissionsClientOptions) {
    if (!options.baseUrl) throw new Error("MissionsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("MissionsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  parse(description: string): Promise<MissionParseResult> { return this.json<MissionParseResult>("/v1/missions/parse", { method: "POST", body: JSON.stringify({ description }) }); }
  experiences(options: ExperienceListOptions = {}): Promise<ExperiencesResponse> { const url = new URL(`${this.baseUrl}/v1/reflect/experiences`); setOptionalQuery(url, "q", options.q); setOptionalQuery(url, "source", options.source); setOptionalQuery(url, "category", options.category); setOptionalQuery(url, "outcome", options.outcome); setOptionalNumberQuery(url, "limit", options.limit); return this.json<ExperiencesResponse>(url); }
  experienceStats(): Promise<ExperienceStatsResponse> { const url = new URL(`${this.baseUrl}/v1/reflect/experiences`); url.searchParams.set("stats", "true"); return this.json<ExperienceStatsResponse>(url); }
  strategies(options: StrategiesOptions = {}): Promise<StrategiesResponse> { const url = new URL(`${this.baseUrl}/v1/reflect/strategies`); setOptionalNumberQuery(url, "limit", options.limit); return this.json<StrategiesResponse>(url); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> { const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl; const headers = this.authHeaders(init.headers); if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json"); const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new MissionsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createMissionsClient(options: MissionsClientOptions): MissionsClient { return new MissionsClient(options); }
