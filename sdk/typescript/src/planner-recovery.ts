/**
 * Lightweight Planner Recovery SDK slice.
 *
 * This module is intentionally hand-written and independent from the large
 * generated `sdk.gen.ts` bundle so embedders can import only the recovery
 * surface they need:
 *
 *   import { createPlannerRecoveryClient } from "yunque-client/planner-recovery";
 */

export type CheckpointRecoveryAction = "continue" | "retry_failed" | "partial";
export type RecoveryNextAction = CheckpointRecoveryAction | "inspect_dependencies" | "create_task";
export type RecoveryAction = RecoveryNextAction;

export type PlannerStepStatus = "pending" | "running" | "done" | "completed" | "failed" | "skipped";

export type PlannerCheckpointStep = {
  id?: number;
  action?: string;
  skill?: string;
  status?: PlannerStepStatus | string;
  depends_on?: number[];
  result?: string;
  error?: string;
};

export type PlannerCheckpoint = {
  plan_id: string;
  task_id?: string;
  goal?: string;
  status?: string;
  current_step?: number;
  completed?: number;
  total?: number;
  steps_used?: number;
  revisions?: number;
  error?: string;
  recoverable?: boolean;
  resume_hint?: string;
  updated_at?: string;
  plan_snapshot?: PlannerCheckpointStep[];
};

export type PlannerRecoveryPlanStep = PlannerCheckpointStep & {
  selected?: boolean;
  reason?: string;
};

export type PlannerRecoveryPlan = {
  mode?: CheckpointRecoveryAction | string;
  executable?: boolean;
  prompt?: string;
  steps?: PlannerRecoveryPlanStep[];
};

export type PlannerResumePlanEvent = {
  id?: string;
  type?: string;
  skill?: string;
  summary?: string;
  timestamp?: string;
};

export type PlannerResumePlanJob = {
  id: string;
  status?: "running" | "completed" | "failed" | "cancelled" | string;
  action?: CheckpointRecoveryAction | string;
  plan_id?: string;
  task_id?: string;
  error?: string;
  friendly_error?: string;
  recoverable?: boolean;
  next_action?: RecoveryNextAction | string;
  events?: PlannerResumePlanEvent[];
  started_at?: string;
  finished_at?: string;
  result?: {
    reply?: string;
    skills_used?: string[];
    steps?: number;
    plan?: PlannerCheckpointStep[];
  };
};

export type PlannerFailureSummary = {
  failed_count?: number;
  completed_count?: number;
  failed_tools?: string[];
  last_summary?: string;
  event_count?: number;
  events?: PlannerResumePlanEvent[];
};

export type ListPlannerCheckpointsResponse = {
  checkpoints: PlannerCheckpoint[];
  limit?: number;
  count?: number;
};

export type RecoverPlannerCheckpointResponse = {
  action: CheckpointRecoveryAction | string;
  plan_id: string;
  task_id?: string;
  prompt?: string;
  recovery_plan?: PlannerRecoveryPlan;
  checkpoint?: PlannerCheckpoint;
};

export type ResumePlannerCheckpointTaskResponse = {
  status?: string;
  task_id?: string;
  run?: boolean;
  recovery_plan?: PlannerRecoveryPlan;
  checkpoint?: PlannerCheckpoint;
};

export type ResumePlannerCheckpointPlanResponse = {
  status?: string;
  action?: CheckpointRecoveryAction | string;
  plan_id?: string;
  job_id?: string;
  result?: PlannerResumePlanJob["result"];
  recovery_plan?: PlannerRecoveryPlan;
  checkpoint?: PlannerCheckpoint;
};

export type GetPlannerResumePlanJobResponse = {
  job?: PlannerResumePlanJob;
};

export type GetPlannerExecutionStateResponse = {
  plan_id: string;
  status?: string;
  action?: CheckpointRecoveryAction | string;
  next_action?: RecoveryNextAction | string;
  updated_at?: string;
  checkpoint?: PlannerCheckpoint;
  latest_job?: PlannerResumePlanJob;
  recovery_plan?: PlannerRecoveryPlan;
  failure_summary?: PlannerFailureSummary;
  events?: PlannerResumePlanEvent[];
};

