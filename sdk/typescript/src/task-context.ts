/** Lightweight task context SDK slice. */
export type TaskGapType = "skill_missing" | "param_error" | "env_error" | "unknown" | string;
export type TaskThreadState = "open" | "paused" | "closed" | string;

export type TaskGapRecord = { id: string; task_id?: string; step_id?: number; step_action?: string; skill_name?: string; error_msg?: string; gap_type?: TaskGapType; suggestion?: string; resolved?: boolean; occurred_at?: string; resolved_at?: string; [key: string]: unknown };
export type TaskGapStats = { total?: number; unresolved?: number; resolved?: number; by_type?: Record<string, number>; [key: string]: unknown };
export type ResolveGapResponse = { resolved: string; [key: string]: unknown };

export type TaskWorkingMemory = { task_id: string; goal?: string; completed_work?: string[]; blockers?: string[]; confirmed?: string[]; pending?: string[]; artifacts?: string[]; next_action?: string; updated_at?: string; token_estimate?: number; [key: string]: unknown };

export type TaskTemplateVariable = { name: string; description?: string; default?: string; required?: boolean };
export type TaskTemplateStep = { action: string; skill_name?: string; args?: Record<string, unknown>; group?: number };
export type TaskTemplate = { id: string; name?: string; description?: string; variables?: TaskTemplateVariable[]; steps?: TaskTemplateStep[]; tags?: string[]; created_at?: string; [key: string]: unknown };
export type TaskTemplatesResponse = { templates: TaskTemplate[]; total: number; [key: string]: unknown };
export type DeleteTaskTemplateResponse = { deleted: string; [key: string]: unknown };
export type TaskTemplateVariables = Record<string, string>;

export type TaskContextTask = { id: string; title?: string; description?: string; status?: string; steps?: Array<Record<string, unknown>>; [key: string]: unknown };

export type TaskChannelBinding = { channel_type: string; channel_id: string; user_id?: string; user_name?: string; message_id?: string };
export type TaskThreadInfo = { task_id: string; session_id?: string; state?: TaskThreadState; binding?: TaskChannelBinding; tenant_id?: string; messages?: number; created_at?: string; updated_at?: string; [key: string]: unknown };
export type TaskThreadMessage = { role: string; content: string; msg_type?: string; channel?: string; step_id?: number; metadata?: Record<string, unknown>; timestamp?: string; [key: string]: unknown };
export type TaskThreadsResponse = { threads: TaskThreadInfo[]; total: number; [key: string]: unknown };
export type TaskThreadResponse = { task_id: string; info?: TaskThreadInfo; messages: TaskThreadMessage[]; [key: string]: unknown };
export type TaskThreadActionResponse = { status: string; task_id: string; state?: TaskThreadState; [key: string]: unknown };

export type TaskContextClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class TaskContextClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) { super(message || `Task context request failed with HTTP ${status}`); this.name = "TaskContextClientError"; this.status = status; this.body = body; }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, value); }

export class TaskContextClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: TaskContextClientOptions) {
    if (!options.baseUrl) throw new Error("TaskContextClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("TaskContextClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  gaps(type?: TaskGapType): Promise<TaskGapRecord[]> { return this.json<TaskGapRecord[]>("/v1/tasks/gaps", { query: { type } }); }
  gapStats(): Promise<TaskGapStats> { return this.json<TaskGapStats>("/v1/tasks/gaps", { query: { stats: "true" } }); }
  resolveGap(id: string): Promise<ResolveGapResponse> { return this.json<ResolveGapResponse>("/v1/tasks/gaps/resolve", { method: "POST", body: { id } }); }

  workingMemory(taskId: string): Promise<TaskWorkingMemory> { return this.json<TaskWorkingMemory>("/v1/tasks/memory", { query: { id: taskId } }); }

  templates(): Promise<TaskTemplatesResponse> { return this.json<TaskTemplatesResponse>("/v1/tasks/templates"); }
  template(id: string): Promise<TaskTemplate> { return this.json<TaskTemplate>("/v1/tasks/templates", { query: { id } }); }
  createTemplate(template: TaskTemplate): Promise<TaskTemplate> { return this.json<TaskTemplate>("/v1/tasks/templates", { method: "POST", body: template }); }
  deleteTemplate(id: string): Promise<DeleteTaskTemplateResponse> { return this.json<DeleteTaskTemplateResponse>("/v1/tasks/templates", { method: "DELETE", query: { id } }); }
  instantiateTemplate(templateId: string, variables: TaskTemplateVariables = {}): Promise<TaskContextTask> { return this.json<TaskContextTask>("/v1/tasks/templates/instantiate", { method: "POST", body: { template_id: templateId, variables } }); }

  threads(state?: TaskThreadState): Promise<TaskThreadsResponse> { return this.json<TaskThreadsResponse>("/v1/tasks/threads", { query: { state } }); }
  thread(taskId: string): Promise<TaskThreadResponse> { return this.json<TaskThreadResponse>("/v1/tasks/threads", { query: { id: taskId } }); }
  postThreadMessage(taskId: string, content: string, channel?: TaskChannelBinding): Promise<TaskThreadActionResponse> { return this.json<TaskThreadActionResponse>("/v1/tasks/threads", { method: "POST", body: { task_id: taskId, content, ...(channel ? { channel } : {}) } }); }
  updateThreadState(taskId: string, state: TaskThreadState): Promise<TaskThreadActionResponse> { return this.json<TaskThreadActionResponse>("/v1/tasks/threads", { method: "PUT", body: { task_id: taskId, state } }); }

  private authHeaders(extra?: HeadersInit): Headers { const headers = mergeHeaders(this.headers, extra); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey); return headers; }
  private async json<T>(path: string, options: { method?: "DELETE" | "GET" | "POST" | "PUT"; body?: unknown; query?: Record<string, string | undefined>; headers?: HeadersInit } = {}): Promise<T> { const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = this.authHeaders(options.headers); const init: RequestInit = { method: options.method ?? "GET", headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); } const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new TaskContextClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T; }
}

export function createTaskContextClient(options: TaskContextClientOptions): TaskContextClient { return new TaskContextClient(options); }
