/** Lightweight Orchestrator SDK slice: IDE worker daemon status, sessions, events, and policy. */
export type OrchestratorPolicy = { allow_auto_launch?: boolean; allow_auto_review?: boolean; allow_auto_requeue?: boolean; require_approval?: boolean; [key: string]: unknown };
export type OrchestratorStatusResponse = { running: boolean; adapters: string[]; active_sessions: number; policy?: OrchestratorPolicy; event_count?: number; [key: string]: unknown };
export type OrchestratorToggleAction = "start" | "stop";
export type OrchestratorToggleResponse = { status: "started" | "stopped" | string; [key: string]: unknown };
export type OrchestratorSession = { session_id: string; adapter: string; task_id: string; started_at: string; [key: string]: unknown };
export type OrchestratorSessionsResponse = { sessions: OrchestratorSession[]; [key: string]: unknown };
export type OrchestratorIDE = { name?: string; path?: string; available?: boolean; version?: string; [key: string]: unknown };
export type OrchestratorDetectResponse = { ides: OrchestratorIDE[]; [key: string]: unknown };
export type OrchestratorEvent = { id: string; type: string; task_id?: string; worker_id?: string; session_id?: string; message: string; meta?: Record<string, unknown>; timestamp: string; [key: string]: unknown };
export type OrchestratorEventsResponse = { events: OrchestratorEvent[]; total?: number; [key: string]: unknown };
export type OrchestratorTaskTimelineResponse = { task_id: string; events: OrchestratorEvent[]; [key: string]: unknown };
export type OrchestratorPolicyUpdateResponse = { status: "updated" | string; policy: OrchestratorPolicy; [key: string]: unknown };
export type OrchestratorAdapterConfig = { adapter_name: string; binary: string; launch_args?: string; mcp_config_path: string; rules_file_path?: string; lifecycle?: "ephemeral" | "persistent" | string; [key: string]: unknown };
export type OrchestratorAdapterResponse = { status: "registered" | string; name: string; available: boolean; [key: string]: unknown };
export type OrchestratorClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class OrchestratorClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Orchestrator request failed with HTTP ${status}`); this.name = "OrchestratorClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | number | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined && value !== "") url.searchParams.set(key, String(value)); }

export class OrchestratorClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: OrchestratorClientOptions) { if (!options.baseUrl) throw new Error("OrchestratorClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("OrchestratorClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  status(): Promise<OrchestratorStatusResponse> { return this.request<OrchestratorStatusResponse>("GET", "/v1/orchestrator/status"); }
  toggle(action: OrchestratorToggleAction): Promise<OrchestratorToggleResponse> { return this.request<OrchestratorToggleResponse>("POST", "/v1/orchestrator/toggle", { body: { action } }); }
  sessions(): Promise<OrchestratorSessionsResponse> { return this.request<OrchestratorSessionsResponse>("GET", "/v1/orchestrator/sessions"); }
  detectIDEs(): Promise<OrchestratorDetectResponse> { return this.request<OrchestratorDetectResponse>("GET", "/v1/orchestrator/detect"); }
  events(limit?: number): Promise<OrchestratorEventsResponse> { return this.request<OrchestratorEventsResponse>("GET", "/v1/orchestrator/events", { query: { limit } }); }
  taskTimeline(taskId: string): Promise<OrchestratorTaskTimelineResponse> { return this.request<OrchestratorTaskTimelineResponse>("GET", "/v1/orchestrator/events/task", { query: { task_id: taskId } }); }
  policy(): Promise<OrchestratorPolicy> { return this.request<OrchestratorPolicy>("GET", "/v1/orchestrator/policy"); }
  updatePolicy(policy: OrchestratorPolicy): Promise<OrchestratorPolicyUpdateResponse> { return this.request<OrchestratorPolicyUpdateResponse>("PUT", "/v1/orchestrator/policy", { body: policy }); }
  addAdapter(config: OrchestratorAdapterConfig): Promise<OrchestratorAdapterResponse> { return this.request<OrchestratorAdapterResponse>("POST", "/v1/orchestrator/adapters/add", { body: config }); }

  private async request<T>(method: "GET" | "POST" | "PUT", path: string, options: { body?: unknown; query?: Record<string, string | number | undefined>; headers?: HeadersInit } = {}): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = mergeHeaders(this.headers, options.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new OrchestratorClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createOrchestratorClient(options: OrchestratorClientOptions): OrchestratorClient { return new OrchestratorClient(options); }
