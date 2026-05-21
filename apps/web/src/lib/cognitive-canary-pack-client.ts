import { fetcher } from "./api-core";

export interface CognitiveCanaryPolicy {
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
}

export interface CognitiveCanaryScenario {
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
}

export interface CognitiveCanaryJudgeScore {
  coherence: number;
  relevance: number;
  helpfulness: number;
  consistency: number;
  safety: string;
  warnings?: string[];
}

export interface CognitiveCanaryResult {
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
}

export interface CognitiveCanaryReport {
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
}

export interface CognitiveCanaryReportSummary {
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
}

export interface CognitiveCanaryStatus {
  pack_id: string;
  stage: string;
  shadow_plan_ready: boolean;
  shadow_traffic_ready: boolean;
  judge_plan_ready: boolean;
  judge_pipeline_ready: boolean;
  response_collector_plan_ready: boolean;
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
  response_collector_store_ready?: boolean;
  response_collector_writeback_ready?: boolean;
  writes_response_collector_store?: boolean;
  response_collector_pipeline_plan_ready?: boolean;
  response_collector_pipeline_ready?: boolean;
  consumes_response_collector_store?: boolean;
  response_collector_store?: CognitiveCanaryResponseCollectorStoreSummary;
  capabilities: string[];
  notes?: string[];
}

export interface CognitiveCanaryEvaluateInput {
  scenario_ids?: string[];
  scenarios?: CognitiveCanaryScenario[];
  persist?: boolean;
  dry_run?: boolean;
  candidate_version?: string;
  stable_version?: string;
  metadata?: Record<string, string>;
}

export interface CognitiveCanaryShadowPlanInput {
  report_id?: string;
  candidate_version?: string;
  stable_version?: string;
  traffic_percent?: number;
  sample_percent?: number;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
}

export interface CognitiveCanaryShadowPairPlan {
  scenario_id: string;
  category: string;
  stable_version: string;
  candidate_version: string;
  sample_percent: number;
  shadow_traffic_ready: boolean;
  response_collector_ready: boolean;
}

export interface CognitiveCanaryResponseCollectorPlan {
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
}

export interface CognitiveCanaryResponseCollectorSummary {
  collector_count: number;
  artifact_count: number;
  writes_files: boolean;
  deterministic: boolean;
  hash_algorithm: string;
  ready: boolean;
}

export interface CognitiveCanaryResponseCollectorStoreSummary {
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
}

export interface CognitiveCanaryResponseCollectorRecord {
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
}

export interface CognitiveCanaryResponseCollectorWritebackInput extends CognitiveCanaryShadowPlanInput {}

export interface CognitiveCanaryResponseCollectorWritebackReport {
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
}

export interface CognitiveCanaryResponseCollectorPipelinePlanInput {
  report_id?: string;
  record_id?: string;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
}

export interface CognitiveCanaryResponseCollectorPipelineHandoffPlan {
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
}

export interface CognitiveCanaryResponseCollectorPipelinePlan {
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
}

export interface CognitiveCanaryJudgeBatchPlan {
  name: string;
  source: string;
  scenario_count: number;
  judge_type: string;
  dimensions: string[];
  judge_pipeline_ready: boolean;
}

export interface CognitiveCanaryMetricPlan {
  name: string;
  type: string;
  value: number;
  threshold?: number;
  labels?: Record<string, string>;
}

export interface CognitiveCanaryRollbackActionPlan {
  target: string;
  trigger: string;
  decision: string;
  reason: string;
  auto_rollback_ready: boolean;
}

export interface CognitiveCanaryShadowPlan {
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
}

export interface CognitiveCanaryEvidence {
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
}

export interface CognitiveCanaryPackClient {
  status(): Promise<CognitiveCanaryStatus>;
  scenarios(): Promise<{ scenarios: CognitiveCanaryScenario[]; count: number }>;
  saveScenarios(input: { scenarios: CognitiveCanaryScenario[]; replace?: boolean }): Promise<{ scenarios: CognitiveCanaryScenario[]; count: number; status: string }>;
  evaluate(input?: CognitiveCanaryEvaluateInput): Promise<{ report: CognitiveCanaryReport; status: string }>;
  shadowPlan(input?: CognitiveCanaryShadowPlanInput): Promise<{ plan: CognitiveCanaryShadowPlan }>;
  responseCollectorWriteback(input?: CognitiveCanaryResponseCollectorWritebackInput): Promise<{ writeback: CognitiveCanaryResponseCollectorWritebackReport }>;
  responseCollectorPipelinePlan(input?: CognitiveCanaryResponseCollectorPipelinePlanInput): Promise<{ plan: CognitiveCanaryResponseCollectorPipelinePlan }>;
  reports(): Promise<{ reports: CognitiveCanaryReportSummary[]; count: number }>;
  report(id: string): Promise<{ report: CognitiveCanaryReport }>;
  evidence(id: string): Promise<CognitiveCanaryEvidence>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

export function createCognitiveCanaryPackClient(): CognitiveCanaryPackClient {
  return {
    status: () => fetcher<CognitiveCanaryStatus>("/v1/cognitive-canary/status"),
    scenarios: () => fetcher<{ scenarios: CognitiveCanaryScenario[]; count: number }>("/v1/cognitive-canary/scenarios"),
    saveScenarios: (input) =>
      fetcher<{ scenarios: CognitiveCanaryScenario[]; count: number; status: string }>("/v1/cognitive-canary/scenarios", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    evaluate: (input = {}) =>
      fetcher<{ report: CognitiveCanaryReport; status: string }>("/v1/cognitive-canary/evaluate", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    shadowPlan: (input = {}) =>
      fetcher<{ plan: CognitiveCanaryShadowPlan }>("/v1/cognitive-canary/shadow/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    responseCollectorWriteback: (input = {}) =>
      fetcher<{ writeback: CognitiveCanaryResponseCollectorWritebackReport }>("/v1/cognitive-canary/response-collector/writeback", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    responseCollectorPipelinePlan: (input = {}) =>
      fetcher<{ plan: CognitiveCanaryResponseCollectorPipelinePlan }>("/v1/cognitive-canary/response-collector/pipeline/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    reports: () => fetcher<{ reports: CognitiveCanaryReportSummary[]; count: number }>("/v1/cognitive-canary/reports"),
    report: (id) => fetcher<{ report: CognitiveCanaryReport }>(`/v1/cognitive-canary/reports/${enc(id)}`),
    evidence: (id) => fetcher<CognitiveCanaryEvidence>(`/v1/cognitive-canary/evidence/${enc(id)}`),
  };
}
