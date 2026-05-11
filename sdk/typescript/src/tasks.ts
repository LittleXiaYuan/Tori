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

  private action(path: string, id: string): Promise<TaskActionResponse> {
    return this.request<TaskActionResponse>("POST", path, { id });
  }

  private async request<T>(
    method: "DELETE" | "GET" | "POST",
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
