import { fetcher } from "./api-core";

export interface GuardrailFuzzerPolicy {
  mutants_per_seed: number;
  max_input_len: number;
  max_mutations_per_seed: number;
  bypass_fail_threshold: number;
  false_positive_warn_threshold: number;
}

export interface GuardrailFuzzerSeed {
  id: string;
  input: string;
  source: string;
  category: string;
  expected_blocked: boolean;
  tags?: string[];
}

export interface GuardrailFuzzerMutation {
  id: string;
  name: string;
  description: string;
}

export interface GuardrailFuzzerStatus {
  pack_id: string;
  stage: string;
  fuzzer_ready: boolean;
  ci_gate_ready: boolean;
  rule_writeback_ready: boolean;
  seed_count: number;
  report_count: number;
  store_dir?: string;
  policy: GuardrailFuzzerPolicy;
  mutations: GuardrailFuzzerMutation[];
  capabilities: string[];
  notes?: string[];
}

export interface GuardrailFuzzerResult {
  seed_id: string;
  seed: string;
  mutant: string;
  mutations: string[];
  source: string;
  category: string;
  expected_blocked: boolean;
  actual_blocked: boolean;
  rule?: string;
  threat_type?: string;
  bypassed: boolean;
  false_positive: boolean;
  sanitized?: string;
}

export interface GuardrailFuzzerRuleCandidate {
  category: string;
  reason: string;
  mutations: string[];
  strategy: string;
  confidence: number;
}

export interface GuardrailFuzzerReport {
  id: string;
  pack_id: string;
  created_at: string;
  stage: string;
  seed_count: number;
  mutant_count: number;
  bypass_count: number;
  false_positive_count: number;
  blocked_count: number;
  pass_count: number;
  risk_level: string;
  gate_status: string;
  results: GuardrailFuzzerResult[];
  rule_candidates?: GuardrailFuzzerRuleCandidate[];
  notes?: string[];
}

export interface GuardrailFuzzerReportSummary {
  id: string;
  created_at: string;
  seed_count: number;
  mutant_count: number;
  bypass_count: number;
  false_positive_count: number;
  risk_level: string;
  gate_status: string;
}

export interface GuardrailFuzzerRunInput {
  seeds?: GuardrailFuzzerSeed[];
  categories?: string[];
  mutations?: string[];
  mutants_per_seed?: number;
  persist?: boolean;
  dry_run?: boolean;
}

export interface GuardrailFuzzerPackClient {
  status(): Promise<GuardrailFuzzerStatus>;
  corpus(): Promise<{ seeds: GuardrailFuzzerSeed[]; count: number }>;
  saveCorpus(input: { seeds: GuardrailFuzzerSeed[]; replace?: boolean }): Promise<{ seeds: GuardrailFuzzerSeed[]; count: number; status: string }>;
  run(input?: GuardrailFuzzerRunInput): Promise<{ report: GuardrailFuzzerReport; status: string }>;
  reports(): Promise<{ reports: GuardrailFuzzerReportSummary[]; count: number }>;
  report(id: string): Promise<{ report: GuardrailFuzzerReport }>;
  evidence(id: string): Promise<{ pack_id: string; exported_at: string; format: string; files: string[]; report: GuardrailFuzzerReport }>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

export function createGuardrailFuzzerPackClient(): GuardrailFuzzerPackClient {
  return {
    status: () => fetcher<GuardrailFuzzerStatus>("/v1/guardrail-fuzzer/status"),
    corpus: () => fetcher<{ seeds: GuardrailFuzzerSeed[]; count: number }>("/v1/guardrail-fuzzer/corpus"),
    saveCorpus: (input) =>
      fetcher<{ seeds: GuardrailFuzzerSeed[]; count: number; status: string }>("/v1/guardrail-fuzzer/corpus", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    run: (input = {}) =>
      fetcher<{ report: GuardrailFuzzerReport; status: string }>("/v1/guardrail-fuzzer/run", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    reports: () => fetcher<{ reports: GuardrailFuzzerReportSummary[]; count: number }>("/v1/guardrail-fuzzer/reports"),
    report: (id) => fetcher<{ report: GuardrailFuzzerReport }>(`/v1/guardrail-fuzzer/reports/${enc(id)}`),
    evidence: (id) => fetcher<{ pack_id: string; exported_at: string; format: string; files: string[]; report: GuardrailFuzzerReport }>(`/v1/guardrail-fuzzer/evidence/${enc(id)}`),
  };
}
