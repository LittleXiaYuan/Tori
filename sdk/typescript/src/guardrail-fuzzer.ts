/**
 * Lightweight Guardrail Fuzzer Pack SDK slice.
 *
 * This keeps adversarial guardrail corpus management, deterministic fuzz runs,
 * bypass reports, rule-candidate hints, non-destructive CI/rule/alert plans,
 * Go native fuzz corpus planning, and evidence export usable without importing
 * the full generated OpenAPI SDK:
 *
 *   import { createGuardrailFuzzerClient } from "yunque-client/guardrail-fuzzer";
 */

export type GuardrailFuzzerPolicy = {
  mutants_per_seed: number;
  max_input_len: number;
  max_mutations_per_seed: number;
  bypass_fail_threshold: number;
  false_positive_warn_threshold: number;
};

export type GuardrailFuzzerSeed = {
  id: string;
  input: string;
  source: string;
  category: string;
  expected_blocked: boolean;
  tags?: string[];
};

export type GuardrailFuzzerMutation = {
  id: string;
  name: string;
  description: string;
};

export type GuardrailFuzzerStatusResponse = {
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
};

export type GuardrailFuzzerCorpusResponse = {
  seeds: GuardrailFuzzerSeed[];
  count: number;
};

export type GuardrailFuzzerSaveCorpusRequest = {
  seeds: GuardrailFuzzerSeed[];
  replace?: boolean;
};

export type GuardrailFuzzerFuzzRequest = {
  seeds?: GuardrailFuzzerSeed[];
  categories?: string[];
  mutations?: string[];
  mutants_per_seed?: number;
  persist?: boolean;
  dry_run?: boolean;
};

export type GuardrailFuzzerResult = {
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
};

export type GuardrailFuzzerRuleCandidate = {
  category: string;
  reason: string;
  mutations: string[];
  strategy: string;
  confidence: number;
};

export type GuardrailFuzzerReport = {
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
};

export type GuardrailFuzzerReportSummary = {
  id: string;
  created_at: string;
  seed_count: number;
  mutant_count: number;
  bypass_count: number;
  false_positive_count: number;
  risk_level: string;
  gate_status: string;
};

export type GuardrailFuzzerRunResponse = {
  report: GuardrailFuzzerReport;
  status: string;
};

export type GuardrailFuzzerCIGatePlanRequest = {
  report_id?: string;
  schedule?: string;
  branch?: string;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
};

export type GuardrailFuzzerCIGateJobPlan = {
  name: string;
  trigger: string;
  branch: string;
  command: string;
  artifacts: string[];
  gate_on_bypass: boolean;
  ci_gate_ready: boolean;
};

export type GuardrailFuzzerRuleWritebackPlan = {
  category: string;
  strategy: string;
  confidence: number;
  mutations?: string[];
  writeback_ready: boolean;
};

export type GuardrailFuzzerAlertPlan = {
  severity: string;
  route: string;
  message: string;
  alert_ready: boolean;
};

export type GuardrailFuzzerCIGatePlan = {
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
};

export type GuardrailFuzzerCIGatePlanResponse = {
  plan: GuardrailFuzzerCIGatePlan;
};

export type GuardrailFuzzerNativeCorpusPlanRequest = {
  categories?: string[];
  include_benign?: boolean;
  max_seeds?: number;
  package?: string;
  fuzz_target?: string;
  corpus_dir?: string;
  requested_by?: string;
  reason?: string;
  metadata?: Record<string, string>;
};

export type GuardrailFuzzerNativeCorpusSeedPlan = {
  seed_id: string;
  category: string;
  source: string;
  expected_blocked: boolean;
  tags?: string[];
  testdata_file: string;
  add_call: string;
  corpus_entry: string;
};

export type GuardrailFuzzerNativeCorpusManifestEntry = {
  seed_id: string;
  testdata_file: string;
  action: string;
  content_sha256: string;
  content_bytes: number;
  source: string;
  category: string;
  expected_blocked: boolean;
  tags?: string[];
};

