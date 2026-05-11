/** Lightweight Connectors SDK slice: connector catalog, auth, and action execution. */
export type ConnectorStatus = "connected" | "disconnected" | "error" | string;
export type ConnectorView = { id: string; name: string; description?: string; icon?: string; category?: string; auth_type?: string; beta?: boolean; supported: boolean; status: ConnectorStatus; user_info?: string; error?: string; action_count?: number; [key: string]: unknown };
export type ConnectorAction = { id: string; name?: string; description?: string; params?: unknown; [key: string]: unknown };
export type ConnectorDefinition = { id: string; name: string; description?: string; icon?: string; category?: string; auth_type?: string; beta?: boolean; actions?: ConnectorAction[]; [key: string]: unknown };
export type ConnectorListResponse = { connectors: ConnectorView[]; error?: string; [key: string]: unknown };
export type ConnectorDetailResponse = { connector: ConnectorDefinition; supported: boolean; status: ConnectorStatus; user_info?: string; error?: string; [key: string]: unknown };
export type ConnectorConnectRequest = { connector_id: string; token?: string; api_key?: string };
export type ConnectorConnectResponse = { ok: boolean; status: ConnectorStatus; user_info?: string; [key: string]: unknown };
export type ConnectorDisconnectResponse = { ok: boolean; [key: string]: unknown };
export type ConnectorExecuteRequest = { connector_id: string; action_id: string; params?: Record<string, unknown> };
export type ConnectorExecuteResponse<T = unknown> = { ok: boolean; result: T; [key: string]: unknown };
export type ConnectorsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class ConnectorsClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Connectors request failed with HTTP ${status}`); this.name = "ConnectorsClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, value); }

export class ConnectorsClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: ConnectorsClientOptions) { if (!options.baseUrl) throw new Error("ConnectorsClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("ConnectorsClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  list(): Promise<ConnectorListResponse> { return this.request<ConnectorListResponse>("GET", "/api/connectors"); }
  detail(id: string): Promise<ConnectorDetailResponse> { return this.request<ConnectorDetailResponse>("GET", "/api/connectors/detail", { query: { id } }); }
  connect(request: ConnectorConnectRequest): Promise<ConnectorConnectResponse> { return this.request<ConnectorConnectResponse>("POST", "/api/connectors/connect", { body: request }); }
  disconnect(connectorId: string): Promise<ConnectorDisconnectResponse> { return this.request<ConnectorDisconnectResponse>("POST", "/api/connectors/disconnect", { body: { connector_id: connectorId } }); }
  execute<T = unknown>(request: ConnectorExecuteRequest): Promise<ConnectorExecuteResponse<T>> { return this.request<ConnectorExecuteResponse<T>>("POST", "/api/connectors/execute", { body: { ...request, params: request.params ?? {} } }); }

  private async request<T>(method: "GET" | "POST", path: string, options: { body?: unknown; query?: Record<string, string | undefined>; headers?: HeadersInit } = {}): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = mergeHeaders(this.headers, options.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new ConnectorsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createConnectorsClient(options: ConnectorsClientOptions): ConnectorsClient { return new ConnectorsClient(options); }
