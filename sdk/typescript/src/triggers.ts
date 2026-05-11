/** Lightweight Triggers SDK slice. */
export type TriggerPayload = { event: string; text?: string; data?: Record<string, unknown>; timestamp?: string; [key: string]: unknown };
export type TriggerLegacy = { id?: string; name: string; [key: string]: unknown };
export type TriggerDef = { id?: string; name: string; tenant_id?: string; type?: string; status?: string; actions?: unknown[]; [key: string]: unknown };
export type TriggerListResponse<T = TriggerLegacy> = { triggers: T[]; total: number; [key: string]: unknown };
export type TriggerEmitResponse = { status: string; event: string; [key: string]: unknown };
export type TriggerDeleteResponse = { deleted: string; [key: string]: unknown };
export type TriggerRunsResponse = { runs: unknown[]; total: number; [key: string]: unknown };
export type TriggerEventsResponse = { events: unknown[]; total: number; [key: string]: unknown };
export type TriggerV2ListOptions = { tenantId?: string; type?: string; status?: string };
export type TriggerHistoryOptions = { triggerId?: string; limit?: number };
export type TriggersClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class TriggersClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Triggers request failed with HTTP ${status}`); this.name = "TriggersClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function setOptionalQuery(url: URL, key: string, value: string | number | undefined): void { if (value === undefined || value === "") return; url.searchParams.set(key, String(value)); }

export class TriggersClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: TriggersClientOptions) {
    if (!options.baseUrl) throw new Error("TriggersClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("TriggersClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  listLegacy(): Promise<TriggerListResponse<TriggerLegacy>> { return this.json<TriggerListResponse<TriggerLegacy>>("/v1/triggers"); }
  getLegacy(id: string): Promise<TriggerLegacy> { const url = new URL(`${this.baseUrl}/v1/triggers`); url.searchParams.set("id", id); return this.json<TriggerLegacy>(url); }
  createLegacy(trigger: TriggerLegacy): Promise<TriggerLegacy> { return this.json<TriggerLegacy>("/v1/triggers", { method: "POST", body: JSON.stringify(trigger) }); }
  deleteLegacy(id: string): Promise<TriggerDeleteResponse> { const url = new URL(`${this.baseUrl}/v1/triggers`); url.searchParams.set("id", id); return this.json<TriggerDeleteResponse>(url, { method: "DELETE" }); }
  emitLegacy(payload: TriggerPayload): Promise<TriggerEmitResponse> { return this.json<TriggerEmitResponse>("/v1/triggers/emit", { method: "POST", body: JSON.stringify(payload) }); }

  list(options: TriggerV2ListOptions = {}): Promise<TriggerListResponse<TriggerDef>> { const url = new URL(`${this.baseUrl}/v1/triggers/v2`); setOptionalQuery(url, "tenant_id", options.tenantId); setOptionalQuery(url, "type", options.type); setOptionalQuery(url, "status", options.status); return this.json<TriggerListResponse<TriggerDef>>(url); }
  get(id: string): Promise<TriggerDef> { const url = new URL(`${this.baseUrl}/v1/triggers/v2`); url.searchParams.set("id", id); return this.json<TriggerDef>(url); }
  create(trigger: TriggerDef): Promise<TriggerDef> { return this.json<TriggerDef>("/v1/triggers/v2", { method: "POST", body: JSON.stringify(trigger) }); }
  update(trigger: TriggerDef): Promise<TriggerDef> { return this.json<TriggerDef>("/v1/triggers/v2", { method: "PUT", body: JSON.stringify(trigger) }); }
  delete(id: string): Promise<TriggerDeleteResponse> { const url = new URL(`${this.baseUrl}/v1/triggers/v2`); url.searchParams.set("id", id); return this.json<TriggerDeleteResponse>(url, { method: "DELETE" }); }
  emit(payload: TriggerPayload): Promise<TriggerEmitResponse> { return this.json<TriggerEmitResponse>("/v1/triggers/v2/emit", { method: "POST", body: JSON.stringify(payload) }); }
  runs(options: TriggerHistoryOptions = {}): Promise<TriggerRunsResponse> { const url = new URL(`${this.baseUrl}/v1/triggers/v2/runs`); setOptionalQuery(url, "trigger_id", options.triggerId); setOptionalQuery(url, "limit", options.limit); return this.json<TriggerRunsResponse>(url); }
  events(options: TriggerHistoryOptions = {}): Promise<TriggerEventsResponse> { const url = new URL(`${this.baseUrl}/v1/triggers/v2/events`); setOptionalQuery(url, "trigger_id", options.triggerId); setOptionalQuery(url, "limit", options.limit); return this.json<TriggerEventsResponse>(url); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> { const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl; const headers = this.authHeaders(init.headers); if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json"); const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new TriggersClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createTriggersClient(options: TriggersClientOptions): TriggersClient { return new TriggersClient(options); }
