/** Lightweight Sandbox SDK slice. */
export type SandboxExecRequest = { command: string; args?: string[] };
export type SandboxExecResponse = { stdout?: string; stderr?: string; exit_code?: number; output?: string; [key: string]: unknown };
export type SandboxProbeResponse = { sandbox_cloud_api_key_set?: boolean; sandbox_cloud_base_url_set?: boolean; tori_api_base_url_set?: boolean; llm_api_key_set?: boolean; key_source?: string; cloud_runner_ready?: boolean; desktop_running?: boolean; probe_error?: string; probe_status_code?: number; tori_sandbox_status?: unknown; tori_sandbox_raw?: string; [key: string]: unknown };
export type DesktopSandboxResponse = { ok: boolean; sandbox?: unknown; message?: string; running?: boolean; alive?: boolean; upstream?: unknown; error?: string; [key: string]: unknown };
export type SandboxClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class SandboxClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Sandbox request failed with HTTP ${status}`); this.name = "SandboxClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class SandboxClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: SandboxClientOptions) {
    if (!options.baseUrl) throw new Error("SandboxClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("SandboxClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  exec(request: SandboxExecRequest): Promise<SandboxExecResponse> { return this.json<SandboxExecResponse>("/v1/sandbox/exec", { method: "POST", body: JSON.stringify(request) }); }
  probe(): Promise<SandboxProbeResponse> { return this.json<SandboxProbeResponse>("/v1/sandbox/probe"); }
  createDesktop(): Promise<DesktopSandboxResponse> { return this.json<DesktopSandboxResponse>("/v1/sandbox/desktop", { method: "POST" }); }
  desktopStatus(): Promise<DesktopSandboxResponse> { return this.json<DesktopSandboxResponse>("/v1/sandbox/desktop/status"); }
  destroyDesktop(method: "POST" | "DELETE" = "POST"): Promise<DesktopSandboxResponse> { return this.json<DesktopSandboxResponse>("/v1/sandbox/desktop/destroy", { method }); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> { const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl; const headers = this.authHeaders(init.headers); if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json"); const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new SandboxClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createSandboxClient(options: SandboxClientOptions): SandboxClient { return new SandboxClient(options); }
