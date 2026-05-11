/** Lightweight State Kernel SDK slice. */
export type StateGoal = { id?: string; title: string; description?: string; priority?: number; status?: string; progress?: number; parent_goal?: string; task_ids?: string[]; [key: string]: unknown };
export type StateResource = { id?: string; path: string; type?: string; description?: string; status?: string; [key: string]: unknown };
export type StateSnapshotResponse = Record<string, unknown>;
export type StateGoalsResponse = StateGoal[];
export type StateGoalMutationResponse = { id?: string; status: string; [key: string]: unknown };
export type StateFocusResponse = { focus: string; [key: string]: unknown };
export type StateFocusUpdateResponse = { status: string; [key: string]: unknown };
export type StateResourcesResponse = StateResource[];
export type StateResourceMutationResponse = { status: string; [key: string]: unknown };
export type StateClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class StateClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `State request failed with HTTP ${status}`); this.name = "StateClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }

export class StateClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: StateClientOptions) {
    if (!options.baseUrl) throw new Error("StateClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("StateClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  snapshot(): Promise<StateSnapshotResponse> { return this.json<StateSnapshotResponse>("/v1/state"); }
  goals(): Promise<StateGoalsResponse> { return this.json<StateGoalsResponse>("/v1/state/goals"); }
  saveGoal(goal: StateGoal): Promise<StateGoalMutationResponse> { return this.json<StateGoalMutationResponse>("/v1/state/goals", { method: "POST", body: JSON.stringify(goal) }); }
  deleteGoal(id: string): Promise<StateGoalMutationResponse> { const url = new URL(`${this.baseUrl}/v1/state/goals`); url.searchParams.set("id", id); return this.json<StateGoalMutationResponse>(url, { method: "DELETE" }); }
  focus(): Promise<StateFocusResponse> { return this.json<StateFocusResponse>("/v1/state/focus"); }
  updateFocus(focus?: string, topics?: string[]): Promise<StateFocusUpdateResponse> { return this.json<StateFocusUpdateResponse>("/v1/state/focus", { method: "POST", body: JSON.stringify({ focus, topics }) }); }
  resources(): Promise<StateResourcesResponse> { return this.json<StateResourcesResponse>("/v1/state/resources"); }
  trackResource(resource: StateResource): Promise<StateResourceMutationResponse> { return this.json<StateResourceMutationResponse>("/v1/state/resources", { method: "POST", body: JSON.stringify(resource) }); }
  releaseResource(id: string): Promise<StateResourceMutationResponse> { const url = new URL(`${this.baseUrl}/v1/state/resources`); url.searchParams.set("id", id); return this.json<StateResourceMutationResponse>(url, { method: "DELETE" }); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> { const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl; const headers = this.authHeaders(init.headers); if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json"); const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new StateClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createStateClient(options: StateClientOptions): StateClient { return new StateClient(options); }
