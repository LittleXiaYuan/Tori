/**
 * Lightweight Cognitive Canary Pack SDK slice.
 *
 * This keeps canary scenario sets, deterministic local judge evaluation,
 * cognitive SLI summaries, promotion/block decisions, shadow/judge/metrics/
 * rollback plans, pack-local response collector writeback persistence,
 * response collector pipeline plan-only handoff, and evidence export usable
 * without importing the full generated OpenAPI SDK:
 *
 *   import { createCognitiveCanaryClient } from "yunque-client/cognitive-canary";
 */

export type CognitiveCanaryPolicy = {
  quality_score_slo: number;
  block_quality_score: number;
  min_delta_score: number;
  block_delta_score: number;
  max_latency_ratio: number;
  block_latency_ratio: number;
  max_error_rate: number;
  block_error_rate: number;
  min_samples_for_promotion: number;
  max_question_len: number;
  max_response_len: number;
};

export type CognitiveCanaryScenario = {
  id: string;
  name: string;
  category: string;
  question: string;
  stable_response: string;
  canary_response: string;
  expected_keywords?: string[];
  stable_latency_ms?: number;
  canary_latency_ms?: number;
  canary_error?: boolean;
  enabled: boolean;
  weight: number;
  tags?: string[];
};

export type CognitiveCanaryJudgeScore = {
  coherence: number;
  relevance: number;
  helpfulness: number;
  consistency: number;
  safety: string;
  warnings?: string[];
};

export type CognitiveCanaryResult = {
  scenario_id: string;
  name: string;
  category: string;
  quality_score: number;
  stable_score: number;
  delta_score: number;
  keyword_coverage: number;
  latency_ratio: number;
  canary_error: boolean;
  gate_status: string;
  judge: CognitiveCanaryJudgeScore;
  reasons?: string[];
};

