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
  shadow_traffic_ready: boolean;
  judge_pipeline_ready: boolean;
  quality_sli_ready: boolean;
  auto_rollback_ready: boolean;
  scenario_count: number;
  report_count: number;
  store_dir?: string;
  policy: CognitiveCanaryPolicy;
  last_report?: CognitiveCanaryReportSummary | null;
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

export interface CognitiveCanaryPackClient {
  status(): Promise<CognitiveCanaryStatus>;
  scenarios(): Promise<{ scenarios: CognitiveCanaryScenario[]; count: number }>;
  saveScenarios(input: { scenarios: CognitiveCanaryScenario[]; replace?: boolean }): Promise<{ scenarios: CognitiveCanaryScenario[]; count: number; status: string }>;
  evaluate(input?: CognitiveCanaryEvaluateInput): Promise<{ report: CognitiveCanaryReport; status: string }>;
  reports(): Promise<{ reports: CognitiveCanaryReportSummary[]; count: number }>;
  report(id: string): Promise<{ report: CognitiveCanaryReport }>;
  evidence(id: string): Promise<{ pack_id: string; exported_at: string; format: string; files: string[]; report: CognitiveCanaryReport }>;
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
    reports: () => fetcher<{ reports: CognitiveCanaryReportSummary[]; count: number }>("/v1/cognitive-canary/reports"),
    report: (id) => fetcher<{ report: CognitiveCanaryReport }>(`/v1/cognitive-canary/reports/${enc(id)}`),
    evidence: (id) => fetcher<{ pack_id: string; exported_at: string; format: string; files: string[]; report: CognitiveCanaryReport }>(`/v1/cognitive-canary/evidence/${enc(id)}`),
  };
}
