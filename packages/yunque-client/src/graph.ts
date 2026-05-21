/** Lightweight Knowledge Graph SDK slice. */
export type GraphEntity = { id?: string; name: string; type?: string; properties?: Record<string, string>; created_at?: string; updated_at?: string; mentions?: number; [key: string]: unknown };
export type GraphRelation = { id?: string; from_id: string; to_id: string; type: string; weight?: number; context?: string; created_at?: string; [key: string]: unknown };
export type GraphEntitiesResponse = { entities: GraphEntity[]; [key: string]: unknown };
export type GraphRelationsResponse = { relations: GraphRelation[]; [key: string]: unknown };
export type GraphDeleteEntityResponse = { ok: boolean; [key: string]: unknown };
export type GraphContextResponse = { context: string; neighbors?: unknown[]; [key: string]: unknown };
export type GraphStatsResponse = { entities: number; relations: number; [key: string]: unknown };
export type GraphClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class GraphClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Graph request failed with HTTP ${status}`); this.name = "GraphClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function setOptionalQuery(url: URL, key: string, value: string | undefined): void { if (value === undefined || value === "") return; url.searchParams.set(key, value); }

export class GraphClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: GraphClientOptions) {
    if (!options.baseUrl) throw new Error("GraphClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("GraphClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  entities(query?: string): Promise<GraphEntitiesResponse> { const url = new URL(`${this.baseUrl}/v1/graph/entities`); setOptionalQuery(url, "q", query); return this.json<GraphEntitiesResponse>(url); }
  putEntity(entity: GraphEntity): Promise<GraphEntity> { return this.json<GraphEntity>("/v1/graph/entities", { method: "POST", body: JSON.stringify(entity) }); }
  deleteEntity(id: string): Promise<GraphDeleteEntityResponse> { const url = new URL(`${this.baseUrl}/v1/graph/entities`); url.searchParams.set("id", id); return this.json<GraphDeleteEntityResponse>(url, { method: "DELETE" }); }
  relations(entityId?: string): Promise<GraphRelationsResponse> { const url = new URL(`${this.baseUrl}/v1/graph/relations`); setOptionalQuery(url, "entity_id", entityId); return this.json<GraphRelationsResponse>(url); }
  putRelation(relation: GraphRelation): Promise<GraphRelation> { return this.json<GraphRelation>("/v1/graph/relations", { method: "POST", body: JSON.stringify(relation) }); }
  contextByEntityId(entityId: string): Promise<GraphContextResponse> { const url = new URL(`${this.baseUrl}/v1/graph/context`); url.searchParams.set("entity_id", entityId); return this.json<GraphContextResponse>(url); }
  contextByName(name: string): Promise<GraphContextResponse> { const url = new URL(`${this.baseUrl}/v1/graph/context`); url.searchParams.set("name", name); return this.json<GraphContextResponse>(url); }
  stats(): Promise<GraphStatsResponse> { return this.json<GraphStatsResponse>("/v1/graph/stats"); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> { const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl; const headers = this.authHeaders(init.headers); if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json"); const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new GraphClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createGraphClient(options: GraphClientOptions): GraphClient { return new GraphClient(options); }
