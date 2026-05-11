/** Lightweight Subagents SDK slice. */
export type SubagentMessage = Record<string, unknown> & { role?: string; content?: unknown };
export type Subagent = { id: string; name: string; description?: string; parent_id?: string; messages?: SubagentMessage[]; skills?: string[]; metadata?: Record<string, unknown>; created_at?: string; updated_at?: string; [key: string]: unknown };
export type SubagentsResponse = { subagents: Subagent[]; [key: string]: unknown };
export type SpawnSubagentRequest = { parent_id?: string; name: string; description?: string; skills?: string[] };
export type AppendSubagentMessagesRequest = { id: string; messages: SubagentMessage[] };
export type AppendSubagentMessagesResponse = { ok: boolean; [key: string]: unknown };
export type DeleteSubagentResponse = { deleted: boolean; [key: string]: unknown };

export type SubagentsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class SubagentsClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Subagents request failed with HTTP ${status}`); this.name = "SubagentsClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, value); }

export class SubagentsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: SubagentsClientOptions) {
    if (!options.baseUrl) throw new Error("SubagentsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("SubagentsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  list(parentId?: string): Promise<SubagentsResponse> { return this.json<SubagentsResponse>("/v1/subagent", { query: { parent_id: parentId } }); }
  get(id: string): Promise<Subagent> { return this.json<Subagent>("/v1/subagent", { query: { id } }); }
  spawn(request: SpawnSubagentRequest): Promise<Subagent> { return this.json<Subagent>("/v1/subagent", { method: "POST", body: { parent_id: request.parent_id ?? "", name: request.name, description: request.description ?? "", skills: request.skills ?? [] } }); }
  destroy(id: string): Promise<DeleteSubagentResponse> { return this.json<DeleteSubagentResponse>("/v1/subagent", { method: "DELETE", query: { id } }); }
  appendMessages(id: string, messages: SubagentMessage[]): Promise<AppendSubagentMessagesResponse> { return this.json<AppendSubagentMessagesResponse>("/v1/subagent/message", { method: "POST", body: { id, messages } }); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(path: string, options: { method?: "DELETE" | "GET" | "POST"; body?: unknown; query?: Record<string, string | undefined>; headers?: HeadersInit } = {}): Promise<T> { const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = this.authHeaders(options.headers); const init: RequestInit = { method: options.method ?? "GET", headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); } const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new SubagentsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createSubagentsClient(options: SubagentsClientOptions): SubagentsClient { return new SubagentsClient(options); }
