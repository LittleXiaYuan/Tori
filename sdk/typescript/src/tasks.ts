/**
 * Lightweight Tasks SDK slice.
 *
 * This keeps background task orchestration usable without importing the full
 * generated OpenAPI SDK:
 *
 *   import { createTasksClient } from "yunque-client/tasks";
 */

export type TaskStatus =
  | "pending"
  | "planning"
  | "running"
  | "paused"
  | "completed"
  | "failed"
  | "cancelled"
  | "interrupted"
  | string;

export type TaskStepStatus = "pending" | "running" | "done" | "failed" | "skipped" | "retrying" | string;

export type TaskStep = {
  id?: number;
  action?: string;
  skill_name?: string;
  args?: Record<string, unknown>;
  status?: TaskStepStatus;
  result?: string;
  error?: string;
  input?: string;
  retry_count?: number;
  max_retries?: number;
  gap_type?: string;
  group?: number;
  depends_on?: number[];
  metadata?: Record<string, unknown>;
  started_at?: string;
  done_at?: string;
};

export type TaskConstraints = {
  max_steps?: number;
  timeout_sec?: number;
  max_cost_usd?: number;
  success_criteria?: string;
  test_command?: string;
  priority?: "low" | "medium" | "high" | string;
  risk_level?: "low" | "medium" | "high" | string;
  auto_approve?: boolean;
  tags?: string[];
  extra?: Record<string, unknown>;
};

// ── Task Templates ──

export type TaskTemplateVariable = {
  name: string;
  description?: string;
  default?: string;
  required?: boolean;
};

export type TaskTemplateStep = {
  action: string;
  skill_name?: string;
  args?: Record<string, unknown>;
  group?: number;
};

export type TaskTemplate = {
  id: string;
  name?: string;
  description?: string;
  variables?: TaskTemplateVariable[];
  steps?: TaskTemplateStep[];
  tags?: string[];
  created_at?: string;
  updated_at?: string;
};

export type CreateTaskTemplateRequest = {
  id: string;
  name?: string;
  description?: string;
  variables?: TaskTemplateVariable[];
  steps?: TaskTemplateStep[];
  tags?: string[];
};

export type InstantiateTemplateRequest = {
  template_id: string;
  variables?: Record<string, string>;
};

export type TaskTemplatesResponse = {
  templates?: TaskTemplate[];
  total?: number;
  [key: string]: unknown;
};

export type DeleteTaskTemplateResponse = {
  deleted?: string;
  [key: string]: unknown;
};

// ── Task Gaps ──

export type TaskGap = {
  id: string;
  task_id?: string;
  type?: string;
  description?: string;
  status?: string;
  created_at?: string;
  [key: string]: unknown;
};

export type TaskGapStats = {
  total?: number;
  by_type?: Record<string, number>;
  [key: string]: unknown;
};

export type ResolveTaskGapResponse = {
  status?: string;
  [key: string]: unknown;
};

// ── Task Working Memory ──

export type TaskWorkingMemory = {
  task_id?: string;
  data?: Record<string, unknown>;
  created_at?: string;
  updated_at?: string;
  [key: string]: unknown;
};

// ── Task Threads ──

export type TaskChannelBinding = {
  channel_type: string;
  channel_id: string;
  user_id?: string;
  user_name?: string;
  message_id?: string;
};

export type PostTaskThreadMessageRequest = {
  task_id: string;
  content: string;
  channel?: TaskChannelBinding;
};

export type UpdateTaskThreadStateRequest = {
  task_id: string;
  state: string;
};

export type TaskThread = {
  id?: string;
  task_id?: string;
  state?: string;
  messages?: Array<Record<string, unknown>>;
  created_at?: string;
  updated_at?: string;
  [key: string]: unknown;
};

export type TaskThreadsResponse = {
  threads?: TaskThread[];
  total?: number;
  [key: string]: unknown;
};

export type TaskThreadActionResponse = {
  status?: string;
  [key: string]: unknown;
};

// ── Task Trace ──

export type TaskTraceQueryOptions = {
  raw?: boolean;
};

export type TaskTraceEvent = {
  trace_id: string;
  step?: string;
  action?: string;
  status?: string;
  started_at?: string;
  done_at?: string;
  error?: string;
  [key: string]: unknown;
};

export type TaskTraceResponse = {
  task_id: string;
  events?: TaskTraceEvent[];
  [key: string]: unknown;
};

export type Task = {
  id: string;
  title?: string;
  description?: string;
  status?: TaskStatus;
  steps?: TaskStep[];
  artifacts?: Array<Record<string, unknown>>;
  error?: string;
  tenant_id?: string;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  finished_at?: string;
  constraints?: TaskConstraints;
};

export type CreateTaskRequest = {
  title?: string;
  description: string;
  constraints?: TaskConstraints;
};

export type TaskActionResponse = {
  status?: string;
  task_id?: string;
  deleted?: string;
  [key: string]: unknown;
};

