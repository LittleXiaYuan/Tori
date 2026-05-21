/** Lightweight System/Ops SDK slice. */
export type SystemProbeStatus = "ok" | "degraded" | "ready" | "not_ready" | "healthy" | "unhealthy" | string;
export type SystemHealthResponse = { status: SystemProbeStatus; version?: string; breaker_state?: string; uptime_sec?: number; [key: string]: unknown };
export type SystemSubsystemCheck = { status: string; detail?: string };
export type SystemReadinessResponse = { status: SystemProbeStatus; version?: string; uptime_sec?: number; checks?: Record<string, SystemSubsystemCheck>; [key: string]: unknown };
export type SystemCognitiveHealthResponse = SystemReadinessResponse & { summary?: Record<string, number>; resources?: Record<string, unknown>; timestamp?: string };
export type SystemVersionResponse = { version?: string; git_commit?: string; build_date?: string; go_version?: string; os?: string; arch?: string; update_available?: boolean; latest_version?: string; latest_url?: string; [key: string]: unknown };
export type SystemInfoResponse = { system?: Record<string, unknown>; breaker?: { state?: string; failures?: number; [key: string]: unknown }; [key: string]: unknown };
export type SystemStatsResponse = { requests_total?: number; tenants?: number; skills?: number; plugins?: number; scheduler_jobs?: number; conversations?: number; memory?: unknown; [key: string]: unknown };
export type SystemMetricsResponse = Record<string, unknown>;
export type SystemCacheStatsResponse = { llm_response_cache?: unknown; [key: string]: unknown };
export type SystemModulesResponse = { modules: unknown[]; profile?: string; [key: string]: unknown };
export type SystemSBOMResponse = { bomFormat?: string; specVersion?: string; serialNumber?: string; version?: number; metadata?: Record<string, unknown>; components?: unknown[]; dependencies?: unknown[]; [key: string]: unknown };
export type SystemClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class SystemClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `System request failed with HTTP ${status}`); this.name = "SystemClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class SystemClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: SystemClientOptions) { if (!options.baseUrl) throw new Error("SystemClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("SystemClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  health(): Promise<SystemHealthResponse> { return this.request<SystemHealthResponse>("/healthz"); }
  livez(): Promise<SystemHealthResponse> { return this.request<SystemHealthResponse>("/livez"); }
  readyz(): Promise<SystemReadinessResponse> { return this.request<SystemReadinessResponse>("/readyz", { allowStatuses: [503] }); }
  cognitiveHealth(): Promise<SystemCognitiveHealthResponse> { return this.request<SystemCognitiveHealthResponse>("/healthz/cognitive", { allowStatuses: [503] }); }
  version(): Promise<SystemVersionResponse> { return this.request<SystemVersionResponse>("/v1/version"); }
  systemInfo(): Promise<SystemInfoResponse> { return this.request<SystemInfoResponse>("/v1/system/info"); }
  systemStats(): Promise<SystemStatsResponse> { return this.request<SystemStatsResponse>("/v1/system/stats"); }
  metrics(): Promise<SystemMetricsResponse> { return this.request<SystemMetricsResponse>("/v1/metrics"); }
  metricsPrometheus(): Promise<string> { return this.request<string>("/v1/metrics/prometheus"); }
  cacheStats(): Promise<SystemCacheStatsResponse> { return this.request<SystemCacheStatsResponse>("/v1/cache/stats"); }
  modules(): Promise<SystemModulesResponse> { return this.request<SystemModulesResponse>("/v1/modules"); }
  sbom(): Promise<SystemSBOMResponse> { return this.request<SystemSBOMResponse>("/sbom"); }
  private async request<T>(path: string, options?: { allowStatuses?: number[] }): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const response = await this.fetchImpl(url, { method: "GET", headers }); const parsed = await parseResponse(response);
    const allowed = response.ok || options?.allowStatuses?.includes(response.status); if (!allowed) throw new SystemClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}
export function createSystemClient(options: SystemClientOptions): SystemClient { return new SystemClient(options); }