export type CognitiveCanaryReport = {
  id: string;
  pack_id: string;
  created_at: string;
  stage: string;
  candidate_version?: string;
  stable_version?: string;
  scenario_count: number;
  safety_failure_count: number;
  error_count: number;
  quality_score: number;
  safety_pass_rate: number;
  delta_score: number;
  latency_p99_ratio: number;
  canary_error_rate: number;
  gate_status: string;
  promotion_decision: string;
  results: CognitiveCanaryResult[];
  recommendations?: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type CognitiveCanaryReportSummary = {
  id: string;
  created_at: string;
  scenario_count: number;
  safety_failure_count: number;
  error_count: number;
  quality_score: number;
  safety_pass_rate: number;
  delta_score: number;
  latency_p99_ratio: number;
  canary_error_rate: number;
  gate_status: string;
  promotion_decision: string;
};

export type CognitiveCanaryStatusResponse = {
  pack_id: string;
  stage: string;
  shadow_plan_ready: boolean;
  shadow_traffic_ready: boolean;
  judge_plan_ready: boolean;
  judge_pipeline_ready: boolean;
  response_collector_plan_ready: boolean;
  response_collector_store_ready?: boolean;
  response_collector_writeback_ready?: boolean;
  writes_response_collector_store?: boolean;
  response_collector_pipeline_plan_ready?: boolean;
  response_collector_pipeline_ready?: boolean;
  consumes_response_collector_store?: boolean;
  response_collector_ready: boolean;
  metrics_plan_ready: boolean;
  prometheus_ready: boolean;
  quality_sli_ready: boolean;
  auto_rollback_plan_ready: boolean;
  auto_rollback_ready: boolean;
  scenario_count: number;
  report_count: number;
  store_dir?: string;
  policy: CognitiveCanaryPolicy;
  last_report?: CognitiveCanaryReportSummary | null;
  response_collector_store?: CognitiveCanaryResponseCollectorStoreSummary;
  capabilities: string[];
  notes?: string[];
};

export type CognitiveCanaryScenariosResponse = {
  scenarios: CognitiveCanaryScenario[];
  count: number;
};

export type CognitiveCanarySaveScenariosRequest = {
  scenarios: CognitiveCanaryScenario[];
  replace?: boolean;
};

export type CognitiveCanaryEvaluateRequest = {
  scenario_ids?: string[];
  scenarios?: CognitiveCanaryScenario[];
  persist?: boolean;
  dry_run?: boolean;
  candidate_version?: string;
  stable_version?: string;
  metadata?: Record<string, string>;
};

export type CognitiveCanaryEvaluateResponse = {
  report: CognitiveCanaryReport;
  status: string;
};

export type CognitiveCanaryShadowPlanRequest = {
  report_id?: string;
  candidate_version?: string;
  stable_version?: string;
  traffic_percent?: number;
  sample_percent?: number;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
};

export type CognitiveCanaryShadowPairPlan = {
  scenario_id: string;
  category: string;
  stable_version: string;
  candidate_version: string;
  sample_percent: number;
  shadow_traffic_ready: boolean;
  response_collector_ready: boolean;
};

export type CognitiveCanaryResponseCollectorPlan = {
  pair_id: string;
  scenario_id: string;
  category: string;
  stable_version: string;
  candidate_version: string;
  sample_percent: number;
  collector_route: string;
  artifact: string;
  artifact_sha256: string;
  artifact_bytes: number;
  writes_files: boolean;
  ready: boolean;
  labels?: Record<string, string>;
};

export type CognitiveCanaryResponseCollectorSummary = {
  collector_count: number;
  artifact_count: number;
  writes_files: boolean;
  deterministic: boolean;
  hash_algorithm: string;
  ready: boolean;
};

export type CognitiveCanaryResponseCollectorStoreSummary = {
  pack_id: string;
  store: string;
  store_ready: boolean;
  record_count: number;
  artifact: string;
  response_collector_store_ready: boolean;
  response_collector_writeback_ready: boolean;
  writes_response_collector_store: boolean;
  response_collector_pipeline_plan_ready?: boolean;
  consumes_response_collector_store?: boolean;
  response_collector_pipeline_ready?: boolean;
  response_collector_ready: boolean;
  shadow_traffic_ready: boolean;
  judge_pipeline_ready: boolean;
  prometheus_ready: boolean;
  auto_rollback_ready: boolean;
  latest_record_id?: string;
  notes?: string[];
};

export type CognitiveCanaryResponseCollectorRecord = {
  pack_id: string;
  record_id: string;
  record_key: string;
  report_id: string;
  pair_id: string;
  scenario_id: string;
  category: string;
  stable_version: string;
  candidate_version: string;
  sample_percent: number;
  collector_route: string;
  artifact: string;
  artifact_sha256: string;
  artifact_bytes: number;
  source: string;
  status: string;
  requested_by?: string;
  reason?: string;
  created_at: string;
  updated_at: string;
  report_summary: CognitiveCanaryReportSummary;
  collector_plan: CognitiveCanaryResponseCollectorPlan;
  response_collector_store_ready: boolean;
  response_collector_writeback_ready: boolean;
  writes_response_collector_store: boolean;
  response_collector_ready: boolean;
  shadow_traffic_ready: boolean;
  judge_pipeline_ready: boolean;
  prometheus_ready: boolean;
  auto_rollback_ready: boolean;
  writes_files: boolean;
  metadata?: Record<string, string>;
  artifacts: string[];
  labels: string[];
  notes?: string[];
};

export type CognitiveCanaryResponseCollectorWritebackRequest = CognitiveCanaryShadowPlanRequest;

export type CognitiveCanaryResponseCollectorWritebackReport = {
  pack_id: string;
  generated_at: string;
  status: string;
  report_id: string;
  candidate_version?: string;
  stable_version?: string;
  sample_percent: number;
  requested_by?: string;
  reason?: string;
  response_collector_store_ready: boolean;
  response_collector_writeback_ready: boolean;
  writes_response_collector_store: boolean;
  response_collector_ready: boolean;
  shadow_traffic_ready: boolean;
  judge_pipeline_ready: boolean;
  prometheus_ready: boolean;
  auto_rollback_ready: boolean;
  writes_files: boolean;
  record_count: number;
  records: CognitiveCanaryResponseCollectorRecord[];
  response_collector_store: CognitiveCanaryResponseCollectorStoreSummary;
  shadow_plan: CognitiveCanaryShadowPlan;
  artifacts: string[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type CognitiveCanaryResponseCollectorPipelinePlanRequest = {
  report_id?: string;
  record_id?: string;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
};

export type CognitiveCanaryResponseCollectorPipelineHandoffPlan = {
  target: string;
  source_store: string;
  report_id: string;
  record_ids: string[];
  pair_ids: string[];
  artifacts: string[];
  artifact: string;
  artifact_sha256: string;
  artifact_bytes: number;
  dedup_key: string;
  consumes_response_collector_store: boolean;
  writes_live_response_artifacts: boolean;
  writes_judge_batches: boolean;
  writes_prometheus_metrics: boolean;
  writes_rollback_state: boolean;
  response_collector_pipeline_ready: boolean;
  response_collector_ready: boolean;
  shadow_traffic_ready: boolean;
  judge_pipeline_ready: boolean;
  prometheus_ready: boolean;
  auto_rollback_ready: boolean;
  approval_required: boolean;
  metadata?: Record<string, string>;
  actions: string[];
  blocked_by: string[];
  notes?: string[];
};

export type CognitiveCanaryResponseCollectorPipelinePlan = {
  pack_id: string;
  generated_at: string;
  status: string;
  report_id: string;
  record_id?: string;
  record_count: number;
  requested_by?: string;
  reason?: string;
  response_collector_pipeline_plan_ready: boolean;
  response_collector_pipeline_ready: boolean;
  consumes_response_collector_store: boolean;
  response_collector_store_ready: boolean;
  response_collector_writeback_ready: boolean;
  writes_response_collector_store: boolean;
  response_collector_ready: boolean;
  shadow_traffic_ready: boolean;
  judge_pipeline_ready: boolean;
  prometheus_ready: boolean;
  auto_rollback_ready: boolean;
  writes_files: boolean;
  records: CognitiveCanaryResponseCollectorRecord[];
  response_collector_store: CognitiveCanaryResponseCollectorStoreSummary;
  response_collector_pipeline_plan: CognitiveCanaryResponseCollectorPipelineHandoffPlan;
  artifacts: string[];
  actions: string[];
  labels: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type CognitiveCanaryJudgeBatchPlan = {
  name: string;
  source: string;
  scenario_count: number;
  judge_type: string;
  dimensions: string[];
  judge_pipeline_ready: boolean;
};

export type CognitiveCanaryMetricPlan = {
  name: string;
  type: string;
  value: number;
  threshold?: number;
  labels?: Record<string, string>;
};

export type CognitiveCanaryRollbackActionPlan = {
  target: string;
  trigger: string;
  decision: string;
  reason: string;
  auto_rollback_ready: boolean;
};

export type CognitiveCanaryShadowPlan = {
  pack_id: string;
  generated_at: string;
  status: string;
  report_id?: string;
  candidate_version?: string;
  stable_version?: string;
  traffic_percent: number;
  sample_percent: number;
  shadow_plan_ready: boolean;
  shadow_traffic_ready: boolean;
  judge_plan_ready: boolean;
  judge_pipeline_ready: boolean;
  response_collector_plan_ready: boolean;
  response_collector_ready: boolean;
  metrics_plan_ready: boolean;
  prometheus_ready: boolean;
  auto_rollback_plan_ready: boolean;
  auto_rollback_ready: boolean;
  requested_by?: string;
  reason?: string;
  quality_score: number;
  safety_pass_rate: number;
  delta_score: number;
  latency_p99_ratio: number;
  canary_error_rate: number;
  gate_status: string;
  promotion_decision: string;
  shadow_pairs: CognitiveCanaryShadowPairPlan[];
  response_collectors: CognitiveCanaryResponseCollectorPlan[];
  response_collector_summary: CognitiveCanaryResponseCollectorSummary;
  judge_batches: CognitiveCanaryJudgeBatchPlan[];
  metrics: CognitiveCanaryMetricPlan[];
  rollback_actions: CognitiveCanaryRollbackActionPlan[];
  actions: string[];
  metadata?: Record<string, string>;
  notes?: string[];
};

export type CognitiveCanaryShadowPlanResponse = {
  plan: CognitiveCanaryShadowPlan;
};

export type CognitiveCanaryResponseCollectorWritebackResponse = {
  writeback: CognitiveCanaryResponseCollectorWritebackReport;
};

export type CognitiveCanaryResponseCollectorPipelinePlanResponse = {
  plan: CognitiveCanaryResponseCollectorPipelinePlan;
};

export type CognitiveCanaryReportsResponse = {
  reports: CognitiveCanaryReportSummary[];
  count: number;
};

export type CognitiveCanaryReportResponse = {
  report: CognitiveCanaryReport;
};

export type CognitiveCanaryEvidenceResponse = {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  report: CognitiveCanaryReport;
  shadow_plan?: CognitiveCanaryShadowPlan;
  response_collector_store?: CognitiveCanaryResponseCollectorStoreSummary;
  response_collector_records?: CognitiveCanaryResponseCollectorRecord[];
  response_collector_pipeline_plan?: CognitiveCanaryResponseCollectorPipelinePlan;
  response_collector_pipeline_plan_ready?: boolean;
};

export type CognitiveCanaryClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class CognitiveCanaryClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Cognitive Canary request failed with HTTP ${status}`);
    this.name = "CognitiveCanaryClientError";
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

export class CognitiveCanaryClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: CognitiveCanaryClientOptions) {
    if (!options.baseUrl) throw new Error("CognitiveCanaryClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("CognitiveCanaryClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<CognitiveCanaryStatusResponse> {
    return this.request<CognitiveCanaryStatusResponse>("GET", "/v1/cognitive-canary/status");
  }

  scenarios(): Promise<CognitiveCanaryScenariosResponse> {
    return this.request<CognitiveCanaryScenariosResponse>("GET", "/v1/cognitive-canary/scenarios");
  }

  saveScenarios(input: CognitiveCanarySaveScenariosRequest): Promise<CognitiveCanaryScenariosResponse & { status: string }> {
    return this.request<CognitiveCanaryScenariosResponse & { status: string }>("POST", "/v1/cognitive-canary/scenarios", input);
  }

  evaluate(input: CognitiveCanaryEvaluateRequest = {}): Promise<CognitiveCanaryEvaluateResponse> {
    return this.request<CognitiveCanaryEvaluateResponse>("POST", "/v1/cognitive-canary/evaluate", input);
  }

  shadowPlan(input: CognitiveCanaryShadowPlanRequest = {}): Promise<CognitiveCanaryShadowPlanResponse> {
    return this.request<CognitiveCanaryShadowPlanResponse>("POST", "/v1/cognitive-canary/shadow/plan", input);
  }

  responseCollectorWriteback(input: CognitiveCanaryResponseCollectorWritebackRequest = {}): Promise<CognitiveCanaryResponseCollectorWritebackResponse> {
    return this.request<CognitiveCanaryResponseCollectorWritebackResponse>("POST", "/v1/cognitive-canary/response-collector/writeback", input);
  }

  responseCollectorPipelinePlan(input: CognitiveCanaryResponseCollectorPipelinePlanRequest = {}): Promise<CognitiveCanaryResponseCollectorPipelinePlanResponse> {
    return this.request<CognitiveCanaryResponseCollectorPipelinePlanResponse>("POST", "/v1/cognitive-canary/response-collector/pipeline/plan", input);
  }

  reports(): Promise<CognitiveCanaryReportsResponse> {
    return this.request<CognitiveCanaryReportsResponse>("GET", "/v1/cognitive-canary/reports");
  }

  report(id: string): Promise<CognitiveCanaryReportResponse> {
    return this.request<CognitiveCanaryReportResponse>("GET", `/v1/cognitive-canary/reports/${enc(id)}`);
  }

  evidence(id: string): Promise<CognitiveCanaryEvidenceResponse> {
    return this.request<CognitiveCanaryEvidenceResponse>("GET", `/v1/cognitive-canary/evidence/${enc(id)}`);
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
    if (!response.ok) throw new CognitiveCanaryClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createCognitiveCanaryClient(options: CognitiveCanaryClientOptions): CognitiveCanaryClient {
  return new CognitiveCanaryClient(options);
}
