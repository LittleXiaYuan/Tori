import { fetcher } from "./api-core";

export interface ChaosProbePolicy {
  max_probe_duration_ms: number;
  min_health_score_warn: number;
  fail_gate_threshold: number;
  memory_warn_bytes: number;
}

export interface ChaosProbeDefinition {
  id: string;
  name: string;
  category: string;
  description: string;
  safe: boolean;
  enabled: boolean;
  interval_seconds: number;
  weight: number;
  tags?: string[];
}

export interface ChaosProbeResult {
  probe_id: string;
  name: string;
  category: string;
  status: string;
  latency_ms: number;
  message: string;
  remediation?: string;
  safe: boolean;
  timestamp: string;
}

export interface ChaosProbeReport {
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
}

export interface ChaosProbeReportSummary {
  id: string;
  created_at: string;
  probe_count: number;
  pass_count: number;
  degraded_count: number;
  fail_count: number;
  health_score: number;
  degrade_level: number;
  gate_status: string;
}

export interface ChaosProbeStatus {
  pack_id: string;
  stage: string;
  safe_probe_ready: boolean;
  scheduler_ready: boolean;
  degrade_engine_ready: boolean;
  alert_writeback_ready: boolean;
  probe_count: number;
  report_count: number;
  store_dir?: string;
  policy: ChaosProbePolicy;
  last_report?: ChaosProbeReportSummary | null;
  capabilities: string[];
  notes?: string[];
}

export interface ChaosProbeRunInput {
  probe_ids?: string[];
  categories?: string[];
  persist?: boolean;
  dry_run?: boolean;
  unsafe_allowed?: boolean;
  metadata?: Record<string, string>;
}

export interface ChaosProbePackClient {
  status(): Promise<ChaosProbeStatus>;
  probes(): Promise<{ probes: ChaosProbeDefinition[]; count: number }>;
  saveProbes(input: { probes: ChaosProbeDefinition[]; replace?: boolean }): Promise<{ probes: ChaosProbeDefinition[]; count: number; status: string }>;
  run(input?: ChaosProbeRunInput): Promise<{ report: ChaosProbeReport; status: string }>;
  reports(): Promise<{ reports: ChaosProbeReportSummary[]; count: number }>;
  report(id: string): Promise<{ report: ChaosProbeReport }>;
  evidence(id: string): Promise<{ pack_id: string; exported_at: string; format: string; files: string[]; report: ChaosProbeReport }>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

export function createChaosProbePackClient(): ChaosProbePackClient {
  return {
    status: () => fetcher<ChaosProbeStatus>("/v1/chaos-probe/status"),
    probes: () => fetcher<{ probes: ChaosProbeDefinition[]; count: number }>("/v1/chaos-probe/probes"),
    saveProbes: (input) =>
      fetcher<{ probes: ChaosProbeDefinition[]; count: number; status: string }>("/v1/chaos-probe/probes", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    run: (input = {}) =>
      fetcher<{ report: ChaosProbeReport; status: string }>("/v1/chaos-probe/run", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    reports: () => fetcher<{ reports: ChaosProbeReportSummary[]; count: number }>("/v1/chaos-probe/reports"),
    report: (id) => fetcher<{ report: ChaosProbeReport }>(`/v1/chaos-probe/reports/${enc(id)}`),
    evidence: (id) => fetcher<{ pack_id: string; exported_at: string; format: string; files: string[]; report: ChaosProbeReport }>(`/v1/chaos-probe/evidence/${enc(id)}`),
  };
}
