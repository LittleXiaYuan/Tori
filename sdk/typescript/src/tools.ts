/** Lightweight Tools process SDK slice. */
export type ToolProcessState = "running" | "exited" | "failed" | "killed" | (string & {});
export type ToolExecOptions = { Command: string; Cwd?: string; Background?: boolean; TimeoutMs?: number; YieldMs?: number; Env?: string[] };
export type ToolExecResult = { output?: string; exit_code?: number; state?: ToolProcessState; session_id?: string; [key: string]: unknown };
export type ToolProcessSession = { id: string; command: string; state: ToolProcessState; exit_code?: number; started_at?: string; ended_at?: string; cwd?: string; [key: string]: unknown };
export type ToolListResponse = { sessions: ToolProcessSession[]; [key: string]: unknown };
export type ToolPollResponse = { lines: string[] | null; state: ToolProcessState; [key: string]: unknown };
export type ToolKillResponse = { killed: string; [key: string]: unknown };
export type ToolsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class ToolsClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Tools request failed with HTTP ${status}`); this.name = "ToolsClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class ToolsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: ToolsClientOptions) {
    if (!options.baseUrl) throw new Error("ToolsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("ToolsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  exec(options: ToolExecOptions): Promise<ToolExecResult> { return this.json<ToolExecResult>("/v1/tools/exec", { method: "POST", body: JSON.stringify(options) }); }
  list(): Promise<ToolListResponse> { return this.json<ToolListResponse>("/v1/tools/list"); }
  poll(id: string): Promise<ToolPollResponse> { const url = new URL(`${this.baseUrl}/v1/tools/poll`); url.searchParams.set("id", id); return this.json<ToolPollResponse>(url); }
  kill(id: string): Promise<ToolKillResponse> { const url = new URL(`${this.baseUrl}/v1/tools/kill`); url.searchParams.set("id", id); return this.json<ToolKillResponse>(url, { method: "POST" }); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> { const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl; const headers = this.authHeaders(init.headers); if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json"); const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new ToolsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createToolsClient(options: ToolsClientOptions): ToolsClient { return new ToolsClient(options); }