export type TasksClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class TasksClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Task request failed with HTTP ${status}`);
    this.name = "TasksClientError";
    this.status = status;
    this.body = body;
  }
}

function trimBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

function appendQuery(url: URL, query?: Record<string, string | undefined>): void {
  if (!query) return;
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) url.searchParams.set(key, value);
  }
}

function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers {
  const headers = new Headers(base);
  if (!extra) return headers;
  new Headers(extra).forEach((value, key) => headers.set(key, value));
  return headers;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function messageFromErrorBody(body: unknown): string | undefined {
  if (typeof body === "string" && body.trim()) return body.trim();
  if (!isRecord(body)) return undefined;
  for (const key of ["message", "detail", "error", "reason"]) {
    const value = body[key];
    if (typeof value === "string" && value.trim()) return value;
    if (key === "error" && isRecord(value)) {
      const nested = messageFromErrorBody(value);
      if (nested) return nested;
    }
  }
  return undefined;
}

async function parseResponse(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

export class TasksClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: TasksClientOptions) {
    if (!options.baseUrl) throw new Error("TasksClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("TasksClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  list(): Promise<Task[]> {
    return this.request<Task[]>("GET", "/v1/tasks");
  }

  get(id: string): Promise<Task> {
    return this.request<Task>("GET", "/v1/tasks", undefined, { id });
  }

  create(body: CreateTaskRequest): Promise<Task> {
    return this.request<Task>("POST", "/v1/tasks", body);
  }

  run(id: string): Promise<TaskActionResponse> {
    return this.action("/v1/tasks/run", id);
  }

  pause(id: string): Promise<TaskActionResponse> {
    return this.action("/v1/tasks/pause", id);
  }

  resume(id: string): Promise<TaskActionResponse> {
    return this.action("/v1/tasks/resume", id);
  }

  restart(id: string): Promise<TaskActionResponse> {
    return this.action("/v1/tasks/restart", id);
  }

  cancel(id: string): Promise<TaskActionResponse> {
    return this.action("/v1/tasks/cancel", id);
  }

  delete(id: string): Promise<TaskActionResponse> {
    return this.request<TaskActionResponse>("DELETE", "/v1/tasks", undefined, { id });
  }

  // ── Task Templates ──

  templates(): Promise<TaskTemplatesResponse> {
    return this.request<TaskTemplatesResponse>("GET", "/v1/tasks/templates");
  }

  template(id: string): Promise<TaskTemplate> {
    return this.request<TaskTemplate>("GET", "/v1/tasks/templates", undefined, { id });
  }

  createTemplate(body: CreateTaskTemplateRequest): Promise<TaskTemplate> {
    return this.request<TaskTemplate>("POST", "/v1/tasks/templates", body);
  }

  deleteTemplate(id: string): Promise<DeleteTaskTemplateResponse> {
    return this.request<DeleteTaskTemplateResponse>("DELETE", "/v1/tasks/templates", undefined, { id });
  }

  instantiateTemplate(templateId: string, variables?: Record<string, string>): Promise<Task> {
    return this.request<Task>("POST", "/v1/tasks/templates/instantiate", {
      template_id: templateId,
      variables: variables ?? {},
    } satisfies InstantiateTemplateRequest);
  }

  // ── Task Gaps ──

  gaps(gapType?: string): Promise<TaskGap[]> {
    const path = gapType ? `/v1/tasks/gaps?type=${encodeURIComponent(gapType)}` : "/v1/tasks/gaps";
    return this.request<TaskGap[]>("GET", path);
  }

  gapStats(): Promise<TaskGapStats> {
    return this.request<TaskGapStats>("GET", "/v1/tasks/gaps?stats=true");
  }

  resolveGap(id: string): Promise<ResolveTaskGapResponse> {
    return this.request<ResolveTaskGapResponse>("POST", "/v1/tasks/gaps/resolve", { id });
  }

  // ── Task Working Memory ──

  workingMemory(taskId: string): Promise<TaskWorkingMemory> {
    return this.request<TaskWorkingMemory>("GET", "/v1/tasks/memory", undefined, { id: taskId });
  }

  // ── Task Threads ──

  threads(state?: string): Promise<TaskThreadsResponse> {
    const path = state ? `/v1/tasks/threads?state=${encodeURIComponent(state)}` : "/v1/tasks/threads";
    return this.request<TaskThreadsResponse>("GET", path);
  }

  thread(taskId: string): Promise<TaskThread> {
    return this.request<TaskThread>("GET", "/v1/tasks/threads", undefined, { id: taskId });
  }

  postThreadMessage(body: PostTaskThreadMessageRequest): Promise<TaskThreadActionResponse> {
    return this.request<TaskThreadActionResponse>("POST", "/v1/tasks/threads", body);
  }

  updateThreadState(body: UpdateTaskThreadStateRequest): Promise<TaskThreadActionResponse> {
    return this.request<TaskThreadActionResponse>("PUT", "/v1/tasks/threads", body);
  }

  // ── Task Trace ──

  trace(taskId: string, options?: TaskTraceQueryOptions): Promise<TaskTraceResponse> {
    const query: Record<string, string | undefined> = { id: taskId };
    if (options?.raw !== undefined) query.raw = String(options.raw);
    return this.request<TaskTraceResponse>("GET", "/v1/trace/by-task", undefined, query);
  }

  private action(path: string, id: string): Promise<TaskActionResponse> {
    return this.request<TaskActionResponse>("POST", path, { id });
  }

  private async request<T>(
    method: "DELETE" | "GET" | "POST" | "PUT",
    path: string,
    body?: unknown,
    query?: Record<string, string | undefined>,
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    appendQuery(url, query);
    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(url, init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new TasksClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createTasksClient(options: TasksClientOptions): TasksClient {
  return new TasksClient(options);
}