export type GuardrailFuzzerNativeCorpusSyncSummary = {
  manifest_entry_count: number;
  would_create: number;
  would_update: number;
  would_skip: number;
  writes_files: boolean;
  deterministic: boolean;
  hash_algorithm: string;
};

export type GuardrailFuzzerNativeFuzzCommandPlan = {
  name: string;
  command: string;
  artifacts: string[];
  writes_files: boolean;
  ready: boolean;
};

export type GuardrailFuzzerNativeCorpusPlan = {
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
};

export type GuardrailFuzzerNativeCorpusPlanResponse = {
  plan: GuardrailFuzzerNativeCorpusPlan;
};

export type GuardrailFuzzerReportsResponse = {
  reports: GuardrailFuzzerReportSummary[];
  count: number;
};

export type GuardrailFuzzerReportResponse = {
  report: GuardrailFuzzerReport;
};

export type GuardrailFuzzerEvidenceResponse = {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  report: GuardrailFuzzerReport;
  ci_gate_plan?: GuardrailFuzzerCIGatePlan;
  native_corpus_plan?: GuardrailFuzzerNativeCorpusPlan;
};

export type GuardrailFuzzerClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class GuardrailFuzzerClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Guardrail Fuzzer request failed with HTTP ${status}`);
    this.name = "GuardrailFuzzerClientError";
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

export class GuardrailFuzzerClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: GuardrailFuzzerClientOptions) {
    if (!options.baseUrl) throw new Error("GuardrailFuzzerClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("GuardrailFuzzerClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<GuardrailFuzzerStatusResponse> {
    return this.request<GuardrailFuzzerStatusResponse>("GET", "/v1/guardrail-fuzzer/status");
  }

  corpus(): Promise<GuardrailFuzzerCorpusResponse> {
    return this.request<GuardrailFuzzerCorpusResponse>("GET", "/v1/guardrail-fuzzer/corpus");
  }

  saveCorpus(input: GuardrailFuzzerSaveCorpusRequest): Promise<GuardrailFuzzerCorpusResponse & { status: string }> {
    return this.request<GuardrailFuzzerCorpusResponse & { status: string }>("POST", "/v1/guardrail-fuzzer/corpus", input);
  }

  run(input: GuardrailFuzzerFuzzRequest = {}): Promise<GuardrailFuzzerRunResponse> {
    return this.request<GuardrailFuzzerRunResponse>("POST", "/v1/guardrail-fuzzer/run", input);
  }

  ciGatePlan(input: GuardrailFuzzerCIGatePlanRequest = {}): Promise<GuardrailFuzzerCIGatePlanResponse> {
    return this.request<GuardrailFuzzerCIGatePlanResponse>("POST", "/v1/guardrail-fuzzer/ci-gate/plan", input);
  }

  nativeCorpusPlan(input: GuardrailFuzzerNativeCorpusPlanRequest = {}): Promise<GuardrailFuzzerNativeCorpusPlanResponse> {
    return this.request<GuardrailFuzzerNativeCorpusPlanResponse>("POST", "/v1/guardrail-fuzzer/native-corpus/plan", input);
  }

  reports(): Promise<GuardrailFuzzerReportsResponse> {
    return this.request<GuardrailFuzzerReportsResponse>("GET", "/v1/guardrail-fuzzer/reports");
  }

  report(id: string): Promise<GuardrailFuzzerReportResponse> {
    return this.request<GuardrailFuzzerReportResponse>("GET", `/v1/guardrail-fuzzer/reports/${enc(id)}`);
  }

  evidence(id: string): Promise<GuardrailFuzzerEvidenceResponse> {
    return this.request<GuardrailFuzzerEvidenceResponse>("GET", `/v1/guardrail-fuzzer/evidence/${enc(id)}`);
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
    if (!response.ok) throw new GuardrailFuzzerClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createGuardrailFuzzerClient(options: GuardrailFuzzerClientOptions): GuardrailFuzzerClient {
  return new GuardrailFuzzerClient(options);
}
