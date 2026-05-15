import { fetcher } from "./api-core";

export interface SBOMDriftComponent {
  ecosystem: string;
  name: string;
  version?: string;
  scope?: string;
  path?: string;
  direct: boolean;
}

export interface SBOMDriftSnapshotSummary {
  id: string;
  source: string;
  created_at: string;
  component_count: number;
  ecosystems: Record<string, number>;
}

export interface SBOMDriftSnapshot extends SBOMDriftSnapshotSummary {
  components: SBOMDriftComponent[];
}

export interface SBOMDriftStatus {
  pack_id: string;
  stage: string;
  scanner_ready: boolean;
  cyclonedx_ready: boolean;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  vulnerability_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  snapshot_count: number;
  repo_root?: string;
  store_dir?: string;
  capabilities: string[];
  notes?: string[];
}

export interface SBOMDriftChange {
  ecosystem: string;
  name: string;
  path?: string;
  old_version?: string;
  new_version?: string;
  risk: string;
}

export interface SBOMDriftDiff {
  base: SBOMDriftSnapshotSummary;
  target: SBOMDriftSnapshotSummary;
  added: SBOMDriftChange[];
  removed: SBOMDriftChange[];
  changed: SBOMDriftChange[];
  risk_level: string;
  notes?: string[];
}

export interface SBOMDriftCycloneDXDocument {
  bomFormat: string;
  specVersion: string;
  version: number;
  metadata: Record<string, unknown>;
  components: Array<Record<string, unknown>>;
  dependencies?: Array<Record<string, unknown>>;
}

export interface SBOMDriftGovulncheckPackagePlan {
  ecosystem: string;
  module: string;
  version?: string;
  scope?: string;
  path?: string;
  direct: boolean;
  labels?: string[];
}

export interface SBOMDriftGovulncheckPlan {
  plan_ready: boolean;
  ready: boolean;
  status: string;
  command: string;
  target_package: string;
  report_artifact: string;
  executes: boolean;
  writes_files: boolean;
  vulnerability_db_fetch: boolean;
  package_count: number;
  module_count: number;
  packages: SBOMDriftGovulncheckPackagePlan[];
  labels: string[];
  notes?: string[];
}

export interface SBOMDriftCIGatePlan {
  pack_id: string;
  generated_at: string;
  status: string;
  blocked: boolean;
  fail_on_risk: string;
  cyclonedx_ready: boolean;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  requested_by?: string;
  reason?: string;
  diff: SBOMDriftDiff;
  govulncheck_plan: SBOMDriftGovulncheckPlan;
  artifacts: string[];
  commands: string[];
  actions: string[];
  notes?: string[];
}

export interface SBOMDriftEvidence {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  snapshot: SBOMDriftSnapshot;
  cyclonedx?: SBOMDriftCycloneDXDocument;
  ci_gate_plan?: SBOMDriftCIGatePlan;
  govulncheck_plan?: SBOMDriftGovulncheckPlan;
}

export interface SBOMDriftPackClient {
  status(): Promise<SBOMDriftStatus>;
  snapshots(): Promise<{ snapshots: SBOMDriftSnapshotSummary[]; count: number }>;
  createSnapshot(input?: { id?: string; source?: string }): Promise<{ snapshot: SBOMDriftSnapshot; status: string }>;
  snapshot(id: string): Promise<{ snapshot: SBOMDriftSnapshot }>;
  diff(input: { base_id: string; target_id?: string; target_current?: boolean }): Promise<{ diff: SBOMDriftDiff }>;
  cycloneDX(id?: string): Promise<{ bom: SBOMDriftCycloneDXDocument; snapshot: SBOMDriftSnapshotSummary }>;
  ciGatePlan(input: { base_id: string; target_id?: string; target_current?: boolean; fail_on_risk?: string; requested_by?: string; reason?: string }): Promise<{ plan: SBOMDriftCIGatePlan }>;
  evidence(id: string): Promise<SBOMDriftEvidence>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

export function createSBOMDriftPackClient(): SBOMDriftPackClient {
  return {
    status: () => fetcher<SBOMDriftStatus>("/v1/sbom-drift/status"),
    snapshots: () => fetcher<{ snapshots: SBOMDriftSnapshotSummary[]; count: number }>("/v1/sbom-drift/snapshots"),
    createSnapshot: (input = {}) =>
      fetcher<{ snapshot: SBOMDriftSnapshot; status: string }>("/v1/sbom-drift/snapshots", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    snapshot: (id) => fetcher<{ snapshot: SBOMDriftSnapshot }>(`/v1/sbom-drift/snapshots/${enc(id)}`),
    diff: (input) =>
      fetcher<{ diff: SBOMDriftDiff }>("/v1/sbom-drift/diff", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    cycloneDX: (id = "current") => fetcher<{ bom: SBOMDriftCycloneDXDocument; snapshot: SBOMDriftSnapshotSummary }>(`/v1/sbom-drift/cyclonedx/${enc(id)}`),
    ciGatePlan: (input) =>
      fetcher<{ plan: SBOMDriftCIGatePlan }>("/v1/sbom-drift/ci-gate/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    evidence: (id) => fetcher<SBOMDriftEvidence>(`/v1/sbom-drift/evidence/${enc(id)}`),
  };
}
