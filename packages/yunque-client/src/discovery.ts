/** Lightweight Discovery SDK slice: identity, embeddings and web search. */
export type IdentityProfile = { unified_id: string; display_name?: string; channels?: Record<string, string>; metadata?: Record<string, string>; first_seen?: string; last_seen?: string; message_count?: number; [key: string]: unknown };
export type ResolveIdentityRequest = { channel: string; user_id: string; display_name?: string };
export type IdentityProfilesResponse = { profiles: IdentityProfile[]; [key: string]: unknown };

export type EmbeddingProvider = string | Record<string, unknown>;
export type EmbeddingProvidersResponse = { providers: EmbeddingProvider[]; [key: string]: unknown };
export type EmbeddingResponse = { embedding: number[]; dimensions: number; model?: string; [key: string]: unknown };

export type SearchResult = { title?: string; url?: string; snippet?: string; [key: string]: unknown };
export type SearchResponse = { results: SearchResult[] | unknown; total?: number; enabled?: boolean; providers?: string[]; [key: string]: unknown };
export type SearchProvidersResponse = { enabled: boolean; providers: string[]; [key: string]: unknown };

export type DiscoveryClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class DiscoveryClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Discovery request failed with HTTP ${status}`); this.name = "DiscoveryClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | number | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, String(value)); }

export class DiscoveryClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: DiscoveryClientOptions) {
    if (!options.baseUrl) throw new Error("DiscoveryClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("DiscoveryClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  resolveIdentity(request: ResolveIdentityRequest): Promise<IdentityProfile> { return this.json<IdentityProfile>("/v1/identity/resolve", { method: "POST", body: { channel: request.channel, user_id: request.user_id, display_name: request.display_name ?? "" } }); }
  identityProfiles(): Promise<IdentityProfilesResponse> { return this.json<IdentityProfilesResponse>("/v1/identity/profiles"); }
  embeddingProviders(): Promise<EmbeddingProvidersResponse> { return this.json<EmbeddingProvidersResponse>("/v1/embeddings"); }
  embed(text: string, provider?: string): Promise<EmbeddingResponse> { return this.json<EmbeddingResponse>("/v1/embeddings", { method: "POST", body: { text, provider: provider ?? "" } }); }
  search(q: string, options: { limit?: number; provider?: string } = {}): Promise<SearchResponse> { return this.json<SearchResponse>("/v1/search", { query: { q, limit: options.limit, provider: options.provider } }); }
  searchProviders(): Promise<SearchProvidersResponse> { return this.json<SearchProvidersResponse>("/v1/search/providers"); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(path: string, options: { method?: "GET" | "POST"; body?: unknown; query?: Record<string, string | number | undefined>; headers?: HeadersInit } = {}): Promise<T> { const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = this.authHeaders(options.headers); const init: RequestInit = { method: options.method ?? "GET", headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); } const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new DiscoveryClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createDiscoveryClient(options: DiscoveryClientOptions): DiscoveryClient { return new DiscoveryClient(options); }
