/** Lightweight Plugin API runtime SDK slice. */
export type PluginLLMMessage = { role: string; content: string; [key: string]: unknown };
export type PluginLLMRequest = { messages: PluginLLMMessage[]; temperature?: number; model?: string };
export type PluginLLMResponse = { reply: string; [key: string]: unknown };
export type PluginSearchResponse = { results: unknown[]; [key: string]: unknown };
export type PluginSendResponse = { ok: boolean; [key: string]: unknown };
export type PluginMemoryValueResponse = { value: string; [key: string]: unknown };
export type PluginMemoryListResponse = { entries: unknown[]; [key: string]: unknown };
export type PluginMemorySearchResponse = { results: unknown[]; [key: string]: unknown };
export type PluginAgentMemorySearchResponse = { context: string; [key: string]: unknown };
export type PluginOkResponse = { ok: boolean; [key: string]: unknown };
export type PluginKnowledgeSearchResponse = { results: unknown[]; [key: string]: unknown };
export type PluginCronAddResponse = { id: string; status: string; [key: string]: unknown };
export type PluginCronListResponse = { jobs: unknown[]; [key: string]: unknown };
export type PluginExtensionRegisterResponse = { ok: boolean; provider_id?: string; channel?: string; search?: string; guardrail?: string; embedding?: string; speech?: string; error?: string; [key: string]: unknown };
export type PluginExtensionsResponse = { extensions: unknown[]; [key: string]: unknown };
export type PluginApiClientOptions = { baseUrl: string; token: string; headers?: HeadersInit; fetch?: typeof fetch };

export class PluginApiClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Plugin API request failed with HTTP ${status}`); this.name = "PluginApiClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class PluginApiClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly token: string;
  private readonly headers: HeadersInit | undefined;

  constructor(options: PluginApiClientOptions) {
    if (!options.baseUrl) throw new Error("PluginApiClient requires baseUrl");
    if (!options.token) throw new Error("PluginApiClient requires plugin token");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("PluginApiClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.token = options.token;
    this.headers = options.headers;
  }

  llm(request: PluginLLMRequest): Promise<PluginLLMResponse> { return this.post<PluginLLMResponse>("/v1/plugin-api/llm", request); }
  search(query: string, limit?: number): Promise<PluginSearchResponse> { return this.post<PluginSearchResponse>("/v1/plugin-api/search", { query, limit }); }
  send(channel: string, target: string, content: string, format?: string): Promise<PluginSendResponse> { return this.post<PluginSendResponse>("/v1/plugin-api/send", { channel, target, content, format }); }
  memoryGet(key: string): Promise<PluginMemoryValueResponse> { return this.post<PluginMemoryValueResponse>("/v1/plugin-api/memory/get", { key }); }
  memorySet(key: string, value: string): Promise<PluginOkResponse> { return this.post<PluginOkResponse>("/v1/plugin-api/memory/set", { key, value }); }
  memoryDelete(key: string): Promise<PluginOkResponse> { return this.post<PluginOkResponse>("/v1/plugin-api/memory/delete", { key }); }
  memoryList(prefix?: string): Promise<PluginMemoryListResponse> { return this.post<PluginMemoryListResponse>("/v1/plugin-api/memory/list", { prefix }); }
  memorySearch(query: string, limit?: number): Promise<PluginMemorySearchResponse> { return this.post<PluginMemorySearchResponse>("/v1/plugin-api/memory/search", { query, limit }); }
  agentMemorySearch(query: string, topK?: number): Promise<PluginAgentMemorySearchResponse> { return this.post<PluginAgentMemorySearchResponse>("/v1/plugin-api/agent-memory/search", { query, top_k: topK }); }
  agentMemoryAdd(fact: string, source?: string): Promise<PluginOkResponse> { return this.post<PluginOkResponse>("/v1/plugin-api/agent-memory/add", { fact, source }); }
  knowledgeSearch(query: string, limit?: number): Promise<PluginKnowledgeSearchResponse> { return this.post<PluginKnowledgeSearchResponse>("/v1/plugin-api/knowledge/search", { query, limit }); }
  knowledgeIngest(content: string, source?: string, filename?: string): Promise<PluginOkResponse> { return this.post<PluginOkResponse>("/v1/plugin-api/knowledge/ingest", { content, source, filename }); }
  cronAdd(name: string, expression: string, message: string): Promise<PluginCronAddResponse> { return this.post<PluginCronAddResponse>("/v1/plugin-api/cron/add", { name, expression, message }); }
  cronRemove(id: string): Promise<PluginOkResponse> { return this.post<PluginOkResponse>("/v1/plugin-api/cron/remove", { id }); }
  cronList(plugin?: string): Promise<PluginCronListResponse> { const url = new URL(`${this.baseUrl}/v1/plugin-api/cron/list`); if (plugin) url.searchParams.set("plugin", plugin); return this.json<PluginCronListResponse>(url); }
  registerProvider(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> { return this.post<PluginExtensionRegisterResponse>("/v1/plugin-api/register/provider", config); }
  registerChannel(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> { return this.post<PluginExtensionRegisterResponse>("/v1/plugin-api/register/channel", config); }
  registerSearch(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> { return this.post<PluginExtensionRegisterResponse>("/v1/plugin-api/register/search", config); }
  registerGuardrail(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> { return this.post<PluginExtensionRegisterResponse>("/v1/plugin-api/register/guardrail", config); }
  registerEmbedding(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> { return this.post<PluginExtensionRegisterResponse>("/v1/plugin-api/register/embedding", config); }
  registerSpeech(config: Record<string, unknown>): Promise<PluginExtensionRegisterResponse> { return this.post<PluginExtensionRegisterResponse>("/v1/plugin-api/register/speech", config); }
  extensions(): Promise<PluginExtensionsResponse> { return this.json<PluginExtensionsResponse>("/v1/plugin-api/extensions"); }

  private post<T>(path: string, body: unknown): Promise<T> { return this.json<T>(path, { method: "POST", body: JSON.stringify(body) }); }
  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (!headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); return headers; }
  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> { const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl; const headers = this.authHeaders(init.headers); if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json"); const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new PluginApiClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createPluginApiClient(options: PluginApiClientOptions): PluginApiClient { return new PluginApiClient(options); }
