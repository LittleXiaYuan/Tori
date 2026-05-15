/**
 * Lightweight Chaos Probe Pack SDK slice.
 *
 * This keeps safe probe definitions, one-shot health checks, scheduler /
 * metrics / alert write-back plans, degrade summaries, remediation hints, and
 * evidence export usable without importing the full generated OpenAPI SDK:
 *
 *   import { createChaosProbeClient } from "yunque-client/chaos-probe";
 */

export type ChaosProbePolicy = {
  max_probe_duration_ms: number;
  min_health_score_warn: number;
  fail_gate_threshold: number;
  memory_warn_bytes: number;
};

export type ChaosProbeDefinition = {
  id: string;
  name: string;
  category: string;
  description: string;
  safe: boolean;
  enabled: boolean;
  interval_seconds: number;
  weight: number;
  tags?: string[];
};

export type ChaosProbeResult = {
  probe_id: string;
  name: string;
  category: string;
  status: string;
  latency_ms: number;
  message: string;
  remediation?: string;
  safe: boolean;
  timestamp: string;
};

export type ChaosProbeReport = {
  id: string;
  pack_id: string;
  created_at: string;
  stage: string;
  probe_count: number;
  pass_count: number;
  degraded_count: number;
  fail_count: number;
  health_score: number;
  degrade_level: number;
  gate_status: string;
  results: ChaosProbeResult[];
  remediations?: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type ChaosProbeReportSummary = {
  id: string;
  created_at: string;
  probe_count: number;
  pass_count: number;
  degraded_count: number;
  fail_count: number;
  health_score: number;
  degrade_level: number;
  gate_status: string;
};

export type ChaosProbeStatusResponse = {
  pack_id: string;
  stage: string;
  safe_probe_ready: boolean;
  scheduler_plan_ready: boolean;
  scheduler_ready: boolean;
  metrics_plan_ready: boolean;
  prometheus_ready: boolean;
  degrade_writeback_plan_ready: boolean;
  degrade_engine_ready: boolean;
  alert_writeback_plan_ready: boolean;
  alert_writeback_ready: boolean;
  probe_count: number;
  report_count: number;
  store_dir?: string;
  policy: ChaosProbePolicy;
  last_report?: ChaosProbeReportSummary | null;
  capabilities: string[];
  notes?: string[];
};

export type ChaosProbeDefinitionsResponse = {
  probes: ChaosProbeDefinition[];
  count: number;
};

export type ChaosProbeSaveDefinitionsRequest = {
  probes: ChaosProbeDefinition[];
  replace?: boolean;
};

export type ChaosProbeRunRequest = {
  probe_ids?: string[];
  categories?: string[];
  persist?: boolean;
  dry_run?: boolean;
  unsafe_allowed?: boolean;
  metadata?: Record<string, string>;
};

export type ChaosProbeRunResponse = {
  report: ChaosProbeReport;
  status: string;
};

export type ChaosProbeSchedulerPlanRequest = {
  report_id?: string;
  interval?: string;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
};

export type ChaosProbeMetricPlan = {
  name: string;
  type: string;
  value: number;
  labels?: Record<string, string>;
};

export type ChaosProbeAlertPlan = {
  severity: string;
  route: string;
  message: string;
  writeback_ready: boolean;
};

export type ChaosProbeDegradeWritebackPlan = {
  target: string;
  level: number;
  reason: string;
  writeback_ready: boolean;
};

export type ChaosProbeSchedulerPlan = {
  pack_id: string;
  generated_at: string;
  status: string;
  report_id?: string;
  interval: string;
  scheduler_plan_ready: boolean;
  scheduler_ready: boolean;
  metrics_plan_ready: boolean;
  prometheus_ready: boolean;
  degrade_writeback_plan_ready: boolean;
  degrade_engine_ready: boolean;
  alert_writeback_plan_ready: boolean;
  alert_writeback_ready: boolean;
  requested_by?: string;
  reason?: string;
  health_score: number;
  degrade_level: number;
  gate_status: string;
  metrics: ChaosProbeMetricPlan[];
  alerts?: ChaosProbeAlertPlan[];
  writebacks?: ChaosProbeDegradeWritebackPlan[];
  actions: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type ChaosProbeSchedulerPlanResponse = {
  plan: ChaosProbeSchedulerPlan;
};

export type ChaosProbeReportsResponse = {
  reports: ChaosProbeReportSummary[];
  count: number;
};

export type ChaosProbeReportResponse = {
  report: ChaosProbeReport;
};

export type ChaosProbeEvidenceResponse = {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  report: ChaosProbeReport;
  scheduler_plan?: ChaosProbeSchedulerPlan;
};

export type ChaosProbeClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class ChaosProbeClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Chaos Probe request failed with HTTP ${status}`);
    this.name = "ChaosProbeClientError";
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

export class ChaosProbeClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: ChaosProbeClientOptions) {
    if (!options.baseUrl) throw new Error("ChaosProbeClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("ChaosProbeClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<ChaosProbeStatusResponse> {
    return this.request<ChaosProbeStatusResponse>("GET", "/v1/chaos-probe/status");
  }

  probes(): Promise<ChaosProbeDefinitionsResponse> {
    return this.request<ChaosProbeDefinitionsResponse>("GET", "/v1/chaos-probe/probes");
  }

  saveProbes(input: ChaosProbeSaveDefinitionsRequest): Promise<ChaosProbeDefinitionsResponse & { status: string }> {
    return this.request<ChaosProbeDefinitionsResponse & { status: string }>("POST", "/v1/chaos-probe/probes", input);
  }

  run(input: ChaosProbeRunRequest = {}): Promise<ChaosProbeRunResponse> {
    return this.request<ChaosProbeRunResponse>("POST", "/v1/chaos-probe/run", input);
  }

  schedulerPlan(input: ChaosProbeSchedulerPlanRequest = {}): Promise<ChaosProbeSchedulerPlanResponse> {
    return this.request<ChaosProbeSchedulerPlanResponse>("POST", "/v1/chaos-probe/scheduler/plan", input);
  }

  reports(): Promise<ChaosProbeReportsResponse> {
    return this.request<ChaosProbeReportsResponse>("GET", "/v1/chaos-probe/reports");
  }

  report(id: string): Promise<ChaosProbeReportResponse> {
    return this.request<ChaosProbeReportResponse>("GET", `/v1/chaos-probe/reports/${enc(id)}`);
  }

  evidence(id: string): Promise<ChaosProbeEvidenceResponse> {
    return this.request<ChaosProbeEvidenceResponse>("GET", `/v1/chaos-probe/evidence/${enc(id)}`);
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
    if (!response.ok) throw new ChaosProbeClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createChaosProbeClient(options: ChaosProbeClientOptions): ChaosProbeClient {
  return new ChaosProbeClient(options);
}
