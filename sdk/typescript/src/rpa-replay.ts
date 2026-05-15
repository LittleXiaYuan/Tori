/**
 * Lightweight RPA Replay Pack SDK slice.
 *
 * This keeps RPA trace storage, recording shells, dry-run replay planning, and
 * evidence export usable without importing the full generated OpenAPI SDK:
 *
 *   import { createRPAReplayClient } from "yunque-client/rpa-replay";
 */

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

export type RPAReplayStatusResponse = {
  pack_id: string;
  stage: string;
  executor_ready: boolean;
  trace_count: number;
  active_recordings: number;
  store_dir?: string;
  capabilities: string[];
};

export type RPAReplayTracesResponse = {
  traces: RPAReplayTraceSummary[];
  count: number;
};

export type RPAReplayTraceResponse = {
  trace: RPAReplayTrace;
};

export type RPAReplayCreateTraceRequest = Partial<RPAReplayTrace> & {
  slug: string;
  name: string;
  steps: RPAReplayTraceStep[];
};

export type RPAReplayCreateTraceResponse = RPAReplayTraceResponse & {
  status: string;
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

export type RPAReplayStartRecordingRequest = Partial<Omit<RPAReplayRecordingSession, "id" | "started_at" | "status">> & {
  parameters?: Record<string, RPAReplayParamDef>;
};

export type RPAReplayStartRecordingResponse = {
  session: RPAReplayRecordingSession;
  status: string;
  note?: string;
};

export type RPAReplayStopRecordingRequest = {
  session_id: string;
  slug?: string;
  name?: string;
  steps?: RPAReplayTraceStep[];
};

export type RPAReplayStopRecordingResponse = RPAReplayTraceResponse & {
  status: string;
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

export type RPAReplayResponse = {
  result: RPAReplayResult;
  trace: string;
};

export type RPAReplayEvidenceResponse = {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  trace: RPAReplayTrace;
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

function enc(value: string): string {
  return encodeURIComponent(value);
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
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<RPAReplayStatusResponse> {
    return this.request<RPAReplayStatusResponse>("GET", "/v1/rpa-replay/status");
  }

  traces(): Promise<RPAReplayTracesResponse> {
    return this.request<RPAReplayTracesResponse>("GET", "/v1/rpa-replay/traces");
  }

  createTrace(trace: RPAReplayCreateTraceRequest): Promise<RPAReplayCreateTraceResponse> {
    return this.request<RPAReplayCreateTraceResponse>("POST", "/v1/rpa-replay/traces", trace);
  }

  trace(slug: string): Promise<RPAReplayTraceResponse> {
    return this.request<RPAReplayTraceResponse>("GET", `/v1/rpa-replay/traces/${enc(slug)}`);
  }

  startRecording(input: RPAReplayStartRecordingRequest = {}): Promise<RPAReplayStartRecordingResponse> {
    return this.request<RPAReplayStartRecordingResponse>("POST", "/v1/rpa-replay/recordings/start", input);
  }

  stopRecording(input: RPAReplayStopRecordingRequest): Promise<RPAReplayStopRecordingResponse> {
    return this.request<RPAReplayStopRecordingResponse>("POST", "/v1/rpa-replay/recordings/stop", input);
  }

  replay(input: RPAReplayRequest): Promise<RPAReplayResponse> {
    return this.request<RPAReplayResponse>("POST", "/v1/rpa-replay/replay", input);
  }

  evidence(slug: string): Promise<RPAReplayEvidenceResponse> {
    return this.request<RPAReplayEvidenceResponse>("GET", `/v1/rpa-replay/evidence/${enc(slug)}`);
  }

  private async request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
    const headers = mergeHeaders(this.headers);
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
