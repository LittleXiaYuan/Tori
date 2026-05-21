export type RPAReplayParamDef = {
  type?: string;
  description?: string;
  required?: boolean;
  default?: string;
};

export type RPAReplayStepAssertion = {
  type: string;
  selector?: string;
  expected?: string;
};

export type RPAReplayTraceStep = {
  index?: number;
  action: string;
  selector?: string;
  value?: string;
  param_ref?: string;
  screenshot?: string;
  assertion?: RPAReplayStepAssertion;
  timestamp_ms?: number;
};

export type RPAReplayTraceSummary = {
  slug: string;
  name: string;
  description?: string;
  target_url?: string;
  recorded_at: string;
  step_count: number;
  success_rate?: number;
  avg_duration_ms?: number;
};

export type RPAReplayTrace = Omit<RPAReplayTraceSummary, "step_count"> & {
  type: "rpa-replay";
  parameters?: Record<string, RPAReplayParamDef>;
  steps: RPAReplayTraceStep[];
};

type JsonObject = Record<string, unknown>;

export type RPAReplayStatusResponse = {
  pack_id: string;
  stage: string;
  trace_count: number;
  active_recordings: number;
  store_dir?: string;
  capabilities: string[];
  notes?: string[];
} & Partial<RPAReplayPlanBoundary> & { executor_ready: boolean };

export type RPAReplayCreateTraceRequest = Partial<RPAReplayTrace> & {
  slug: string;
  name: string;
  steps: RPAReplayTraceStep[];
};

export type RPAReplayRecordingSession = {
  id: string;
  slug?: string;
  name?: string;
  description?: string;
  target_url?: string;
  started_at: string;
  status: string;
};

export type RPAReplayStopRecordingRequest = {
  session_id: string;
  slug?: string;
  name?: string;
  steps?: RPAReplayTraceStep[];
};

export type RPAReplayRequest = {
  slug: string;
  params?: Record<string, string>;
  dry_run?: boolean;
};

export type RPAReplayResult = {
  success: boolean;
  dry_run: boolean;
  output?: string;
  steps_run: number;
  failed_step: number;
  fail_reason?: string;
  duration_ms: number;
  planned_steps?: RPAReplayTraceStep[];
};

export type RPAReplayPlanBoundary = {
  executor_plan_ready: boolean;
  executor_ready: boolean;
  action_tracer_plan_ready: boolean;
  action_tracer_ready: boolean;
  browser_intent_gate_plan_ready: boolean;
  browser_intent_ready: boolean;
  consumes_browser_intent: boolean;
  executes_browser_actions: boolean;
  writes_browser_state: boolean;
  writes_files: boolean;
  network_access: boolean;
};

export type RPAReplayExecutorStepPlan = RPAReplayTraceStep & {
  executor_action: string;
  requires_browser_intent: boolean;
  requires_action_tracer: boolean;
  executes_browser_action: boolean;
  writes_browser_state: boolean;
  consumes_external_target: boolean;
};

export type RPAReplayExecutorPlan = RPAReplayPlanBoundary & {
  status: string;
  action_count: number;
  planned_steps: RPAReplayExecutorStepPlan[];
  executor_handoff_plan: JsonObject;
  browser_intent_gate_plan: JsonObject;
  action_tracer_handoff_plan: JsonObject;
  artifacts: string[];
  [key: string]: unknown;
};

export type RPAReplayExecutorPlanRequest = RPAReplayRequest & {
  executor?: string;
  requested_by?: string;
  reason?: string;
};

export type RPAReplayEvidenceResponse = {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  trace: RPAReplayTrace;
  executor_plan?: RPAReplayExecutorPlan;
  executor_handoff_plan?: JsonObject;
  browser_intent_gate_plan?: JsonObject;
  action_tracer_handoff_plan?: JsonObject;
};

export type RPAReplayClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class RPAReplayClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `RPA Replay request failed with HTTP ${status}`);
    this.name = "RPAReplayClientError";
    this.status = status;
    this.body = body;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function messageFromErrorBody(body: unknown): string | undefined {
  if (typeof body === "string" && body.trim()) return body.trim();
  if (!isRecord(body)) return undefined;
  for (const value of [body.message, body.detail, body.error, body.reason]) {
    if (typeof value === "string" && value.trim()) return value;
    if (isRecord(value)) {
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

export class RPAReplayClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: RPAReplayClientOptions) {
    if (!options.baseUrl) throw new Error("RPAReplayClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("RPAReplayClient requires a fetch implementation");
    this.baseUrl = options.baseUrl.replace(/\/+$/, "");
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<RPAReplayStatusResponse> {
    return this.request<RPAReplayStatusResponse>("GET", "/v1/rpa-replay/status");
  }

  traces(): Promise<{ traces: RPAReplayTraceSummary[]; count: number }> {
    return this.request<{ traces: RPAReplayTraceSummary[]; count: number }>("GET", "/v1/rpa-replay/traces");
  }

  createTrace(trace: RPAReplayCreateTraceRequest): Promise<{ trace: RPAReplayTrace; status: string }> {
    return this.request<{ trace: RPAReplayTrace; status: string }>("POST", "/v1/rpa-replay/traces", trace);
  }

  trace(slug: string): Promise<{ trace: RPAReplayTrace }> {
    return this.request<{ trace: RPAReplayTrace }>("GET", `/v1/rpa-replay/traces/${encodeURIComponent(slug)}`);
  }

  startRecording(input: Partial<Omit<RPAReplayRecordingSession, "id" | "started_at" | "status">> & { parameters?: Record<string, RPAReplayParamDef> } = {}): Promise<{ session: RPAReplayRecordingSession; status: string; note?: string }> {
    return this.request<{ session: RPAReplayRecordingSession; status: string; note?: string }>("POST", "/v1/rpa-replay/recordings/start", input);
  }

  stopRecording(input: RPAReplayStopRecordingRequest): Promise<{ trace: RPAReplayTrace; status: string }> {
    return this.request<{ trace: RPAReplayTrace; status: string }>("POST", "/v1/rpa-replay/recordings/stop", input);
  }

  replay(input: RPAReplayRequest): Promise<{ result: RPAReplayResult; trace: string }> {
    return this.request<{ result: RPAReplayResult; trace: string }>("POST", "/v1/rpa-replay/replay", input);
  }

  executorPlan(input: RPAReplayExecutorPlanRequest): Promise<{ plan: RPAReplayExecutorPlan }> {
    return this.request<{ plan: RPAReplayExecutorPlan }>("POST", "/v1/rpa-replay/executor/plan", input);
  }

  evidence(slug: string): Promise<RPAReplayEvidenceResponse> {
    return this.request<RPAReplayEvidenceResponse>("GET", `/v1/rpa-replay/evidence/${encodeURIComponent(slug)}`);
  }

  private async request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
    const headers = new Headers(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new RPAReplayClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createRPAReplayClient(options: RPAReplayClientOptions): RPAReplayClient {
  return new RPAReplayClient(options);
}
