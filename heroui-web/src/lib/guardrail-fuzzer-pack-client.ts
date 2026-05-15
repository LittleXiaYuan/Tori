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
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  rule_writeback_plan_ready: boolean;
  rule_writeback_ready: boolean;
  alert_plan_ready: boolean;
  alert_ready: boolean;
  native_corpus_plan_ready: boolean;
  native_corpus_sync_ready: boolean;
  go_native_fuzz_plan_ready: boolean;
  go_native_fuzz_ready: boolean;
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

export interface GuardrailFuzzerCIGatePlanInput {
  report_id?: string;
  schedule?: string;
  branch?: string;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
}

export interface GuardrailFuzzerCIGateJobPlan {
  name: string;
  trigger: string;
  branch: string;
  command: string;
  artifacts: string[];
  gate_on_bypass: boolean;
  ci_gate_ready: boolean;
}

export interface GuardrailFuzzerRuleWritebackPlan {
  category: string;
  strategy: string;
  confidence: number;
  mutations?: string[];
  writeback_ready: boolean;
}

export interface GuardrailFuzzerAlertPlan {
  severity: string;
  route: string;
  message: string;
  alert_ready: boolean;
}

export interface GuardrailFuzzerCIGatePlan {
  pack_id: string;
  generated_at: string;
  status: string;
  report_id?: string;
  schedule: string;
  branch: string;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  rule_writeback_plan_ready: boolean;
  rule_writeback_ready: boolean;
  alert_plan_ready: boolean;
  alert_ready: boolean;
  requested_by?: string;
  reason?: string;
  risk_level: string;
  gate_status: string;
  seed_count: number;
  mutant_count: number;
  bypass_count: number;
  false_positive_count: number;
  ci_jobs: GuardrailFuzzerCIGateJobPlan[];
  rule_writebacks?: GuardrailFuzzerRuleWritebackPlan[];
  alerts?: GuardrailFuzzerAlertPlan[];
  rule_candidates?: GuardrailFuzzerRuleCandidate[];
  actions: string[];
  metadata?: Record<string, string>;
  notes?: string[];
}

export interface GuardrailFuzzerNativeCorpusPlanInput {
  categories?: string[];
  include_benign?: boolean;
  max_seeds?: number;
  package?: string;
  fuzz_target?: string;
  corpus_dir?: string;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
}

export interface GuardrailFuzzerNativeCorpusSeedPlan {
  seed_id: string;
  category: string;
  source: string;
  expected_blocked: boolean;
  tags?: string[];
  testdata_file: string;
  add_call: string;
  corpus_entry: string;
}

export interface GuardrailFuzzerNativeCorpusManifestEntry {
  seed_id: string;
  testdata_file: string;
  action: string;
  content_sha256: string;
  content_bytes: number;
  source: string;
  category: string;
  expected_blocked: boolean;
  tags?: string[];
}

export interface GuardrailFuzzerNativeCorpusSyncSummary {
  manifest_entry_count: number;
  would_create: number;
  would_update: number;
  would_skip: number;
  writes_files: boolean;
  deterministic: boolean;
  hash_algorithm: string;
}

export interface GuardrailFuzzerNativeFuzzCommandPlan {
  name: string;
  command: string;
  artifacts: string[];
  writes_files: boolean;
  ready: boolean;
}

export interface GuardrailFuzzerNativeCorpusPlan {
  pack_id: string;
  generated_at: string;
  status: string;
  package: string;
  fuzz_target: string;
  corpus_dir: string;
  native_corpus_plan_ready: boolean;
  native_corpus_sync_ready: boolean;
  go_native_fuzz_plan_ready: boolean;
  go_native_fuzz_ready: boolean;
  seed_count: number;
  attack_seed_count: number;
  benign_seed_count: number;
  seeds: GuardrailFuzzerNativeCorpusSeedPlan[];
  corpus_manifest: GuardrailFuzzerNativeCorpusManifestEntry[];
  sync_summary: GuardrailFuzzerNativeCorpusSyncSummary;
  commands: GuardrailFuzzerNativeFuzzCommandPlan[];
  requested_by?: string;
  reason?: string;
  actions: string[];
  metadata?: Record<string, string>;
  notes?: string[];
}

export interface GuardrailFuzzerEvidence {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  report: GuardrailFuzzerReport;
  ci_gate_plan?: GuardrailFuzzerCIGatePlan;
  native_corpus_plan?: GuardrailFuzzerNativeCorpusPlan;
}

export interface GuardrailFuzzerPackClient {
  status(): Promise<GuardrailFuzzerStatus>;
  corpus(): Promise<{ seeds: GuardrailFuzzerSeed[]; count: number }>;
  saveCorpus(input: { seeds: GuardrailFuzzerSeed[]; replace?: boolean }): Promise<{ seeds: GuardrailFuzzerSeed[]; count: number; status: string }>;
  run(input?: GuardrailFuzzerRunInput): Promise<{ report: GuardrailFuzzerReport; status: string }>;
  ciGatePlan(input?: GuardrailFuzzerCIGatePlanInput): Promise<{ plan: GuardrailFuzzerCIGatePlan }>;
  nativeCorpusPlan(input?: GuardrailFuzzerNativeCorpusPlanInput): Promise<{ plan: GuardrailFuzzerNativeCorpusPlan }>;
  reports(): Promise<{ reports: GuardrailFuzzerReportSummary[]; count: number }>;
  report(id: string): Promise<{ report: GuardrailFuzzerReport }>;
  evidence(id: string): Promise<GuardrailFuzzerEvidence>;
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
    ciGatePlan: (input = {}) =>
      fetcher<{ plan: GuardrailFuzzerCIGatePlan }>("/v1/guardrail-fuzzer/ci-gate/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    nativeCorpusPlan: (input = {}) =>
      fetcher<{ plan: GuardrailFuzzerNativeCorpusPlan }>("/v1/guardrail-fuzzer/native-corpus/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    reports: () => fetcher<{ reports: GuardrailFuzzerReportSummary[]; count: number }>("/v1/guardrail-fuzzer/reports"),
    report: (id) => fetcher<{ report: GuardrailFuzzerReport }>(`/v1/guardrail-fuzzer/reports/${enc(id)}`),
    evidence: (id) => fetcher<GuardrailFuzzerEvidence>(`/v1/guardrail-fuzzer/evidence/${enc(id)}`),
  };
}
