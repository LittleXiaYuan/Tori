/** Lightweight Auth SDK slice: API-key to JWT token exchange. */
export type AuthRole = "user" | "viewer" | string;
export type GenerateTokenRequest = { role?: AuthRole };
export type GenerateTokenResponse = { token: string; type: "Bearer" | string; [key: string]: unknown };
export type AuthClientOptions = { baseUrl: string; apiKey?: string; token?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class AuthClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Auth request failed with HTTP ${status}`); this.name = "AuthClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class AuthClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly apiKey: string | undefined; private readonly token: string | undefined;
  constructor(options: AuthClientOptions) { if (!options.baseUrl) throw new Error("AuthClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("AuthClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.apiKey = options.apiKey; this.token = options.token; }

  generateToken(request: GenerateTokenRequest = {}): Promise<GenerateTokenResponse> { return this.request<GenerateTokenResponse>("/v1/token", request); }

  private async request<T>(path: string, body: unknown): Promise<T> {
    const headers = mergeHeaders(this.headers); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); headers.set("Content-Type", "application/json");
    const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), { method: "POST", headers, body: JSON.stringify(body) }); const parsed = await parseResponse(response); if (!response.ok) throw new AuthClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createAuthClient(options: AuthClientOptions): AuthClient { return new AuthClient(options); }