export type PlannerRecoveryClientOptions = {
  /**
   * Yunque gateway base URL, for example `http://localhost:9090`.
   */
  baseUrl: string;
  /**
   * Bearer token. If omitted, callers may provide Authorization in `headers`.
   */
  token?: string;
  /**
   * Extra headers merged into every request.
   */
  headers?: HeadersInit;
  /**
   * Custom fetch implementation for tests, Workers, Electron, or React Native.
   */
  fetch?: typeof fetch;
};

export class PlannerRecoveryError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Planner recovery request failed with HTTP ${status}`);
    this.name = "PlannerRecoveryError";
    this.status = status;
    this.body = body;
  }
}

const jsonContentType = "application/json";

function trimBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

function appendQuery(url: URL, query?: Record<string, boolean | number | string | undefined>): void {
  if (!query) return;
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined) continue;
    url.searchParams.set(key, String(value));
  }
}

function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers {
  const headers = new Headers(base);
  if (!extra) return headers;
  new Headers(extra).forEach((value, key) => headers.set(key, value));
  return headers;
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

export class PlannerRecoveryClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;

  constructor(options: PlannerRecoveryClientOptions) {
    if (!options.baseUrl) throw new Error("PlannerRecoveryClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("PlannerRecoveryClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
  }

  listCheckpoints(query?: { limit?: number; plan_id?: string; include_snapshot?: boolean }): Promise<ListPlannerCheckpointsResponse> {
    return this.request<ListPlannerCheckpointsResponse>("GET", "/v1/planner/checkpoints", { query });
  }

  recoverCheckpoint(body: { plan_id: string; action?: CheckpointRecoveryAction }): Promise<RecoverPlannerCheckpointResponse> {
    return this.request<RecoverPlannerCheckpointResponse>("POST", "/v1/planner/checkpoints/recover", { body });
  }

  resumeCheckpointTask(body: { plan_id: string; action?: CheckpointRecoveryAction; run?: boolean }): Promise<ResumePlannerCheckpointTaskResponse> {
    return this.request<ResumePlannerCheckpointTaskResponse>("POST", "/v1/planner/checkpoints/resume", { body });
  }

  resumeCheckpointPlan(body: { plan_id: string; action?: CheckpointRecoveryAction; async?: boolean }): Promise<ResumePlannerCheckpointPlanResponse> {
    return this.request<ResumePlannerCheckpointPlanResponse>("POST", "/v1/planner/checkpoints/resume-plan", { body });
  }

  getResumePlanJob(query: { job_id?: string; id?: string; plan_id?: string }): Promise<GetPlannerResumePlanJobResponse> {
    return this.request<GetPlannerResumePlanJobResponse>("GET", "/v1/planner/checkpoints/resume-plan/jobs", { query });
  }

  getExecutionState(query: { plan_id: string; action?: CheckpointRecoveryAction }): Promise<GetPlannerExecutionStateResponse> {
    return this.request<GetPlannerExecutionStateResponse>("GET", "/v1/planner/execution-state", { query });
  }

  private async request<T>(
    method: "GET" | "POST",
    path: string,
    options?: {
      body?: unknown;
      query?: Record<string, boolean | number | string | undefined>;
    },
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    appendQuery(url, options?.query);

    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) {
      headers.set("Authorization", `Bearer ${this.token}`);
    }

    const init: RequestInit = { method, headers };
    if (options?.body !== undefined) {
      headers.set("Content-Type", jsonContentType);
      init.body = JSON.stringify(options.body);
    }

    const response = await this.fetchImpl(url, init);
    const parsed = await parseResponse(response);
    if (!response.ok) {
      const message =
        typeof parsed === "object" && parsed && "error" in parsed && typeof (parsed as { error?: unknown }).error === "string"
          ? (parsed as { error: string }).error
          : undefined;
      throw new PlannerRecoveryError(response.status, parsed, message);
    }
    return parsed as T;
  }
}

export function createPlannerRecoveryClient(options: PlannerRecoveryClientOptions): PlannerRecoveryClient {
  return new PlannerRecoveryClient(options);
}
