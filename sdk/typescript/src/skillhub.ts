/** Lightweight SkillHub SDK slice. */
export type SkillHubSource = "local" | "clawhub" | "torihub" | (string & {});
export type SkillHubListOptions = { limit?: number; source?: SkillHubSource };
export type SkillHubSearchOptions = SkillHubListOptions & { q: string };
export type SkillHubTrendingOptions = SkillHubListOptions & { cursor?: string };
export type SkillHubSkillSummary = {
  name: string;
  description?: string;
  version?: string;
  author?: string;
  rating?: number;
  source?: SkillHubSource;
  installed?: boolean;
  [key: string]: unknown;
};
export type SkillHubSearchResponse = { results: SkillHubSkillSummary[]; count: number; [key: string]: unknown };
export type SkillHubTrendingResponse = { skills: SkillHubSkillSummary[]; count: number; next_cursor?: string; [key: string]: unknown };
export type SkillHubInstalledSkill = { name?: string; slug?: string; version?: string; source?: string; permissions?: string[]; security_score?: number; [key: string]: unknown };
export type SkillHubInstalledResponse = { skills: SkillHubInstalledSkill[]; count: number; [key: string]: unknown };
export type SkillHubInstallResponse = { status: string; slug: string; report?: unknown; [key: string]: unknown };
export type SkillHubUninstallResponse = { status: string; slug: string; [key: string]: unknown };
export type SkillHubDetailResponse = SkillHubSkillSummary & { slug: string; rating_count?: number; installs?: number; category?: string; tags?: string[]; license?: string; permissions?: string[]; security_score?: number; audit_report?: unknown; content?: string; installed_at?: string; updated_at?: string };
export type SkillHubUpdatesResponse = { updates: unknown[]; [key: string]: unknown };
export type SkillHubUpdateResponse = { ok: boolean; report?: unknown; [key: string]: unknown };
export type SkillHubRollbackResponse = { ok: boolean; [key: string]: unknown };
export type SkillHubVersionsResponse = { versions: unknown[]; [key: string]: unknown };
export type SkillHubPolicyResponse = Record<string, unknown>;
export type SkillHubPolicyUpdateResponse = { ok: boolean; [key: string]: unknown };
export type SkillHubPolicyCheckResponse = { allowed?: boolean; reason?: string; [key: string]: unknown };
export type SkillHubAnalyticsResponse = { total_skills?: number; installed_count?: number; total_installs?: number; avg_score?: number; categories?: Record<string, number>; top_installed?: unknown[]; top_rated?: unknown[]; security_stats?: Record<string, number>; [key: string]: unknown };
export type SkillHubClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class SkillHubClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) {
    super(message || `SkillHub request failed with HTTP ${status}`);
    this.name = "SkillHubClientError";
    this.status = status;
    this.body = body;
  }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers {
  const headers = new Headers(base);
  if (!extra) return headers;
  new Headers(extra).forEach((value, key) => headers.set(key, value));
  return headers;
}
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined {
  if (typeof body === "string" && body.trim()) return body.trim();
  if (!isRecord(body)) return undefined;
  for (const key of ["message", "detail", "error", "reason"]) {
    const value = body[key];
    if (typeof value === "string" && value.trim()) return value;
  }
  return undefined;
}
async function parseResponse(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return undefined;
  try { return JSON.parse(text); } catch { return text; }
}
function setOptionalQuery(url: URL, key: string, value: string | number | undefined): void {
  if (value === undefined || value === "") return;
  url.searchParams.set(key, String(value));
}

export class SkillHubClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: SkillHubClientOptions) {
    if (!options.baseUrl) throw new Error("SkillHubClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("SkillHubClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  search(options: SkillHubSearchOptions): Promise<SkillHubSearchResponse> {
    const url = new URL(`${this.baseUrl}/api/skillhub/search`);
    url.searchParams.set("q", options.q);
    setOptionalQuery(url, "limit", options.limit);
    setOptionalQuery(url, "source", options.source);
    return this.json<SkillHubSearchResponse>(url);
  }

  installed(): Promise<SkillHubInstalledResponse> { return this.json<SkillHubInstalledResponse>("/api/skillhub/installed"); }
  install(slug: string): Promise<SkillHubInstallResponse> { return this.post<SkillHubInstallResponse>("/api/skillhub/install", { slug }); }
  uninstall(slug: string, method: "POST" | "DELETE" = "POST"): Promise<SkillHubUninstallResponse> { return this.json<SkillHubUninstallResponse>("/api/skillhub/uninstall", { method, body: JSON.stringify({ slug }) }); }

  trending(options: SkillHubTrendingOptions = {}): Promise<SkillHubTrendingResponse> {
    const url = new URL(`${this.baseUrl}/api/skillhub/trending`);
    setOptionalQuery(url, "limit", options.limit);
    setOptionalQuery(url, "source", options.source);
    setOptionalQuery(url, "cursor", options.cursor);
    return this.json<SkillHubTrendingResponse>(url);
  }

  detail(slug: string): Promise<SkillHubDetailResponse> {
    const url = new URL(`${this.baseUrl}/api/skillhub/detail`);
    url.searchParams.set("slug", slug);
    return this.json<SkillHubDetailResponse>(url);
  }

  checkUpdates(): Promise<SkillHubUpdatesResponse> { return this.json<SkillHubUpdatesResponse>("/api/skillhub/check-updates"); }
  update(slug: string): Promise<SkillHubUpdateResponse> { return this.post<SkillHubUpdateResponse>("/api/skillhub/update", { slug }); }
  rollback(slug: string, version: string): Promise<SkillHubRollbackResponse> { return this.post<SkillHubRollbackResponse>("/api/skillhub/rollback", { slug, version }); }

  versions(slug: string): Promise<SkillHubVersionsResponse> {
    const url = new URL(`${this.baseUrl}/api/skillhub/versions`);
    url.searchParams.set("slug", slug);
    return this.json<SkillHubVersionsResponse>(url);
  }

  policy(): Promise<SkillHubPolicyResponse> { return this.json<SkillHubPolicyResponse>("/api/skillhub/policy"); }
  updatePolicy(policy: Record<string, unknown>): Promise<SkillHubPolicyUpdateResponse> { return this.post<SkillHubPolicyUpdateResponse>("/api/skillhub/policy", policy); }
  policyCheck(slug: string): Promise<SkillHubPolicyCheckResponse> { const url = new URL(`${this.baseUrl}/api/skillhub/policy/check`); url.searchParams.set("slug", slug); return this.json<SkillHubPolicyCheckResponse>(url); }
  analytics(): Promise<SkillHubAnalyticsResponse> { return this.json<SkillHubAnalyticsResponse>("/api/skillhub/analytics"); }

  private post<T>(path: string, body: unknown): Promise<T> { return this.json<T>(path, { method: "POST", body: JSON.stringify(body) }); }

  private authHeaders(extra?: HeadersInit): Headers {
    const headers = mergeHeaders(this.headers, extra);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    return headers;
  }

  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> {
    const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl;
    const headers = this.authHeaders(init.headers);
    if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json");
    const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers });
    const parsed = await parseResponse(response);
    if (!response.ok) throw new SkillHubClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createSkillHubClient(options: SkillHubClientOptions): SkillHubClient { return new SkillHubClient(options); }
