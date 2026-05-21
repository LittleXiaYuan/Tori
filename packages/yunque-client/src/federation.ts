/** Lightweight Federation SDK slice. */
export type FederationPeer = { id?: string; url?: string; status?: string; [key: string]: unknown };
export type FederationPeersResponse = { local_id?: string; peers?: FederationPeer[] | null; error?: string; [key: string]: unknown };
export type FederationStatsResponse = { error?: string; [key: string]: unknown };
export type FederationCapabilityPayload = { agent_id?: string; agent_name?: string; endpoint?: string; models?: unknown[]; tools?: unknown[]; features?: string[]; adapters?: unknown[]; metadata?: Record<string, unknown>; [key: string]: unknown };
export type FederationCapabilitiesResponse = { local?: FederationCapabilityPayload | Record<string, unknown>; peers?: unknown[] | Record<string, unknown>; [key: string]: unknown };
export type FederationStatusResponse = { status: string; [key: string]: unknown };
export type FederationDiscoverRequest = { feature?: string; adapter?: string; intent?: string; min_tier?: string; features?: string[] };
export type FederationDiscoverResult = { peer_id?: string; agent_id?: string; models?: unknown[]; tools?: unknown[]; features?: string[]; reason?: string; [key: string]: unknown };
export type FederationDiscoverResponse = { results: FederationDiscoverResult[]; count: number };
export type FederationDelegatePayload = { peer_id?: string; agent_id?: string; intent?: string; input?: unknown; task?: unknown; metadata?: Record<string, unknown>; [key: string]: unknown };
export type FederationDelegateResponse = { status: string; result?: unknown; [key: string]: unknown };
export type FederationBridgeStatsResponse = { configured: boolean; [key: string]: unknown };
export type FederationClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class FederationClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Federation request failed with HTTP ${status}`); this.name = "FederationClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class FederationClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: FederationClientOptions) { if (!options.baseUrl) throw new Error("FederationClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("FederationClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }
  peers(): Promise<FederationPeersResponse> { return this.request<FederationPeersResponse>("GET", "/v1/federation/peers"); }
  stats(): Promise<FederationStatsResponse> { return this.request<FederationStatsResponse>("GET", "/v1/federation/stats"); }
  capabilities(): Promise<FederationCapabilitiesResponse> { return this.request<FederationCapabilitiesResponse>("GET", "/v1/federation/capabilities"); }
  updateCapabilities(body: FederationCapabilityPayload): Promise<FederationStatusResponse> { return this.request<FederationStatusResponse>("POST", "/v1/federation/capabilities", body); }
  discover(body: FederationDiscoverRequest): Promise<FederationDiscoverResponse> { return this.request<FederationDiscoverResponse>("POST", "/v1/federation/discover", body); }
  delegate(body: FederationDelegatePayload): Promise<FederationDelegateResponse> { return this.request<FederationDelegateResponse>("POST", "/v1/federation/delegate", body); }
  bridgeStats(): Promise<FederationBridgeStatsResponse> { return this.request<FederationBridgeStatsResponse>("GET", "/v1/federation/bridge/stats"); }
  broadcast(): Promise<FederationStatusResponse> { return this.request<FederationStatusResponse>("POST", "/v1/federation/broadcast", {}); }
  private async request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    const headers = mergeHeaders(this.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new FederationClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}
export function createFederationClient(options: FederationClientOptions): FederationClient { return new FederationClient(options); }
