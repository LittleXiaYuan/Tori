/** Lightweight Cognis SDK slice: Cogni registry, health, traces, evolution and federation controls. */
export type CogniDeclaration = { id?: string; name?: string; description?: string; enabled?: boolean; [key: string]: unknown };
export type CogniListResponse = { cognis?: CogniDeclaration[]; items?: CogniDeclaration[]; count?: number; [key: string]: unknown };
export type CogniMutationResponse = { status?: "ok" | string; id?: string; [key: string]: unknown };
export type CogniTraceResponse = { traces?: unknown[]; events?: unknown[]; count?: number; [key: string]: unknown };
export type CogniStatsResponse = Record<string, unknown>;
export type CogniHealthResponse = Record<string, unknown>;
export type CogniAlertsResponse = { alerts?: unknown[]; count?: number; [key: string]: unknown };
export type CogniVerifyResponse = Record<string, unknown>;
export type CogniWorkflowRunRequest = Record<string, unknown>;
export type CogniExperienceStats = { tool_memories?: number; patterns_total?: number; patterns_confirmed?: number; patterns_pending?: number; domain_facts?: number; [key: string]: unknown };
export type CogniToolExperience = { tool?: string; context?: string; result?: string; learned?: string; confidence?: number; verified_by?: string; used_count?: number; success_rate?: number; created_at?: string; last_used?: string; [key: string]: unknown };
export type CogniExperiencePattern = { id?: string; trigger?: string; response?: string; confirmed?: boolean; used_count?: number; success_rate?: number; created_at?: string; last_used?: string; [key: string]: unknown };
export type CogniDomainFact = { fact?: string; source?: string; used_count?: number; created_at?: string; last_used?: string; [key: string]: unknown };
export type CogniExperienceSummary = { stats?: CogniExperienceStats; top_tools?: CogniToolExperience[]; top_facts?: CogniDomainFact[]; pending_patterns?: CogniExperiencePattern[]; updated_at?: string; [key: string]: unknown };
export type CogniExperienceRecordType = "tool_memory" | "pattern" | "fact" | string;
export type CogniExperienceRecordRequest = { type: CogniExperienceRecordType; data: Record<string, unknown> };
export type CogniExperienceResponse = { id?: string; enabled?: boolean; summary?: CogniExperienceSummary; stats?: CogniExperienceStats; tool_memory?: CogniToolExperience[]; patterns?: CogniExperiencePattern[]; domain_facts?: CogniDomainFact[]; [key: string]: unknown };
export type CogniRuntimePackStateResponse = { pack_id?: string; stage?: string; pack_installed?: boolean; pack_enabled?: boolean; pack_status?: string; runtime_loop_pack_state_ready?: boolean; runtime_loop_running?: boolean; stops_runtime_loops?: boolean; starts_runtime_loops?: boolean; clears_runtime_state?: boolean; sentinel_ready?: boolean; scheduler_ready?: boolean; bus_ready?: boolean; experience_store_ready?: boolean; active_bus_cognis?: number; experience_store_count?: number; generated_at?: string; capabilities?: string[]; artifacts?: string[]; notes?: string[]; [key: string]: unknown };
export type CognisClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class CognisClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Cognis request failed with HTTP ${status}`); this.name = "CognisClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | number | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined && value !== "") url.searchParams.set(key, String(value)); }
function enc(value: string): string { return encodeURIComponent(value); }

export class CognisClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: CognisClientOptions) { if (!options.baseUrl) throw new Error("CognisClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("CognisClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  list(): Promise<CogniListResponse> { return this.request<CogniListResponse>("GET", "/v1/cognis"); }
  create(declaration: CogniDeclaration): Promise<CogniDeclaration> { return this.request<CogniDeclaration>("POST", "/v1/cognis", declaration); }
  get(id: string): Promise<CogniDeclaration> { return this.request<CogniDeclaration>("GET", `/v1/cognis/${enc(id)}`); }
  remove(id: string): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("DELETE", `/v1/cognis/${enc(id)}`); }
  enable(id: string): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", `/v1/cognis/${enc(id)}/enable`); }
  disable(id: string): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", `/v1/cognis/${enc(id)}/disable`); }
  reload(): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", "/v1/cognis/reload"); }
  runtimePackState(): Promise<CogniRuntimePackStateResponse> { return this.request<CogniRuntimePackStateResponse>("GET", "/v1/cognis/runtime/pack-state"); }

  traces(limit?: number): Promise<CogniTraceResponse> { return this.request<CogniTraceResponse>("GET", "/v1/cognis/traces", undefined, { limit }); }
  trace(id: string, limit?: number): Promise<CogniTraceResponse> { return this.request<CogniTraceResponse>("GET", `/v1/cognis/${enc(id)}/trace`, undefined, { limit }); }
  stats(): Promise<CogniStatsResponse> { return this.request<CogniStatsResponse>("GET", "/v1/cognis/stats"); }
  health(id?: string): Promise<CogniHealthResponse> { return this.request<CogniHealthResponse>("GET", id ? `/v1/cognis/${enc(id)}/health` : "/v1/cognis/health"); }
  verify(id?: string): Promise<CogniVerifyResponse> { return this.request<CogniVerifyResponse>("GET", id ? `/v1/cognis/${enc(id)}/verify` : "/v1/cognis/verify"); }

  alerts(): Promise<CogniAlertsResponse> { return this.request<CogniAlertsResponse>("GET", "/v1/cognis/alerts"); }
  scanAlerts(): Promise<CogniAlertsResponse> { return this.request<CogniAlertsResponse>("POST", "/v1/cognis/alerts/scan"); }
  generate(request: Record<string, unknown>): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", "/v1/cognis/generate", request); }
  exportBundle(ids?: string[], notes?: string): Promise<Record<string, unknown>> {
    const query: Record<string, string | undefined> = {};
    if (ids && ids.length > 0) query.ids = ids.join(",");
    if (notes) query.notes = notes;
    return this.request<Record<string, unknown>>("GET", "/v1/cognis/export", undefined, query);
  }
  importBundle(bundle: Record<string, unknown>): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", "/v1/cognis/import", bundle); }

  workflows(id: string): Promise<Record<string, unknown>> { return this.request<Record<string, unknown>>("GET", `/v1/cognis/${enc(id)}/workflows`); }
  runWorkflow(id: string, workflow: string, request: CogniWorkflowRunRequest = {}): Promise<Record<string, unknown>> { return this.request<Record<string, unknown>>("POST", `/v1/cognis/${enc(id)}/workflow/${enc(workflow)}`, request); }
  experience(id: string): Promise<CogniExperienceResponse> { return this.request<CogniExperienceResponse>("GET", `/v1/cognis/${enc(id)}/experience`); }
  recordExperience(id: string, request: CogniExperienceRecordRequest): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", `/v1/cognis/${enc(id)}/experience/record`, request); }
  confirmExperiencePattern(id: string, patternId: string): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", `/v1/cognis/${enc(id)}/experience/patterns/${enc(patternId)}/confirm`); }
  evolve(id: string, request: Record<string, unknown> = {}): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", `/v1/cognis/${enc(id)}/evolve`, request); }
  evolution(id?: string): Promise<Record<string, unknown>> { return this.request<Record<string, unknown>>("GET", id ? `/v1/cognis/${enc(id)}/evolution` : "/v1/cognis/evolution"); }

  federation(): Promise<Record<string, unknown>> { return this.request<Record<string, unknown>>("GET", "/v1/cognis/federation"); }
  federationPeers(): Promise<Record<string, unknown>> { return this.request<Record<string, unknown>>("GET", "/v1/cognis/federation/peers"); }
  discoverFederation(request: Record<string, unknown> = {}): Promise<Record<string, unknown>> { return this.request<Record<string, unknown>>("POST", "/v1/cognis/federation/discover", request); }
  expose(id: string): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", `/v1/cognis/${enc(id)}/expose`); }
  unexpose(id: string): Promise<CogniMutationResponse> { return this.request<CogniMutationResponse>("POST", `/v1/cognis/${enc(id)}/unexpose`); }
  economics(): Promise<Record<string, unknown>> { return this.request<Record<string, unknown>>("GET", "/v1/cognis/economics"); }

  private async request<T>(method: "DELETE" | "GET" | "POST", path: string, body?: unknown, query?: Record<string, string | number | undefined>): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, query); const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new CognisClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createCognisClient(options: CognisClientOptions): CognisClient { return new CognisClient(options); }
