/**
 * Lightweight Workflow SDK slice.
 *
 * This keeps DAG workflow definition and instance operations usable without
 * importing the full generated OpenAPI SDK:
 *
 *   import { createWorkflowClient } from "yunque-client/workflow";
 */

export type WorkflowNodeType =
  | "skill"
  | "llm"
  | "condition"
  | "parallel"
  | "join"
  | "subflow"
  | "input"
  | "transform"
  | "browser"
  | "code"
  | "knowledge"
  | "start"
  | "end"
  | string;

export type WorkflowPosition = {
  x: number;
  y: number;
};

export type WorkflowRetryPolicy = {
  max_retries?: number;
  backoff_ms?: number;
  multiplier?: number;
};

export type WorkflowNode = {
  id: string;
  name: string;
  type: WorkflowNodeType;
  config?: Record<string, unknown>;
  position?: WorkflowPosition;
  timeout?: string;
  retry_policy?: WorkflowRetryPolicy;
};

export type WorkflowEdge = {
  id: string;
  from_node: string;
  to_node: string;
  condition?: string;
  label?: string;
};

export type WorkflowVariable = {
  name: string;
  type: "string" | "number" | "boolean" | "json" | string;
  required?: boolean;
  default_value?: unknown;
  description?: string;
};

export type WorkflowDefinition = {
  id?: string;
  name: string;
  description?: string;
  version?: number;
  nodes?: WorkflowNode[];
  edges?: WorkflowEdge[];
  variables?: WorkflowVariable[];
  tenant_id?: string;
  created_at?: string;
  updated_at?: string;
  [key: string]: unknown;
};

export type WorkflowNodeStatus = "pending" | "running" | "done" | "failed" | "skipped" | "waiting" | string;
export type WorkflowInstanceStatus = "pending" | "running" | "paused" | "completed" | "failed" | "cancelled" | string;

export type WorkflowNodeState = {
  node_id: string;
  status: WorkflowNodeStatus;
  input?: unknown;
  output?: unknown;
  error?: string;
  retry_count?: number;
  started_at?: string;
  finished_at?: string;
};

export type WorkflowInstance = {
  id: string;
  definition_id: string;
  version?: number;
  status: WorkflowInstanceStatus;
  variables?: Record<string, unknown>;
  node_states?: Record<string, WorkflowNodeState>;
  error?: string;
  tenant_id?: string;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  finished_at?: string;
  [key: string]: unknown;
};

export type ListWorkflowsResponse = {
  workflows: WorkflowDefinition[];
  total: number;
};

export type ListWorkflowInstancesResponse = {
  instances: WorkflowInstance[];
  total: number;
};

export type RunWorkflowRequest = {
  definition_id: string;
  variables?: Record<string, unknown>;
};

export type RunWorkflowResponse = {
  status: "accepted" | string;
  instance_id: string;
  instance: WorkflowInstance;
};

export type CancelWorkflowRequest = {
  instance_id: string;
};

export type CancelWorkflowResponse = {
  status?: string;
  instance_id?: string;
  [key: string]: unknown;
};

export type DeleteWorkflowResponse = {
  deleted?: string;
  [key: string]: unknown;
};

export type WorkflowClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class WorkflowClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Workflow request failed with HTTP ${status}`);
    this.name = "WorkflowClientError";
    this.status = status;
    this.body = body;
  }
}

function trimBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
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

function setOptionalQuery(url: URL, key: string, value: string | undefined): void {
  if (!value) return;
  url.searchParams.set(key, value);
}

export class WorkflowClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: WorkflowClientOptions) {
    if (!options.baseUrl) throw new Error("WorkflowClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("WorkflowClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  list(): Promise<ListWorkflowsResponse> {
    return this.request<ListWorkflowsResponse>("GET", "/v1/workflows");
  }

  get(id: string): Promise<WorkflowDefinition> {
    return this.request<WorkflowDefinition>("GET", "/v1/workflows", undefined, { id });
  }

  save(definition: WorkflowDefinition): Promise<WorkflowDefinition> {
    return this.request<WorkflowDefinition>("POST", "/v1/workflows", definition);
  }

  delete(id: string): Promise<DeleteWorkflowResponse> {
    return this.request<DeleteWorkflowResponse>("DELETE", "/v1/workflows", undefined, { id });
  }

  run(body: RunWorkflowRequest): Promise<RunWorkflowResponse> {
    return this.request<RunWorkflowResponse>("POST", "/v1/workflows/run", body);
  }

  instances(): Promise<ListWorkflowInstancesResponse> {
    return this.request<ListWorkflowInstancesResponse>("GET", "/v1/workflows/instances");
  }

  getInstance(id: string): Promise<WorkflowInstance> {
    return this.request<WorkflowInstance>("GET", "/v1/workflows/instances", undefined, { id });
  }

  cancel(body: CancelWorkflowRequest): Promise<CancelWorkflowResponse> {
    return this.request<CancelWorkflowResponse>("POST", "/v1/workflows/cancel", body);
  }

  private async request<T>(
    method: "GET" | "POST" | "DELETE",
    path: string,
    body?: unknown,
    query?: Record<string, string | undefined>,
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    for (const [key, value] of Object.entries(query ?? {})) setOptionalQuery(url, key, value);

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
    if (!response.ok) throw new WorkflowClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createWorkflowClient(options: WorkflowClientOptions): WorkflowClient {
  return new WorkflowClient(options);
}
